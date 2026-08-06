package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	_ "github.com/sivchari/kumo/internal/registry"
	"github.com/sivchari/kumo/internal/service"
)

func TestClassifySkipReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		reason string
		want   string
	}{
		{"no matching aws-sdk-go-v2 client method (likely a kumo-only custom endpoint)", "no matching SDK method"},
		{"manually overridden in cli/kms_overrides.go, skipping auto-generation", "manual override file"},
		{"field KeyId is an interface/union type (e.g. a Smithy union); it needs a hand-written override", "unsupported field type"},
		{"some future reason cligen doesn't classify yet", "other"},
	}

	for _, tt := range tests {
		if got := classifySkipReason(tt.reason); got != tt.want {
			t.Errorf("classifySkipReason(%q) = %q, want %q", tt.reason, got, tt.want)
		}
	}
}

func TestSummarizeSkipReasons(t *testing.T) {
	t.Parallel()

	if got := summarizeSkipReasons(nil); got != "" {
		t.Errorf("summarizeSkipReasons(nil) = %q, want empty", got)
	}

	got := summarizeSkipReasons(map[string]int{"other": 1, "no matching SDK method": 2})
	want := "no matching SDK method (2), other (1)"

	if got != want {
		t.Errorf("summarizeSkipReasons() = %q, want %q", got, want)
	}
}

func TestCountLeafCommands(t *testing.T) {
	t.Parallel()

	leaf := &cobra.Command{Use: "leaf"}
	if n := countLeafCommands(leaf); n != 1 {
		t.Errorf("countLeafCommands(leaf) = %d, want 1", n)
	}

	group := &cobra.Command{Use: "group"}
	group.AddCommand(&cobra.Command{Use: "a"}, &cobra.Command{Use: "b"})

	if n := countLeafCommands(group); n != 2 {
		t.Errorf("countLeafCommands(group) = %d, want 2", n)
	}

	nested := &cobra.Command{Use: "nested"}
	sub := &cobra.Command{Use: "sub"}
	sub.AddCommand(&cobra.Command{Use: "c"}, &cobra.Command{Use: "d"})
	nested.AddCommand(sub, &cobra.Command{Use: "e"})

	if n := countLeafCommands(nested); n != 3 {
		t.Errorf("countLeafCommands(nested) = %d, want 3", n)
	}
}

func TestManualCommandCount(t *testing.T) {
	t.Parallel()

	counts := map[string]int{"s3": 1, "s3api": 3, "amplify": 9}

	if n, ok := manualCommandCount("s3", counts); !ok || n != 4 {
		t.Errorf("manualCommandCount(s3) = (%d, %v), want (4, true)", n, ok)
	}

	if n, ok := manualCommandCount("amplify", counts); !ok || n != 9 {
		t.Errorf("manualCommandCount(amplify) = (%d, %v), want (9, true)", n, ok)
	}

	if _, ok := manualCommandCount("route53", counts); ok {
		t.Error("manualCommandCount(route53) reported found, want not found")
	}
}

// TestRunCoverage_CoversEveryService verifies runCoverage emits exactly one
// row per registered service, categorizes every row into one of the three
// known categories, and reports a totals line consistent with those rows.
func TestRunCoverage_CoversEveryService(t *testing.T) {
	services := service.Services()

	var buf bytes.Buffer
	if err := runCoverage(&buf, services); err != nil {
		t.Fatalf("runCoverage: %v", err)
	}

	seen, totalsLine := parseCoverageOutput(t, buf.String())

	for _, svc := range services {
		if !seen[svc.Name()] {
			t.Errorf("coverage table missing row for service %q", svc.Name())
		}
	}

	if len(seen) != len(services) {
		t.Errorf("coverage table has %d row(s), want %d", len(seen), len(services))
	}

	assertCoverageTotals(t, totalsLine, len(services))
}

// parseCoverageOutput extracts the set of service names seen in the
// coverage table (validating each row's category along the way) and the
// totals line, failing the test if either is malformed.
func parseCoverageOutput(t *testing.T, output string) (seen map[string]bool, totalsLine string) {
	t.Helper()

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected a header line, service rows, and a totals line; got:\n%s", output)
	}

	seen = make(map[string]bool)

	for _, line := range lines[1:] { // skip the header row
		if strings.HasPrefix(line, "total:") {
			totalsLine = line

			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		name, category := fields[0], fields[1]
		seen[name] = true

		assertKnownCategory(t, name, category)
	}

	if totalsLine == "" {
		t.Fatalf("coverage table missing totals line, got:\n%s", output)
	}

	return seen, totalsLine
}

func assertKnownCategory(t *testing.T, name, category string) {
	t.Helper()

	switch category {
	case "auto-generated", "manual", "uncovered":
	default:
		t.Errorf("service %q has unknown category %q", name, category)
	}
}

func assertCoverageTotals(t *testing.T, totalsLine string, wantTotal int) {
	t.Helper()

	var covered, total, commands int
	if _, err := fmt.Sscanf(totalsLine, "total: %d/%d service(s) covered, %d command(s)", &covered, &total, &commands); err != nil {
		t.Fatalf("parse totals line %q: %v", totalsLine, err)
	}

	if total != wantTotal {
		t.Errorf("totals service count = %d, want %d", total, wantTotal)
	}

	if covered > total {
		t.Errorf("covered (%d) > total (%d)", covered, total)
	}

	if commands <= 0 {
		t.Errorf("total commands = %d, want > 0", commands)
	}
}
