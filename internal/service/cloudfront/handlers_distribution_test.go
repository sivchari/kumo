package cloudfront

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func distributionRequestBody(t *testing.T, callerRef string) string {
	t.Helper()

	raw, err := xml.Marshal(&CreateDistributionRequest{
		CallerReference: callerRef,
		Enabled:         true,
		Origins: &OriginsXML{
			Quantity: 1,
			Items:    &OriginList{Origin: []OriginXML{{ID: "o", DomainName: oacTestPlainDomain}}},
		},
		DefaultCacheBehavior: &DefaultCacheBehaviorXML{TargetOriginID: "o"},
	})
	if err != nil {
		t.Fatalf("marshal distribution request: %v", err)
	}

	return string(raw)
}

func TestCreateDistribution_DuplicateCallerReferenceConflicts(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	create := func(callerRef string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		svc.CreateDistribution(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/2020-05-31/distribution", strings.NewReader(distributionRequestBody(t, callerRef))))

		return w
	}

	if w := create("dup-ref"); w.Code != http.StatusCreated {
		t.Fatalf("first CreateDistribution: status %d body=%s", w.Code, w.Body.String())
	}

	w := create("dup-ref")
	if w.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want 409 (DistributionAlreadyExists) body=%s", w.Code, w.Body.String())
	}

	var resp ErrorResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}

	if resp.Error.Code != errDistributionAlreadyExists {
		t.Errorf("code: got %q, want %q", resp.Error.Code, errDistributionAlreadyExists)
	}

	if w := create("another-ref"); w.Code != http.StatusCreated {
		t.Fatalf("new CallerReference must create a new distribution: status %d body=%s", w.Code, w.Body.String())
	}
}
