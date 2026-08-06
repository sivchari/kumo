package cli

import (
	"reflect"
	"testing"

	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type decodeDynamoDBJSONTestCase struct {
	name    string
	raw     string
	want    map[string]ddbtypes.AttributeValue
	wantErr bool
}

// decodeDynamoDBJSONScalarTestCases covers each scalar AttributeValue member
// kind (S, N, B, BOOL, NULL) in isolation.
func decodeDynamoDBJSONScalarTestCases() []decodeDynamoDBJSONTestCase {
	return []decodeDynamoDBJSONTestCase{
		{
			name: "S string member",
			raw:  `{"pk":{"S":"a"}}`,
			want: map[string]ddbtypes.AttributeValue{
				"pk": &ddbtypes.AttributeValueMemberS{Value: "a"},
			},
		},
		{
			name: "N number member",
			raw:  `{"n":{"N":"1"}}`,
			want: map[string]ddbtypes.AttributeValue{
				"n": &ddbtypes.AttributeValueMemberN{Value: "1"},
			},
		},
		{
			name: "B binary member",
			raw:  `{"b":{"B":"aGVsbG8="}}`,
			want: map[string]ddbtypes.AttributeValue{
				"b": &ddbtypes.AttributeValueMemberB{Value: []byte("hello")},
			},
		},
		{
			name: "BOOL member true",
			raw:  `{"flag":{"BOOL":true}}`,
			want: map[string]ddbtypes.AttributeValue{
				"flag": &ddbtypes.AttributeValueMemberBOOL{Value: true},
			},
		},
		{
			name: "BOOL member false",
			raw:  `{"flag":{"BOOL":false}}`,
			want: map[string]ddbtypes.AttributeValue{
				"flag": &ddbtypes.AttributeValueMemberBOOL{Value: false},
			},
		},
		{
			name: "NULL member",
			raw:  `{"n":{"NULL":true}}`,
			want: map[string]ddbtypes.AttributeValue{
				"n": &ddbtypes.AttributeValueMemberNULL{Value: true},
			},
		},
	}
}

// decodeDynamoDBJSONCollectionTestCases covers each set-typed AttributeValue
// member kind (SS, NS, BS).
func decodeDynamoDBJSONCollectionTestCases() []decodeDynamoDBJSONTestCase {
	return []decodeDynamoDBJSONTestCase{
		{
			name: "SS string set member",
			raw:  `{"ss":{"SS":["a","b"]}}`,
			want: map[string]ddbtypes.AttributeValue{
				"ss": &ddbtypes.AttributeValueMemberSS{Value: []string{"a", "b"}},
			},
		},
		{
			name: "NS number set member",
			raw:  `{"ns":{"NS":["1","2"]}}`,
			want: map[string]ddbtypes.AttributeValue{
				"ns": &ddbtypes.AttributeValueMemberNS{Value: []string{"1", "2"}},
			},
		},
		{
			name: "BS binary set member",
			raw:  `{"bs":{"BS":["aGVsbG8=","d29ybGQ="]}}`,
			want: map[string]ddbtypes.AttributeValue{
				"bs": &ddbtypes.AttributeValueMemberBS{Value: [][]byte{[]byte("hello"), []byte("world")}},
			},
		},
	}
}

// decodeDynamoDBJSONNestedTestCases covers L (list) and M (map) recursion,
// including L-within-M-within-L nesting, and multiple/empty top-level
// attributes.
func decodeDynamoDBJSONNestedTestCases() []decodeDynamoDBJSONTestCase {
	return []decodeDynamoDBJSONTestCase{
		{
			name: "L list member",
			raw:  `{"l":{"L":[{"S":"a"},{"N":"1"}]}}`,
			want: map[string]ddbtypes.AttributeValue{
				"l": &ddbtypes.AttributeValueMemberL{Value: []ddbtypes.AttributeValue{
					&ddbtypes.AttributeValueMemberS{Value: "a"},
					&ddbtypes.AttributeValueMemberN{Value: "1"},
				}},
			},
		},
		{
			name: "M map member",
			raw:  `{"m":{"M":{"nested":{"S":"a"}}}}`,
			want: map[string]ddbtypes.AttributeValue{
				"m": &ddbtypes.AttributeValueMemberM{Value: map[string]ddbtypes.AttributeValue{
					"nested": &ddbtypes.AttributeValueMemberS{Value: "a"},
				}},
			},
		},
		{
			name: "nested L within M within L",
			raw:  `{"pk":{"L":[{"M":{"a":{"L":[{"S":"x"}]}}}]}}`,
			want: map[string]ddbtypes.AttributeValue{
				"pk": &ddbtypes.AttributeValueMemberL{Value: []ddbtypes.AttributeValue{
					&ddbtypes.AttributeValueMemberM{Value: map[string]ddbtypes.AttributeValue{
						"a": &ddbtypes.AttributeValueMemberL{Value: []ddbtypes.AttributeValue{
							&ddbtypes.AttributeValueMemberS{Value: "x"},
						}},
					}},
				}},
			},
		},
		{
			name: "multiple top-level attributes",
			raw:  `{"pk":{"S":"a"},"sk":{"N":"1"}}`,
			want: map[string]ddbtypes.AttributeValue{
				"pk": &ddbtypes.AttributeValueMemberS{Value: "a"},
				"sk": &ddbtypes.AttributeValueMemberN{Value: "1"},
			},
		},
		{
			name: "empty document",
			raw:  `{}`,
			want: map[string]ddbtypes.AttributeValue{},
		},
	}
}

// decodeDynamoDBJSONErrorTestCases covers malformed input at every level:
// the top-level document, a single attribute value, and values nested inside
// B/BS base64 and L/M recursion.
func decodeDynamoDBJSONErrorTestCases() []decodeDynamoDBJSONTestCase {
	return []decodeDynamoDBJSONTestCase{
		{
			name:    "invalid top-level JSON",
			raw:     `not json`,
			wantErr: true,
		},
		{
			name:    "attribute value is not an object",
			raw:     `{"pk":"a"}`,
			wantErr: true,
		},
		{
			name:    "no type member set",
			raw:     `{"pk":{}}`,
			wantErr: true,
		},
		{
			name:    "ambiguous type members",
			raw:     `{"pk":{"S":"a","N":"1"}}`,
			wantErr: true,
		},
		{
			name:    "invalid base64 for B",
			raw:     `{"b":{"B":"not-base64!"}}`,
			wantErr: true,
		},
		{
			name:    "invalid base64 within BS",
			raw:     `{"bs":{"BS":["aGVsbG8=","not-base64!"]}}`,
			wantErr: true,
		},
		{
			name:    "error inside nested L propagates",
			raw:     `{"l":{"L":[{"S":"a"},{}]}}`,
			wantErr: true,
		},
		{
			name:    "error inside nested M propagates",
			raw:     `{"m":{"M":{"bad":{"S":"a","N":"1"}}}}`,
			wantErr: true,
		},
	}
}

func TestDecodeDynamoDBJSON(t *testing.T) {
	t.Parallel()

	scalar := decodeDynamoDBJSONScalarTestCases()
	collection := decodeDynamoDBJSONCollectionTestCases()
	nested := decodeDynamoDBJSONNestedTestCases()
	errCases := decodeDynamoDBJSONErrorTestCases()

	cases := make([]decodeDynamoDBJSONTestCase, 0, len(scalar)+len(collection)+len(nested)+len(errCases))
	cases = append(cases, scalar...)
	cases = append(cases, collection...)
	cases = append(cases, nested...)
	cases = append(cases, errCases...)

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checkDecodeDynamoDBJSON(t, &tt)
		})
	}
}

func checkDecodeDynamoDBJSON(t *testing.T, tt *decodeDynamoDBJSONTestCase) {
	t.Helper()

	got, err := decodeDynamoDBJSON(tt.raw)
	if tt.wantErr {
		if err == nil {
			t.Fatal("expected an error, got nil")
		}

		return
	}

	if err != nil {
		t.Fatalf("decodeDynamoDBJSON: %v", err)
	}

	if !reflect.DeepEqual(got, tt.want) {
		t.Errorf("decodeDynamoDBJSON() = %#v, want %#v", got, tt.want)
	}
}
