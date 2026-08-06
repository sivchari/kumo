//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// kumoCLIBinRelPath is where the repo Makefile's build target writes the
// kumo CLI binary (`make build` -> bin/kumo), relative to this package.
const kumoCLIBinRelPath = "../../bin/kumo"

// resolveKumoCLIBin returns the path to the kumo CLI binary these tests
// drive. KUMO_CLI_BIN (a bare name resolved via PATH, or an absolute path)
// takes precedence; otherwise it looks for the binary at kumoCLIBinRelPath.
// Tests are skipped, not failed, if neither resolves, mirroring
// terraform/runner_test.go's resolveTFBinary.
func resolveKumoCLIBin(t *testing.T) string {
	t.Helper()

	if override := os.Getenv("KUMO_CLI_BIN"); override != "" {
		if filepath.IsAbs(override) {
			return override
		}

		p, err := exec.LookPath(override)
		if err != nil {
			t.Fatalf("KUMO_CLI_BIN=%q not found on PATH: %v", override, err)
		}

		return p
	}

	abs, err := filepath.Abs(kumoCLIBinRelPath)
	if err != nil {
		t.Fatalf("resolve absolute path for %s: %v", kumoCLIBinRelPath, err)
	}

	if _, err := os.Stat(abs); err != nil {
		t.Skipf("kumo CLI binary not found at %s; run `make build` or set KUMO_CLI_BIN", abs)
	}

	return abs
}

// runCLI runs the kumo CLI binary with args against testEndpoint() and
// returns stdout. It fails the test, including stderr, on a non-zero exit.
//
// Every invocation execs the built binary as a subprocess rather than
// calling into the cli package in-process: the generated commands
// (cli/gen_*.go) read package-level globals such as endpointURL and region,
// which makes concurrent in-process invocation racy across tests.
func runCLI(t *testing.T, args ...string) []byte {
	t.Helper()

	stdout, stderr, err := execCLI(t, args...)
	if err != nil {
		t.Fatalf("kumo %s: %v\nstderr:\n%s", strings.Join(args, " "), err, stderr)
	}

	return stdout
}

// runCLIExpectError runs the kumo CLI binary with args against
// testEndpoint() and returns stdout, stderr, and the exec error without
// failing the test, for asserting on CLI failure paths.
func runCLIExpectError(t *testing.T, args ...string) (stdout, stderr []byte, err error) {
	t.Helper()

	return execCLI(t, args...)
}

func execCLI(t *testing.T, args ...string) (stdout, stderr []byte, err error) {
	t.Helper()

	bin := resolveKumoCLIBin(t)

	fullArgs := make([]string, 0, len(args)+2)
	fullArgs = append(fullArgs, args...)
	fullArgs = append(fullArgs, "--endpoint-url", testEndpoint())

	cmd := exec.CommandContext(t.Context(), bin, fullArgs...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()

	return stdoutBuf.Bytes(), stderrBuf.Bytes(), err
}

// decodeCLIJSON unmarshals a kumo CLI JSON stdout payload into a generic
// map so a test can pull values (QueueUrl, KeyId, ...) out to chain into
// the next CLI invocation.
//
// Golden assertions in this file pass the raw stdout []byte to
// golden.Assert instead of a decoded value: golden.Golden.Assert only
// strips WithIgnoreFields keys at write time for a map[string]interface{}
// or []interface{} value (see golden.Golden.formatValue /
// filterIgnoredFields); a struct pointer - which is what every SDK-based
// _test.go in this package passes - falls through unfiltered, same as raw
// []byte. Passing raw bytes here keeps the CLI goldens' file format
// consistent with the rest of the package: ignored fields still appear in
// the committed file with a point-in-time value, and are still correctly
// excluded from the pass/fail comparison itself, which golden's comparator
// applies independently at compare time regardless of how the file was
// written.
func decodeCLIJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()

	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("decode CLI JSON output: %v\noutput:\n%s", err, data)
	}

	return v
}
