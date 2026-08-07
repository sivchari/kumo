package cligen

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func fieldByName(t reflect.Type, name string) *reflect.StructField {
	f, ok := t.FieldByName(name)
	if !ok {
		panic("no field " + name + " on " + t.String())
	}

	return &f
}

type classifyFieldTestCase struct {
	name             string
	field            *reflect.StructField
	wantKind         FlagKind
	wantOverride     bool
	wantElemNonNil   bool
	wantPointerField bool
}

// provisionedThroughputHolder gives ProvisionedThroughput (a flat,
// all-scalar struct: *int64, *int64) a StructField to classify, the same way
// it appears embedded in a real SDK Input struct.
type provisionedThroughputHolder struct {
	ProvisionedThroughput *dynamodbtypes.ProvisionedThroughput
}

func classifyFieldTestCases() []classifyFieldTestCase {
	return []classifyFieldTestCase{
		{
			name:             "string pointer field",
			field:            fieldByName(reflect.TypeOf(sqs.CreateQueueInput{}), "QueueName"),
			wantKind:         FlagString,
			wantPointerField: true,
		},
		{
			name:     "map[string]string field",
			field:    fieldByName(reflect.TypeOf(sqs.CreateQueueInput{}), "Attributes"),
			wantKind: FlagStringMap,
		},
		{
			name:     "int32 pointer field",
			field:    fieldByName(reflect.TypeOf(kinesis.CreateStreamInput{}), "ShardCount"),
			wantKind: FlagInt32,
		},
		{
			name:           "map[string]struct field falls back to JSON blob",
			field:          fieldByName(reflect.TypeOf(sqs.SendMessageInput{}), "MessageAttributes"),
			wantKind:       FlagJSONBlob,
			wantElemNonNil: false,
		},
		{
			name:           "named string enum in a types subpackage",
			field:          fieldByName(reflect.TypeOf(eventbridge.PutRuleInput{}), "State"),
			wantKind:       FlagStringEnum,
			wantElemNonNil: true,
		},
		{
			name:           "flat scalar-only struct pointer",
			field:          fieldByName(reflect.TypeOf(provisionedThroughputHolder{}), "ProvisionedThroughput"),
			wantKind:       FlagKV,
			wantElemNonNil: true,
		},
	}
}

func TestClassifyField_Kinds(t *testing.T) {
	t.Parallel()

	for _, tt := range classifyFieldTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checkClassifyField(t, &tt)
		})
	}
}

func checkClassifyField(t *testing.T, tt *classifyFieldTestCase) {
	t.Helper()

	flag, requiresOverride := classifyField(tt.field)
	if requiresOverride != tt.wantOverride {
		t.Fatalf("requiresOverride = %v, want %v", requiresOverride, tt.wantOverride)
	}

	if requiresOverride {
		return
	}

	if flag.Kind != tt.wantKind {
		t.Errorf("Kind = %v, want %v", flag.Kind, tt.wantKind)
	}

	if (flag.ElemType != nil) != tt.wantElemNonNil {
		t.Errorf("ElemType non-nil = %v, want %v", flag.ElemType != nil, tt.wantElemNonNil)
	}
}

func TestClassifyField_InterfaceUnionRequiresOverride(t *testing.T) {
	t.Parallel()

	// dynamodb PutItemInput.Item is map[string]types.AttributeValue, where
	// AttributeValue is a Smithy union (interface) type.
	type holder struct {
		Item map[string]dynamodbtypes.AttributeValue
	}

	f := fieldByName(reflect.TypeOf(holder{}), "Item")

	_, requiresOverride := classifyField(f)
	if !requiresOverride {
		t.Fatal("expected requiresOverride for a map with an interface-typed element")
	}
}

func TestClassifyField_ByteSliceIsBase64(t *testing.T) {
	t.Parallel()

	type holder struct {
		Blob []byte
	}

	f := fieldByName(reflect.TypeOf(holder{}), "Blob")

	flag, requiresOverride := classifyField(f)
	if requiresOverride {
		t.Fatal("unexpected requiresOverride")
	}

	if flag.Kind != FlagBytesBase64 {
		t.Errorf("Kind = %v, want FlagBytesBase64", flag.Kind)
	}
}

func TestClassifyField_StringSliceIsStringArray(t *testing.T) {
	t.Parallel()

	type holder struct {
		Names []string
	}

	f := fieldByName(reflect.TypeOf(holder{}), "Names")

	flag, _ := classifyField(f)
	if flag.Kind != FlagStringArray {
		t.Errorf("Kind = %v, want FlagStringArray", flag.Kind)
	}
}

func TestClassifyField_TimeIsRFC3339(t *testing.T) {
	t.Parallel()

	type holder struct {
		StartedAt *time.Time
	}

	f := fieldByName(reflect.TypeOf(holder{}), "StartedAt")

	flag, _ := classifyField(f)
	if flag.Kind != FlagTime {
		t.Errorf("Kind = %v, want FlagTime", flag.Kind)
	}
}

func TestAllScalarLeaves(t *testing.T) {
	t.Parallel()

	if !allScalarLeaves(reflect.TypeOf(dynamodbtypes.ProvisionedThroughput{})) {
		t.Error("ProvisionedThroughput should be all-scalar-leaves")
	}

	// MessageAttributeValue has a BinaryListValues [][]byte field, which is
	// not a scalar leaf.
	type nested struct {
		BinaryListValues [][]byte
	}

	if allScalarLeaves(reflect.TypeOf(nested{})) {
		t.Error("a struct with a slice-of-slice field should not be all-scalar-leaves")
	}
}

func TestHasOutputContent(t *testing.T) {
	t.Parallel()

	if hasOutputContent(reflect.TypeOf(kinesis.CreateStreamOutput{})) {
		t.Error("CreateStreamOutput has no fields beyond ResultMetadata, want hasOutputContent = false")
	}

	if !hasOutputContent(reflect.TypeOf(sqs.CreateQueueOutput{})) {
		t.Error("CreateQueueOutput has QueueUrl beyond ResultMetadata, want hasOutputContent = true")
	}
}

// TestDeriveFields_FlagNameOverride proves states.SendTaskSuccessInput's
// Output field is renamed to --task-output (the AWS CLI's own name for that
// flag), since the auto-derived --output would otherwise collide with the
// root command's persistent --output format flag.
func TestDeriveFields_FlagNameOverride(t *testing.T) {
	t.Parallel()

	result := DeriveFields("states", "SendTaskSuccess", reflect.TypeOf(sfn.SendTaskSuccessInput{}))
	if result.RequiresOverride {
		t.Fatalf("SendTaskSuccessInput unexpectedly requires an override: %s", result.OverrideReason)
	}

	var found bool

	for _, f := range result.Flags {
		if f.FieldName != "Output" {
			continue
		}

		found = true

		if f.FlagName != "task-output" {
			t.Errorf("Output field FlagName = %q, want %q", f.FlagName, "task-output")
		}
	}

	if !found {
		t.Fatal("no Output field found in derived flags")
	}
}

func TestSetFieldsFromKV(t *testing.T) {
	t.Parallel()

	var pt dynamodbtypes.ProvisionedThroughput

	if err := SetFieldsFromKV(reflect.ValueOf(&pt).Elem(), "read-capacity-units=5,write-capacity-units=10"); err != nil {
		t.Fatalf("SetFieldsFromKV: %v", err)
	}

	if pt.ReadCapacityUnits == nil || *pt.ReadCapacityUnits != 5 {
		t.Errorf("ReadCapacityUnits = %v, want 5", pt.ReadCapacityUnits)
	}

	if pt.WriteCapacityUnits == nil || *pt.WriteCapacityUnits != 10 {
		t.Errorf("WriteCapacityUnits = %v, want 10", pt.WriteCapacityUnits)
	}
}

func TestSetFieldsFromKV_UnknownField(t *testing.T) {
	t.Parallel()

	var pt dynamodbtypes.ProvisionedThroughput

	if err := SetFieldsFromKV(reflect.ValueOf(&pt).Elem(), "does-not-exist=1"); err == nil {
		t.Fatal("expected an error for an unknown field key")
	}
}

// TestSetFieldsFromKV_PascalCaseKeys proves the AWS CLI's standard shorthand
// casing (SDK member names, e.g. "ReadCapacityUnits=5") is accepted
// alongside the kebab-case flag names cli-gen otherwise derives, since real
// AWS CLI users and scripts routinely pass PascalCase keys.
func TestSetFieldsFromKV_PascalCaseKeys(t *testing.T) {
	t.Parallel()

	var pt dynamodbtypes.ProvisionedThroughput

	if err := SetFieldsFromKV(reflect.ValueOf(&pt).Elem(), "ReadCapacityUnits=5,WriteCapacityUnits=10"); err != nil {
		t.Fatalf("SetFieldsFromKV: %v", err)
	}

	if pt.ReadCapacityUnits == nil || *pt.ReadCapacityUnits != 5 {
		t.Errorf("ReadCapacityUnits = %v, want 5", pt.ReadCapacityUnits)
	}

	if pt.WriteCapacityUnits == nil || *pt.WriteCapacityUnits != 10 {
		t.Errorf("WriteCapacityUnits = %v, want 10", pt.WriteCapacityUnits)
	}
}

// TestJSONUnmarshalIntoSDKStruct proves the assumption FlagJSONBlob relies
// on: aws-sdk-go-v2 generated types carry no json struct tags, but
// encoding/json matches field names case-insensitively by default, so a raw
// JSON blob unmarshals correctly into an SDK nested struct.
func TestJSONUnmarshalIntoSDKStruct(t *testing.T) {
	t.Parallel()

	var pt dynamodbtypes.ProvisionedThroughput

	raw := []byte(`{"ReadCapacityUnits":5,"WriteCapacityUnits":10}`)
	if err := json.Unmarshal(raw, &pt); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if pt.ReadCapacityUnits == nil || *pt.ReadCapacityUnits != 5 {
		t.Errorf("ReadCapacityUnits = %v, want 5", pt.ReadCapacityUnits)
	}

	if pt.WriteCapacityUnits == nil || *pt.WriteCapacityUnits != 10 {
		t.Errorf("WriteCapacityUnits = %v, want 10", pt.WriteCapacityUnits)
	}

	// Also prove the double-indirection trick used for pointer fields
	// (json.Unmarshal into &input.Field where Field is itself a pointer type).
	var target *dynamodbtypes.ProvisionedThroughput

	if err := json.Unmarshal(raw, &target); err != nil {
		t.Fatalf("json.Unmarshal into pointer field: %v", err)
	}

	if target == nil || target.ReadCapacityUnits == nil || *target.ReadCapacityUnits != 5 {
		t.Errorf("target = %+v, want ReadCapacityUnits=5", target)
	}

	// Case-insensitive field matching: lowercase JSON keys still match.
	var lower dynamodbtypes.ProvisionedThroughput

	if err := json.Unmarshal([]byte(`{"readcapacityunits":7,"writecapacityunits":8}`), &lower); err != nil {
		t.Fatalf("json.Unmarshal (lowercase keys): %v", err)
	}

	if lower.ReadCapacityUnits == nil || *lower.ReadCapacityUnits != 7 {
		t.Errorf("ReadCapacityUnits = %v, want 7 (case-insensitive match)", lower.ReadCapacityUnits)
	}
}

func TestIsStringEnum(t *testing.T) {
	t.Parallel()

	if !isStringEnum(reflect.TypeOf(ebtypes.RuleState(""))) {
		t.Error("RuleState should be classified as a string enum")
	}

	if isStringEnum(reflect.TypeOf("")) {
		t.Error("plain string should not be classified as a string enum")
	}
}
