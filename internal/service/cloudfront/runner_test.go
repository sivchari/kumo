package cloudfront

import (
	"strings"
	"testing"
)

func TestRunFunction_HandlerMutatesRequest(t *testing.T) {
	code := []byte(`
		function handler(event) {
			console.log("touched " + event.request.uri);
			event.request.headers["x-from-kumo"] = { value: "hello" };
			return event.request;
		}
	`)
	event := `{"request":{"method":"GET","uri":"/x","headers":{}}}`

	got := runFunction(code, event)
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}

	if !strings.Contains(got.Output, `"x-from-kumo"`) || !strings.Contains(got.Output, `"hello"`) {
		t.Errorf("Output missing injected header: %s", got.Output)
	}

	if len(got.LogLines) != 1 || !strings.HasPrefix(got.LogLines[0], "touched ") {
		t.Errorf("LogLines = %v", got.LogLines)
	}
}

func TestRunFunction_MissingHandlerReturnsError(t *testing.T) {
	got := runFunction([]byte(`var x = 1;`), `{}`)
	if got.Error == "" {
		t.Fatalf("expected error when handler is missing")
	}
}

func TestRunFunction_SyntaxErrorIsReported(t *testing.T) {
	got := runFunction([]byte(`function handler(event) {`), `{}`)
	if got.Error == "" {
		t.Fatalf("expected syntax error")
	}
}
