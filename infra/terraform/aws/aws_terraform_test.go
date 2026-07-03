package aws_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKumoAwsTerraformDefinesFargateServiceWithPersistentData(t *testing.T) {
	main := readTerraformFile(t, "main.tf")
	variables := readTerraformFile(t, "variables.tf")
	outputs := readTerraformFile(t, "outputs.tf")
	readme := readTerraformFile(t, "README.md")

	assertContains(t, main, `resource "aws_ecs_cluster" "this"`)
	assertContains(t, main, `resource "aws_ecs_task_definition" "kumo"`)
	assertContains(t, main, `resource "aws_ecs_service" "kumo"`)
	assertContains(t, main, `resource "aws_lb_target_group" "kumo"`)
	assertContains(t, main, `"/health"`)
	assertContains(t, main, `KUMO_DATA_DIR`)
	assertContains(t, main, `containerPath = var.data_dir`)
	assertContains(t, main, `resource "aws_efs_file_system" "data"`)
	assertContains(t, main, `count          = var.enable_efs ? 1 : 0`)
	assertContains(t, main, `resource "aws_efs_access_point" "data"`)
	assertContains(t, main, `uid = 10000`)
	assertContains(t, main, `gid = 10000`)
	assertContains(t, main, `readonlyRootFilesystem = true`)
	assertContains(t, main, `"SYS_PTRACE"`)
	assertContains(t, main, `count      = var.attach_task_execution_policy ? 1 : 0`)
	assertContains(t, main, `dynamic "volume"`)
	assertContains(t, main, `mountPoints = var.enable_efs ?`)

	assertContains(t, variables, `variable "image"`)
	assertContains(t, variables, `default     = "ghcr.io/sivchari/kumo:latest"`)
	assertContains(t, variables, `variable "alb_internal"`)
	assertContains(t, variables, `default     = true`)
	assertContains(t, variables, `variable "allowed_cidr_blocks"`)
	assertContains(t, variables, `variable "enable_pprof"`)
	assertContains(t, variables, `variable "enable_efs"`)
	assertContains(t, variables, `variable "attach_task_execution_policy"`)
	assertContains(t, variables, `default     = "/data"`)

	assertContains(t, outputs, `output "endpoint_url"`)
	assertContains(t, outputs, `aws_lb.this.dns_name`)
	assertContains(t, outputs, `try(aws_efs_file_system.data[0].id, null)`)

	assertContains(t, readme, "sivchari/kumo")
	assertContains(t, readme, "ECS/Fargate")
	assertContains(t, readme, "EFS")
	assertContains(t, readme, "KUMO_DATA_DIR=/data")
	assertContains(t, readme, "enable_efs=false")
}

func readTerraformFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(".", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func assertContains(t *testing.T, text string, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("expected file to contain %q", want)
	}
}
