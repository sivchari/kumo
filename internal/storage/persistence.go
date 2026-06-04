// Package storage provides common storage utilities.
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Load reads a JSON snapshot from dataDir/{name}.json and unmarshals it into v.
// Returns nil if the file does not exist.
func Load(dataDir, name string, v json.Unmarshaler) error {
	path := filepath.Join(dataDir, name+".json")

	data, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to read snapshot %s: %w", path, err)
	}

	if err := v.UnmarshalJSON(data); err != nil {
		return fmt.Errorf("failed to unmarshal snapshot %s: %w", path, err)
	}

	return nil
}

// Save marshals v to JSON and writes it atomically to dataDir/{name}.json.
// It also clears any pending debounced snapshot for (dataDir, name): callers use
// Save for synchronous, authoritative persistence (e.g. a service's Close), after
// which the background flusher must not resurrect the entry.
func Save(dataDir, name string, v json.Marshaler) error {
	data, err := v.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot %s: %w", name, err)
	}

	if err := SaveBytes(dataDir, name, data); err != nil {
		return err
	}

	markClean(dataDir, name)

	return nil
}

// SaveBytes writes pre-marshaled JSON data atomically to dataDir/{name}.json,
// creating dataDir if needed. This is useful when the caller already holds a lock
// and cannot use MarshalJSON (which may also acquire a lock, causing a deadlock).
func SaveBytes(dataDir, name string, data []byte) error {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return fmt.Errorf("failed to create data directory %s: %w", dataDir, err)
	}

	return writeSnapshot(dataDir, name, data)
}

// writeSnapshot atomically writes data to dataDir/{name}.json WITHOUT creating
// dataDir. The background snapshotter uses it so that a flush which races with the
// removal of dataDir (e.g. a test's t.TempDir cleanup) fails cleanly instead of
// resurrecting the directory. dataDir is established up front in ScheduleSave.
func writeSnapshot(dataDir, name string, data []byte) error {
	path := filepath.Join(dataDir, name+".json")
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("failed to write temporary snapshot %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("failed to rename snapshot %s to %s: %w", tmp, path, err)
	}

	return nil
}
