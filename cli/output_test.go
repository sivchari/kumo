package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
)

// testConnectionArn and testConnectionArnField are shared across the
// queryJSON / formatText / writeOutput test tables below.
const (
	testConnectionArn      = "arn:aws:events:us-east-1:123456789012:connection/x"
	testConnectionArnField = "ConnectionArn"
)

type queryJSONTestCase struct {
	name    string
	query   string
	want    any
	wantErr bool
}

func queryJSONTestCases() []queryJSONTestCase {
	return []queryJSONTestCase{
		{
			name:  "top-level field",
			query: testConnectionArnField,
			want:  testConnectionArn,
		},
		{
			name:  "nested field",
			query: "Nested.Inner",
			want:  "value",
		},
		{
			name:  "array index",
			query: "AttributeDefinitions[0].AttributeName",
			want:  "pk",
		},
		{
			name:  "second array index",
			query: "AttributeDefinitions[1].AttributeName",
			want:  "sk",
		},
		{
			name:    "unknown field",
			query:   "DoesNotExist",
			wantErr: true,
		},
		{
			name:    "index out of range",
			query:   "AttributeDefinitions[5].AttributeName",
			wantErr: true,
		},
		{
			name:    "index on non-array",
			query:   "ConnectionArn[0]",
			wantErr: true,
		},
	}
}

func TestQueryJSON(t *testing.T) {
	var doc any

	raw := fmt.Sprintf(`{%q:%q,"AttributeDefinitions":[{"AttributeName":"pk"},{"AttributeName":"sk"}],"Nested":{"Inner":"value"}}`,
		testConnectionArnField, testConnectionArn)
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	for _, tt := range queryJSONTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			got, err := queryJSON(doc, tt.query)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("queryJSON(%q): want error, got nil", tt.query)
				}

				return
			}

			if err != nil {
				t.Fatalf("queryJSON(%q): %v", tt.query, err)
			}

			if got != tt.want {
				t.Errorf("queryJSON(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestFormatText(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{name: "string", in: testConnectionArn, want: testConnectionArn},
		{name: "nil", in: nil, want: ""},
		{name: "bool", in: true, want: "true"},
		{name: "number", in: float64(42), want: "42"},
		{name: "object falls back to JSON", in: map[string]any{"A": "b"}, want: `{"A":"b"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatText(tt.in); got != tt.want {
				t.Errorf("formatText(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// sample mirrors an aws-sdk-go-v2 Output struct, which carries no json
// struct tags: encoding/json then marshals it using the Go field name
// verbatim (PascalCase), the same shape real generated commands pass to
// writeOutput.
type sample struct {
	ConnectionArn string
}

func TestWriteOutput(t *testing.T) {
	tests := []struct {
		name         string
		queryFlag    string
		outputFormat string
		want         string
	}{
		{
			name: "no query prints JSON",
			want: "{\"ConnectionArn\":\"arn:aws:events:us-east-1:123456789012:connection/x\"}\n",
		},
		{
			name:      "query without --output=text still prints JSON",
			queryFlag: testConnectionArnField,
			want:      "\"arn:aws:events:us-east-1:123456789012:connection/x\"\n",
		},
		{
			name:         "query with --output=text prints raw text",
			queryFlag:    testConnectionArnField,
			outputFormat: outputFormatText,
			want:         "arn:aws:events:us-east-1:123456789012:connection/x\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				queryFlag = ""
				outputFormat = ""
			})

			queryFlag = tt.queryFlag
			outputFormat = tt.outputFormat

			got := captureStdout(t, func() {
				if err := writeOutput(sample{ConnectionArn: testConnectionArn}); err != nil {
					t.Fatalf("writeOutput: %v", err)
				}
			})

			if got != tt.want {
				t.Errorf("writeOutput output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteOutput_QueryNotFound(t *testing.T) {
	t.Cleanup(func() {
		queryFlag = ""
		outputFormat = ""
	})

	queryFlag = "DoesNotExist"

	err := writeOutput(sample{ConnectionArn: "x"})
	if err == nil {
		t.Fatal("writeOutput: want error for unmatched query, got nil")
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	orig := os.Stdout
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}

	os.Stdout = orig

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}

	return buf.String()
}
