package storage_test

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/sivchari/kumo/internal/storage"
)

func TestScheduleSave_FlushWritesSnapshot(t *testing.T) {
	dir := t.TempDir()

	storage.ScheduleSave(dir, "snaptest-write", func() ([]byte, error) {
		return []byte(`{"hello":"world"}`), nil
	})

	// Debounced: the marshal callback runs on flush, not on the ScheduleSave call.
	storage.FlushSnapshots()

	data, err := os.ReadFile(filepath.Join(dir, "snaptest-write.json")) //nolint:gosec // fixed name under t.TempDir
	if err != nil {
		t.Fatalf("snapshot not written: %v", err)
	}

	if string(data) != `{"hello":"world"}` {
		t.Fatalf("unexpected snapshot content: %s", data)
	}
}

func TestScheduleSave_EmptyDataDirIsNoop(t *testing.T) {
	// Must not panic or start writing anywhere when persistence is disabled.
	storage.ScheduleSave("", "snaptest-noop", func() ([]byte, error) {
		t.Fatal("marshal must not be called when dataDir is empty")

		return nil, nil
	})

	storage.FlushSnapshots()
}

func TestScheduleSave_MarshalErrorKeepsRetrying(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snaptest-retry.json")

	// atomic: the background flush goroutine may read this concurrently.
	var fail atomic.Bool

	fail.Store(true)

	storage.ScheduleSave(dir, "snaptest-retry", func() ([]byte, error) {
		if fail.Load() {
			return nil, errors.New("boom")
		}

		return []byte(`{"ok":true}`), nil
	})

	// First flush fails to marshal; nothing should be written and the entry stays dirty.
	storage.FlushSnapshots()

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot should not exist after marshal error, stat err = %v", err)
	}

	// Recover: the still-dirty entry is retried on the next flush without a new ScheduleSave.
	fail.Store(false)
	storage.FlushSnapshots()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("snapshot should be written on retry: %v", err)
	}
}
