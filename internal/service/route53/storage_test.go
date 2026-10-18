package route53

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const (
	testZoneID        = "/hostedzone/test"
	testActionDelete  = "DELETE"
	testActionUpsert  = "UPSERT"
	testUpdatedRecord = "www.example.com. 192.0.2.2"
)

func newTestRecordSet(name, value string) ResourceRecordSet {
	ttl := int64(300)

	return ResourceRecordSet{
		Name:            name,
		Type:            "A",
		TTL:             &ttl,
		ResourceRecords: []ResourceRecord{{Value: value}},
	}
}

// recordSummaries returns "name value" for every record set in the zone.
func recordSummaries(t *testing.T, s *MemoryStorage) []string {
	t.Helper()

	records, err := s.GetRecordSets(testZoneID)
	if err != nil {
		t.Fatal(err)
	}

	summaries := make([]string, 0, len(records))
	for i := range records {
		summaries = append(summaries, records[i].Name+" "+records[i].ResourceRecords[0].Value)
	}

	return summaries
}

var recordNameNormalizationTests = []struct {
	name       string
	createName string
	change     Change
	want       []string
}{
	{
		name:       "delete with trailing dot",
		createName: "www.example.com",
		change:     Change{Action: testActionDelete, ResourceRecordSet: newTestRecordSet("www.example.com.", "192.0.2.1")},
		want:       []string{},
	},
	{
		name:       "delete without trailing dot",
		createName: "www.example.com.",
		change:     Change{Action: testActionDelete, ResourceRecordSet: newTestRecordSet("www.example.com", "192.0.2.1")},
		want:       []string{},
	},
	{
		name:       "upsert with trailing dot",
		createName: "www.example.com",
		change:     Change{Action: testActionUpsert, ResourceRecordSet: newTestRecordSet("www.example.com.", "192.0.2.2")},
		want:       []string{testUpdatedRecord},
	},
	{
		name:       "upsert without trailing dot",
		createName: "www.example.com.",
		change:     Change{Action: testActionUpsert, ResourceRecordSet: newTestRecordSet("www.example.com", "192.0.2.2")},
		want:       []string{testUpdatedRecord},
	},
}

func TestChangeRecordSetsNormalizesRecordNames(t *testing.T) {
	t.Parallel()

	for _, tt := range recordNameNormalizationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := NewMemoryStorage()
			if err := s.CreateHostedZone(&HostedZone{ID: testZoneID, Name: "example.com."}); err != nil {
				t.Fatal(err)
			}

			create := Change{Action: "CREATE", ResourceRecordSet: newTestRecordSet(tt.createName, "192.0.2.1")}
			if err := s.ChangeRecordSets(testZoneID, []Change{create}); err != nil {
				t.Fatalf("CREATE: %v", err)
			}

			if got, want := recordSummaries(t, s), []string{"www.example.com. 192.0.2.1"}; !slices.Equal(got, want) {
				t.Fatalf("records after CREATE = %q, want %q", got, want)
			}

			if err := s.ChangeRecordSets(testZoneID, []Change{create}); !errors.Is(err, ErrRecordSetAlreadyExists) {
				t.Fatalf("duplicate CREATE error = %v, want %v", err, ErrRecordSetAlreadyExists)
			}

			if err := s.ChangeRecordSets(testZoneID, []Change{tt.change}); err != nil {
				t.Fatalf("%s: %v", tt.change.Action, err)
			}

			if got := recordSummaries(t, s); !slices.Equal(got, tt.want) {
				t.Fatalf("records after %s = %q, want %q", tt.change.Action, got, tt.want)
			}
		})
	}
}

func TestLoadNormalizesPersistedRecordNames(t *testing.T) {
	t.Parallel()

	// Build a snapshot as an earlier version stored it, with the record name
	// kept exactly as sent.
	legacy := NewMemoryStorage()
	legacy.HostedZones[testZoneID] = &HostedZone{ID: testZoneID, Name: "example.com."}
	legacy.RecordSets[testZoneID] = []ResourceRecordSet{newTestRecordSet("www.example.com", "192.0.2.1")}

	data, err := legacy.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "route53.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	s := NewMemoryStorage(WithDataDir(dataDir))

	if got, want := recordSummaries(t, s), []string{"www.example.com. 192.0.2.1"}; !slices.Equal(got, want) {
		t.Fatalf("records after load = %q, want %q", got, want)
	}

	upsert := Change{Action: testActionUpsert, ResourceRecordSet: newTestRecordSet("www.example.com.", "192.0.2.2")}
	if err := s.ChangeRecordSets(testZoneID, []Change{upsert}); err != nil {
		t.Fatalf("UPSERT: %v", err)
	}

	if got, want := recordSummaries(t, s), []string{testUpdatedRecord}; !slices.Equal(got, want) {
		t.Fatalf("records after UPSERT = %q, want %q", got, want)
	}

	del := Change{Action: testActionDelete, ResourceRecordSet: newTestRecordSet("www.example.com", "192.0.2.2")}
	if err := s.ChangeRecordSets(testZoneID, []Change{del}); err != nil {
		t.Fatalf("DELETE: %v", err)
	}

	if got := recordSummaries(t, s); len(got) != 0 {
		t.Fatalf("records after DELETE = %q, want none", got)
	}
}
