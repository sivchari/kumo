package cloudwatch

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const targetPrefix = "GraniteServiceVersion20100801"

// TestDispatchAction_PutMetricData confirms that the JSON 1.0
// dispatcher routes the X-Amz-Target header to the existing
// PutMetricData handler — the wire that aws-cli / boto3 / older AWS
// SDKs use against CloudWatch. Until this PR lands, those callers
// land on the JSON dispatcher's "Unknown service" path because
// kumo's CloudWatch only registered the Smithy CBOR route.
func TestDispatchAction_PutMetricData(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage("")
	svc := New(store)

	body := `{"Namespace":"TestApp","MetricData":[{"MetricName":"Latency","Value":42.0,"Unit":"Milliseconds"}]}`

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", targetPrefix+".PutMetricData")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PutMetricData: got %d, body=%s", w.Code, w.Body.String())
	}

	listed, err := store.ListMetrics(req.Context(), &ListMetricsRequest{Namespace: "TestApp"})
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}

	if len(listed.Metrics) != 1 || listed.Metrics[0].MetricName != "Latency" {
		t.Fatalf("metric not persisted: got %+v", listed.Metrics)
	}
}

// TestDispatchAction_UnknownTarget keeps the dispatcher honest: a
// well-formed but unimplemented action returns InvalidAction without
// crashing.
func TestDispatchAction_UnknownTarget(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage(""))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	req.Header.Set("X-Amz-Target", targetPrefix+".SomeUnknownAction")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown action, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not JSON: %v\n%s", err, w.Body.String())
	}

	if resp["__type"] != "InvalidAction" {
		t.Fatalf("expected __type=InvalidAction, got %v", resp)
	}
}

// TestTargetPrefix nails the AWS-SDK-facing constant. Boto3 / aws-cli
// derive it from the service model; we just need the string match.
func TestTargetPrefix(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage(""))
	if got := svc.TargetPrefix(); got != targetPrefix {
		t.Fatalf("TargetPrefix: got %q, want %q", got, targetPrefix)
	}
}
