package lambda

import (
	"context"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// scriptedDeliverer returns results[i] for the i-th attempt (the last result
// repeats) and records each attempt's time on the bubble's virtual clock.
type scriptedDeliverer struct {
	mu      sync.Mutex
	results []deliveryResult
	times   []time.Time
}

func (s *scriptedDeliverer) deliver(_ context.Context, _ string, _ []byte) deliveryResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	i := len(s.times)
	s.times = append(s.times, time.Now())

	return s.results[min(i, len(s.results)-1)]
}

func (s *scriptedDeliverer) attempts() []time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.times)
}

// gaps returns the intervals between consecutive attempts.
func (s *scriptedDeliverer) gaps() []time.Duration {
	times := s.attempts()
	gaps := make([]time.Duration, 0, max(len(times)-1, 0))

	for i := 1; i < len(times); i++ {
		gaps = append(gaps, times[i].Sub(times[i-1]))
	}

	return gaps
}

// newSynctestEvent returns an event for d with its deadline set the way
// enqueue does, so the production defaults are exercised unchanged.
func newSynctestEvent(d *asyncDispatcher, deliverer asyncDeliverer) *asyncEvent {
	return &asyncEvent{deliverer: deliverer, payload: []byte(`{}`), deadline: time.Now().Add(d.maxEventAge)}
}

func TestAsyncDeliver_BackoffDoublesUpToCap(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		d := newAsyncDispatcher()
		deliverer := &scriptedDeliverer{results: []deliveryResult{
			asyncSystemError, asyncSystemError, asyncSystemError, asyncSystemError, asyncSystemError,
			asyncSystemError, asyncSystemError, asyncSystemError, asyncSystemError, asyncDelivered,
		}}

		d.deliver(t.Context(), "fn", newSynctestEvent(d, deliverer))

		want := []time.Duration{
			100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond,
			800 * time.Millisecond, 1600 * time.Millisecond, 3200 * time.Millisecond,
			asyncMaxBackoff, asyncMaxBackoff, asyncMaxBackoff,
		}

		if got := deliverer.gaps(); !slices.Equal(got, want) {
			t.Errorf("retry gaps = %v, want %v", got, want)
		}
	})
}

func TestAsyncDeliver_SystemErrorDroppedAfterMaxEventAge(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		d := newAsyncDispatcher()
		deliverer := &scriptedDeliverer{results: []deliveryResult{asyncSystemError}}
		start := time.Now()
		ev := newSynctestEvent(d, deliverer)

		d.deliver(t.Context(), "fn", ev)

		attempts := deliverer.attempts()
		last := attempts[len(attempts)-1]

		if !last.After(ev.deadline) {
			t.Errorf("last attempt at +%v, want after the %v max event age", last.Sub(start), asyncMaxEventAge)
		}

		if len(attempts) < 2 {
			t.Fatalf("attempts = %d, want the event retried before expiring", len(attempts))
		}

		// An attempt is only followed by a retry while the event is still
		// within its maximum age.
		if prev := attempts[len(attempts)-2]; prev.After(ev.deadline) {
			t.Errorf("attempt at +%v ran past the %v max event age", prev.Sub(start), asyncMaxEventAge)
		}

		if time.Since(start) > asyncMaxEventAge+asyncMaxBackoff {
			t.Errorf("deliver kept retrying until +%v, want it to stop within one backoff of the max event age",
				time.Since(start))
		}
	})
}

func TestAsyncDispatcher_NoDeliveryAfterMaxEventAge(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		d := newAsyncDispatcher()
		deliverer := &scriptedDeliverer{results: []deliveryResult{asyncSystemError}}

		d.enqueue("fn", deliverer, []byte(`{}`))

		time.Sleep(asyncMaxEventAge - time.Hour)
		synctest.Wait()

		if got := len(deliverer.attempts()); got < 2 {
			t.Fatalf("attempts within max event age = %d, want the event kept retrying", got)
		}

		time.Sleep(time.Hour + 2*asyncMaxBackoff)
		synctest.Wait()

		expired := len(deliverer.attempts())

		time.Sleep(time.Hour)
		synctest.Wait()

		if got := len(deliverer.attempts()); got != expired {
			t.Errorf("attempts after expiry grew from %d to %d, want the event dropped", expired, got)
		}

		d.close()
	})
}

func TestAsyncDeliver_FunctionErrorStopsAfterTwoRetries(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		d := newAsyncDispatcher()
		deliverer := &scriptedDeliverer{results: []deliveryResult{asyncFunctionError}}

		d.deliver(t.Context(), "fn", newSynctestEvent(d, deliverer))

		want := []time.Duration{asyncInitialBackoff, 2 * asyncInitialBackoff}

		if got := deliverer.gaps(); !slices.Equal(got, want) {
			t.Errorf("retry gaps = %v, want %v (initial attempt + %d retries)",
				got, want, asyncMaxFunctionErrorRetries)
		}
	})
}
