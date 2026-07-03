package storage

import (
	"os"
	"strconv"
	"sync"
	"time"
)

// Marshaler returns a fresh JSON snapshot of a service's state. Implementations
// must be safe to call WITHOUT the caller holding the service lock: the background
// flusher invokes them off the mutation path, so they acquire their own read lock
// (this is exactly what each service's MarshalJSON does).
type Marshaler func() ([]byte, error)

// defaultFlushIntervalMS is how often dirty snapshots are written to disk.
const defaultFlushIntervalMS = 1000

// snapKey identifies a snapshot by its destination. Keying by both dataDir and
// name (not name alone) keeps two storage instances that share a service name but
// persist to different directories — e.g. successive instances in a test — from
// clobbering each other's pending entry.
type snapKey struct {
	dataDir string
	name    string
}

type snapshotEntry struct {
	marshal Marshaler
	dirty   bool
}

//nolint:gocheckglobals // a single process-wide debouncer coalesces all services' snapshots.
var (
	snapMu      sync.Mutex
	snapEntries = map[snapKey]*snapshotEntry{}
	snapStarted bool
)

// ScheduleSave coalesces frequent persistence requests for a (dataDir, name) into
// periodic background snapshots, instead of marshalling the whole store on every
// mutation (which is O(n) per write and, under high write rates, dominates both
// CPU and transient allocation, driving RSS up until OOM).
//
// It is cheap to call while holding a service lock: it only records the latest
// marshal callback and marks the entry dirty. A single background goroutine
// flushes dirty entries every KUMO_PERSIST_FLUSH_INTERVAL_MS (default 1000ms) by
// calling the marshal callback off the lock.
//
// Durability trade-off: up to one flush interval of mutations may be lost on a
// hard crash. Graceful shutdown still persists synchronously via Save in each
// service's Close (which also clears the pending entry, see markClean).
func ScheduleSave(dataDir, name string, marshal Marshaler) {
	if dataDir == "" {
		return
	}

	snapMu.Lock()

	key := snapKey{dataDir: dataDir, name: name}

	e := snapEntries[key]
	isNew := e == nil

	if isNew {
		e = &snapshotEntry{}
		snapEntries[key] = e
	}

	e.marshal = marshal
	e.dirty = true

	startLoop := !snapStarted
	if startLoop {
		snapStarted = true
	}

	snapMu.Unlock()

	// Done outside the lock so a disk syscall never serializes other services'
	// mutations. Establish the data directory once, on first registration, so the
	// background flusher can write without MkdirAll and therefore never resurrects
	// a dataDir removed mid-run. Best-effort: a genuine failure surfaces (as a
	// retried flush) rather than being silently lost.
	if isNew {
		_ = os.MkdirAll(dataDir, 0o750)
	}

	if startLoop {
		go flushLoop()
	}
}

// markClean clears any pending dirty state for (dataDir, name). Save calls it
// after a synchronous write so a service's Close persists the final state and the
// background flusher will not later resurrect it (e.g. recreating a just-removed
// data directory).
func markClean(dataDir, name string) {
	snapMu.Lock()
	defer snapMu.Unlock()

	if e := snapEntries[snapKey{dataDir: dataDir, name: name}]; e != nil {
		e.dirty = false
	}
}

func flushInterval() time.Duration {
	ms := defaultFlushIntervalMS

	if v := os.Getenv("KUMO_PERSIST_FLUSH_INTERVAL_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ms = n
		}
	}

	return time.Duration(ms) * time.Millisecond
}

func flushLoop() {
	ticker := time.NewTicker(flushInterval())
	defer ticker.Stop()

	for range ticker.C {
		FlushSnapshots()
	}
}

// FlushSnapshots writes every dirty snapshot to disk now. The background loop
// calls it on each tick; it is also exported so tests can force a flush.
func FlushSnapshots() {
	type job struct {
		key     snapKey
		marshal Marshaler
	}

	snapMu.Lock()

	var jobs []job

	for key, e := range snapEntries {
		if e.dirty {
			e.dirty = false

			jobs = append(jobs, job{key: key, marshal: e.marshal})
		}
	}

	snapMu.Unlock()

	// Marshal and write off the lock so a slow disk never blocks mutations.
	// writeSnapshot does not create dataDir, so a flush racing with dataDir
	// removal fails here instead of resurrecting the directory.
	for _, j := range jobs {
		data, err := j.marshal()
		if err == nil {
			err = writeSnapshot(j.key.dataDir, j.key.name, data)
		}

		if err != nil {
			// Marshal or write failed: re-mark dirty so the next tick retries.
			snapMu.Lock()

			if e := snapEntries[j.key]; e != nil {
				e.dirty = true
			}

			snapMu.Unlock()
		}
	}
}
