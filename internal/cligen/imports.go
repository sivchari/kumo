package cligen

import "sort"

// importSet accumulates the Go imports a generated cli/gen_{service}.go file
// needs, split into the standard-library and third-party groups. Which
// imports are needed depends on which FlagKinds actually appear among the
// service's rendered actions, so flagView construction calls the need*
// methods as it goes.
type importSet struct {
	sdkImportPath string
	std           map[string]bool
	third         map[string]bool
}

func newImportSet(sdkImportPath string) *importSet {
	s := &importSet{
		sdkImportPath: sdkImportPath,
		std:           map[string]bool{"fmt": true},
		third: map[string]bool{
			"github.com/aws/aws-sdk-go-v2/aws": true,
			sdkImportPath:                      true,
			"github.com/spf13/cobra":           true,
		},
	}

	return s
}

func (s *importSet) needJSONEncode() {
	s.std["encoding/json"] = true
	s.std["os"] = true
}

func (s *importSet) needJSON() {
	s.std["encoding/json"] = true
}

func (s *importSet) needBase64() {
	s.std["encoding/base64"] = true
}

func (s *importSet) needTime() {
	s.std["time"] = true
}

func (s *importSet) needKV() {
	s.std["reflect"] = true
	s.third["github.com/sivchari/kumo/internal/cligen"] = true
}

func (s *importSet) needTypes() {
	s.third[s.sdkImportPath+"/types"] = true
}

// sorted returns the accumulated imports as sorted, deduplicated import
// paths (without quotes; the template adds them).
func (s *importSet) sorted() (std, third []string) {
	return sortedKeys(s.std), sortedKeys(s.third)
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
