package s3

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const httpLoggingBucket = "http-logging-test"

// TestBucketLogging_PutGet exercises the storage round-trip. AWS lets
// you POST a BucketLoggingStatus payload with LoggingEnabled to opt
// in, or POST an empty status to opt out. terraform aws_s3_bucket_-
// logging uses the LoggingEnabled form; audit consumers care whether
// TargetBucket is non-empty.
func TestBucketLogging_PutGet(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	store := NewMemoryStorage()

	if err := store.CreateBucket(ctx, "logging-test"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	cfg := BucketLoggingConfig{TargetBucket: "log-target", TargetPrefix: "logs/"}

	if err := store.PutBucketLogging(ctx, "logging-test", cfg); err != nil {
		t.Fatalf("PutBucketLogging: %v", err)
	}

	got, err := store.GetBucketLogging(ctx, "logging-test")
	if err != nil {
		t.Fatalf("GetBucketLogging: %v", err)
	}

	if got == nil || got.TargetBucket != "log-target" || got.TargetPrefix != "logs/" {
		t.Fatalf("logging round-trip mismatch: got %+v, want %+v", got, cfg)
	}
}

// TestBucketLogging_OptOut empties the LoggingEnabled element to turn
// logging off. Get then returns nil.
func TestBucketLogging_OptOut(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	store := NewMemoryStorage()

	_ = store.CreateBucket(ctx, "logging-off")
	_ = store.PutBucketLogging(ctx, "logging-off", BucketLoggingConfig{TargetBucket: "x"})

	if err := store.PutBucketLogging(ctx, "logging-off", BucketLoggingConfig{}); err != nil {
		t.Fatalf("PutBucketLogging (off): %v", err)
	}

	got, err := store.GetBucketLogging(ctx, "logging-off")
	if err != nil {
		t.Fatalf("GetBucketLogging: %v", err)
	}

	if got != nil {
		t.Fatalf("expected nil after opt-out, got %+v", got)
	}
}

// TestBucketLogging_HTTP exercises the HTTP layer end-to-end. PUT
// accepts the AWS XML payload; GET returns the same structure (or an
// empty BucketLoggingStatus when not configured).
func TestBucketLogging_HTTP(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	store := NewMemoryStorage()
	svc := New(store, "")

	if err := store.CreateBucket(ctx, httpLoggingBucket); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	const putBody = `<?xml version="1.0" encoding="UTF-8"?>
<BucketLoggingStatus xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <LoggingEnabled>
    <TargetBucket>log-target</TargetBucket>
    <TargetPrefix>logs/</TargetPrefix>
  </LoggingEnabled>
</BucketLoggingStatus>`

	t.Run("PUT", func(t *testing.T) {
		w := callLoggingHandler(svc.PutBucketLogging, http.MethodPut, strings.NewReader(putBody))
		if w.Code != http.StatusOK {
			t.Fatalf("PUT got %d, want 200; body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("GET", func(t *testing.T) {
		w := callLoggingHandler(svc.GetBucketLogging, http.MethodGet, http.NoBody)
		if w.Code != http.StatusOK {
			t.Fatalf("GET got %d, want 200; body=%s", w.Code, w.Body.String())
		}

		var resp BucketLoggingStatus
		if err := xml.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("xml unmarshal: %v\nbody=%s", err, w.Body.String())
		}

		if resp.LoggingEnabled == nil || resp.LoggingEnabled.TargetBucket != "log-target" {
			t.Fatalf("expected LoggingEnabled with TargetBucket=log-target, got %+v", resp)
		}
	})

	t.Run("GET on bucket without logging returns empty status", func(t *testing.T) {
		assertEmptyLoggingStatus(ctx, t, svc, store)
	})
}

// assertEmptyLoggingStatus exercises the "logging disabled" GET path
// against a fresh bucket. Lifted out of TestBucketLogging_HTTP to keep
// that function under the funlen lint cap.
func assertEmptyLoggingStatus(ctx context.Context, t *testing.T, svc *Service, store *MemoryStorage) {
	t.Helper()

	_ = store.CreateBucket(ctx, "no-logging")

	req := httptest.NewRequest(http.MethodGet, "/no-logging?logging", http.NoBody)
	req.SetPathValue("bucket", "no-logging")

	w := httptest.NewRecorder()
	svc.GetBucketLogging(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp BucketLoggingStatus
	if err := xml.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("xml unmarshal: %v\nbody=%s", err, w.Body.String())
	}

	if resp.LoggingEnabled != nil {
		t.Fatalf("expected no LoggingEnabled element, got %+v", resp.LoggingEnabled)
	}
}

// callLoggingHandler dispatches one of the logging handlers with the
// path value already wired up.
func callLoggingHandler(h http.HandlerFunc, method string, body interface{ Read(p []byte) (int, error) }) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/"+httpLoggingBucket+"?logging", body)
	req.SetPathValue("bucket", httpLoggingBucket)

	w := httptest.NewRecorder()
	h(w, req)

	return w
}
