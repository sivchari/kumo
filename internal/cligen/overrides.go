package cligen

import (
	"reflect"
	"strings"
)

// FlagOverride replaces the auto-derived flag kind for a specific Input
// struct field. It is a compiled-in escape hatch: PR1 seeds no entries, this
// only wires up the mechanism so later PRs (e.g. kms Sign/Verify's raw
// message bytes) can hook in without changing discover/flags logic.
type FlagOverride struct {
	Kind FlagKind
}

// fieldOverrides overrides flag derivation for specific fields, keyed by the
// SDK Input struct's reflect.Type and then by Go field name.
var fieldOverrides = map[reflect.Type]map[string]FlagOverride{}

// skipAutoGeneration lists, per kumo service name, the SDK action names that
// must never be auto-generated because a hand-written
// cli/{service}_overrides.go implementation exists (or will exist) instead.
var skipAutoGeneration = map[string][]string{
	// kms Sign/Verify operate on raw message bytes and need bespoke
	// base64/raw-bytes handling that the generic field-kind rules don't cover.
	"kms": {"Sign", "Verify"},
}

// ignoredBackCompatFlags lists, per kumo service and SDK action name, Input
// fields the generator intentionally does not expose as flags, to preserve
// existing hand-written CLI behavior once a service migrates to generated
// commands.
var ignoredBackCompatFlags = map[string]map[string][]string{
	"dynamodb": {
		"CreateTable": {"TableClass"},
	},
}

// overrideServices lists kumo service names whose generated parent command
// must also call {CLIName}OverrideCommands() (implemented in a hand-written
// cli/{service}_overrides.go) to add commands the generator cannot produce.
var overrideServices = map[string]bool{
	"s3":    true,
	"s3api": true,
	"kms":   true,
}

// isSkippedAction reports whether action has been manually overridden for
// serviceName and must not be auto-generated.
func isSkippedAction(serviceName, action string) bool {
	for _, a := range skipAutoGeneration[serviceName] {
		if strings.EqualFold(a, action) {
			return true
		}
	}

	return false
}

// isIgnoredBackCompatFlag reports whether fieldName should be omitted from
// the generated flags for serviceName's action, to preserve existing
// hand-written CLI behavior.
func isIgnoredBackCompatFlag(serviceName, action, fieldName string) bool {
	for _, f := range ignoredBackCompatFlags[serviceName][action] {
		if f == fieldName {
			return true
		}
	}

	return false
}
