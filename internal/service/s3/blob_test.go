package s3

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

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
	snapshot, err := os.ReadFile(filepath.Join(dir, "s3.json")) //nolint:gosec // fixed name under t.TempDir
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
	if err := os.WriteFile(filepath.Join(dir, "s3.json"), []byte(legacy), 0o600); err != nil {
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
