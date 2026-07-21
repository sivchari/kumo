package lambda

import "testing"

func TestExtractEventSourceMappingUUID(t *testing.T) {
	t.Parallel()

	const mappingUUID = "12345678-1234-1234-1234-123456789012"

	for _, path := range []string{
		"/lambda/2015-03-31/event-source-mappings/" + mappingUUID,
		"/2015-03-31/event-source-mappings/" + mappingUUID,
	} {
		if got := extractEventSourceMappingUUID(path); got != mappingUUID {
			t.Errorf("extractEventSourceMappingUUID(%q) = %q, want %q", path, got, mappingUUID)
		}
	}
}
