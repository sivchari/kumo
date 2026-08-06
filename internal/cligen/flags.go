package cligen

import (
	"reflect"
	"strings"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})

// DeriveFields derives CLI flags for every exported, non-marker field of
// inputType (an aws-sdk-go-v2 *XxxInput struct's element type). serviceName
// and action select ignoredBackCompatFlags and fieldOverrides entries.
func DeriveFields(serviceName, action string, inputType reflect.Type) FieldsResult {
	var result FieldsResult

	for i := range inputType.NumField() {
		f := inputType.Field(i)
		if !f.IsExported() {
			continue
		}

		if isIgnoredBackCompatFlag(serviceName, action, f.Name) {
			continue
		}

		if override, ok := fieldOverrides[inputType][f.Name]; ok {
			result.Flags = append(result.Flags, FieldFlag{
				FieldName:  f.Name,
				FlagName:   toKebabCase(f.Name),
				Kind:       override.Kind,
				CustomFunc: override.CustomFunc,
				Pointer:    f.Type.Kind() == reflect.Pointer,
			})

			continue
		}

		flag, requiresOverride := classifyField(&f)
		if requiresOverride {
			result.RequiresOverride = true
			result.OverrideReason = "field " + f.Name + " is an interface/union type (e.g. a Smithy union such as dynamodb's AttributeValue); it needs a hand-written override"

			continue
		}

		result.Flags = append(result.Flags, flag)
	}

	return result
}

// simpleKindFlags maps reflect.Kind values that need no further inspection
// directly onto a FlagKind (string and the container kinds - Slice, Map,
// Struct - need extra logic and are handled separately in classifyField).
var simpleKindFlags = map[reflect.Kind]FlagKind{
	reflect.Int32: FlagInt32,
	reflect.Int64: FlagInt64,
	reflect.Bool:  FlagBool,
}

// classifyField derives the FieldFlag for a single Input struct field, or
// reports requiresOverride when the field's type cannot be auto-derived
// (an interface/union type, directly or as a map/slice element).
func classifyField(f *reflect.StructField) (FieldFlag, bool) {
	flag := FieldFlag{
		FieldName: f.Name,
		FlagName:  toKebabCase(f.Name),
		Pointer:   f.Type.Kind() == reflect.Pointer,
	}

	t := f.Type
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if fieldRequiresOverride(t) {
		return FieldFlag{}, true
	}

	classifyFieldType(&flag, t)

	return flag, false
}

// classifyFieldType sets flag.Kind (and flag.ElemType, where relevant) from
// t, t's reflect.Kind having already been confirmed not to require an
// override.
func classifyFieldType(flag *FieldFlag, t reflect.Type) {
	if t.Kind() == reflect.String {
		classifyStringField(flag, t)

		return
	}

	if kind, ok := simpleKindFlags[t.Kind()]; ok {
		flag.Kind = kind

		return
	}

	if t.Kind() == reflect.Slice {
		classifySliceField(flag, t)

		return
	}

	if t.Kind() == reflect.Map {
		classifyMapField(flag, t)

		return
	}

	if t.Kind() == reflect.Struct {
		classifyStructField(flag, t)

		return
	}

	flag.Kind = FlagJSONBlob
}

func classifyStringField(flag *FieldFlag, t reflect.Type) {
	if isStringEnum(t) {
		flag.Kind = FlagStringEnum
		flag.ElemType = t

		return
	}

	flag.Kind = FlagString
}

func classifyMapField(flag *FieldFlag, t reflect.Type) {
	if t.Key().Kind() == reflect.String && t.Elem().Kind() == reflect.String {
		flag.Kind = FlagStringMap

		return
	}

	flag.Kind = FlagJSONBlob
}

func classifyStructField(flag *FieldFlag, t reflect.Type) {
	switch {
	case t == timeType:
		flag.Kind = FlagTime
	case allScalarLeaves(t):
		flag.Kind = FlagKV
		flag.ElemType = t
	default:
		flag.Kind = FlagJSONBlob
	}
}

func classifySliceField(flag *FieldFlag, t reflect.Type) {
	switch {
	case t.Elem().Kind() == reflect.Uint8:
		flag.Kind = FlagBytesBase64
	case t.Elem().Kind() == reflect.String && isStringEnum(t.Elem()):
		flag.Kind = FlagStringEnumArray
		flag.ElemType = t.Elem()
	case t.Elem().Kind() == reflect.String:
		flag.Kind = FlagStringArray
	case t.Elem().Kind() == reflect.Struct && allScalarLeaves(t.Elem()):
		flag.Kind = FlagKVArray
		flag.ElemType = t.Elem()
	default:
		flag.Kind = FlagJSONBlob
	}
}

// fieldRequiresOverride reports whether t (already pointer-unwrapped) is, or
// directly contains as a map/slice element, an interface (Smithy union) type
// that cli-gen cannot derive a flag for.
func fieldRequiresOverride(t reflect.Type) bool {
	if t.Kind() == reflect.Interface {
		return true
	}

	if t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
		return t.Elem().Kind() == reflect.Interface
	}

	return false
}

// isStringEnum reports whether t is a named string type declared in an SDK
// "types" subpackage (e.g. sqs/types.QueueAttributeName), as opposed to the
// plain builtin string type.
func isStringEnum(t reflect.Type) bool {
	return t.Name() != "" && strings.Contains(t.PkgPath(), "/types")
}

// isScalarLeaf reports whether t (a struct field type) is simple enough to
// be parsed from a single "key=value" pair by SetFieldsFromKV: a string,
// integer, bool, enum, or []byte (base64) - anything setScalarField can
// actually populate. A struct with any other field (a nested struct, a
// []string, a map, ...) is not all-scalar-leaves and instead falls back to
// FlagJSONBlob, where json.Unmarshal handles every field uniformly.
func isScalarLeaf(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() == reflect.String || t.Kind() == reflect.Int32 || t.Kind() == reflect.Int64 || t.Kind() == reflect.Bool {
		return true
	}

	return t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8
}

// allScalarLeaves reports whether every exported field of t is a scalar
// leaf, i.e. t can be fully populated from a flat "key=value,key2=value2"
// string by setFieldsFromKV.
func allScalarLeaves(t reflect.Type) bool {
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		if !isScalarLeaf(f.Type) {
			return false
		}
	}

	return true
}

// hasOutputContent reports whether outputType (an aws-sdk-go-v2 *XxxOutput
// struct's element type) has any exported field beyond ResultMetadata, i.e.
// whether the generated command should print the response.
func hasOutputContent(outputType reflect.Type) bool {
	for i := range outputType.NumField() {
		f := outputType.Field(i)
		if f.Name == "ResultMetadata" || !f.IsExported() {
			continue
		}

		return true
	}

	return false
}
