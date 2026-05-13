package server

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/chaos"
)

func writeChaosError(w http.ResponseWriter, info *requestInfo, spec chaos.FaultErrorSpec) {
	if spec.RetryAfterSeconds > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(spec.RetryAfterSeconds))
	}

	switch {
	case info.Protocol == "cbor":
		WriteCBORError(w, spec.Code, spec.Message, spec.Status)
	case info.Service == "s3":
		writeChaosXMLError(w, spec)
	default:
		writeChaosJSONError(w, spec)
	}
}

func writeChaosJSONError(w http.ResponseWriter, spec chaos.FaultErrorSpec) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(spec.Status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"__type":  spec.Code,
		"message": spec.Message,
	})
}

func writeChaosXMLError(w http.ResponseWriter, spec chaos.FaultErrorSpec) {
	errResp := struct {
		XMLName   xml.Name `xml:"Error"`
		Code      string   `xml:"Code"`
		Message   string   `xml:"Message"`
		RequestID string   `xml:"RequestId"`
	}{
		Code:      spec.Code,
		Message:   spec.Message,
		RequestID: uuid.New().String(),
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(spec.Status)
	_, _ = io.WriteString(w, xml.Header)
	_ = xml.NewEncoder(w).Encode(errResp)
}
