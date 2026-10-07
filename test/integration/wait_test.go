//go:build integration

package integration

import (
	"testing"
	"time"
)

// waitForPollInterval is the delay between condition checks in waitFor.
const waitForPollInterval = 100 * time.Millisecond

// waitFor polls cond until it returns true or the timeout elapses. It fails
// the test when the deadline passes, so callers can rely on the condition
// holding afterwards. Use it instead of a fixed time.Sleep so tests wait
// exactly as long as the server needs instead of guessing a safe margin.
func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for {
		if cond() {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("condition %q not met within %v", what, timeout)
		}

		time.Sleep(waitForPollInterval)
	}
}
