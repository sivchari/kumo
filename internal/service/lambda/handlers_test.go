package lambda

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractEventSourceMappingUUIDWithEndpointPrefixes(t *testing.T) {
	t.Parallel()

	const mappingUUID = "12345678-1234-1234-1234-123456789012"

	for name, path := range map[string]string{
		"SDK BaseEndpoint":       "/lambda/2015-03-31/event-source-mappings/" + mappingUUID,
		"terraform AWS provider": "/2015-03-31/event-source-mappings/" + mappingUUID,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := extractEventSourceMappingUUID(path); got != mappingUUID {
				t.Errorf("extractEventSourceMappingUUID(%q) = %q, want %q", path, got, mappingUUID)
			}
		})
	}
}

// TestInvoke_ByARNReachesRuntimeHandler verifies that Invoke with the
// function's ARN reaches a handler polling the Runtime API under the
// function's name.
func TestInvoke_ByARNReachesRuntimeHandler(t *testing.T) {
	t.Parallel()

	svc := newInvokeTestService(t, "fn", "")

	fn, err := svc.storage.GetFunction(t.Context(), "fn")
	if err != nil {
		t.Fatalf("get function: %v", err)
	}

	runHandler(t, svc.broker, "fn", func(*runtimeInvocation) ([]byte, bool) {
		return []byte(`{"handled":true}`), false
	})

	if !waitFor(t, 5*time.Second, func() bool { return svc.broker.registered("fn") }) {
		t.Fatal("handler never registered by polling next")
	}

	rec := httptest.NewRecorder()
	svc.Invoke(rec, invokeRequest(t, fn.FunctionArn, ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("invoke by ARN status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if got := rec.Body.String(); got != `{"handled":true}` {
		t.Errorf("invoke by ARN payload = %s, want the handler's response", got)
	}
}
