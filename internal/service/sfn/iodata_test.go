package sfn

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestParsePathField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		raw        json.RawMessage
		wantPath   string
		wantIsNull bool
		wantErr    bool
	}{
		{name: "absent defaults to $", raw: nil, wantPath: "$", wantIsNull: false},
		{name: "explicit null", raw: json.RawMessage(`null`), wantIsNull: true},
		{name: "string path", raw: json.RawMessage(`"$.a.b"`), wantPath: "$.a.b"},
		{name: "non-string value errors", raw: json.RawMessage(`42`), wantErr: true},
		{name: "malformed JSON errors", raw: json.RawMessage(`{not json`), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path, isNull, err := parsePathField(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parsePathField: want error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("parsePathField: %v", err)
			}

			if isNull != tt.wantIsNull {
				t.Fatalf("isNull: got %v, want %v", isNull, tt.wantIsNull)
			}

			if !isNull && path != tt.wantPath {
				t.Fatalf("path: got %q, want %q", path, tt.wantPath)
			}
		})
	}
}

func TestApplyInputPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     json.RawMessage
		input   string
		want    string
		wantErr bool
	}{
		{name: "absent leaves input unchanged", raw: nil, input: `{"a":1}`, want: `{"a":1}`},
		{name: "explicit $ leaves input unchanged", raw: json.RawMessage(`"$"`), input: `{"a":1}`, want: `{"a":1}`},
		{name: "explicit null discards input", raw: json.RawMessage(`null`), input: `{"a":1}`, want: `{}`},
		{name: "single-level field", raw: json.RawMessage(`"$.a"`), input: `{"a":{"b":1}}`, want: `{"b":1}`},
		{name: "arbitrary depth field", raw: json.RawMessage(`"$.a.b.c"`), input: `{"a":{"b":{"c":42}}}`, want: `42`},
		{name: "missing field errors", raw: json.RawMessage(`"$.missing"`), input: `{"a":1}`, wantErr: true},
		{name: "field access on non-object errors", raw: json.RawMessage(`"$.a.b"`), input: `{"a":"not-an-object"}`, wantErr: true},
		{name: "malformed input passes through unchanged when path is absent", raw: nil, input: `{not json`, want: `{not json`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := applyInputPath(tt.raw, tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("applyInputPath: want error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("applyInputPath: %v", err)
			}

			if got != tt.want {
				t.Fatalf("applyInputPath: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyInputPathMalformedInputWithPathErrors(t *testing.T) {
	t.Parallel()

	// Unlike the "$"-default case (which returns malformed input verbatim,
	// since nothing needs to be parsed), a non-"$" path requires parsing the
	// input and must fail cleanly on malformed JSON.
	_, err := applyInputPath(json.RawMessage(`"$.a"`), `{not json`)
	if err == nil {
		t.Fatal("applyInputPath: want error for malformed input, got nil")
	}
}

func TestApplyOutputPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     json.RawMessage
		output  string
		want    string
		wantErr bool
	}{
		{name: "absent leaves output unchanged", raw: nil, output: `{"a":1}`, want: `{"a":1}`},
		{name: "explicit null discards output", raw: json.RawMessage(`null`), output: `{"a":1}`, want: `{}`},
		{name: "narrows to a field", raw: json.RawMessage(`"$.a"`), output: `{"a":{"b":2}}`, want: `{"b":2}`},
		{name: "path matching nothing errors", raw: json.RawMessage(`"$.missing"`), output: `{"a":1}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := applyOutputPath(tt.raw, tt.output)
			if tt.wantErr {
				if err == nil {
					t.Fatal("applyOutputPath: want error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("applyOutputPath: %v", err)
			}

			if got != tt.want {
				t.Fatalf("applyOutputPath: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyResultSelector(t *testing.T) {
	t.Parallel()

	// A nil selector has no effect: the result passes through unchanged.
	got, err := applyResultSelector(nil, `{"status":"ok"}`)
	if err != nil {
		t.Fatalf("applyResultSelector: %v", err)
	}

	if got != `{"status":"ok"}` {
		t.Fatalf("applyResultSelector with nil selector: got %q, want unchanged result", got)
	}

	// A selector resolves ".$" references against the result, not the
	// state's input.
	selector := map[string]any{
		"status.$": "$.status",
		"fixed":    "value",
	}

	got, err = applyResultSelector(selector, `{"status":"ok","ignored":true}`)
	if err != nil {
		t.Fatalf("applyResultSelector: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("unmarshal applyResultSelector output %q: %v", got, err)
	}

	if decoded["status"] != "ok" || decoded["fixed"] != "value" {
		t.Fatalf("applyResultSelector output: got %v, want status=ok fixed=value", decoded)
	}

	if _, ok := decoded["ignored"]; ok {
		t.Fatalf("applyResultSelector output %v: must not carry fields the selector did not select", decoded)
	}
}

// applyResultPathTests exercises applyResultPath's null/"$"/arbitrary-depth
// semantics. Kept as a package-level var, mirroring choiceRuleTests in
// choice_test.go, so the driving test function itself stays short.
var applyResultPathTests = []struct {
	name           string
	raw            json.RawMessage
	effectiveInput string
	result         string
	want           string
	wantErr        bool
}{
	{
		name: "absent defaults to $ replaces input entirely",
		raw:  nil, effectiveInput: `{"a":1}`, result: `{"sum":7}`,
		want: `{"sum":7}`,
	},
	{
		name: "explicit null discards result, keeps effective input",
		raw:  json.RawMessage(`null`), effectiveInput: `{"a":1}`, result: `{"sum":7}`,
		want: `{"a":1}`,
	},
	{
		name: "single-level path merges into input",
		raw:  json.RawMessage(`"$.sum"`), effectiveInput: `{"a":1}`, result: `7`,
		want: `{"a":1,"sum":7}`,
	},
	{
		name: "arbitrary depth path creates intermediate objects",
		raw:  json.RawMessage(`"$.master.result.sum"`), effectiveInput: `{"master":{"detail":[1,2,3]}}`, result: `6`,
		want: `{"master":{"detail":[1,2,3],"result":{"sum":6}}}`,
	},
	{
		name: "path overwrites an existing field",
		raw:  json.RawMessage(`"$.master.detail"`), effectiveInput: `{"master":{"detail":[1,2,3]}}`, result: `6`,
		want: `{"master":{"detail":6}}`,
	},
	{
		name: "non-object base fails to apply",
		raw:  json.RawMessage(`"$.x"`), effectiveInput: `"foo"`, result: `"bar"`,
		wantErr: true,
	},
	{
		name: "path without leading $. errors",
		raw:  json.RawMessage(`"foo"`), effectiveInput: `{}`, result: `1`,
		wantErr: true,
	},
}

func TestApplyResultPath(t *testing.T) {
	t.Parallel()

	for _, tt := range applyResultPathTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := applyResultPath(tt.raw, tt.effectiveInput, tt.result)
			if tt.wantErr {
				if err == nil {
					t.Fatal("applyResultPath: want error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("applyResultPath: %v", err)
			}

			assertJSONEqual(t, got, tt.want)
		})
	}
}

func TestResolveEffectiveInput(t *testing.T) {
	t.Parallel()

	// With neither InputPath nor Parameters set, the effective input is the
	// raw input unchanged.
	got, err := resolveEffectiveInput(nil, nil, `{"a":1}`)
	if err != nil {
		t.Fatalf("resolveEffectiveInput: %v", err)
	}

	if got != `{"a":1}` {
		t.Fatalf("resolveEffectiveInput with neither field: got %q, want unchanged input", got)
	}

	// InputPath narrows the raw input before Parameters resolves against
	// that narrowed value.
	got, err = resolveEffectiveInput(
		json.RawMessage(`"$.numbers"`),
		map[string]any{"total.$": "$.val1"},
		`{"numbers":{"val1":3,"val2":4}}`,
	)
	if err != nil {
		t.Fatalf("resolveEffectiveInput: %v", err)
	}

	assertJSONEqual(t, got, `{"total":3}`)
}

// assertJSONEqual fails the test unless got and want decode to
// deep-equal JSON values, so tests are not sensitive to key ordering
// (encoding/json's map marshaling order is not guaranteed to match a
// hand-written literal).
func assertJSONEqual(t *testing.T, got, want string) {
	t.Helper()

	var gotVal, wantVal any
	if err := json.Unmarshal([]byte(got), &gotVal); err != nil {
		t.Fatalf("unmarshal got %q: %v", got, err)
	}

	if err := json.Unmarshal([]byte(want), &wantVal); err != nil {
		t.Fatalf("unmarshal want %q: %v", want, err)
	}

	gotNorm, err := json.Marshal(gotVal)
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}

	wantNorm, err := json.Marshal(wantVal)
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}

	if !bytes.Equal(gotNorm, wantNorm) {
		t.Fatalf("JSON mismatch: got %s, want %s", got, want)
	}
}
