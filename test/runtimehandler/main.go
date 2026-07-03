// Command runtimehandler is a real AWS Lambda handler (lambda.Start) used by
// the integration tests to verify that an unmodified lambda.Start binary runs
// against kumo's Runtime API via AWS_LAMBDA_RUNTIME_API, with no external RIE.
package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
)

func handler(_ context.Context, event map[string]any) (map[string]any, error) {
	return map[string]any{
		"handled": true,
		"echo":    event,
	}, nil
}

func main() {
	lambda.Start(handler)
}
