package s3

import (
	"encoding/json"
	"testing"
)

// TestCreateMultipartUpload_AfterSnapshotRestore reproduces the regression
// where CreateMultipartUpload panicked with "assignment to entry in nil map"
// against a bucket restored from a persistence snapshot.
//
// MemoryBucket.MultipartUploads is tagged json:"-", so it is absent from the
// snapshot and comes back nil after UnmarshalJSON. CreateBucket is the only
// place that make()s the map, so a restored bucket (never re-created in the
// running process) had a nil map and panicked on the first multipart upload.
// UnmarshalJSON must re-initialize the map for every restored bucket.
func TestCreateMultipartUpload_AfterSnapshotRestore(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Build a snapshot from a store that owns one bucket, exactly as the
	// persistence layer does on shutdown.
	src := NewMemoryStorage()
	if err := src.CreateBucket(ctx, "restore-test"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	snapshot, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	// Restore into a fresh store, simulating a process restart.
	restored := NewMemoryStorage()
	if err := json.Unmarshal(snapshot, restored); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}

	b, ok := restored.Buckets["restore-test"]
	if !ok {
		t.Fatalf("restored store is missing bucket %q", "restore-test")
	}

	if b.MultipartUploads == nil {
		t.Fatalf("MultipartUploads was not re-initialized on restore (nil map)")
	}

	// This is the call that previously panicked on a restored bucket.
	upload, err := restored.CreateMultipartUpload(ctx, "restore-test", "path/to/object", nil)
	if err != nil {
		t.Fatalf("CreateMultipartUpload on restored bucket: %v", err)
	}

	if upload == nil || upload.UploadID == "" {
		t.Fatalf("CreateMultipartUpload returned no upload: %+v", upload)
	}
}
