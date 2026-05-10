//go:build integration

package integration

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
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

const terraformProviderTF = `
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
    s3 = "http://localhost:4566"
  }
}
`

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

// TestTerraform_S3 drives the AWS provider against kumo via terraform-exec
// to confirm that PUT bucket / PUT object via the provider's code path
// resolves the same shapes kumo's direct-SDK tests already cover.
//
// Skipped when no terraform-compatible binary is on PATH, so this test
// is a no-op in environments that have not installed tofu/terraform.
func TestTerraform_S3(t *testing.T) {
	bin := findTofuOrTerraform(t)
	if bin == "" {
		t.Skip("no tofu or terraform binary on PATH")
	}

	workDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(workDir, "provider.tf"), []byte(terraformProviderTF), 0o600); err != nil {
		t.Fatalf("write provider.tf: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workDir, "main.tf"), []byte(terraformS3MainTF), 0o600); err != nil {
		t.Fatalf("write main.tf: %v", err)
	}

	tf, err := tfexec.NewTerraform(workDir, bin)
	if err != nil {
		t.Fatalf("tfexec.NewTerraform: %v", err)
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

	client := newS3Client(t)

	headOut, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String("tf-integration-demo"),
	})
	if err != nil {
		t.Fatalf("HeadBucket after terraform apply: %v", err)
	}

	_ = headOut

	getOut, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String("tf-integration-demo"),
		Key:    aws.String("hello.txt"),
	})
	if err != nil {
		t.Fatalf("GetObject after terraform apply: %v", err)
	}

	defer func() { _ = getOut.Body.Close() }()

	body, err := io.ReadAll(getOut.Body)
	if err != nil {
		t.Fatalf("read object body: %v", err)
	}

	if got, want := string(body), "hello from terraform"; got != want {
		t.Fatalf("object body = %q, want %q", got, want)
	}
}
