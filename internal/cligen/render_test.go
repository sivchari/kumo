package cligen_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"strconv"
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
func TestGeneratedCLIUpToDate(t *testing.T) {
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

// Import groups in gci's order (see gci sections in .golangci.yaml:
// standard, default, localmodule).
const (
	groupStd = iota
	groupThirdParty
	groupLocal
)

const localModulePrefix = "github.com/sivchari/kumo"

func importGroup(path string) int {
	first, _, _ := strings.Cut(path, "/")

	switch {
	case path == localModulePrefix || strings.HasPrefix(path, localModulePrefix+"/"):
		return groupLocal
	case !strings.Contains(first, "."):
		return groupStd
	default:
		return groupThirdParty
	}
}

// TestRender_ImportGroups verifies every generated file lays out its imports
// the way gci/goimports expect: standard library, third-party, then local
// module, each group separated by a blank line and sorted within. golangci-lint
// skips generated files, so this is the only guard against import drift.
func TestRender_ImportGroups(t *testing.T) {
	t.Parallel()

	files, _, err := cligen.Render(service.Services())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	for path, src := range files {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()

			f, err := parser.ParseFile(fset, path, src, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			var (
				prevPath  string
				prevGroup = -1
				prevLine  int
			)

			for _, imp := range f.Imports {
				p, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					t.Fatalf("unquote %s: %v", imp.Path.Value, err)
				}

				group := importGroup(p)
				line := fset.Position(imp.Pos()).Line

				switch {
				case group < prevGroup:
					t.Errorf("%s (group %d) is placed after %s (group %d); want standard, third-party, local module order", p, group, prevPath, prevGroup)
				case group > prevGroup && prevGroup >= 0 && line != prevLine+2:
					t.Errorf("%s must be separated from %s by exactly one blank line", p, prevPath)
				case group == prevGroup && line != prevLine+1:
					t.Errorf("%s must not be separated from %s by a blank line", p, prevPath)
				case group == prevGroup && p < prevPath:
					t.Errorf("%s must sort before %s", p, prevPath)
				}

				prevPath, prevGroup, prevLine = p, group, line
			}
		})
	}
}
