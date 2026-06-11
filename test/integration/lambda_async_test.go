//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// TestLambda_AsyncInvokeRetriesUntilEndpointUp is the regression test for
// issue #803: Invoke with InvocationType Event must queue the event and
// retry delivery, so events sent while the InvokeEndpoint is down are
// delivered exactly once, in order, after the endpoint comes up.
func TestLambda_AsyncInvokeRetriesUntilEndpointUp(t *testing.T) {
	client := newLambdaClient(t)
	ctx := t.Context()
	functionName := "test-function-async-retry"

	// Reserve a port for the InvokeEndpoint, then close the listener so the
	// endpoint refuses connections at invoke time.
	var lc net.ListenConfig

	lis, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	endpointAddr := lis.Addr().String()

	if err := lis.Close(); err != nil {
		t.Fatal(err)
	}

	// Create function with InvokeEndpoint using raw HTTP request (the field
	// is a kumo extension not present in the SDK input type).
	createReq := map[string]any{
		"FunctionName":   functionName,
		"Runtime":        "python3.12",
		"Role":           "arn:aws:iam::000000000000:role/test-role",
		"Handler":        "index.handler",
		"InvokeEndpoint": "http://" + endpointAddr + "/invoke",
		"Code": map[string]any{
			"ZipFile": []byte("fake-zip-content"),
		},
	}
	createBody, _ := json.Marshal(createReq)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://localhost:4566/lambda/2015-03-31/functions", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to create function: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteFunction(context.Background(), &lambda.DeleteFunctionInput{
			FunctionName: aws.String(functionName),
		})
	})

	// Async invokes while the endpoint is down must still return 202.
	for i := 1; i <= 3; i++ {
		out, err := client.Invoke(ctx, &lambda.InvokeInput{
			FunctionName:   aws.String(functionName),
			InvocationType: types.InvocationTypeEvent,
			Payload:        fmt.Appendf(nil, `{"seq":%d}`, i),
		})
		if err != nil {
			t.Fatalf("async invoke %d failed: %v", i, err)
		}

		if out.StatusCode != http.StatusAccepted {
			t.Fatalf("async invoke %d: expected 202, got %d", i, out.StatusCode)
		}
	}

	// Bring the endpoint up and collect deliveries.
	var (
		mu       sync.Mutex
		payloads []string
	)

	lis, err = lc.Listen(ctx, "tcp", endpointAddr)
	if err != nil {
		t.Fatalf("rebind %s: %v", endpointAddr, err)
	}

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)

			mu.Lock()
			payloads = append(payloads, string(body))
			mu.Unlock()

			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: time.Second,
	}
	go func() { _ = srv.Serve(lis) }()

	t.Cleanup(func() { _ = srv.Close() })

	snapshot := func() []string {
		mu.Lock()
		defer mu.Unlock()

		return append([]string(nil), payloads...)
	}

	// kumo retries with exponential backoff (capped at 5s); all three events
	// must arrive once the endpoint is reachable.
	deadline := time.Now().Add(20 * time.Second)
	for len(snapshot()) < 3 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}

	// Allow a settle period to catch duplicate deliveries.
	time.Sleep(500 * time.Millisecond)

	got := snapshot()
	want := []string{`{"seq":1}`, `{"seq":2}`, `{"seq":3}`}

	if len(got) != len(want) {
		t.Fatalf("expected exactly %d deliveries, got %d: %v", len(want), len(got), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("delivery %d = %s, want %s (per-function order must be preserved)", i, got[i], want[i])
		}
	}
}
