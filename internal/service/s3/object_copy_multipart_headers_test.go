package s3

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCopyObject_CopiesTagsAndSSEHeaders(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")

	if err := store.CreateBucket(t.Context(), "copy-bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	putObjectWithTagsAndSSE(t, svc, "copy-bucket", "source")
	copyObject(t, svc, "copy-bucket", "source", "dest")
	assertObjectTags(t, svc, "copy-bucket", "dest", map[string]string{"env": "dev", "team": "platform"})
	assertObjectSSE(t, svc, "copy-bucket", "dest")
}

func TestMultipartUpload_TagsAndSSEHeadersRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")

	if err := store.CreateBucket(t.Context(), "multipart-bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	uploadID := createMultipartUploadWithTagsAndSSE(t, svc, "multipart-bucket", "large-object")
	partETag := uploadPart(t, svc, "multipart-bucket", "large-object", uploadID, "payload")
	completeMultipartUpload(t, svc, "multipart-bucket", "large-object", uploadID, partETag)

	assertObjectTags(t, svc, "multipart-bucket", "large-object", map[string]string{"env": "prod"})
	assertObjectSSE(t, svc, "multipart-bucket", "large-object")
}

func putObjectWithTagsAndSSE(t *testing.T, svc *Service, bucket, key string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, strings.NewReader("body"))
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", key)
	req.Header.Set("X-Amz-Tagging", "env=dev&team=platform")
	setSSEKMSRequestHeaders(req)

	w := httptest.NewRecorder()
	svc.PutObject(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}

func copyObject(t *testing.T, svc *Service, bucket, srcKey, dstKey string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+dstKey, http.NoBody)
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", dstKey)
	req.Header.Set("X-Amz-Copy-Source", "/"+bucket+"/"+srcKey)

	w := httptest.NewRecorder()
	svc.CopyObject(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CopyObject status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}

func createMultipartUploadWithTagsAndSSE(t *testing.T, svc *Service, bucket, key string) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/"+bucket+"/"+key+"?uploads", http.NoBody)
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", key)
	req.Header.Set("X-Amz-Tagging", "env=prod")
	setSSEKMSRequestHeaders(req)

	w := httptest.NewRecorder()
	svc.CreateMultipartUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	var result InitiateMultipartUploadResult
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal initiate multipart: %v body=%s", err, w.Body.String())
	}

	if result.UploadID == "" {
		t.Fatal("UploadId is empty")
	}

	assertSSEKMSHeaders(t, "CreateMultipartUpload", w.Header())

	return result.UploadID
}

func uploadPart(t *testing.T, svc *Service, bucket, key, uploadID, body string) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key+"?partNumber=1&uploadId="+uploadID, strings.NewReader(body))
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", key)

	w := httptest.NewRecorder()
	svc.UploadPart(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("UploadPart status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	return w.Header().Get("ETag")
}

func completeMultipartUpload(t *testing.T, svc *Service, bucket, key, uploadID, etag string) {
	t.Helper()

	body := `<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>` + etag + `</ETag></Part></CompleteMultipartUpload>`
	req := httptest.NewRequest(http.MethodPost, "/"+bucket+"/"+key+"?uploadId="+uploadID, strings.NewReader(body))
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", key)

	w := httptest.NewRecorder()
	svc.CompleteMultipartUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	assertSSEKMSHeaders(t, "CompleteMultipartUpload", w.Header())
}

func assertObjectTags(t *testing.T, svc *Service, bucket, key string, want map[string]string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key+"?tagging", http.NoBody)
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", key)

	w := httptest.NewRecorder()
	svc.GetObjectTagging(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GetObjectTagging status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	var got Tagging
	if err := xml.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal tagging: %v body=%s", err, w.Body.String())
	}

	gotTags := make(map[string]string, len(got.TagSet.Tags))
	for _, tag := range got.TagSet.Tags {
		gotTags[tag.Key] = tag.Value
	}

	for key, value := range want {
		if gotTags[key] != value {
			t.Fatalf("tag %q: got %q, want %q (all=%v)", key, gotTags[key], value, gotTags)
		}
	}
}

func assertObjectSSE(t *testing.T, svc *Service, bucket, key string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodHead, "/"+bucket+"/"+key, http.NoBody)
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", key)

	w := httptest.NewRecorder()
	svc.HeadObject(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HeadObject status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	assertSSEKMSHeaders(t, "HeadObject", w.Header())
}

func setSSEKMSRequestHeaders(req *http.Request) {
	req.Header.Set("X-Amz-Server-Side-Encryption", sseKMSAlgorithm)
	req.Header.Set("X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id", sseKMSKeyID)
	req.Header.Set("X-Amz-Server-Side-Encryption-Bucket-Key-Enabled", "true")
}
