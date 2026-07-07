package s3

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMultipartUploadCarriesContentTypeMetadataAndSSE drives the full
// CreateMultipartUpload -> UploadPart -> CompleteMultipartUpload cycle
// through the HTTP handlers with Content-Type, x-amz-meta-*, and SSE-KMS
// headers on the initiate request, and asserts the completed object carries
// all three groups.
func TestMultipartUploadCarriesContentTypeMetadataAndSSE(t *testing.T) {
	t.Parallel()

	store, svc := setupMultipartMetadataFixture(t)

	uploadID := issueCreateMultipartUpload(t, svc, map[string]string{
		"Content-Type":                                "image/jpeg",
		"X-Amz-Meta-Origin":                           "cam1",
		"X-Amz-Server-Side-Encryption":                "aws:kms",
		"X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id": "mpu-key",
	})

	etag := issueUploadPart(t, svc, uploadID, 1, "hello multipart")

	issueCompleteMultipartUpload(t, svc, uploadID, []PartRequest{
		{PartNumber: 1, ETag: etag},
	})

	dstObj, err := store.GetObject(context.Background(), "mpu-metadata-test", "object")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}

	if dstObj.ContentType != "image/jpeg" {
		t.Fatalf("ContentType: got %q, want image/jpeg", dstObj.ContentType)
	}

	if got := dstObj.Metadata["origin"]; got != "cam1" {
		t.Fatalf("Metadata[origin]: got %q, want cam1", got)
	}

	if dstObj.ServerSideEncryption != "aws:kms" {
		t.Fatalf("ServerSideEncryption: got %q, want aws:kms", dstObj.ServerSideEncryption)
	}

	if dstObj.SSEKMSKeyID != "mpu-key" {
		t.Fatalf("SSEKMSKeyID: got %q, want mpu-key", dstObj.SSEKMSKeyID)
	}
}

// TestMultipartUploadWithNoHeadersFallsBackToOctetStream is a regression
// guard: when the initiate request carries no Content-Type, the completed
// object still defaults to application/octet-stream.
func TestMultipartUploadWithNoHeadersFallsBackToOctetStream(t *testing.T) {
	t.Parallel()

	store, svc := setupMultipartMetadataFixture(t)

	uploadID := issueCreateMultipartUpload(t, svc, map[string]string{})
	etag := issueUploadPart(t, svc, uploadID, 1, "plain body")
	issueCompleteMultipartUpload(t, svc, uploadID, []PartRequest{
		{PartNumber: 1, ETag: etag},
	})

	dstObj, err := store.GetObject(context.Background(), "mpu-metadata-test", "object")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}

	if dstObj.ContentType != "application/octet-stream" {
		t.Fatalf("ContentType: got %q, want application/octet-stream", dstObj.ContentType)
	}

	if len(dstObj.Metadata) != 0 {
		t.Fatalf("Metadata: got %v, want empty", dstObj.Metadata)
	}

	if dstObj.ServerSideEncryption != "" {
		t.Fatalf("ServerSideEncryption: got %q, want empty", dstObj.ServerSideEncryption)
	}
}

func setupMultipartMetadataFixture(t *testing.T) (*MemoryStorage, *Service) {
	t.Helper()

	store := NewMemoryStorage()
	svc := New(store, "")

	if err := store.CreateBucket(context.Background(), "mpu-metadata-test"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	return store, svc
}

func issueCreateMultipartUpload(t *testing.T, svc *Service, headers map[string]string) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/mpu-metadata-test/object?uploads", http.NoBody)
	req.SetPathValue("bucket", "mpu-metadata-test")
	req.SetPathValue("key", "object")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	w := httptest.NewRecorder()
	svc.CreateMultipartUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	var result InitiateMultipartUploadResult
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode InitiateMultipartUploadResult: %v", err)
	}

	if result.UploadID == "" {
		t.Fatal("UploadID is empty")
	}

	return result.UploadID
}

func issueUploadPart(t *testing.T, svc *Service, uploadID string, partNumber int, body string) string {
	t.Helper()

	url := fmt.Sprintf("/mpu-metadata-test/object?partNumber=%d&uploadId=%s", partNumber, uploadID)
	req := httptest.NewRequest(http.MethodPut, url, strings.NewReader(body))
	req.SetPathValue("bucket", "mpu-metadata-test")
	req.SetPathValue("key", "object")

	w := httptest.NewRecorder()
	svc.UploadPart(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("UploadPart status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("UploadPart: missing ETag response header")
	}

	return etag
}

func issueCompleteMultipartUpload(t *testing.T, svc *Service, uploadID string, parts []PartRequest) {
	t.Helper()

	var b strings.Builder

	b.WriteString("<CompleteMultipartUpload>")

	for _, p := range parts {
		fmt.Fprintf(&b, "<Part><PartNumber>%d</PartNumber><ETag>%s</ETag></Part>", p.PartNumber, p.ETag)
	}

	b.WriteString("</CompleteMultipartUpload>")

	url := "/mpu-metadata-test/object?uploadId=" + uploadID
	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(b.String()))
	req.SetPathValue("bucket", "mpu-metadata-test")
	req.SetPathValue("key", "object")

	w := httptest.NewRecorder()
	svc.CompleteMultipartUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}
}
