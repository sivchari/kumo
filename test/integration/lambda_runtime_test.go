//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// lambdaRuntimeAPIHost is the host:port a handler uses to reach kumo's
// Runtime API (AWS_LAMBDA_RUNTIME_API=<host>/_runtime/{functionName}).
var lambdaRuntimeAPIHost = strings.TrimPrefix(testEndpoint(), "http://")

// startRuntimeFunction creates functionName in kumo with the given Timeout
// (seconds; 0 leaves Lambda's default) and no InvokeEndpoint, then starts the
// real lambda.Start handler (test/runtimehandler) pointed at kumo's Runtime
// API to serve it.
func startRuntimeFunction(t *testing.T, client *lambda.Client, functionName string, timeoutSeconds int32) {
	t.Helper()

	ctx := t.Context()

	// Build the real lambda.Start handler (test module root is the parent dir).
	bin := filepath.Join(t.TempDir(), "runtimehandler")

	build := exec.CommandContext(ctx, "go", "build", "-o", bin, "./runtimehandler")
	build.Dir = ".."

	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build handler: %v\n%s", err, out)
	}

	input := &lambda.CreateFunctionInput{
		FunctionName: aws.String(functionName),
		Runtime:      types.RuntimeProvidedal2,
		Role:         aws.String("arn:aws:iam::000000000000:role/test-role"),
		Handler:      aws.String("bootstrap"),
		Code:         &types.FunctionCode{ZipFile: []byte("fake")},
	}
	if timeoutSeconds > 0 {
		input.Timeout = aws.Int32(timeoutSeconds)
	}

	if _, err := client.CreateFunction(ctx, input); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteFunction(context.Background(), &lambda.DeleteFunctionInput{
			FunctionName: aws.String(functionName),
		})
	})

	// lambda.Start polls .../_runtime/{functionName}/2018-06-01/runtime/invocation/next.
	handler := exec.CommandContext(ctx, bin)
	handler.Env = append(os.Environ(),
		"AWS_LAMBDA_RUNTIME_API="+lambdaRuntimeAPIHost+"/_runtime/"+functionName,
	)

	if err := handler.Start(); err != nil {
		t.Fatalf("start handler: %v", err)
	}

	t.Cleanup(func() { _ = handler.Process.Kill() })
}

// invokeUntilServed invokes functionName until the Runtime API handler is
// serving it (the handler registers by polling next) and returns the payload.
func invokeUntilServed(t *testing.T, client *lambda.Client, functionName string) string {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)

	for {
		out, err := client.Invoke(t.Context(), &lambda.InvokeInput{
			FunctionName: aws.String(functionName),
			Payload:      []byte(`{"key":"value"}`),
		})
		if err == nil && out.FunctionError == nil {
			return string(out.Payload)
		}

		if time.Now().After(deadline) {
			t.Fatalf("invoke never succeeded via runtime handler: err=%v", err)
		}

		time.Sleep(200 * time.Millisecond)
	}
}

// TestLambdaRuntime_Invoke proves that an unmodified lambda.Start binary runs
// against kumo via the Runtime API (no external RIE): the binary is started
// with AWS_LAMBDA_RUNTIME_API pointing at kumo, and a normal client.Invoke is
// served by that handler.
func TestLambdaRuntime_Invoke(t *testing.T) {
	client := newLambdaClient(t)
	functionName := "runtime-api-fn"

	startRuntimeFunction(t, client, functionName, 0)

	payload := invokeUntilServed(t, client, functionName)

	if !strings.Contains(payload, `"handled":true`) {
		t.Errorf("unexpected handler response: %s", payload)
	}

	if !strings.Contains(payload, `"key":"value"`) {
		t.Errorf("handler did not receive the event payload: %s", payload)
	}
}

// TestLambdaRuntime_DeadlineFollowsFunctionTimeout verifies that the deadline
// lambda.Start sees (ctx.Deadline, from Lambda-Runtime-Deadline-Ms) is the
// function's configured Timeout rather than a fixed kumo-side value.
func TestLambdaRuntime_DeadlineFollowsFunctionTimeout(t *testing.T) {
	client := newLambdaClient(t)
	functionName := "runtime-api-deadline-fn"

	startRuntimeFunction(t, client, functionName, 120)

	payload := invokeUntilServed(t, client, functionName)

	var resp struct {
		RemainingMs int64 `json:"remainingMs"`
	}
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("decode handler response %s: %v", payload, err)
	}

	remaining := time.Duration(resp.RemainingMs) * time.Millisecond
	if remaining < 110*time.Second || remaining > 120*time.Second {
		t.Errorf("handler saw %v remaining, want ~120s (the function's Timeout)", remaining)
	}
}
