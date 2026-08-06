package cligen_test

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/sivchari/kumo/internal/cligen"
	_ "github.com/sivchari/kumo/internal/registry"
	"github.com/sivchari/kumo/internal/service"
)

// TestRender_MatchesExistingHandWrittenCommands is the automated half of the
// PR1 "compare against cli/sqs.go and cli/kinesis.go" comparison exercise:
// the auto-derived command and flag names for sqs's CreateQueue and
// kinesis's CreateStream - the two actions kumo already exposes by hand -
// must exactly match the existing hand-written cli/sqs.go / cli/kinesis.go
// naming.
func TestRender_MatchesExistingHandWrittenCommands(t *testing.T) {
	t.Parallel()

	files, _, err := cligen.Render(service.Services())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	sqs, ok := files["cli/gen_sqs.go"]
	if !ok {
		t.Fatal("expected cli/gen_sqs.go to be generated")
	}

	for _, want := range []string{
		`Use:   "create-queue"`,
		`"queue-name"`,
		`"attributes"`,
	} {
		if !strings.Contains(sqs, want) {
			t.Errorf("gen_sqs.go missing %q (existing cli/sqs.go create-queue uses this exact name)", want)
		}
	}

	kinesis, ok := files["cli/gen_kinesis.go"]
	if !ok {
		t.Fatal("expected cli/gen_kinesis.go to be generated")
	}

	for _, want := range []string{
		`Use:   "create-stream"`,
		`"stream-name"`,
		`"shard-count"`,
	} {
		if !strings.Contains(kinesis, want) {
			t.Errorf("gen_kinesis.go missing %q (existing cli/kinesis.go create-stream uses this exact name)", want)
		}
	}
}

// TestGeneratedCLIUpToDate is cli-gen's drift test, the same shape as
// internal/catalog's TestREADMEUpToDate: it verifies every committed
// cli/gen_*.go file matches cligen.Render(service.Services()) output. Run
// `make cli-gen` to regenerate and commit the output after modifying a
// covered service or cli-gen itself.
//
// PR1 is intentionally inert (it ships the generator without committing any
// generated cli/gen_*.go file), so this test is skipped for now. Remove the
// Skip when PR2 lands the first generated files - see the "PR 2" checklist
// in plan-feat/cli-gen.md.
func TestGeneratedCLIUpToDate(t *testing.T) {
	t.Skip("enabled when gen files land in PR 2 (see plan-feat/cli-gen.md)")

	files, diagnostics, err := cligen.Render(service.Services())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	for _, d := range diagnostics {
		if d.Level == cligen.LevelSkip {
			t.Logf("skip: %s %s: %s", d.Service, d.Action, d.Reason)
		}
	}

	// This test package lives in internal/cligen, two directories below the
	// repo root; os.DirFS roots every subsequent read there, so fs.ReadFile
	// can never escape the repo regardless of what a file's key contains.
	repo := os.DirFS("../..")

	for path, want := range files {
		got, err := fs.ReadFile(repo, path)
		if err != nil {
			t.Errorf("read %s: %v (run `make cli-gen` to regenerate it)", path, err)

			continue
		}

		if string(got) != want {
			t.Errorf("%s is out of date. Run `make cli-gen` to regenerate it.", path)
		}
	}
}
