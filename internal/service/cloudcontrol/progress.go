package cloudcontrol

import "sync"

// maxTrackedEvents bounds the progressTracker map. Without a cap, every
// Create / Update / Delete leaks one ProgressEvent into the map for the
// process lifetime, which adds up under sustained load. Real Cloud
// Control surfaces a recent window (the API doc says ~7 days);
// remembering the most-recent maxTrackedEvents keeps memory bounded
// while preserving the SDK polling contract for fresh requests.
const maxTrackedEvents = 1024

// progressTracker keeps the per-RequestToken outcome of every Cloud
// Control operation kumo serves. Real Cloud Control is asynchronous, so
// SDK clients call GetResourceRequestStatus until they get SUCCESS plus
// the Identifier of the resource. kumo runs everything synchronously,
// but the SDK still polls — so we have to remember what each token
// resolved to so the polling loop completes with the correct Identifier
// + TypeName.
type progressTracker struct {
	// RWMutex because lookup is the dominant operation: terraform-awscc
	// polls GetResourceRequestStatus far more often than it fires
	// Create / Update / Delete, so concurrent readers shouldn't be
	// serialised by a plain Mutex.
	mu     sync.RWMutex
	events map[string]ProgressEvent
	// order mirrors insertion order so we can evict the oldest entry
	// when the map hits maxTrackedEvents. A real LRU would track touch
	// time; for kumo the polling loop only revisits a token a handful
	// of times immediately after the operation, so insertion-order FIFO
	// is enough.
	order []string
}

func newProgressTracker() *progressTracker {
	return &progressTracker{
		events: make(map[string]ProgressEvent),
		order:  make([]string, 0, maxTrackedEvents),
	}
}

// record stores the event keyed by its RequestToken. Pointer-passed to
// avoid the 100+ byte struct copy the linter flags. The token is
// generated client-side (or by us when the client sent none) and is
// echoed back through every subsequent status poll. When the map is
// full, the oldest entry is evicted FIFO so memory stays bounded under
// long-running load.
func (p *progressTracker) record(ev *ProgressEvent) {
	if ev == nil || ev.RequestToken == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.events[ev.RequestToken]; !exists {
		if len(p.order) >= maxTrackedEvents {
			oldest := p.order[0]
			p.order = p.order[1:]
			delete(p.events, oldest)
		}

		p.order = append(p.order, ev.RequestToken)
	}

	p.events[ev.RequestToken] = *ev
}

// lookup returns the previously-recorded event for the token, if any.
// Read-locked because every CC poll path lands here; with the prior
// Mutex contention serialised the polling loop and capped throughput
// at ~5K req/s on a 32-way concurrent client.
func (p *progressTracker) lookup(requestToken string) (ProgressEvent, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ev, ok := p.events[requestToken]

	return ev, ok
}
