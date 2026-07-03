//go:build integration

package integration

import (
	"context"
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
const lambdaRuntimeAPIHost = "localhost:4566"

// TestLambdaRuntime_Invoke proves that an unmodified lambda.Start binary runs
// against kumo via the Runtime API (no external RIE): the binary is started
// with AWS_LAMBDA_RUNTIME_API pointing at kumo, and a normal client.Invoke is
// served by that handler.
func TestLambdaRuntime_Invoke(t *testing.T) {
	client := newLambdaClient(t)
	ctx := t.Context()
	functionName := "runtime-api-fn"

	// Build the real lambda.Start handler (test module root is the parent dir).
	bin := filepath.Join(t.TempDir(), "runtimehandler")

	build := exec.CommandContext(ctx, "go", "build", "-o", bin, "./runtimehandler")
	build.Dir = ".."

	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build handler: %v\n%s", err, out)
	}

	// Create the function in kumo (no InvokeEndpoint: it is served by the
	// Runtime API handler started below).
	if _, err := client.CreateFunction(ctx, &lambda.CreateFunctionInput{
		FunctionName: aws.String(functionName),
		Runtime:      types.RuntimeProvidedal2,
		Role:         aws.String("arn:aws:iam::000000000000:role/test-role"),
		Handler:      aws.String("bootstrap"),
		Code:         &types.FunctionCode{ZipFile: []byte("fake")},
	}); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteFunction(context.Background(), &lambda.DeleteFunctionInput{
			FunctionName: aws.String(functionName),
		})
	})

	// Start the handler pointed at kumo's Runtime API. lambda.Start polls
	// .../_runtime/{functionName}/2018-06-01/runtime/invocation/next.
	handler := exec.CommandContext(ctx, bin)
	handler.Env = append(os.Environ(),
		"AWS_LAMBDA_RUNTIME_API="+lambdaRuntimeAPIHost+"/_runtime/"+functionName,
	)

	if err := handler.Start(); err != nil {
		t.Fatalf("start handler: %v", err)
	}

	t.Cleanup(func() { _ = handler.Process.Kill() })

	// The handler registers by polling next; retry until it is serving.
	var payload string

	deadline := time.Now().Add(15 * time.Second)

	for {
		out, err := client.Invoke(ctx, &lambda.InvokeInput{
			FunctionName: aws.String(functionName),
			Payload:      []byte(`{"key":"value"}`),
		})
		if err == nil && out.FunctionError == nil {
			payload = string(out.Payload)

			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("invoke never succeeded via runtime handler: err=%v", err)
		}

		time.Sleep(200 * time.Millisecond)
	}

	if !strings.Contains(payload, `"handled":true`) {
		t.Errorf("unexpected handler response: %s", payload)
	}

	if !strings.Contains(payload, `"key":"value"`) {
		t.Errorf("handler did not receive the event payload: %s", payload)
	}
}
