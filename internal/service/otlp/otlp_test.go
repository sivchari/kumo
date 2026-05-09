package otlp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestIngest_RoundTrip — POST a JSON payload to /v1/traces, then read
// it back via /kumo/otlp/traces. Asserts the body lands verbatim and
// the OTLP success shape comes back. This is the primary contract for
// integration tests asserting "my app emitted these spans".
func TestIngest_RoundTrip(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store)

	const body = `{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"test-app"}}]},"scopeSpans":[{"spans":[{"name":"GET /healthz","kind":1}]}]}]}`

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	svc.PutTraces(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PutTraces: got %d, body=%s", w.Code, w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "partialSuccess") {
		t.Fatalf("response missing partialSuccess shape: %s", w.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/kumo/otlp/traces", http.NoBody)
	listReq.SetPathValue("signal", "traces")

	listRec := httptest.NewRecorder()
	svc.ListReceived(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("ListReceived: got %d", listRec.Code)
	}

	var listResp struct {
		Signal   string    `json:"signal"`
		Count    int       `json:"count"`
		Payloads []Payload `json:"payloads"`
	}

	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("readback unmarshal: %v\n%s", err, listRec.Body.String())
	}

	if listResp.Count != 1 {
		t.Fatalf("expected 1 payload, got %d", listResp.Count)
	}

	if listResp.Payloads[0].Body != body {
		t.Fatalf("body not stored verbatim:\ngot:  %s\nwant: %s",
			listResp.Payloads[0].Body, body)
	}
}

// TestIngest_AllSignals exercises each of the three OTLP signal
// endpoints and confirms readback partitions correctly by signal.
func TestIngest_AllSignals(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store)

	cases := []struct {
		signal  Signal
		handler http.HandlerFunc
		path    string
		body    string
	}{
		{SignalTraces, svc.PutTraces, "/v1/traces", `{"resourceSpans":[]}`},
		{SignalMetrics, svc.PutMetrics, "/v1/metrics", `{"resourceMetrics":[]}`},
		{SignalLogs, svc.PutLogs, "/v1/logs", `{"resourceLogs":[]}`},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		w := httptest.NewRecorder()

		tc.handler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("%s ingest: got %d", tc.signal, w.Code)
		}
	}

	for _, tc := range cases {
		got := store.List(t.Context(), tc.signal)
		if len(got) != 1 {
			t.Fatalf("%s: expected 1 payload, got %d", tc.signal, len(got))
		}

		if got[0].Body != tc.body {
			t.Fatalf("%s: body mismatch", tc.signal)
		}
	}
}

// TestReset clears all stored payloads. Tests use this between runs
// so the readback contract is independent of test ordering.
func TestReset(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store)

	// Seed one of each.
	for _, h := range []http.HandlerFunc{svc.PutTraces, svc.PutMetrics, svc.PutLogs} {
		req := httptest.NewRequest(http.MethodPost, "/v1/x", strings.NewReader("{}"))
		h(httptest.NewRecorder(), req)
	}

	if total := len(store.Payloads); total != 3 {
		t.Fatalf("seed: expected 3 payloads, got %d", total)
	}

	resetReq := httptest.NewRequest(http.MethodDelete, "/kumo/otlp", http.NoBody)
	resetRec := httptest.NewRecorder()

	svc.ResetReceived(resetRec, resetReq)

	if resetRec.Code != http.StatusNoContent {
		t.Fatalf("Reset: got %d", resetRec.Code)
	}

	if total := len(store.Payloads); total != 0 {
		t.Fatalf("after reset: expected 0 payloads, got %d", total)
	}
}

// TestListReceived_InvalidSignal — the readback rejects bogus signal
// names with 400 rather than returning empty / 404. terraform-ish
// error shape for tests.
func TestListReceived_InvalidSignal(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	req := httptest.NewRequest(http.MethodGet, "/kumo/otlp/bogus", http.NoBody)
	req.SetPathValue("signal", "bogus")

	w := httptest.NewRecorder()
	svc.ListReceived(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bogus signal, got %d", w.Code)
	}
}
