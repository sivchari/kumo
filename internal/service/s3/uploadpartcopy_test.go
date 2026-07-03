package s3

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseCopySourceRange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		header string
		start  int64
		end    int64
		want   bool // valid
		isNil  bool // result is nil (absent header)
	}{
		{"", 0, 0, true, true},
		{"bytes=0-99", 0, 99, true, false},
		{"bytes=100-199", 100, 199, true, false},
		{"bytes= 0 - 99 ", 0, 99, true, false}, // tolerate spaces
		{"bytes=-99", 0, 0, false, false},      // suffix not allowed
		{"bytes=100-", 0, 0, false, false},     // open not allowed
		{"bytes=200-100", 0, 0, false, false},  // inverted
		{"items=0-99", 0, 0, false, false},     // wrong unit
	}

	for _, tc := range cases {
		t.Run(tc.header, func(t *testing.T) {
			got, err := parseCopySourceRange(tc.header)
			if tc.want && err != nil {
				t.Fatalf("want valid, got err=%v", err)
			}

			if !tc.want && err == nil {
				t.Fatalf("want error, got nil")
			}

			if tc.isNil && got != nil {
				t.Fatalf("want nil result, got %+v", got)
			}

			if !tc.isNil && got != nil && (got.Start != tc.start || got.End != tc.end) {
				t.Fatalf("got (%d, %d), want (%d, %d)", got.Start, got.End, tc.start, tc.end)
			}
		})
	}
}

// uploadPartCopyCase pins one HTTP-layer scenario for the table test.
type uploadPartCopyCase struct {
	name        string
	srcPayload  string
	copySource  string // X-Amz-Copy-Source
	copyRange   string // X-Amz-Copy-Source-Range
	wantStatus  int
	wantPartLen int // expected stored part body length on success
}

var uploadPartCopyCases = []uploadPartCopyCase{
	{"full source", "0123456789", "/src/file.txt", "", http.StatusOK, 10},
	{"ranged copy", "0123456789", "/src/file.txt", "bytes=2-5", http.StatusOK, 4},
	{"invalid range", "0123456789", "/src/file.txt", "bytes=20-30", http.StatusBadRequest, 0},
	{"malformed source", "0123456789", "/single-segment", "", http.StatusBadRequest, 0},
	{"source not found", "0123456789", "/src/missing.txt", "", http.StatusNotFound, 0},
}

// TestUploadPartCopy_HTTPRoundTrip exercises the dispatcher + handler
// + storage end-to-end through the HTTP layer.
func TestUploadPartCopy_HTTPRoundTrip(t *testing.T) {
	t.Parallel()

	for _, tc := range uploadPartCopyCases {
		t.Run(tc.name, func(t *testing.T) {
			runUploadPartCopyCase(t, &tc)
		})
	}
}

// runUploadPartCopyCase sets up a fresh storage with a single source
// object + open multipart upload, then issues UploadPartCopy via the
// dispatcher.
func runUploadPartCopyCase(t *testing.T, tc *uploadPartCopyCase) {
	t.Helper()

	store, svc, uploadID := setupUploadPartCopyFixture(t, tc.srcPayload)
	w := issueUploadPartCopy(svc, uploadID, tc.copySource, tc.copyRange)

	if w.Code != tc.wantStatus {
		t.Fatalf("status: got %d, want %d (body=%s)", w.Code, tc.wantStatus, w.Body.String())
	}

	if tc.wantStatus != http.StatusOK {
		return
	}

	verifyStoredPart(t, store, uploadID, w, tc.wantPartLen)
}

// setupUploadPartCopyFixture wires up src+dst buckets, the source
// payload, and an open multipart upload. Returns the store, service,
// and the new uploadID.
func setupUploadPartCopyFixture(t *testing.T, srcPayload string) (*MemoryStorage, *Service, string) {
	t.Helper()

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	if err := store.CreateBucket(ctx, "src"); err != nil {
		t.Fatalf("CreateBucket src: %v", err)
	}

	if err := store.CreateBucket(ctx, "dst"); err != nil {
		t.Fatalf("CreateBucket dst: %v", err)
	}

	if _, err := store.PutObject(ctx, "src", "file.txt", strings.NewReader(srcPayload), nil); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	upload, err := store.CreateMultipartUpload(ctx, "dst", "out.txt")
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}

	return store, svc, upload.UploadID
}

// issueUploadPartCopy builds + dispatches the PUT request through the
// real handleObjectPut router so the dispatch logic is exercised too.
func issueUploadPartCopy(svc *Service, uploadID, copySource, copyRange string) *httptest.ResponseRecorder {
	url := "/dst/out.txt?partNumber=1&uploadId=" + uploadID
	req := httptest.NewRequest(http.MethodPut, url, http.NoBody)
	req.SetPathValue("bucket", "dst")
	req.SetPathValue("key", "out.txt")

	if copySource != "" {
		req.Header.Set("X-Amz-Copy-Source", copySource)
	}

	if copyRange != "" {
		req.Header.Set("X-Amz-Copy-Source-Range", copyRange)
	}

	w := httptest.NewRecorder()
	svc.handleObjectPut(w, req)

	return w
}

// verifyStoredPart confirms the response is a valid CopyPartResult
// AND that the storage actually has one part of the expected size.
func verifyStoredPart(t *testing.T, store *MemoryStorage, uploadID string, w *httptest.ResponseRecorder, wantLen int) {
	t.Helper()

	var got CopyPartResult
	if err := xml.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal CopyPartResult: %v body=%s", err, w.Body.String())
	}

	if got.ETag == "" {
		t.Fatalf("CopyPartResult.ETag is empty")
	}

	parts, err := store.ListParts(context.Background(), "dst", "out.txt", uploadID, 100)
	if err != nil {
		t.Fatalf("ListParts: %v", err)
	}

	if len(parts) != 1 {
		t.Fatalf("parts: got %d, want 1", len(parts))
	}

	if int(parts[0].Size) != wantLen {
		t.Fatalf("part size: got %d, want %d", parts[0].Size, wantLen)
	}
}

// TestUploadPartCopy_RoundTripsThroughComplete confirms the copied
// part can actually be assembled by CompleteMultipartUpload.
func TestUploadPartCopy_RoundTripsThroughComplete(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	_ = store.CreateBucket(ctx, "src")
	_ = store.CreateBucket(ctx, "dst")
	_, _ = store.PutObject(ctx, "src", "blob", strings.NewReader("ABCDEFGHIJ"), nil)

	upload, err := store.CreateMultipartUpload(ctx, "dst", "joined")
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}

	p1, err := store.UploadPartCopy(ctx, "dst", "joined", upload.UploadID, 1, "src", "blob", "", &CopyRange{Start: 0, End: 4})
	if err != nil {
		t.Fatalf("UploadPartCopy 1: %v", err)
	}

	p2, err := store.UploadPartCopy(ctx, "dst", "joined", upload.UploadID, 2, "src", "blob", "", &CopyRange{Start: 5, End: 9})
	if err != nil {
		t.Fatalf("UploadPartCopy 2: %v", err)
	}

	obj, err := store.CompleteMultipartUpload(ctx, "dst", "joined", upload.UploadID, []PartRequest{
		{PartNumber: 1, ETag: p1.ETag},
		{PartNumber: 2, ETag: p2.ETag},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload: %v", err)
	}

	if string(obj.Body) != "ABCDEFGHIJ" {
		t.Fatalf("assembled body: got %q, want ABCDEFGHIJ", obj.Body)
	}
}
