// Command cli-gen discovers CLI-able actions from kumo's service registry
// (see internal/cligen) and renders generated Cobra command files
// (cli/gen_{service}.go). Run it with `make cli-gen` or `go run ./cmd/cli-gen`;
// pass -dry-run to only print discovery/render diagnostics without writing
// any files.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/sivchari/kumo/internal/cligen"
	// Register all services so service.Services() returns the full catalog.
	_ "github.com/sivchari/kumo/internal/registry"
	"github.com/sivchari/kumo/internal/service"
)

func main() {
	outDir := flag.String("out", "cli", "directory to write generated cli/gen_*.go files into (relative paths resolve against the repo root)")
	dryRun := flag.Bool("dry-run", false, "print discovery/render diagnostics without writing any files")
	flag.Parse()

	if err := run(*outDir, *dryRun); err != nil {
		fmt.Fprintln(os.Stderr, "cli-gen:", err)
		os.Exit(1)
	}
}

func run(outDir string, dryRun bool) error {
	files, diagnostics, err := cligen.Render(service.Services())
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	printDiagnostics(diagnostics)

	if dryRun {
		fmt.Printf("\ndry-run: would write %d file(s) under %s\n", len(files), outDir)

		return nil
	}

	dir, err := resolveOutDir(outDir)
	if err != nil {
		return err
	}

	if err := writeFiles(dir, files); err != nil {
		return err
	}

	fmt.Printf("\nwrote %d file(s) under %s\n", len(files), dir)

	return nil
}

func writeFiles(dir string, files map[string]string) error {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		path := filepath.Join(dir, filepath.Base(name))
		if err := os.WriteFile(path, []byte(files[name]), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	return nil
}

// printDiagnostics prints a per-service generated/skipped summary followed
// by the reason for every skip, so `-dry-run` output makes every
// auto-discovery decision visible instead of silent.
func printDiagnostics(diagnostics []cligen.Diagnostic) {
	byService := make(map[string][]cligen.Diagnostic)

	var services []string

	for _, d := range diagnostics {
		if _, ok := byService[d.Service]; !ok {
			services = append(services, d.Service)
		}

		byService[d.Service] = append(byService[d.Service], d)
	}

	sort.Strings(services)

	var totalGenerated, totalSkipped int

	for _, svc := range services {
		var generated, skipped int

		for _, d := range byService[svc] {
			if d.Level == cligen.LevelGenerated {
				generated++
			} else {
				skipped++
			}
		}

		totalGenerated += generated
		totalSkipped += skipped

		fmt.Printf("%-16s generated=%-4d skipped=%-4d\n", svc, generated, skipped)

		for _, d := range byService[svc] {
			if d.Level != cligen.LevelSkip {
				continue
			}

			if d.Action != "" {
				fmt.Printf("  skip %s.%s: %s\n", svc, d.Action, d.Reason)
			} else {
				fmt.Printf("  skip %s: %s\n", svc, d.Reason)
			}
		}
	}

	fmt.Printf("\ntotal: generated=%d skipped=%d across %d service(s)\n", totalGenerated, totalSkipped, len(services))
}

// resolveOutDir returns outDir unchanged if it is already absolute (e.g. a
// scratch directory for manual comparison); otherwise it resolves outDir
// relative to the repository root, so `go run ./cmd/cli-gen` behaves the
// same regardless of the caller's working directory.
func resolveOutDir(outDir string) (string, error) {
	if filepath.IsAbs(outDir) {
		return outDir, nil
	}

	root, err := repoRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, outDir), nil
}

// repoRoot resolves the repository root relative to this source file, so the
// tool works regardless of the caller's working directory.
func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot determine source location")
	}

	// file = <repo>/cmd/cli-gen/main.go -> repo root is two dirs up.
	return filepath.Join(filepath.Dir(file), "..", ".."), nil
}
