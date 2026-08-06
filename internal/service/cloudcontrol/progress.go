package cloudcontrol

import "sync"

// maxTrackedEvents bounds the progressTracker map so unbounded
// Create/Update/Delete traffic doesn't leak ProgressEvents for the
// process lifetime; real Cloud Control only retains a recent window too.
const maxTrackedEvents = 1024

// progressTracker keeps the per-RequestToken outcome of every Cloud
// Control operation. kumo runs everything synchronously but SDK clients
// still poll GetResourceRequestStatus expecting SUCCESS plus the original
// Identifier + TypeName, so we remember what each token resolved to.
type progressTracker struct {
	// RWMutex: lookup (status polling) is far more frequent than record.
	mu     sync.RWMutex
	events map[string]ProgressEvent
	// order mirrors insertion so the oldest entry can be evicted FIFO
	// when the map hits maxTrackedEvents.
	order []string
}

func newProgressTracker() *progressTracker {
	return &progressTracker{
		events: make(map[string]ProgressEvent),
		order:  make([]string, 0, maxTrackedEvents),
	}
}

// record stores the event keyed by its RequestToken, evicting the oldest
// entry FIFO once the map is full. Pointer-passed to avoid the 100+ byte
// struct copy the linter flags.
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
// Read-locked: a plain Mutex here previously serialised the polling loop
// and capped throughput at ~5K req/s on a 32-way concurrent client.
func (p *progressTracker) lookup(requestToken string) (ProgressEvent, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ev, ok := p.events[requestToken]

	return ev, ok
}
