package s3

import (
	"bytes"
	"context"
	"io/fs"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// snapshotFile is the metadata snapshot MemoryStorage writes to its data dir.
const snapshotFile = "s3.json"

// saveAndReload closes the storage (a synchronous, authoritative save) and
// returns a fresh storage loaded from the same directory.
func saveAndReload(t *testing.T, s *MemoryStorage, dir string) *MemoryStorage {
	t.Helper()

	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	return NewMemoryStorage(WithDataDir(dir))
}

func TestMemoryStorage_BodyBlobRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	body := []byte("hello world")

	s := NewMemoryStorage(WithDataDir(dir))
	if err := s.CreateBucket(ctx, "b"); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	if _, err := s.PutObject(ctx, "b", "k", bytes.NewReader(body), nil); err != nil {
		t.Fatalf("put object: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// The snapshot must not carry the body inline; only a content reference.
	snapshot, err := os.ReadFile(filepath.Join(dir, snapshotFile)) //nolint:gosec // fixed name under t.TempDir
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	if bytes.Contains(snapshot, body) {
		t.Errorf("snapshot contains raw body; body was not externalized")
	}

	if bytes.Contains(snapshot, []byte(`"Body"`)) {
		t.Errorf("snapshot contains the legacy inline Body field")
	}

	if !bytes.Contains(snapshot, []byte("bodyRef")) {
		t.Errorf("snapshot missing bodyRef reference")
	}

	// The body lives in its own blob file, addressed by content hash.
	ref := bodyRefOf(body)
	if _, err := os.Stat(blobPath(dir, ref)); err != nil {
		t.Errorf("blob file missing for ref %s: %v", ref, err)
	}

	// A fresh load reconstructs the body from the blob.
	s2 := NewMemoryStorage(WithDataDir(dir))

	obj, err := s2.GetObject(ctx, "b", "k")
	if err != nil {
		t.Fatalf("get object after reload: %v", err)
	}

	if !bytes.Equal(obj.Body, body) {
		t.Errorf("body after reload = %q, want %q", obj.Body, body)
	}
}

func TestMemoryStorage_PersistentPutKeepsBodyOnDisk(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	body := bytes.Repeat([]byte("streamed-body"), 1024)

	s := NewMemoryStorage(WithDataDir(dir))
	if err := s.CreateBucket(ctx, "b"); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	obj, err := s.PutObject(ctx, "b", "k", bytes.NewReader(body), nil)
	if err != nil {
		t.Fatalf("put object: %v", err)
	}

	if len(obj.Body) != 0 {
		t.Fatalf("returned object retained %d body bytes", len(obj.Body))
	}

	stored := s.Buckets["b"].Objects["k"]
	if len(stored.Body) != 0 {
		t.Fatalf("stored object retained %d body bytes", len(stored.Body))
	}

	bodyOnDisk, err := os.ReadFile(blobPath(dir, bodyRefOf(body)))
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}

	if !bytes.Equal(bodyOnDisk, body) {
		t.Fatal("blob body differs from uploaded body")
	}

	got, err := s.GetObject(ctx, "b", "k")
	if err != nil {
		t.Fatalf("get object: %v", err)
	}

	if !bytes.Equal(got.Body, body) {
		t.Fatal("get object body differs from uploaded body")
	}

	if len(stored.Body) != 0 {
		t.Fatal("GetObject populated the stored object's body")
	}
}

func TestMemoryStorage_ReloadKeepsBodyOnDiskUntilGet(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	body := bytes.Repeat([]byte("persisted-body"), 1024)

	s := NewMemoryStorage(WithDataDir(dir))
	if err := s.CreateBucket(ctx, "b"); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	if _, err := s.PutObject(ctx, "b", "k", bytes.NewReader(body), nil); err != nil {
		t.Fatalf("put object: %v", err)
	}

	s2 := saveAndReload(t, s, dir)

	stored := s2.Buckets["b"].Objects["k"]
	if len(stored.Body) != 0 {
		t.Fatalf("reload retained %d body bytes", len(stored.Body))
	}

	got, err := s2.GetObject(ctx, "b", "k")
	if err != nil {
		t.Fatalf("get object: %v", err)
	}

	if !bytes.Equal(got.Body, body) {
		t.Fatal("get object body differs from uploaded body")
	}
}

func TestMemoryStorage_PersistentMultipartKeepsPartsAndObjectOnDisk(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	s := NewMemoryStorage(WithDataDir(dir))
	if err := s.CreateBucket(ctx, "b"); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	upload, err := s.CreateMultipartUpload(ctx, "b", "k", nil)
	if err != nil {
		t.Fatalf("create multipart upload: %v", err)
	}

	partBodies := [][]byte{[]byte("first part"), []byte("second part")}
	requests := make([]PartRequest, 0, len(partBodies))

	for i, body := range partBodies {
		part, err := s.UploadPart(ctx, "b", "k", upload.UploadID, i+1, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("upload part %d: %v", i+1, err)
		}

		if len(part.Body) != 0 {
			t.Fatalf("part %d retained %d body bytes", i+1, len(part.Body))
		}

		requests = append(requests, PartRequest{PartNumber: i + 1, ETag: part.ETag})
	}

	obj, err := s.CompleteMultipartUpload(ctx, "b", "k", upload.UploadID, requests)
	if err != nil {
		t.Fatalf("complete multipart upload: %v", err)
	}

	if len(obj.Body) != 0 {
		t.Fatalf("completed object retained %d body bytes", len(obj.Body))
	}

	want := bytes.Join(partBodies, nil)

	got, err := s.GetObject(ctx, "b", "k")
	if err != nil {
		t.Fatalf("get completed object: %v", err)
	}

	if !bytes.Equal(got.Body, want) {
		t.Fatalf("completed body = %q, want %q", got.Body, want)
	}
}

func TestMemoryStorage_VersionedBodiesRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	s := NewMemoryStorage(WithDataDir(dir))
	if err := s.CreateBucket(ctx, "b"); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	if err := s.PutBucketVersioning(ctx, "b", VersioningEnabled); err != nil {
		t.Fatalf("enable versioning: %v", err)
	}

	if _, err := s.PutObject(ctx, "b", "k", bytes.NewReader([]byte("v1-body")), nil); err != nil {
		t.Fatalf("put v1: %v", err)
	}

	if _, err := s.PutObject(ctx, "b", "k", bytes.NewReader([]byte("v2-body")), nil); err != nil {
		t.Fatalf("put v2: %v", err)
	}

	s2 := saveAndReload(t, s, dir)

	current, err := s2.GetObject(ctx, "b", "k")
	if err != nil {
		t.Fatalf("get current: %v", err)
	}

	if got := string(current.Body); got != "v2-body" {
		t.Errorf("current body = %q, want %q", got, "v2-body")
	}

	v1, err := s2.GetObjectVersion(ctx, "b", "k", "v1")
	if err != nil {
		t.Fatalf("get v1: %v", err)
	}

	if got := string(v1.Body); got != "v1-body" {
		t.Errorf("v1 body = %q, want %q", got, "v1-body")
	}
}

func TestMemoryStorage_LegacyInlineBodyMigrates(t *testing.T) {
	const legacyBody = "hello"

	ctx := context.Background()
	dir := t.TempDir()

	// A snapshot written by an older kumo: the body is inline base64 ("hello"
	// == aGVsbG8=) and there is no bodyRef and no blob directory.
	legacy := `{"buckets":{"b":{"name":"b","creationDate":"2020-01-01T00:00:00Z",` +
		`"objects":{"k":{"Key":"k","Body":"aGVsbG8=","ETag":"\"x\"","Size":5}},"versions":{}}}}`
	if err := os.WriteFile(filepath.Join(dir, snapshotFile), []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy snapshot: %v", err)
	}

	s := NewMemoryStorage(WithDataDir(dir))

	obj, err := s.GetObject(ctx, "b", "k")
	if err != nil {
		t.Fatalf("get object from legacy snapshot: %v", err)
	}

	if got := string(obj.Body); got != legacyBody {
		t.Fatalf("legacy body = %q, want %q", got, legacyBody)
	}

	// On the next save the inline body is migrated to a blob.
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if _, err := os.Stat(blobPath(dir, bodyRefOf([]byte(legacyBody)))); err != nil {
		t.Errorf("body not migrated to blob: %v", err)
	}
}

func TestMemoryStorage_OrphanBlobGarbageCollected(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	s := NewMemoryStorage(WithDataDir(dir))
	if err := s.CreateBucket(ctx, "b"); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	// First body, persisted.
	old := []byte("old-body")
	if _, err := s.PutObject(ctx, "b", "k", bytes.NewReader(old), nil); err != nil {
		t.Fatalf("put old: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("close after old: %v", err)
	}

	if _, err := os.Stat(blobPath(dir, bodyRefOf(old))); err != nil {
		t.Fatalf("old blob missing before overwrite: %v", err)
	}

	// Overwrite the key (no versioning): the old body becomes unreferenced.
	s2 := NewMemoryStorage(WithDataDir(dir))

	newBody := []byte("new-body")
	if _, err := s2.PutObject(ctx, "b", "k", bytes.NewReader(newBody), nil); err != nil {
		t.Fatalf("put new: %v", err)
	}

	if err := s2.Close(); err != nil {
		t.Fatalf("close after new: %v", err)
	}

	if _, err := os.Stat(blobPath(dir, bodyRefOf(old))); !os.IsNotExist(err) {
		t.Errorf("orphan blob not garbage collected (err=%v)", err)
	}

	if _, err := os.Stat(blobPath(dir, bodyRefOf(newBody))); err != nil {
		t.Errorf("new blob missing after overwrite: %v", err)
	}
}

// objectRef names an object by bucket and key.
type objectRef struct {
	bucket string
	key    string
}

// TestMemoryStorage_KeysNeverBecomeFilePaths pins that object keys and bucket
// names never reach the filesystem with a data dir set: bodies are stored
// under their content hash and keys live only inside the snapshot. Keys with
// ".." and empty segments are put, assembled by multipart upload and copied
// with CopyObject; afterwards root holds nothing but the snapshot and
// hash-named blobs, and every object reads back, also after a reload.
func TestMemoryStorage_KeysNeverBecomeFilePaths(t *testing.T) {
	root := t.TempDir()
	// Nested so deep that the ".." segments below, joined under the data dir
	// or a bucket directory, would still land inside root.
	dataDir := filepath.Join(root, "1", "2", "3", "4", "5", "6", "7", "data")

	s := NewMemoryStorage(WithDataDir(dataDir))

	want := writeTraversalKeys(t, s, []string{
		"a/../secret",
		"../../x",
		"a//b/../../../c",
		"/../../../../../../escape",
		"dir/../../",
	})

	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	wantBlobs := make(map[string]bool, len(want))
	for _, body := range want {
		wantBlobs[bodyRefOf([]byte(body))] = true
	}

	if got := dataDirBlobs(t, root, dataDir); !maps.Equal(got, wantBlobs) {
		t.Errorf("blobs = %v, want the content hashes %v", got, wantBlobs)
	}

	assertObjectBodies(t, s, want)
	assertObjectBodies(t, NewMemoryStorage(WithDataDir(dataDir)), want)
}

// writeTraversalKeys stores every key three ways: PutObject into one bucket,
// a multipart upload (one uploaded part, one part copied from the put
// object) into another, and CopyObject from the put object into a bucket
// whose name holds ".." segments. It returns the body each object must have.
func writeTraversalKeys(t *testing.T, s *MemoryStorage, keys []string) map[objectRef]string {
	t.Helper()

	const (
		putBucket       = "put"
		multipartBucket = "multipart"
		copyBucket      = "../../copies"
	)

	ctx := t.Context()
	svc := New(s, "")

	for _, b := range []string{putBucket, multipartBucket, copyBucket} {
		if err := s.CreateBucket(ctx, b); err != nil {
			t.Fatalf("create bucket %q: %v", b, err)
		}
	}

	want := make(map[objectRef]string, 3*len(keys))

	for _, key := range keys {
		putBody, firstPart := "put body of "+key, "first part of "+key

		if _, err := s.PutObject(ctx, putBucket, key, strings.NewReader(putBody), nil); err != nil {
			t.Fatalf("put %q: %v", key, err)
		}

		completeUploadWithCopiedPart(t, s, objectRef{multipartBucket, key}, firstPart, objectRef{putBucket, key})
		copyObjectWithHandler(t, svc, objectRef{copyBucket, key}, objectRef{putBucket, key})

		want[objectRef{putBucket, key}] = putBody
		want[objectRef{multipartBucket, key}] = firstPart + putBody
		want[objectRef{copyBucket, key}] = putBody
	}

	return want
}

// completeUploadWithCopiedPart creates dst by multipart upload from an
// uploaded first part and a second part copied from src.
func completeUploadWithCopiedPart(t *testing.T, s *MemoryStorage, dst objectRef, firstPart string, src objectRef) {
	t.Helper()

	ctx := t.Context()

	upload, err := s.CreateMultipartUpload(ctx, dst.bucket, dst.key, nil)
	if err != nil {
		t.Fatalf("create multipart upload %q: %v", dst.key, err)
	}

	uploaded, err := s.UploadPart(ctx, dst.bucket, dst.key, upload.UploadID, 1, strings.NewReader(firstPart))
	if err != nil {
		t.Fatalf("upload part %q: %v", dst.key, err)
	}

	copied, err := s.UploadPartCopy(ctx, dst.bucket, dst.key, upload.UploadID, 2, src.bucket, src.key, "", nil)
	if err != nil {
		t.Fatalf("upload part copy %q: %v", dst.key, err)
	}

	parts := []PartRequest{{PartNumber: 1, ETag: uploaded.ETag}, {PartNumber: 2, ETag: copied.ETag}}
	if _, err := s.CompleteMultipartUpload(ctx, dst.bucket, dst.key, upload.UploadID, parts); err != nil {
		t.Fatalf("complete multipart upload %q: %v", dst.key, err)
	}
}

// copyObjectWithHandler copies src to dst through the CopyObject handler, the
// way PUT /{bucket}/{key...} with X-Amz-Copy-Source is served.
func copyObjectWithHandler(t *testing.T, svc *Service, dst, src objectRef) {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/", http.NoBody)
	req.SetPathValue("bucket", dst.bucket)
	req.SetPathValue("key", dst.key)
	req.Header.Set("X-Amz-Copy-Source", src.bucket+"/"+src.key)

	rec := httptest.NewRecorder()
	svc.CopyObject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("copy %q: status %d: %s", dst.key, rec.Code, rec.Body.String())
	}
}

// dataDirBlobs walks root and returns the blob file names in dataDir's
// objects directory. Any other entry under root, apart from dataDir, its
// ancestors and the snapshot, is reported as an error.
func dataDirBlobs(t *testing.T, root, dataDir string) map[string]bool {
	t.Helper()

	allowed := map[string]bool{filepath.Join(dataDir, snapshotFile): true}

	for dir := dataDir; ; dir = filepath.Dir(dir) {
		allowed[dir] = true

		if dir == root {
			break
		}
	}

	objects := filepath.Join(dataDir, objectsDir)
	allowed[objects] = true
	blobName := regexp.MustCompile(`^[0-9a-f]{64}$`)
	blobs := make(map[string]bool)

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case allowed[p]:
		case filepath.Dir(p) == objects && d.Type().IsRegular() && blobName.MatchString(d.Name()):
			blobs[d.Name()] = true
		default:
			t.Errorf("unexpected entry %s", p)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	return blobs
}

// assertObjectBodies checks that every object in want reads back from s with
// its body.
func assertObjectBodies(t *testing.T, s *MemoryStorage, want map[objectRef]string) {
	t.Helper()

	for ref, body := range want {
		obj, err := s.GetObject(t.Context(), ref.bucket, ref.key)
		if err != nil {
			t.Errorf("get %s/%s: %v", ref.bucket, ref.key, err)

			continue
		}

		if string(obj.Body) != body {
			t.Errorf("get %s/%s body = %q, want %q", ref.bucket, ref.key, obj.Body, body)
		}
	}
}
