package route53

import "testing"

const (
	actionCreate = "CREATE"
	actionUpsert = "UPSERT"
	actionDelete = "DELETE"
	unlistedType = "FOO"
)

func TestUnlistedTypeToStore(t *testing.T) {
	t.Parallel()

	change := func(action, recordType string) Change {
		return Change{Action: action, ResourceRecordSet: ResourceRecordSet{Name: aName, Type: recordType}}
	}

	tests := []struct {
		name     string
		changes  []Change
		wantType string
		wantOK   bool
	}{
		{name: "listed types", changes: []Change{change(actionCreate, "A"), change(actionUpsert, typeTXT)}},
		{name: "unlisted type created", changes: []Change{change(actionCreate, "A"), change(actionCreate, unlistedType)}, wantType: unlistedType, wantOK: true},
		{name: "listed type in lower case upserted", changes: []Change{change(actionUpsert, "a")}, wantType: "a", wantOK: true},
		{name: "missing type created", changes: []Change{change(actionCreate, "")}, wantOK: true},
		// A record set stored with an unlisted type can still be deleted.
		{name: "unlisted type deleted", changes: []Change{change(actionDelete, unlistedType)}},
	}

	for _, tt := range tests {
		if gotType, gotOK := unlistedTypeToStore(tt.changes); gotType != tt.wantType || gotOK != tt.wantOK {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", tt.name, gotType, gotOK, tt.wantType, tt.wantOK)
		}
	}
}
