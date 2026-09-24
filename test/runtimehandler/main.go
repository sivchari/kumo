// Command runtimehandler is a real AWS Lambda handler (lambda.Start) used by
// the integration tests to verify that an unmodified lambda.Start binary runs
// against kumo's Runtime API via AWS_LAMBDA_RUNTIME_API, with no external RIE.
package main

import (
	"context"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, event map[string]any) (map[string]any, error) {
	resp := map[string]any{
		"handled": true,
		"echo":    event,
	}

	// lambda.Start derives ctx's deadline from Lambda-Runtime-Deadline-Ms.
	if deadline, ok := ctx.Deadline(); ok {
		resp["remainingMs"] = time.Until(deadline).Milliseconds()
	}

	return resp, nil
}

func main() {
	lambda.Start(handler)
}
