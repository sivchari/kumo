package catalog_test

import (
	"os"
	"testing"

	// Register all services so service.Services() returns the full catalog.
	"github.com/sivchari/kumo/internal/catalog"
	_ "github.com/sivchari/kumo/internal/registry"
	"github.com/sivchari/kumo/internal/service"
)

// readmePath is the README location relative to this package directory.
const readmePath = "../../README.md"

// TestREADMEUpToDate verifies that the README "Supported Services" section
// matches the registered services. The registry (each service's Meta()) is the
// single source of truth. Run `make readme` to regenerate the README after
// adding, removing, or editing a service.
func TestREADMEUpToDate(t *testing.T) {
	current, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README: %v", err)
	}

	want, err := catalog.Render(string(current), service.Services())
	if err != nil {
		t.Fatalf("render catalog: %v", err)
	}

	if want != string(current) {
		t.Error("README service catalog is out of date. Run `make readme` to regenerate it.")
	}
}
