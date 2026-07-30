package sfn

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
)

// fakeS3Put records one PUT request the fake S3 server received.
type fakeS3Put struct {
	bucket string
	key    string
	body   []byte
}

// fakeAWSServer is an httptest server standing in for kumo's own S3 (and,
// optionally, Lambda) endpoints, addressed the same way the real engine
// addresses them: path-style S3 (GET/PUT /{bucket}/{key}) and the Lambda
// invoke path. Tests use it as the engine's WithBaseURL target for
// ItemReader/ResultWriter (and, for the end-to-end test, Task processors).
type fakeAWSServer struct {
	*httptest.Server

	mux *http.ServeMux

	mu      sync.Mutex
	objects map[string][]byte
	puts    []fakeS3Put
}

// newFakeAWSServer starts a fake S3 server. Every GET/PUT on
// /{bucket}/{key...} is served in-memory; a Lambda invoke handler is added
// separately by tests that need one, via withEchoLambda.
func newFakeAWSServer(t *testing.T) *fakeAWSServer {
	t.Helper()

	s := &fakeAWSServer{objects: map[string][]byte{}, mux: http.NewServeMux()}

	s.mux.HandleFunc("GET /{bucket}/{key...}", s.handleGet)
	s.mux.HandleFunc("PUT /{bucket}/{key...}", s.handlePut)
	s.mux.HandleFunc("GET /{bucket}", s.handleListObjectsV2)

	s.Server = httptest.NewServer(s.mux)
	t.Cleanup(s.Close)

	return s
}

// withEchoLambda adds a Lambda invoke handler that echoes the request body
// back verbatim as its response, so a Task processor's output reflects
// exactly what it sent.
func (s *fakeAWSServer) withEchoLambda() *fakeAWSServer {
	s.mux.HandleFunc("POST /lambda/2015-03-31/functions/{name}/invocations", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})

	return s
}

func (s *fakeAWSServer) handleGet(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	s.mu.Lock()
	body, ok := s.objects[bucket+"/"+key]
	s.mu.Unlock()

	if !ok {
		http.Error(w, "NoSuchKey: the specified key does not exist", http.StatusNotFound)

		return
	}

	_, _ = w.Write(body)
}

func (s *fakeAWSServer) handlePut(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)

		return
	}

	s.mu.Lock()
	s.objects[bucket+"/"+key] = body
	s.puts = append(s.puts, fakeS3Put{bucket: bucket, key: key, body: body})
	s.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

// handleListObjectsV2 handles GET /{bucket}?list-type=2&prefix=...: a
// minimal single-page ListObjectsV2 XML response over whatever objects were
// seeded via putObject, synthesizing ETag/LastModified/StorageClass the way
// kumo's own S3 service would (see ListObjects in
// internal/service/s3/handlers.go). Test keys are expected to be simple
// (no XML-special characters), so no escaping is done.
func (s *fakeAWSServer) handleListObjectsV2(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	prefix := r.URL.Query().Get("prefix")

	s.mu.Lock()

	sizes := make(map[string]int, len(s.objects))

	for objectKey, body := range s.objects {
		b, k, ok := strings.Cut(objectKey, "/")
		if !ok || b != bucket || !strings.HasPrefix(k, prefix) {
			continue
		}

		sizes[k] = len(body)
	}

	s.mu.Unlock()

	keys := make([]string, 0, len(sizes))
	for k := range sizes {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var buf strings.Builder

	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult>`)

	for _, k := range keys {
		fmt.Fprintf(&buf,
			`<Contents><Key>%s</Key><LastModified>2024-01-02T03:04:05.000Z</LastModified><ETag>&quot;etag-%s&quot;</ETag><Size>%d</Size><StorageClass>STANDARD</StorageClass></Contents>`,
			k, k, sizes[k],
		)
	}

	buf.WriteString(`<IsTruncated>false</IsTruncated></ListBucketResult>`)

	w.Header().Set("Content-Type", "application/xml")
	_, _ = io.WriteString(w, buf.String())
}

// putObject pre-seeds an object for a test's ItemReader to read.
func (s *fakeAWSServer) putObject(bucket, key, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.objects[bucket+"/"+key] = []byte(body)
}

// lastPut returns the most recent PUT request received, failing the test
// if none were.
func (s *fakeAWSServer) lastPut(t *testing.T) fakeS3Put {
	t.Helper()

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.puts) == 0 {
		t.Fatal("fakeAWSServer: no PUT requests recorded")
	}

	return s.puts[len(s.puts)-1]
}
