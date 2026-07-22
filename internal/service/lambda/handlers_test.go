package lambda

import "testing"

func TestExtractEventSourceMappingUUIDWithEndpointPrefixes(t *testing.T) {
	t.Parallel()

	const mappingUUID = "12345678-1234-1234-1234-123456789012"

	for name, path := range map[string]string{
		"SDK BaseEndpoint":       "/lambda/2015-03-31/event-source-mappings/" + mappingUUID,
		"terraform AWS provider": "/2015-03-31/event-source-mappings/" + mappingUUID,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := extractEventSourceMappingUUID(path); got != mappingUUID {
				t.Errorf("extractEventSourceMappingUUID(%q) = %q, want %q", path, got, mappingUUID)
			}
		})
	}
}
