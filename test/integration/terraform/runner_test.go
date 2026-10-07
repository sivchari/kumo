//go:build integration

// Package terraform_test drives every fixture under fixtures/ through the
// AWS provider against a local kumo instance. Adding a fixture is adding a
// directory; this file does not need to change.
package terraform_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-exec/tfexec"
)

// defaultAWSEndpointURL is the kumo endpoint used when KUMO_TEST_ENDPOINT is
// unset.
const defaultAWSEndpointURL = "http://localhost:4566"

// awsEndpointURL returns the single kumo endpoint every fixture's AWS
// provider is pointed at via the AWS_ENDPOINT_URL env var, which the provider
// resolves for every service without a per-service endpoints block. Like the
// SDK integration tests, it is overridable via KUMO_TEST_ENDPOINT.
func awsEndpointURL() string {
	if e := os.Getenv("KUMO_TEST_ENDPOINT"); e != "" {
		return e
	}

	return defaultAWSEndpointURL
}

// fixturesDir holds one directory per fixture, discovered at test time.
const fixturesDir = "fixtures"

// expectedOutputsFile is the optional per-fixture file mapping terraform
// output names to their expected string values.
const expectedOutputsFile = "expected_outputs.json"

// stepsFile is the optional per-fixture manifest that turns a fixture into
// an ordered list of configs applied one after another to the same state.
const stepsFile = "steps.json"

// fixtureStep is one entry of stepsFile. Dir is a directory inside the
// fixture holding the step's full config, which replaces the previous step's
// config. Replace lists resource addresses passed as -replace. ExpectError,
// when set, is a regexp the apply error must match, and the step fails if
// apply succeeds instead.
type fixtureStep struct {
	Name        string   `json:"name"`
	Dir         string   `json:"dir"`
	Replace     []string `json:"replace"`
	ExpectError string   `json:"expect_error"`
}

// providerVersionEnv names the env var overriding the AWS provider version
// constraint written into every fixture's provider.tf.
const providerVersionEnv = "KUMO_TF_AWS_PROVIDER_VERSION"

// defaultAWSProviderVersion is the AWS provider version constraint used when
// KUMO_TF_AWS_PROVIDER_VERSION is unset.
const defaultAWSProviderVersion = "~> 5.0"

// providerTFTemplate is the provider.tf body generated for every fixture,
// with the AWS provider version constraint left to providerTF. It carries
// no per-service endpoints — see awsEndpointURL. Every provider any fixture
// uses must be listed here: warmPluginCache's dependency lock file only
// covers providers recorded in this template, and an unlisted provider is
// re-installed into the shared plugin cache by each fixture's init, which
// races with other fixtures executing that binary (ETXTBSY).
const providerTFTemplate = `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = %q
    }
    archive = {
      source  = "hashicorp/archive"
      version = "~> 2.0"
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
}
`

// providerTF renders the provider.tf body, taking the AWS provider version
// constraint from KUMO_TF_AWS_PROVIDER_VERSION when set.
func providerTF() string {
	version := os.Getenv(providerVersionEnv)
	if version == "" {
		version = defaultAWSProviderVersion
	}

	return fmt.Sprintf(providerTFTemplate, version)
}

// initMu serializes `terraform init`. terraform's plugin cache directory
// (see terraformEnvWithPluginCache) is documented as unsafe for concurrent
// installer access, so every fixture's init is serialized even though the
// fixtures otherwise apply/plan/destroy in parallel.
var initMu sync.Mutex

// TestTerraformFixtures discovers every directory under fixtures/ and runs
// it through init -> apply -> plan (idempotency check) -> destroy, applying
// each step in turn for fixtures with a stepsFile.
func TestTerraformFixtures(t *testing.T) {
	bin := resolveTFBinary(t)
	if bin == "" {
		t.Skip("no tofu or terraform binary on PATH (set KUMO_TF_BIN to override)")
	}

	lockFile := warmPluginCache(t, bin)

	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		srcDir := filepath.Join(fixturesDir, name)

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runFixture(t, bin, srcDir, lockFile)
		})
	}
}

// resolveTFBinary returns the terraform-compatible binary to drive the
// fixtures with. KUMO_TF_BIN (a bare name or an absolute path) takes
// precedence; otherwise PATH is searched, preferring tofu over terraform.
func resolveTFBinary(t *testing.T) string {
	t.Helper()

	if override := os.Getenv("KUMO_TF_BIN"); override != "" {
		if filepath.IsAbs(override) {
			return override
		}

		p, err := exec.LookPath(override)
		if err != nil {
			t.Fatalf("KUMO_TF_BIN=%q not found on PATH: %v", override, err)
		}

		return p
	}

	for _, name := range []string{"tofu", "terraform"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}

	return ""
}

// warmPluginCache runs a throwaway `terraform init` before the fixtures run
// in parallel and returns the generated dependency lock file. Terraform only
// trusts the plugin cache for providers already recorded in
// .terraform.lock.hcl; without a lock entry every init re-installs the
// provider into the cache, which fails with ETXTBSY while another fixture's
// provider process is executing that binary. Seeding each fixture with this
// lock file makes init link straight from the warmed cache.
func warmPluginCache(t *testing.T, bin string) []byte {
	t.Helper()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "provider.tf"), []byte(providerTF()), 0o600); err != nil {
		t.Fatalf("write provider.tf: %v", err)
	}

	tf, err := tfexec.NewTerraform(dir, bin)
	if err != nil {
		t.Fatalf("tfexec.NewTerraform: %v", err)
	}

	if err := tf.SetEnv(fixtureEnv(t)); err != nil {
		t.Fatalf("terraform env: %v", err)
	}

	if err := tf.Init(t.Context()); err != nil {
		t.Fatalf("warm up terraform init: %v", err)
	}

	lockFile, err := os.ReadFile(filepath.Join(dir, ".terraform.lock.hcl"))
	if err != nil {
		t.Fatalf("read warmed dependency lock file: %v", err)
	}

	return lockFile
}

// runFixture writes provider.tf and the warmed dependency lock file into a
// scratch dir, then drives the fixture (or each of its steps) through
// init -> apply -> plan (idempotency check), and destroys what is left in
// the state on cleanup.
func runFixture(t *testing.T, bin, srcDir string, lockFile []byte) {
	t.Helper()

	steps := readSteps(t, srcDir)
	workDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(workDir, "provider.tf"), []byte(providerTF()), 0o600); err != nil {
		t.Fatalf("write provider.tf: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workDir, ".terraform.lock.hcl"), lockFile, 0o600); err != nil {
		t.Fatalf("write dependency lock file: %v", err)
	}

	tf, err := tfexec.NewTerraform(workDir, bin)
	if err != nil {
		t.Fatalf("tfexec.NewTerraform: %v", err)
	}

	if err := tf.SetEnv(fixtureEnv(t)); err != nil {
		t.Fatalf("terraform env: %v", err)
	}

	initialized := false

	t.Cleanup(func() {
		if !initialized {
			return
		}

		if err := tf.Destroy(context.Background()); err != nil {
			t.Errorf("terraform destroy (cleanup): %v", err)
		}
	})

	var copied []string

	run := func(t *testing.T, configDir string, step fixtureStep) {
		t.Helper()

		removeFiles(t, workDir, copied)
		copied = copyFixtureFiles(t, configDir, workDir)

		initMu.Lock()
		err := tf.Init(t.Context())
		initMu.Unlock()

		if err != nil {
			t.Fatalf("terraform init: %v", err)
		}

		initialized = true

		applyStep(t, tf, workDir, configDir, step)
	}

	if steps == nil {
		run(t, srcDir, fixtureStep{})

		return
	}

	for _, step := range steps {
		ok := t.Run(step.Name, func(t *testing.T) {
			run(t, filepath.Join(srcDir, step.Dir), step)
		})
		if !ok {
			return
		}
	}
}

// readSteps parses the fixture's optional stepsFile, returning nil when the
// fixture has none. A step without a name is named after its dir.
func readSteps(t *testing.T, srcDir string) []fixtureStep {
	t.Helper()

	raw, err := os.ReadFile(filepath.Clean(filepath.Join(srcDir, stepsFile)))
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		t.Fatalf("read %s: %v", stepsFile, err)
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	var steps []fixtureStep
	if err := dec.Decode(&steps); err != nil {
		t.Fatalf("parse %s: %v", stepsFile, err)
	}

	if len(steps) == 0 {
		t.Fatalf("%s lists no steps", stepsFile)
	}

	for i := range steps {
		if steps[i].Dir == "" {
			t.Fatalf("%s: step %d has no dir", stepsFile, i)
		}

		if steps[i].Name == "" {
			steps[i].Name = steps[i].Dir
		}
	}

	return steps
}

// applyStep applies the config in workDir. A step with ExpectError only
// asserts the apply error and leaves the state as the failed apply left it;
// any other step must apply cleanly, leave no pending changes and match the
// optional expected outputs in configDir.
func applyStep(t *testing.T, tf *tfexec.Terraform, workDir, configDir string, step fixtureStep) {
	t.Helper()

	opts := make([]tfexec.ApplyOption, 0, len(step.Replace))
	for _, address := range step.Replace {
		opts = append(opts, tfexec.Replace(address))
	}

	err := tf.Apply(t.Context(), opts...)

	if step.ExpectError != "" {
		assertApplyError(t, err, step.ExpectError)

		return
	}

	if err != nil {
		t.Fatalf("terraform apply: %v", err)
	}

	assertPlanHasNoChanges(t, tf, workDir)
	assertExpectedOutputs(t, tf, configDir)
}

// assertApplyError fails unless the apply failed with an error matching
// pattern.
func assertApplyError(t *testing.T, err error, pattern string) {
	t.Helper()

	re, reErr := regexp.Compile(pattern)
	if reErr != nil {
		t.Fatalf("compile expect_error %q: %v", pattern, reErr)
	}

	if err == nil {
		t.Fatalf("terraform apply succeeded, want an error matching %q", pattern)
	}

	if !re.MatchString(err.Error()) {
		t.Fatalf("terraform apply error does not match %q:\n%v", pattern, err)
	}
}

// removeFiles deletes the named files from dir, so a step's config does not
// leak into the next one.
func removeFiles(t *testing.T, dir string, names []string) {
	t.Helper()

	for _, name := range names {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatalf("remove %s: %v", name, err)
		}
	}
}

// copyFixtureFiles copies every file of srcDir except expectedOutputsFile and
// stepsFile into dstDir and returns the copied names. Fixture and step
// directories are flat (main.tf plus optional metadata), so a non-recursive
// copy is enough.
func copyFixtureFiles(t *testing.T, srcDir, dstDir string) []string {
	t.Helper()

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatalf("read fixture dir %s: %v", srcDir, err)
	}

	var copied []string

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == expectedOutputsFile || entry.Name() == stepsFile {
			continue
		}

		data, err := os.ReadFile(filepath.Clean(filepath.Join(srcDir, entry.Name())))
		if err != nil {
			t.Fatalf("read fixture file %s: %v", entry.Name(), err)
		}

		if err := os.WriteFile(filepath.Join(dstDir, entry.Name()), data, 0o600); err != nil {
			t.Fatalf("write fixture file %s: %v", entry.Name(), err)
		}

		copied = append(copied, entry.Name())
	}

	return copied
}

// assertPlanHasNoChanges re-plans the applied fixture and fails with a
// human-readable diff if terraform would still change anything.
func assertPlanHasNoChanges(t *testing.T, tf *tfexec.Terraform, workDir string) {
	t.Helper()

	ctx := t.Context()
	planFile := filepath.Join(workDir, "idempotency.tfplan")

	hasChanges, err := tf.Plan(ctx, tfexec.Out(planFile))
	if err != nil {
		t.Fatalf("terraform plan: %v", err)
	}

	if !hasChanges {
		return
	}

	diff, err := tf.ShowPlanFileRaw(ctx, planFile)
	if err != nil {
		t.Fatalf("plan is not idempotent, and rendering the diff failed: %v", err)
	}

	t.Fatalf("plan is not idempotent, terraform would still apply changes:\n%s", diff)
}

// assertExpectedOutputs compares terraform outputs against the fixture's
// optional expected_outputs.json (a map of output name to expected string).
// Fixtures without the file are skipped.
func assertExpectedOutputs(t *testing.T, tf *tfexec.Terraform, srcDir string) {
	t.Helper()

	raw, err := os.ReadFile(filepath.Clean(filepath.Join(srcDir, expectedOutputsFile)))
	if os.IsNotExist(err) {
		return
	}

	if err != nil {
		t.Fatalf("read %s: %v", expectedOutputsFile, err)
	}

	var want map[string]string
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("parse %s: %v", expectedOutputsFile, err)
	}

	got, err := tf.Output(t.Context())
	if err != nil {
		t.Fatalf("terraform output: %v", err)
	}

	for name, wantValue := range want {
		output, ok := got[name]
		if !ok {
			t.Errorf("output %q not found", name)

			continue
		}

		var gotValue string
		if err := json.Unmarshal(output.Value, &gotValue); err != nil {
			t.Errorf("output %q is not a string: %v", name, err)

			continue
		}

		if gotValue != wantValue {
			t.Errorf("output %q = %q, want %q", name, gotValue, wantValue)
		}
	}
}

// fixtureEnv layers the kumo endpoint on top of terraformEnvWithPluginCache
// so every fixture's AWS provider resolves every service against kumo.
func fixtureEnv(t *testing.T) map[string]string {
	t.Helper()

	env := terraformEnvWithPluginCache(t)
	env["AWS_ENDPOINT_URL"] = awsEndpointURL()

	return env
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
