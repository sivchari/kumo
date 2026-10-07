package lambda

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"testing/synctest"
	"time"
)

// startPoller has a handler pick up the next invocation for fn after delay,
// then respond after respondAfter (never when negative).
func startPoller(t *testing.T, b *runtimeBroker, fn string, delay, respondAfter time.Duration) {
	t.Helper()

	go func() {
		time.Sleep(delay)

		inv, err := b.next(t.Context(), fn)
		if err != nil {
			return
		}

		if respondAfter >= 0 {
			time.Sleep(respondAfter)
			b.respond(fn, inv.id, []byte(`{}`), false)
		}
	}()
}

func TestRuntimeBrokerInvoke_NoPollerAfterPickupTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		b := newRuntimeBroker()
		start := time.Now()

		_, err := b.invoke(t.Context(), "fn", []byte(`{}`), runtimePickupTimeout, defaultFunctionTimeout)

		if !errors.Is(err, errRuntimeNoPoller) {
			t.Fatalf("err = %v, want %v", err, errRuntimeNoPoller)
		}

		if elapsed := time.Since(start); elapsed != runtimePickupTimeout {
			t.Errorf("returned after %v, want %v", elapsed, runtimePickupTimeout)
		}
	})
}

func TestRuntimeBrokerInvoke_PollerArrivingJustBeforePickupTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		b := newRuntimeBroker()

		startPoller(t, b, "fn", runtimePickupTimeout-time.Millisecond, 0)

		res, err := b.invoke(t.Context(), "fn", []byte(`{}`), runtimePickupTimeout, defaultFunctionTimeout)
		if err != nil {
			t.Fatalf("invoke: %v", err)
		}

		if res.errored {
			t.Error("result errored, want success")
		}
	})
}

func TestRuntimeBrokerInvoke_ResponseTimeoutAfterFunctionTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		b := newRuntimeBroker()
		start := time.Now()

		startPoller(t, b, "fn", 0, -1)

		_, err := b.invoke(t.Context(), "fn", []byte(`{}`), runtimePickupTimeout, defaultFunctionTimeout)

		if !errors.Is(err, errRuntimeResponseTimeout) {
			t.Fatalf("err = %v, want %v", err, errRuntimeResponseTimeout)
		}

		if elapsed := time.Since(start); elapsed != defaultFunctionTimeout {
			t.Errorf("returned after %v, want %v", elapsed, defaultFunctionTimeout)
		}
	})
}

func TestRuntimeBrokerInvoke_ResponseJustBeforeFunctionTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		b := newRuntimeBroker()

		startPoller(t, b, "fn", 0, defaultFunctionTimeout-time.Millisecond)

		if _, err := b.invoke(t.Context(), "fn", []byte(`{}`), runtimePickupTimeout, defaultFunctionTimeout); err != nil {
			t.Fatalf("invoke: %v", err)
		}
	})
}

func TestRuntimeNext_DeadlineIsPickupTimePlusDefaultTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		svc := New(NewMemoryStorage(defaultBaseURL), defaultBaseURL)

		t.Cleanup(func() {
			if err := svc.Close(); err != nil {
				t.Errorf("close service: %v", err)
			}
		})

		const pickupDelay = 10 * time.Second

		timeout := functionTimeout(&Function{})

		go func() {
			_, _ = svc.broker.invoke(t.Context(), "fn", []byte(`{}`), runtimePickupTimeout, timeout)
		}()

		time.Sleep(pickupDelay)

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
			"/_runtime/fn/2018-06-01/runtime/invocation/next", nil)

		svc.RuntimeNext(rec, req)

		deadlineMs, err := strconv.ParseInt(rec.Header().Get("Lambda-Runtime-Deadline-Ms"), 10, 64)
		if err != nil {
			t.Fatalf("parse deadline header: %v", err)
		}

		// The deadline counts from pickup (after the pickup delay), not from
		// when the invocation was queued.
		if want := time.Now().Add(timeout).UnixMilli(); deadlineMs != want {
			t.Errorf("deadline = %d, want %d (pickup time + %v)", deadlineMs, want, timeout)
		}

		svc.broker.respond("fn", rec.Header().Get("Lambda-Runtime-Aws-Request-Id"), []byte(`{}`), false)
	})
}
