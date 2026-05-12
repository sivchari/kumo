//go:build integration

package integration

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/terraform-exec/tfexec"
)

// findTofuOrTerraform returns the absolute path to a terraform-compatible
// binary on PATH, preferring tofu, or "" if none is installed.
func findTofuOrTerraform(t *testing.T) string {
	t.Helper()

	for _, name := range []string{"tofu", "terraform"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}

	return ""
}

// providerTFTemplate is a parameterized provider.tf body. Tests pick which
// service endpoints to populate so kumo only sees the call surface for the
// resources under test.
const providerTFTemplate = `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region                      = "us-east-1"
  access_key                  = "test"
  secret_key                  = "test"
  s3_use_path_style           = true
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true

  endpoints {
%s
  }
}
`

// providerTF returns a provider.tf body with the named service endpoints
// pointed at the local kumo instance on :4566.
func providerTF(services ...string) string {
	var b strings.Builder

	for _, svc := range services {
		b.WriteString("    ")
		b.WriteString(svc)

		// Pad short names so the `=` signs line up — keeps the rendered
		// HCL readable when we dump it for debugging.
		if pad := 14 - len(svc); pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}

		b.WriteString(`= "http://localhost:4566"`)
		b.WriteByte('\n')
	}

	return strings.Replace(providerTFTemplate, "%s", strings.TrimRight(b.String(), "\n"), 1)
}

// runTerraformFixture writes provider.tf + main.tf into a temp dir, runs
// init / apply, registers a destroy in t.Cleanup, then invokes verify with
// the still-open Terraform handle so the caller can assert resource state.
//
// The test is t.Skip'd when no terraform-compatible binary is on PATH so
// the suite remains a no-op for contributors that have not installed
// tofu/terraform.
func runTerraformFixture(t *testing.T, providerBody, mainBody string, verify func(t *testing.T)) {
	t.Helper()

	bin := findTofuOrTerraform(t)
	if bin == "" {
		t.Skip("no tofu or terraform binary on PATH")
	}

	workDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(workDir, "provider.tf"), []byte(providerBody), 0o600); err != nil {
		t.Fatalf("write provider.tf: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workDir, "main.tf"), []byte(mainBody), 0o600); err != nil {
		t.Fatalf("write main.tf: %v", err)
	}

	tf, err := tfexec.NewTerraform(workDir, bin)
	if err != nil {
		t.Fatalf("tfexec.NewTerraform: %v", err)
	}

	if err := tf.SetEnv(terraformEnvWithPluginCache(t)); err != nil {
		t.Fatalf("terraform env: %v", err)
	}

	ctx := t.Context()

	if err := tf.Init(ctx); err != nil {
		t.Fatalf("terraform init: %v", err)
	}

	if err := tf.Apply(ctx); err != nil {
		t.Fatalf("terraform apply: %v", err)
	}

	t.Cleanup(func() {
		if err := tf.Destroy(context.Background()); err != nil {
			t.Logf("terraform destroy (cleanup): %v", err)
		}
	})

	verify(t)
}

// terraformEnvWithPluginCache returns a per-Terraform-process environment.
// Each fixture uses its own t.TempDir as the working directory, so without a
// provider cache every tf.Init would re-download the AWS provider. The cache
// directory is shared across test runs without mutating the process env.
func terraformEnvWithPluginCache(t *testing.T) map[string]string {
	t.Helper()

	env := make(map[string]string)
	for _, kv := range os.Environ() {
		key, value, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}

		env[key] = value
	}

	if env["TF_PLUGIN_CACHE_DIR"] == "" {
		cache, err := os.UserCacheDir()
		if err == nil {
			cache = filepath.Join(cache, "kumo-tofu-plugins")
			if err := os.MkdirAll(cache, 0o750); err == nil {
				env["TF_PLUGIN_CACHE_DIR"] = cache
			}
		}
	}

	return tfexec.CleanEnv(env)
}

func TestTerraformEnvWithPluginCacheDoesNotMutateProcessEnv(t *testing.T) {
	t.Setenv("TF_PLUGIN_CACHE_DIR", "")

	env := terraformEnvWithPluginCache(t)

	if got := os.Getenv("TF_PLUGIN_CACHE_DIR"); got != "" {
		t.Fatalf("process TF_PLUGIN_CACHE_DIR = %q, want empty", got)
	}

	if env["TF_PLUGIN_CACHE_DIR"] == "" {
		t.Fatalf("terraform env TF_PLUGIN_CACHE_DIR is empty")
	}
}

const terraformS3MainTF = `
resource "aws_s3_bucket" "demo" {
  bucket = "tf-integration-demo"
}

resource "aws_s3_object" "hello" {
  bucket  = aws_s3_bucket.demo.id
  key     = "hello.txt"
  content = "hello from terraform"
}
`

// TestTerraform_S3 covers the basic CreateBucket + PutObject path through
// the AWS provider.
func TestTerraform_S3(t *testing.T) {
	t.Parallel()

	runTerraformFixture(t, providerTF("s3"), terraformS3MainTF, func(t *testing.T) {
		t.Helper()

		ctx := t.Context()
		client := newS3Client(t)

		if _, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
			Bucket: aws.String("tf-integration-demo"),
		}); err != nil {
			t.Fatalf("HeadBucket: %v", err)
		}

		assertS3ObjectBody(ctx, t, client, "tf-integration-demo", "hello.txt", "hello from terraform")
	})
}

const terraformS3MultiMainTF = `
resource "aws_s3_bucket" "logs" {
  bucket = "tf-integration-multi-logs"
}

resource "aws_s3_bucket" "data" {
  bucket = "tf-integration-multi-data"
}

resource "aws_s3_object" "config" {
  bucket       = aws_s3_bucket.data.id
  key          = "config.json"
  content      = "{\"k\":\"v\"}"
  content_type = "application/json"
}

resource "aws_s3_object" "nested" {
  bucket  = aws_s3_bucket.data.id
  key     = "nested/file.txt"
  content = "nested content"
}
`

// TestTerraform_S3_MultiResource exercises the dependency graph (two buckets,
// two objects with one referencing a sibling bucket) — the ordering catches
// regressions where a sibling resource ID is read before it is materialised.
func TestTerraform_S3_MultiResource(t *testing.T) {
	t.Parallel()

	runTerraformFixture(t, providerTF("s3"), terraformS3MultiMainTF, func(t *testing.T) {
		t.Helper()

		ctx := t.Context()
		client := newS3Client(t)

		for _, bucket := range []string{"tf-integration-multi-logs", "tf-integration-multi-data"} {
			if _, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)}); err != nil {
				t.Errorf("HeadBucket %s: %v", bucket, err)
			}
		}

		assertS3ObjectBody(ctx, t, client, "tf-integration-multi-data", "config.json", `{"k":"v"}`)
		assertS3ObjectBody(ctx, t, client, "tf-integration-multi-data", "nested/file.txt", "nested content")
	})
}

const terraformLogsMainTF = `
resource "aws_cloudwatch_log_group" "demo" {
  name              = "tf-integration-logs"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_stream" "main" {
  name           = "main"
  log_group_name = aws_cloudwatch_log_group.demo.name
}
`

// TestTerraform_CloudWatchLogs covers CreateLogGroup + CreateLogStream.
//
// Implicitly verifies the no-op tag stubs in the CloudWatch Logs service
// (#566): the AWS provider unconditionally calls ListTagsForResource on
// every CreateLogGroup, so this fixture would have failed before that PR.
func TestTerraform_CloudWatchLogs(t *testing.T) {
	t.Parallel()

	runTerraformFixture(t, providerTF("logs"), terraformLogsMainTF, func(t *testing.T) {
		t.Helper()

		ctx := t.Context()
		client := newCloudWatchLogsClient(t)

		groups, err := client.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
			LogGroupNamePrefix: aws.String("tf-integration-logs"),
		})
		if err != nil {
			t.Fatalf("DescribeLogGroups: %v", err)
		}

		if len(groups.LogGroups) != 1 {
			t.Fatalf("expected 1 log group, got %d", len(groups.LogGroups))
		}

		streams, err := client.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String("tf-integration-logs"),
		})
		if err != nil {
			t.Fatalf("DescribeLogStreams: %v", err)
		}

		if len(streams.LogStreams) != 1 {
			t.Fatalf("expected 1 log stream, got %d", len(streams.LogStreams))
		}
	})
}

const terraformVPCMainTF = `
resource "aws_vpc" "demo" {
  cidr_block = "10.42.0.0/16"
  tags = {
    Name = "tf-integration-vpc"
  }
}
`

// TestTerraform_EC2_VPC covers CreateVpc + the subsequent
// DescribeVpcAttribute / DescribeVpcs the provider uses to populate state.
func TestTerraform_EC2_VPC(t *testing.T) {
	t.Parallel()

	runTerraformFixture(t, providerTF("ec2"), terraformVPCMainTF, func(t *testing.T) {
		t.Helper()

		ctx := t.Context()
		client := newEC2Client(t)

		out, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
		if err != nil {
			t.Fatalf("DescribeVpcs: %v", err)
		}

		var found bool

		for i := range out.Vpcs {
			if aws.ToString(out.Vpcs[i].CidrBlock) == "10.42.0.0/16" {
				found = true

				break
			}
		}

		if !found {
			t.Fatalf("VPC with CIDR 10.42.0.0/16 not found in DescribeVpcs response")
		}
	})
}

// assertS3ObjectBody reads the named object and fails the test if its
// body does not equal want.
func assertS3ObjectBody(ctx context.Context, t *testing.T, client *s3.Client, bucket, key, want string) {
	t.Helper()

	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject %s/%s: %v", bucket, key, err)
	}

	defer func() { _ = out.Body.Close() }()

	got, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatalf("read body %s/%s: %v", bucket, key, err)
	}

	if string(got) != want {
		t.Fatalf("body %s/%s = %q, want %q", bucket, key, string(got), want)
	}
}
