package server

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"
)

// cborDecMode is the CBOR decode mode configured to handle time tags.
var cborDecMode cbor.DecMode

// cborEncMode is the CBOR encode mode configured to output time tags.
var cborEncMode cbor.EncMode

func init() {
	// Configure decode mode to accept time tags (Tag 1)
	decOpts := cbor.DecOptions{
		TimeTag: cbor.DecTagOptional,
	}

	decMode, err := decOpts.DecMode()
	if err != nil {
		panic(fmt.Errorf("failed to create CBOR decode mode: %w", err))
	}

	cborDecMode = decMode

	// Configure encode mode to output time as Unix epoch with microseconds (Tag 1)
	// SDK expects float64 timestamp wrapped in CBOR Tag 1
	encOpts := cbor.EncOptions{
		Time:    cbor.TimeUnixMicro,
		TimeTag: cbor.EncTagRequired,
	}

	encMode, err := encOpts.EncMode()
	if err != nil {
		panic(fmt.Errorf("failed to create CBOR encode mode: %w", err))
	}

	cborEncMode = encMode
}

// CBORServiceHandler handles RPC v2 CBOR protocol requests for a specific service.
type CBORServiceHandler func(w http.ResponseWriter, r *http.Request, operation string)

// CBORProtocolDispatcher routes Smithy RPC v2 CBOR protocol requests to the appropriate service
// based on the URL path: /service/{serviceName}/operation/{operationName}.
type CBORProtocolDispatcher struct {
	// handlers maps service name to service handler
	// e.g., "GraniteServiceVersion20100801" -> CloudWatch handler
	handlers map[string]CBORServiceHandler
}

// NewCBORProtocolDispatcher creates a new CBOR protocol dispatcher.
func NewCBORProtocolDispatcher() *CBORProtocolDispatcher {
	return &CBORProtocolDispatcher{
		handlers: make(map[string]CBORServiceHandler),
	}
}

// Register registers a service handler for the given service name.
// The service name matches the one in the URL path:
// /service/{serviceName}/operation/{operationName}.
func (d *CBORProtocolDispatcher) Register(serviceName string, handler CBORServiceHandler) {
	d.handlers[serviceName] = handler
}

// ServeHTTP implements http.Handler and dispatches to the appropriate service.
// It handles requests to /service/{serviceName}/operation/{operationName}.
func (d *CBORProtocolDispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/service/") {
		WriteCBORError(w, "InvalidPath", "Path must start with /service/", http.StatusBadRequest)

		return
	}

	remaining := strings.TrimPrefix(path, "/service/")
	parts := strings.Split(remaining, "/operation/")

	if len(parts) != 2 {
		WriteCBORError(w, "InvalidPath", "Path must be /service/{serviceName}/operation/{operationName}", http.StatusBadRequest)

		return
	}

	serviceName := parts[0]
	operationName := parts[1]

	if serviceName == "" || operationName == "" {
		WriteCBORError(w, "InvalidPath", "Service name and operation name are required", http.StatusBadRequest)

		return
	}

	handler, ok := d.handlers[serviceName]
	if !ok {
		WriteCBORError(w, "UnknownService", "Unknown service: "+serviceName, http.StatusBadRequest)

		return
	}

	handler(w, r, operationName)
}

// DecodeCBORRequest decodes a CBOR request body into the given value.
// It uses a custom DecMode that handles CBOR time tags (Tag 1).
func DecodeCBORRequest(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	if len(body) == 0 {
		return nil
	}

	if err := cborDecMode.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to unmarshal CBOR: %w", err)
	}

	return nil
}

// WriteCBORResponse writes a CBOR response with the smithy-protocol header.
// It uses a custom EncMode that outputs time values as CBOR time tags (Tag 1).
func WriteCBORResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/cbor")
	w.Header().Set("smithy-protocol", "rpc-v2-cbor")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)

	encoded, err := cborEncMode.Marshal(v)
	if err != nil {
		// Fall back to empty response on encode error
		return
	}

	_, _ = w.Write(encoded)
}

// WriteCBORError writes an error response in CBOR format.
func WriteCBORError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/cbor")
	w.Header().Set("smithy-protocol", "rpc-v2-cbor")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)

	errorResponse := map[string]string{
		"__type":  code,
		"message": message,
	}

	encoded, err := cbor.Marshal(errorResponse)
	if err != nil {
		// If CBOR encoding fails, write nothing
		return
	}

	_, _ = w.Write(encoded)
}
