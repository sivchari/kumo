package route53

import (
	"errors"
	"strings"
	"testing"
)

func TestNewIDShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		gen    func() string
		prefix string
		path   string
	}{
		{name: "hosted zone", gen: newHostedZoneID, prefix: "Z", path: "/hostedzone/"},
		{name: "change", gen: newChangeID, prefix: "C", path: "/change/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const draws = 10000

			seen := make(map[string]struct{}, draws)

			for range draws {
				id := tt.gen()

				if len(id) != 1+idRandomLength {
					t.Fatalf("id %q has length %d, want %d", id, len(id), 1+idRandomLength)
				}

				if got := len(tt.path + id); got > 32 {
					t.Fatalf("%s%s has length %d, want at most 32", tt.path, id, got)
				}

				if !strings.HasPrefix(id, tt.prefix) {
					t.Fatalf("id %q does not start with %q", id, tt.prefix)
				}

				if i := strings.IndexFunc(id, func(r rune) bool { return !strings.ContainsRune(idAlphabet, r) }); i >= 0 {
					t.Fatalf("id %q has %q outside [A-Z0-9]", id, id[i])
				}

				if _, dup := seen[id]; dup {
					t.Fatalf("id %q generated twice in %d draws", id, draws)
				}

				seen[id] = struct{}{}
			}
		})
	}
}

func TestNewIDUsesWholeAlphabet(t *testing.T) {
	t.Parallel()

	counts := make(map[byte]int, len(idAlphabet))

	for range 2000 {
		id := newHostedZoneID()
		for i := 1; i < len(id); i++ {
			counts[id[i]]++
		}
	}

	for i := range len(idAlphabet) {
		if counts[idAlphabet[i]] == 0 {
			t.Errorf("character %q never generated", idAlphabet[i])
		}
	}
}

// idSequence returns an ID generator that yields ids in order and fails the
// test if it is called more often.
func idSequence(t *testing.T, ids ...string) func() string {
	t.Helper()

	return func() string {
		if len(ids) == 0 {
			t.Fatal("ID generator called more often than expected")
		}

		id := ids[0]
		ids = ids[1:]

		return id
	}
}

// A generated hosted zone ID that is already taken must neither fail the
// create nor touch the zone stored under it: another ID is drawn instead.
func TestInsertHostedZoneDrawsAgainOnTakenID(t *testing.T) {
	t.Parallel()

	const (
		takenID      = "ZTAKEN"
		freshID      = "ZFRESH"
		existingName = "existing.example.com."
	)

	store := NewMemoryStorage()
	if err := store.CreateHostedZone(&HostedZone{ID: "/hostedzone/" + takenID, Name: existingName}); err != nil {
		t.Fatal(err)
	}

	svc := New(store)
	svc.nextZoneID = idSequence(t, takenID, freshID)

	zone := &HostedZone{Name: "new.example.com."}
	if err := svc.insertHostedZone(zone); err != nil {
		t.Fatalf("insertHostedZone: %v", err)
	}

	if want := "/hostedzone/" + freshID; zone.ID != want {
		t.Errorf("new zone ID = %q, want %q", zone.ID, want)
	}

	existing, err := store.GetHostedZone("/hostedzone/" + takenID)
	if err != nil {
		t.Fatal(err)
	}

	if existing.Name != existingName {
		t.Errorf("zone %s is now named %q, want %q", takenID, existing.Name, existingName)
	}
}

// The same holds for change IDs: GetChange on the earlier change must keep
// returning that change.
func TestRecordInSyncChangeDrawsAgainOnTakenID(t *testing.T) {
	t.Parallel()

	const (
		takenID       = "CTAKEN"
		freshID       = "CFRESH"
		storedComment = "first"
	)

	store := NewMemoryStorage()
	if err := store.PutChange(&ChangeInfo{ID: "/change/" + takenID, Status: "INSYNC", Comment: storedComment}); err != nil {
		t.Fatal(err)
	}

	svc := New(store)
	svc.nextChangeID = idSequence(t, takenID, freshID)

	ci, err := svc.recordInSyncChange("second")
	if err != nil {
		t.Fatalf("recordInSyncChange: %v", err)
	}

	if want := "/change/" + freshID; ci.ID != want {
		t.Errorf("new change ID = %q, want %q", ci.ID, want)
	}

	stored, err := store.GetChange(takenID)
	if err != nil {
		t.Fatal(err)
	}

	if stored.Comment != storedComment {
		t.Errorf("change %s now has comment %q, want %q", takenID, stored.Comment, storedComment)
	}
}

// A generator that only returns taken IDs makes the insert fail after
// maxIDAttempts draws instead of looping forever.
func TestInsertHostedZoneGivesUpAfterMaxIDAttempts(t *testing.T) {
	t.Parallel()

	const takenID = "ZTAKEN"

	store := NewMemoryStorage()
	if err := store.CreateHostedZone(&HostedZone{ID: "/hostedzone/" + takenID, Name: "existing.example.com."}); err != nil {
		t.Fatal(err)
	}

	draws := 0
	svc := New(store)
	svc.nextZoneID = func() string {
		draws++

		return takenID
	}

	if err := svc.insertHostedZone(&HostedZone{Name: "new.example.com."}); !errors.Is(err, errNoFreeID) {
		t.Fatalf("insertHostedZone error = %v, want %v", err, errNoFreeID)
	}

	if draws != maxIDAttempts {
		t.Errorf("generator called %d times, want %d", draws, maxIDAttempts)
	}
}
