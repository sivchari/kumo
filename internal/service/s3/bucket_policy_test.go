package s3

import (
	"context"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	codeNoSuchBucket       = "NoSuchBucket"
	codeNoSuchBucketPolicy = "NoSuchBucketPolicy"
)

// TestBucketPolicy_PutGetDelete exercises the storage round-trip.
// AWS treats a bucket policy as an opaque JSON document for the
// purposes of Put / Get; structural validation belongs in IAM-layer
// rules (which kumo doesn't model). So the storage just persists the
// bytes and Get returns them verbatim.
func TestBucketPolicy_PutGetDelete(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	store := NewMemoryStorage()

	if err := store.CreateBucket(ctx, "policy-test"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	const doc = `{"Version":"2012-10-17","Statement":[{"Effect":"Deny","Principal":"*","Action":"s3:*","Resource":"*","Condition":{"Bool":{"aws:SecureTransport":"false"}}}]}`

	if err := store.PutBucketPolicy(ctx, "policy-test", doc); err != nil {
		t.Fatalf("PutBucketPolicy: %v", err)
	}

	got, err := store.GetBucketPolicy(ctx, "policy-test")
	if err != nil {
		t.Fatalf("GetBucketPolicy: %v", err)
	}

	if got != doc {
		t.Fatalf("policy round-trip mismatch:\ngot:  %s\nwant: %s", got, doc)
	}

	if err := store.DeleteBucketPolicy(ctx, "policy-test"); err != nil {
		t.Fatalf("DeleteBucketPolicy: %v", err)
	}

	if _, err := store.GetBucketPolicy(ctx, "policy-test"); err == nil {
		t.Fatal("expected NoSuchBucketPolicy after delete, got nil")
	}
}

// TestBucketPolicy_Errors covers the AWS error codes terraform and
// the SDK look at — NoSuchBucket on a missing bucket and
// NoSuchBucketPolicy on an unconfigured bucket.
func TestBucketPolicy_Errors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewMemoryStorage()

	if _, err := store.GetBucketPolicy(ctx, "no-such"); err == nil {
		t.Fatal("expected error on missing bucket, got nil")
	} else {
		assertBucketErrorCode(t, err, codeNoSuchBucket)
	}

	if err := store.PutBucketPolicy(ctx, "no-such", "{}"); err == nil {
		t.Fatal("expected NoSuchBucket on Put, got nil")
	}

	_ = store.CreateBucket(ctx, "policy-empty")

	if _, err := store.GetBucketPolicy(ctx, "policy-empty"); err == nil {
		t.Fatal("expected NoSuchBucketPolicy on unconfigured bucket, got nil")
	} else {
		assertBucketErrorCode(t, err, codeNoSuchBucketPolicy)
	}
}

// TestBucketPolicy_HTTP exercises the HTTP layer end-to-end:
// PUT /{bucket}?policy persists; GET /{bucket}?policy returns the
// document; DELETE /{bucket}?policy removes it. terraform aws_s3_-
// bucket_policy uses exactly this surface.
func TestBucketPolicy_HTTP(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	store := NewMemoryStorage()
	svc := New(store, "")

	if err := store.CreateBucket(ctx, "http-policy-test"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	doc := `{"Version":"2012-10-17","Statement":[]}`

	t.Run("PUT", func(t *testing.T) {
		w := callPolicyHandler(svc.PutBucketPolicy, http.MethodPut, strings.NewReader(doc))
		if w.Code != http.StatusNoContent {
			t.Fatalf("PUT got %d, want 204; body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("GET", func(t *testing.T) {
		w := callPolicyHandler(svc.GetBucketPolicy, http.MethodGet, http.NoBody)
		if w.Code != http.StatusOK {
			t.Fatalf("GET got %d, want 200; body=%s", w.Code, w.Body.String())
		}

		if w.Body.String() != doc {
			t.Fatalf("GET body mismatch:\ngot:  %s\nwant: %s", w.Body.String(), doc)
		}
	})

	t.Run("DELETE", func(t *testing.T) {
		w := callPolicyHandler(svc.DeleteBucketPolicy, http.MethodDelete, http.NoBody)
		if w.Code != http.StatusNoContent {
			t.Fatalf("DELETE got %d, want 204; body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("GET after DELETE returns NoSuchBucketPolicy", func(t *testing.T) {
		w := callPolicyHandler(svc.GetBucketPolicy, http.MethodGet, http.NoBody)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d; body=%s", w.Code, w.Body.String())
		}

		var errResp struct {
			XMLName xml.Name `xml:"Error"`
			Code    string   `xml:"Code"`
		}

		if err := xml.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("xml unmarshal: %v\nbody=%s", err, w.Body.String())
		}

		if errResp.Code != codeNoSuchBucketPolicy {
			t.Fatalf("expected Code=NoSuchBucketPolicy, got %q", errResp.Code)
		}
	})
}

// callPolicyHandler dispatches one of the policy handlers with the
// path value already wired up. Cuts test boilerplate.
const httpPolicyBucket = "http-policy-test"

func callPolicyHandler(h http.HandlerFunc, method string, body interface{ Read(p []byte) (int, error) }) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/"+httpPolicyBucket+"?policy", body)
	req.SetPathValue("bucket", httpPolicyBucket)

	w := httptest.NewRecorder()
	h(w, req)

	return w
}

// assertBucketErrorCode unwraps to BucketError and checks Code.
func assertBucketErrorCode(t *testing.T, err error, want string) {
	t.Helper()

	var be *BucketError
	if !errors.As(err, &be) || be.Code != want {
		t.Fatalf("expected %s, got %v", want, err)
	}
}
