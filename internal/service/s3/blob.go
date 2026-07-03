package s3

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// objectsDir is the sub-directory under the data directory that holds object
// bodies as individual content-addressed blob files. Separating bodies from the
// metadata snapshot (s3.json) keeps the snapshot small: persisting the whole
// store no longer marshals every body into one JSON document (which spiked RSS
// to 2-3x the data size via base64 expansion and transient buffers, driving the
// process into the OOM killer). Bodies are written as plain bytes, one file per
// distinct content.
const objectsDir = "s3-objects"

// bodyRefOf returns the content-address (sha256 hex) used as the blob filename
// for the given body. Content addressing means identical bodies (object
// versions, server-side copies) share a single blob file on disk.
func bodyRefOf(data []byte) string {
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

// blobPath returns the on-disk path of the blob identified by ref.
func blobPath(dataDir, ref string) string {
	return filepath.Join(dataDir, objectsDir, ref)
}

// writeBlob persists data as the blob identified by ref, creating the objects
// directory if needed. It is idempotent: because ref is the content hash, an
// existing file already holds identical bytes, so the write is skipped. The
// write is atomic (tmp + rename) so a crash mid-write never leaves a partial
// blob that a later load would read as a corrupt body.
func writeBlob(dataDir, ref string, data []byte) error {
	path := blobPath(dataDir, ref)

	if _, err := os.Stat(path); err == nil {
		return nil
	}

	dir := filepath.Join(dataDir, objectsDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create objects directory %s: %w", dir, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("failed to write temporary blob %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("failed to rename blob %s to %s: %w", tmp, path, err)
	}

	return nil
}

// readBlob loads the body identified by ref from disk.
func readBlob(dataDir, ref string) ([]byte, error) {
	path := blobPath(dataDir, ref)

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read blob %s: %w", path, err)
	}

	return data, nil
}

// gcBlobs removes blob files no longer referenced by any object. It is
// best-effort: a removal failure is ignored so a transient FS error never fails
// a snapshot. referenced holds every live bodyRef; any other file in the objects
// directory is an orphan from an overwritten or deleted object.
func gcBlobs(dataDir string, referenced map[string]struct{}) {
	dir := filepath.Join(dataDir, objectsDir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		if _, ok := referenced[name]; ok {
			continue
		}

		_ = os.Remove(filepath.Join(dir, name))
	}
}
