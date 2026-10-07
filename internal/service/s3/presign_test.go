package s3

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"testing/synctest"
	"time"
)

const (
	amzDateLayout = "20060102T150405Z"
	maxExpiresSec = 604800
)

// newPresignedRequest builds a request carrying only the query parameters that
// validatePresignedURL inspects; an empty value omits the parameter.
func newPresignedRequest(t *testing.T, amzDate, expires string) *http.Request {
	t.Helper()

	q := url.Values{}
	if amzDate != "" {
		q.Set("X-Amz-Date", amzDate)
	}

	if expires != "" {
		q.Set("X-Amz-Expires", expires)
	}

	q.Set("X-Amz-Signature", "deadbeef")

	return httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/bucket/key?"+q.Encode(), http.NoBody)
}

func assertPresignedError(t *testing.T, err error, code, message string) {
	t.Helper()

	var presignErr *PresignedURLError
	if !errors.As(err, &presignErr) {
		t.Fatalf("error = %v, want *PresignedURLError", err)
	}

	if presignErr.Code != code || presignErr.Message != message {
		t.Errorf("error = {%q, %q}, want {%q, %q}", presignErr.Code, presignErr.Message, code, message)
	}
}

// presignStep advances the virtual clock by sleep and then expects validation
// to report expiry (expired) or success.
type presignStep struct {
	sleep   time.Duration
	expired bool
}

// runPresignSteps signs a URL at the bubble's current time and applies steps in order.
func runPresignSteps(t *testing.T, expires string, steps []presignStep) {
	t.Helper()

	synctest.Test(t, func(t *testing.T) {
		r := newPresignedRequest(t, time.Now().UTC().Format(amzDateLayout), expires)

		for i, st := range steps {
			time.Sleep(st.sleep)

			err := validatePresignedURL(r)
			if st.expired {
				assertPresignedError(t, err, "AccessDenied", "Request has expired")

				continue
			}

			if err != nil {
				t.Fatalf("step %d: validatePresignedURL() = %v, want nil", i, err)
			}
		}
	})
}

// TestValidatePresignedURL_Expiry runs on synctest's virtual clock so that
// expiry is exercised without real sleeps.
func TestValidatePresignedURL_Expiry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		expires string
		steps   []presignStep
	}{
		{"within window", "60", []presignStep{{30 * time.Second, false}}},
		{"expired", "60", []presignStep{{61 * time.Second, true}}},
		// The check is a strict After, so the exact expiration instant is still valid.
		{"boundary", "60", []presignStep{
			{60*time.Second - time.Nanosecond, false},
			{time.Nanosecond, false},
			{time.Nanosecond, true},
		}},
		{"zero expires expires immediately", "0", []presignStep{{time.Nanosecond, true}}},
		{"max expires is inclusive", strconv.Itoa(maxExpiresSec), []presignStep{
			{maxExpiresSec * time.Second, false},
			{time.Second, true},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			runPresignSteps(t, tc.expires, tc.steps)
		})
	}
}

// TestValidatePresignedURL_Parameters covers the parameter checks that run
// before the expiry comparison and therefore do not depend on the clock.
func TestValidatePresignedURL_Parameters(t *testing.T) {
	t.Parallel()

	const (
		validDate   = "20000101T000000Z"
		overMaxSec  = "604801"
		overMaxMsg  = "X-Amz-Expires must be less than 604800 seconds"
		invalidDate = "Invalid X-Amz-Date format"
	)

	cases := []struct {
		name    string
		amzDate string
		expires string
		message string
	}{
		{"missing X-Amz-Date", "", "60", "X-Amz-Date must be in the ISO8601 Long Format"},
		{"missing X-Amz-Expires", validDate, "", "X-Amz-Expires must be provided"},
		{"non-numeric X-Amz-Expires", validDate, "soon", "X-Amz-Expires must be a number"},
		{"X-Amz-Expires above 7 days", validDate, overMaxSec, overMaxMsg},
		{"malformed X-Amz-Date", "2000-01-01T00:00:00Z", "60", invalidDate},
		// Expires is checked before the date is parsed.
		{"oversized expires wins over malformed date", "garbage", overMaxSec, overMaxMsg},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validatePresignedURL(newPresignedRequest(t, tc.amzDate, tc.expires))
			assertPresignedError(t, err, errCodeAuthQueryParamsError, tc.message)
		})
	}
}

// TestCheckPresignedURL_ExpiredResponse verifies the HTTP mapping of an expired URL.
func TestCheckPresignedURL_ExpiredResponse(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		r := newPresignedRequest(t, time.Now().UTC().Format(amzDateLayout), "1")

		time.Sleep(2 * time.Second)

		w := httptest.NewRecorder()
		if checkPresignedURL(w, r) {
			t.Fatal("checkPresignedURL() = true, want false")
		}

		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})
}

// TestCheckPresignedURL_NonPresigned verifies requests without a signature skip validation.
func TestCheckPresignedURL_NonPresigned(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/bucket/key", http.NoBody)
	w := httptest.NewRecorder()

	if !checkPresignedURL(w, r) {
		t.Fatal("checkPresignedURL() = false, want true")
	}
}
