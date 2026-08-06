// Package cligen discovers CLI-able actions from kumo's service registry and
// renders generated Cobra command files (cli/gen_{service}.go). It mirrors
// the internal/catalog pattern: a pure Render function plus a drift test that
// verifies committed output stays in sync.
//
// Source of truth is the server itself (internal/registry / service.Services()):
// Query protocol services expose their actions via Actions(), JSON protocol
// services are scanned via reflection for exported handler methods, and REST
// services (RegisterRoutes only) are skipped as auto-discovery candidates.
// Discovered action names are then resolved against the corresponding
// aws-sdk-go-v2 client's method set (case-insensitively) to adopt the SDK's
// canonical wire name and Input/Output types.
package cligen

import "reflect"

// DiagnosticLevel classifies a Diagnostic.
type DiagnosticLevel string

const (
	// LevelGenerated marks a diagnostic describing successfully generated output.
	LevelGenerated DiagnosticLevel = "generated"
	// LevelSkip marks a diagnostic describing something cli-gen intentionally
	// did not generate, along with the reason.
	LevelSkip DiagnosticLevel = "skip"
)

// Diagnostic explains a discovery or render outcome for a service or a single
// action. Diagnostics are cli-gen's way of surfacing "silent" decisions (skips,
// overrides) so they are visible in -dry-run output instead of failing quietly.
type Diagnostic struct {
	// Service is the kumo service name (service.Service.Name()).
	Service string
	// Action is the resolved SDK action name, or empty for service-level diagnostics.
	Action string
	Level  DiagnosticLevel
	Reason string
}

// Action is a discovered, SDK-resolved CLI action candidate for one kumo
// service. It carries everything needed to derive flags and render a command.
type Action struct {
	// ServiceName is the kumo service name, e.g. "sqs".
	ServiceName string
	// GoMethod is the handler method name as implemented on the kumo service,
	// e.g. "GetQueueURL". For Query protocol services this equals SDKMethod,
	// since Actions() already returns wire-style names.
	GoMethod string
	// SDKMethod is the canonical aws-sdk-go-v2 client method / action name,
	// e.g. "GetQueueUrl". This is the name used to build the generated command.
	SDKMethod string
	// InputType is the element type of the SDK method's *XxxInput parameter.
	InputType reflect.Type
	// OutputType is the element type of the SDK method's *XxxOutput result.
	OutputType reflect.Type
}

// FlagKind classifies how a single Input struct field is exposed as a CLI flag.
type FlagKind int

const (
	// FlagString is a plain string flag, e.g. --queue-name.
	FlagString FlagKind = iota
	// FlagInt32 is an int32 flag.
	FlagInt32
	// FlagInt64 is an int64 flag.
	FlagInt64
	// FlagBool is a bool flag.
	FlagBool
	// FlagStringEnum is a string flag cast to a named SDK enum type on assignment.
	FlagStringEnum
	// FlagBytesBase64 is a string flag base64-decoded into a []byte field.
	FlagBytesBase64
	// FlagTime is a string flag parsed as RFC3339 into a time.Time field.
	FlagTime
	// FlagStringArray is a repeatable string flag mapped to a []string field.
	FlagStringArray
	// FlagStringEnumArray is a repeatable string flag, each occurrence cast to
	// a named SDK enum type and appended to a []EnumType field.
	FlagStringEnumArray
	// FlagStringMap is a JSON-object string flag unmarshaled into a
	// map[string]string field.
	FlagStringMap
	// FlagKV is a "key=value,key2=value2" string flag parsed into a flat
	// (all-scalar-field) struct.
	FlagKV
	// FlagKVArray is a repeatable "key=value,key2=value2" string flag, each
	// occurrence parsed into an appended element of a []struct field.
	FlagKVArray
	// FlagJSONBlob is a raw JSON string flag unmarshaled directly into the
	// field (nested structs, maps of structs, or anything deeper than a flat
	// struct).
	FlagJSONBlob
)

// FieldFlag is a single Input struct field classified as a CLI flag.
type FieldFlag struct {
	// FieldName is the Go struct field name, e.g. "QueueName".
	FieldName string
	// FlagName is the derived kebab-case flag name, e.g. "queue-name".
	FlagName string
	Kind     FlagKind
	// ElemType is set for FlagStringEnum (the named enum type), FlagKV and
	// FlagKVArray (the flat struct type). It is nil otherwise.
	ElemType reflect.Type
	// Pointer reports whether the struct field's Go type is a pointer
	// (e.g. *string vs string).
	Pointer bool
}

// FieldsResult is the outcome of deriving flags for one action's Input type.
type FieldsResult struct {
	Flags []FieldFlag
	// RequiresOverride is true when the Input type has a field that cannot be
	// auto-derived (an interface/union-typed field, e.g. dynamodb's
	// types.AttributeValue). The action must not be auto-generated.
	RequiresOverride bool
	// OverrideReason explains why, when RequiresOverride is true.
	OverrideReason string
}
