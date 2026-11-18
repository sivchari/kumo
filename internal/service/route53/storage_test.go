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
	testZoneName      = "example.com."
	testCreatedRecord = "www.example.com. 192.0.2.1"
	testUpdatedRecord = "www.example.com. 192.0.2.2"
	testAPIRecord     = "api.example.com. 192.0.2.10"
	// testUndottedName is wwwName without the trailing dot.
	testUndottedName = "www.example.com"
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
		createName: testUndottedName,
		change:     Change{Action: actionDelete, ResourceRecordSet: newTestRecordSet(wwwName, "192.0.2.1")},
		want:       []string{},
	},
	{
		name:       "delete without trailing dot",
		createName: "www.example.com.",
		change:     Change{Action: actionDelete, ResourceRecordSet: newTestRecordSet(testUndottedName, "192.0.2.1")},
		want:       []string{},
	},
	{
		name:       "upsert with trailing dot",
		createName: testUndottedName,
		change:     Change{Action: actionUpsert, ResourceRecordSet: newTestRecordSet(wwwName, "192.0.2.2")},
		want:       []string{testUpdatedRecord},
	},
	{
		name:       "upsert without trailing dot",
		createName: "www.example.com.",
		change:     Change{Action: actionUpsert, ResourceRecordSet: newTestRecordSet(testUndottedName, "192.0.2.2")},
		want:       []string{testUpdatedRecord},
	},
	{
		name:       "delete in another case",
		createName: "WWW.Example.com",
		change:     Change{Action: actionDelete, ResourceRecordSet: newTestRecordSet(wwwName, "192.0.2.1")},
		want:       []string{},
	},
	{
		name:       "upsert in another case",
		createName: "www.Example.COM.",
		change:     Change{Action: actionUpsert, ResourceRecordSet: newTestRecordSet("WWW.EXAMPLE.COM", "192.0.2.2")},
		want:       []string{testUpdatedRecord},
	},
}

func TestChangeRecordSetsNormalizesRecordNames(t *testing.T) {
	t.Parallel()

	for _, tt := range recordNameNormalizationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := NewMemoryStorage()
			if err := s.CreateHostedZone(&HostedZone{ID: testZoneID, Name: testZoneName}); err != nil {
				t.Fatal(err)
			}

			create := Change{Action: actionCreate, ResourceRecordSet: newTestRecordSet(tt.createName, "192.0.2.1")}
			if err := s.ChangeRecordSets(testZoneID, []Change{create}); err != nil {
				t.Fatalf("CREATE: %v", err)
			}

			if got, want := recordSummaries(t, s), []string{testCreatedRecord}; !slices.Equal(got, want) {
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

// loadSnapshot writes a snapshot holding records as an earlier version
// stored them, with names kept exactly as sent, and loads it.
func loadSnapshot(t *testing.T, records []ResourceRecordSet) *MemoryStorage {
	t.Helper()

	legacy := NewMemoryStorage()
	legacy.HostedZones[testZoneID] = &HostedZone{ID: testZoneID, Name: testZoneName, ResourceRecordSetCount: int64(len(records))}
	legacy.RecordSets[testZoneID] = records

	data, err := legacy.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "route53.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	return NewMemoryStorage(WithDataDir(dataDir))
}

var snapshotRecordNameTests = []struct {
	name      string
	persisted string
}{
	{name: "undotted name", persisted: testUndottedName},
	{name: "upper-case name", persisted: "WWW.EXAMPLE.COM."},
	{name: "upper-case undotted name", persisted: "Www.Example.com"},
}

// TestSnapshotRecordNamesStayReachable checks that a record set an earlier
// version stored under its name as sent is loaded as is, and that UPSERT and
// DELETE still find it under any spelling of the name.
func TestSnapshotRecordNamesStayReachable(t *testing.T) {
	t.Parallel()

	for _, tt := range snapshotRecordNameTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := loadSnapshot(t, []ResourceRecordSet{newTestRecordSet(tt.persisted, "192.0.2.1")})

			if got, want := recordSummaries(t, s), []string{tt.persisted + " 192.0.2.1"}; !slices.Equal(got, want) {
				t.Fatalf("records after load = %q, want %q", got, want)
			}

			create := Change{Action: actionCreate, ResourceRecordSet: newTestRecordSet(wwwName, "192.0.2.1")}
			if err := s.ChangeRecordSets(testZoneID, []Change{create}); !errors.Is(err, ErrRecordSetAlreadyExists) {
				t.Fatalf("CREATE error = %v, want %v", err, ErrRecordSetAlreadyExists)
			}

			// UPSERT replaces the record set, so it is stored fully qualified
			// from then on.
			upsert := Change{Action: actionUpsert, ResourceRecordSet: newTestRecordSet(wwwName, "192.0.2.2")}
			if err := s.ChangeRecordSets(testZoneID, []Change{upsert}); err != nil {
				t.Fatalf("UPSERT: %v", err)
			}

			if got, want := recordSummaries(t, s), []string{testUpdatedRecord}; !slices.Equal(got, want) {
				t.Fatalf("records after UPSERT = %q, want %q", got, want)
			}

			del := Change{Action: actionDelete, ResourceRecordSet: newTestRecordSet(testUndottedName, "192.0.2.2")}
			if err := s.ChangeRecordSets(testZoneID, []Change{del}); err != nil {
				t.Fatalf("DELETE: %v", err)
			}

			if got := recordSummaries(t, s); len(got) != 0 {
				t.Fatalf("records after DELETE = %q, want none", got)
			}
		})
	}
}

// TestSnapshotSpellingsOfOneRecordSetAreKept checks that two spellings of www
// that an earlier version stored side by side are both loaded, and that
// DELETE removes them one at a time.
func TestSnapshotSpellingsOfOneRecordSetAreKept(t *testing.T) {
	t.Parallel()

	s := loadSnapshot(t, []ResourceRecordSet{
		newTestRecordSet(testUndottedName, "192.0.2.1"),
		newTestRecordSet("api.example.com.", "192.0.2.10"),
		newTestRecordSet("WWW.example.com.", "192.0.2.9"),
	})

	want := []string{testUndottedName + " 192.0.2.1", testAPIRecord, "WWW.example.com. 192.0.2.9"}
	if got := recordSummaries(t, s); !slices.Equal(got, want) {
		t.Fatalf("records after load = %q, want %q", got, want)
	}

	del := Change{Action: actionDelete, ResourceRecordSet: newTestRecordSet(wwwName, "192.0.2.1")}

	for _, want := range [][]string{
		{testAPIRecord, "WWW.example.com. 192.0.2.9"},
		{testAPIRecord},
	} {
		if err := s.ChangeRecordSets(testZoneID, []Change{del}); err != nil {
			t.Fatalf("DELETE: %v", err)
		}

		if got := recordSummaries(t, s); !slices.Equal(got, want) {
			t.Fatalf("records after DELETE = %q, want %q", got, want)
		}
	}
}
