package dynamodb

import (
	"errors"
	"testing"
)

const (
	testStartKeyOtherGroup = "other"
	msgOutsideQuery        = "The provided starting key is outside query boundaries based on provided conditions"
	msgInvalidStartKey     = "The provided starting key is invalid: The provided key element does not match the schema"
)

func startKeyFixture(t *testing.T) *MemoryStorage {
	t.Helper()

	return createShapesTable(t,
		[]KeySchemaElement{keyElement("pk", keyTypeHash), keyElement("sk", keyTypeRange)},
		[]KeySchemaElement{keyElement("g", keyTypeHash), keyElement(testShapesOwner, keyTypeRange)},
		nil,
		[]Item{
			sItem("pk", "a", "sk", "1", "g", testShapesGroup, testShapesOwner, "amy"),
			sItem("pk", "b", "sk", "2", "g", testShapesGroup, testShapesOwner, "bob"),
			sItem("pk", "c", "sk", "3", "g", testStartKeyOtherGroup, testShapesOwner, "cat"),
		})
}

func assertValidation(t *testing.T, err error, want string) {
	t.Helper()

	var tErr *TableError
	if !errors.As(err, &tErr) {
		t.Fatalf("err = %v, want ValidationException %q", err, want)
	}

	if tErr.Code != errCodeValidation || tErr.Message != want {
		t.Errorf("err = %s: %q, want %s: %q", tErr.Code, tErr.Message, errCodeValidation, want)
	}
}

type startKeyCase struct {
	name     string
	startKey Item
	want     string
}

func indexStartKeyCases() []startKeyCase {
	return []startKeyCase{
		{
			name:     "valid key of the queried partition",
			startKey: sItem("g", testShapesGroup, testShapesOwner, "amy", "pk", "a", "sk", "1"),
		},
		{
			name:     "partition key of another partition",
			startKey: sItem("g", testStartKeyOtherGroup, testShapesOwner, "cat", "pk", "c", "sk", "3"),
			want:     msgOutsideQuery,
		},
		{name: "table key only", startKey: sItem("pk", "a", "sk", "1"), want: msgInvalidStartKey},
		{name: "index key only", startKey: sItem("g", testShapesGroup, testShapesOwner, "amy"), want: msgInvalidStartKey},
		{
			name:     "extra attribute",
			startKey: sItem("g", testShapesGroup, testShapesOwner, "amy", "pk", "a", "sk", "1", "id", "x"),
			want:     msgInvalidStartKey,
		},
		{
			name: "wrong type",
			startKey: Item{
				"g":             {N: new("1")},
				testShapesOwner: {S: new("amy")},
				"pk":            {S: new("a")},
				"sk":            {S: new("1")},
			},
			want: msgInvalidStartKey,
		},
	}
}

func TestQuery_IndexExclusiveStartKey(t *testing.T) {
	t.Parallel()

	for _, tt := range indexStartKeyCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := startKeyFixture(t)

			_, _, _, err := s.Query(t.Context(), testShapesTable, testShapesIndex, "g = :g", "",
				nil, map[string]AttributeValue{":g": {S: new(testShapesGroup)}}, 0, tt.startKey, true)

			if tt.want == "" {
				if err != nil {
					t.Fatalf("Query = %v, want nil", err)
				}

				return
			}

			assertValidation(t, err, tt.want)
		})
	}
}

func TestScan_ExclusiveStartKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		startKey Item
		want     string
	}{
		{name: "valid", startKey: sItem("pk", "a", "sk", "1")},
		{name: "missing sort key", startKey: sItem("pk", "a"), want: msgInvalidStartKey},
		{name: "extra attribute", startKey: sItem("pk", "a", "sk", "1", "g", testShapesGroup), want: msgInvalidStartKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := startKeyFixture(t)

			_, _, _, err := s.Scan(t.Context(), testShapesTable, "", nil, nil, 0, tt.startKey, nil, nil)

			if tt.want == "" {
				if err != nil {
					t.Fatalf("Scan = %v, want nil", err)
				}

				return
			}

			assertValidation(t, err, tt.want)
		})
	}
}
