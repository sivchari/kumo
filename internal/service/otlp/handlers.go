package otlp

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// PutTraces handles POST /v1/traces — the OTel SDK's trace exporter
// target. We accept the body verbatim so tests can introspect the
// exact JSON the SDK serialised.
func (s *Service) PutTraces(w http.ResponseWriter, r *http.Request) {
	s.ingest(w, r, SignalTraces)
}

// PutMetrics handles POST /v1/metrics.
func (s *Service) PutMetrics(w http.ResponseWriter, r *http.Request) {
	s.ingest(w, r, SignalMetrics)
}

// PutLogs handles POST /v1/logs.
func (s *Service) PutLogs(w http.ResponseWriter, r *http.Request) {
	s.ingest(w, r, SignalLogs)
}

// ingest reads the request body, stores it, and replies with the
// OTLP success shape: an empty `partial_success` object. OTel SDKs
// treat 200 + that shape as "fully accepted".
func (s *Service) ingest(w http.ResponseWriter, r *http.Request, signal Signal) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeOTLPError(w, "InvalidRequest", "failed to read body", http.StatusBadRequest)

		return
	}

	if err := s.storage.Append(r.Context(), Payload{
		Signal:      signal,
		ReceivedAt:  time.Now().UTC(),
		ContentType: r.Header.Get("Content-Type"),
		Body:        string(body),
	}); err != nil {
		writeOTLPError(w, "InternalError", err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"partialSuccess":{}}`))
}

// ListReceived handles GET /kumo/otlp/{signal} — the kumo-private
// readback for tests. Real OTel backends don't expose this endpoint;
// it's how integration tests assert "did my app emit the spans I
// expected?" without re-implementing OTel storage.
func (s *Service) ListReceived(w http.ResponseWriter, r *http.Request) {
	signal := Signal(r.PathValue("signal"))
	switch signal {
	case SignalTraces, SignalMetrics, SignalLogs:
	default:
		writeOTLPError(w, "InvalidPath", "signal must be traces / metrics / logs", http.StatusBadRequest)

		return
	}

	payloads := s.storage.List(r.Context(), signal)

	w.Header().Set("Content-Type", "application/json")

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	_ = enc.Encode(map[string]any{
		"signal":   signal,
		"count":    len(payloads),
		"payloads": payloads,
	})
}

// ResetReceived handles DELETE /kumo/otlp — clears the buffer between
// integration test runs.
func (s *Service) ResetReceived(w http.ResponseWriter, r *http.Request) {
	s.storage.Reset(r.Context())

	w.WriteHeader(http.StatusNoContent)
}

// writeOTLPError mirrors the OTLP error shape: a flat JSON object with
// `code` + `message`. OTel SDKs only really inspect the HTTP status
// code, but a well-formed body keeps the wire honest for test asserts.
func writeOTLPError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"code":"` + code + `","message":"` + message + `"}`))
}
