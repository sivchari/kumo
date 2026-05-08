package cloudcontrol

import "sync"

// progressTracker keeps the per-RequestToken outcome of every Cloud
// Control operation kumo serves. Real Cloud Control is asynchronous, so
// SDK clients call GetResourceRequestStatus until they get SUCCESS plus
// the Identifier of the resource. kumo runs everything synchronously,
// but the SDK still polls — so we have to remember what each token
// resolved to so the polling loop completes with the correct Identifier
// + TypeName.
type progressTracker struct {
	mu     sync.Mutex
	events map[string]ProgressEvent
}

func newProgressTracker() *progressTracker {
	return &progressTracker{events: make(map[string]ProgressEvent)}
}

// record stores the event keyed by its RequestToken. The token is
// generated client-side (or by us when the client sent none) and is
// echoed back through every subsequent status poll.
func (p *progressTracker) record(ev ProgressEvent) {
	if ev.RequestToken == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.events[ev.RequestToken] = ev
}

// lookup returns the previously-recorded event for the token, if any.
// A miss is reported as a synthetic SUCCESS with just the token echoed
// back, matching how real Cloud Control responds for stale tokens.
func (p *progressTracker) lookup(requestToken string) (ProgressEvent, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ev, ok := p.events[requestToken]

	return ev, ok
}
