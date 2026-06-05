// Command readme-gen regenerates the README "Supported Services" section from
// each registered service's Meta(). Run it with `make readme` or
// `go run ./cmd/readme-gen`; the catalog test (internal/catalog) verifies the
// committed README stays in sync.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	// Register all services so service.Services() returns the full catalog.
	"github.com/sivchari/kumo/internal/catalog"
	_ "github.com/sivchari/kumo/internal/registry"
	"github.com/sivchari/kumo/internal/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "readme-gen:", err)
		os.Exit(1)
	}
}

func run() error {
	path, err := readmePath()
	if err != nil {
		return err
	}

	current, err := os.ReadFile(path) //nolint:gosec // G304: path is derived from runtime.Caller, not external input.
	if err != nil {
		return fmt.Errorf("read README: %w", err)
	}

	updated, err := catalog.Render(string(current), service.Services())
	if err != nil {
		return fmt.Errorf("render catalog: %w", err)
	}

	if updated == string(current) {
		fmt.Println("README already up to date")

		return nil
	}

	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		return fmt.Errorf("write README: %w", err)
	}

	fmt.Println("README updated")

	return nil
}

// readmePath resolves README.md at the repository root relative to this source
// file, so the tool works regardless of the caller's working directory.
func readmePath() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot determine source location")
	}

	// file = <repo>/cmd/readme-gen/main.go -> repo root is two dirs up.
	return filepath.Join(filepath.Dir(file), "..", "..", "README.md"), nil
}
