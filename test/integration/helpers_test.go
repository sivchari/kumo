//go:build integration

package integration

import (
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

const (
	defaultRegion   = "us-east-1"
	defaultEndpoint = "http://localhost:4566"
)

// testRegion returns the AWS region integration tests target, overridable
// via KUMO_TEST_REGION.
func testRegion() string {
	if r := os.Getenv("KUMO_TEST_REGION"); r != "" {
		return r
	}

	return defaultRegion
}

// testEndpoint returns the kumo endpoint integration tests target,
// overridable via KUMO_TEST_ENDPOINT.
func testEndpoint() string {
	if e := os.Getenv("KUMO_TEST_ENDPOINT"); e != "" {
		return e
	}

	return defaultEndpoint
}

// awsConfig builds the shared aws-sdk-go-v2 config used by every AWS
// service client constructed in these integration tests.
func awsConfig(t *testing.T) aws.Config {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion(testRegion()),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test", "test", "",
		)),
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	return cfg
}
