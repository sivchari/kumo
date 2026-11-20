package dynamodb

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestParseUpdateClauses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr string
		want []updateClause
	}{
		{
			name: "single set",
			expr: "SET a = :s",
			want: []updateClause{{action: updateActionSet, body: "a = :s"}},
		},
		{
			name: "lower case and newline separated",
			expr: "set a = :s\nremove b\n",
			want: []updateClause{{action: updateActionSet, body: "a = :s"}, {action: updateActionRem, body: "b"}},
		},
		{
			name: "placeholder looking like a keyword stays in its clause",
			expr: "SET #Remove = :s, #Set = if_not_exists(#Set, :Add)",
			want: []updateClause{{action: updateActionSet, body: "#Remove = :s, #Set = if_not_exists(#Set, :Add)"}},
		},
		{
			name: "all four clauses",
			expr: "SET a = a + :v ADD b :n DELETE c :d REMOVE e, f",
			want: []updateClause{
				{action: updateActionSet, body: "a = a + :v"},
				{action: updateActionAdd, body: "b :n"},
				{action: updateActionDel, body: "c :d"},
				{action: updateActionRem, body: "e, f"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseUpdateClauses(tt.expr)
			if err != nil {
				t.Fatalf("parseUpdateClauses(%q): %v", tt.expr, err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseUpdateClauses(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

// assertValidationError fails unless err is a ValidationException TableError
// carrying exactly want.
func assertValidationError(t *testing.T, expr string, err error, want string) {
	t.Helper()

	var tableErr *TableError
	if !errors.As(err, &tableErr) {
		t.Fatalf("parseUpdateClauses(%q) error = %v, want *TableError", expr, err)
	}

	if tableErr.Code != errCodeValidation || tableErr.Message != want {
		t.Errorf("parseUpdateClauses(%q) = %s: %q, want %s: %q", expr, tableErr.Code, tableErr.Message, errCodeValidation, want)
	}
}

func TestParseUpdateClauses_DuplicateSection(t *testing.T) {
	t.Parallel()

	const prefix = "Invalid UpdateExpression: "

	tests := []struct {
		name string
		expr string
		want string
	}{
		{
			name: "same section twice",
			expr: "SET a = :s SET c = :s",
			want: prefix + `The "SET" section can only be used once in an update expression;`,
		},
		{
			name: "remove across case and newline",
			expr: "remove a\n\tREMOVE b",
			want: prefix + `The "REMOVE" section can only be used once in an update expression;`,
		},
		{
			name: "add separated by another clause",
			expr: "ADD a :v SET b = :v Add c :v",
			want: prefix + `The "ADD" section can only be used once in an update expression;`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseUpdateClauses(tt.expr)
			assertValidationError(t, tt.expr, err, tt.want)
		})
	}
}

func TestParseUpdateClauses_SyntaxError(t *testing.T) {
	t.Parallel()

	const prefix = "Invalid UpdateExpression: "

	tests := []struct {
		name string
		expr string
		want string
	}{
		{"unknown trailing clause", "SET a = :s FOO b", prefix + `Syntax error; token: "FOO", near: ":s FOO b"`},
		{"unknown leading clause", "FOO SET a = :s", prefix + `Syntax error; token: "FOO", near: "FOO SET"`},
		{"remove with trailing garbage", "REMOVE a b", prefix + `Syntax error; token: "b", near: "a b"`},
		{"empty clause body at end", "SET a = :s REMOVE", prefix + `Syntax error; token: "<EOF>", near: "REMOVE"`},
		{"empty clause body before next clause", "SET REMOVE a", prefix + `Syntax error; token: "REMOVE", near: "SET REMOVE a"`},
		{"missing value", "SET a =", prefix + `Syntax error; token: "<EOF>", near: "="`},
		{"trailing comma", "REMOVE a,", prefix + `Syntax error; token: "<EOF>", near: ","`},
		{"whitespace only", " \n", prefix + "The expression can not be empty;"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseUpdateClauses(tt.expr)
			assertValidationError(t, tt.expr, err, tt.want)
		})
	}
}

func TestResolveUpdateNames(t *testing.T) {
	t.Parallel()

	names := map[string]string{"#1": "one", "#10": "ten", "#n": "renamed"}

	got := resolveUpdateNames("#1 = #10, #n = #undefined", names)

	const want = "one = ten, renamed = #undefined"
	if got != want {
		t.Errorf("resolveUpdateNames() = %q, want %q", got, want)
	}
}

// A name placeholder that resolves to a clause keyword must not split the
// expression; the update used to be dropped silently.
func TestUpdateItemKeywordNamedAttribute(t *testing.T) {
	t.Parallel()

	const removeName = "Remove"

	s := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName: "test-keyword-attribute",
		KeySchema: []KeySchemaElement{
			{AttributeName: "pk", KeyType: keyTypeHash},
		},
		AttributeDefinitions: []AttributeDefinition{
			{AttributeName: "pk", AttributeType: "S"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	key := Item{"pk": {S: new("k")}}

	result, err := s.UpdateItem(
		ctx,
		"test-keyword-attribute",
		key,
		"SET #n = :s, #m = :s REMOVE #r",
		map[string]string{"#n": removeName, "#m": "set", "#r": "Add"},
		map[string]AttributeValue{":s": {S: new("v")}},
		ReturnValuesAllNew,
		ConditionInput{},
	)
	if err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}

	for _, name := range []string{removeName, "set"} {
		if got := result[name].S; got == nil || *got != "v" {
			t.Errorf("attribute %q = %v, want v", name, result[name])
		}
	}
}
