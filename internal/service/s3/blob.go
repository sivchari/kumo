package s3

import (
	"crypto/md5" //nolint:gosec // MD5 is required for S3 ETag calculation per AWS specification
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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

type streamedBlob struct {
	ref  string
	etag string
	size int64
}

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

// writeBlobStream copies body directly to the blob store while calculating the
// hashes needed for the content address and S3 ETag. At no point does it buffer
// the entire object in memory.
func writeBlobStream(dataDir string, body io.Reader) (_ streamedBlob, retErr error) {
	dir := filepath.Join(dataDir, objectsDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return streamedBlob{}, fmt.Errorf("failed to create objects directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return streamedBlob{}, fmt.Errorf("failed to create temporary blob: %w", err)
	}

	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if retErr != nil {
			_ = os.Remove(tmpName)
		}
	}()

	md5Hash := md5.New() //nolint:gosec // MD5 is required for S3 ETag calculation per AWS specification
	sha256Hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, md5Hash, sha256Hash), body)
	if err != nil {
		return streamedBlob{}, fmt.Errorf("failed to stream body: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return streamedBlob{}, fmt.Errorf("failed to close temporary blob %s: %w", tmpName, err)
	}

	ref := hex.EncodeToString(sha256Hash.Sum(nil))
	path := blobPath(dataDir, ref)
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(tmpName); err != nil {
			return streamedBlob{}, fmt.Errorf("failed to remove duplicate temporary blob %s: %w", tmpName, err)
		}
	} else if !os.IsNotExist(err) {
		return streamedBlob{}, fmt.Errorf("failed to stat blob %s: %w", path, err)
	} else if err := os.Rename(tmpName, path); err != nil {
		return streamedBlob{}, fmt.Errorf("failed to rename blob %s to %s: %w", tmpName, path, err)
	}

	return streamedBlob{
		ref:  ref,
		etag: hex.EncodeToString(md5Hash.Sum(nil)),
		size: size,
	}, nil
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
