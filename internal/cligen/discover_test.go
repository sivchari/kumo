package cligen_test

import (
	"strings"
	"testing"

	"github.com/sivchari/kumo/internal/cligen"
	_ "github.com/sivchari/kumo/internal/registry"
	"github.com/sivchari/kumo/internal/service"
)

// TestDiscover_SQSCasing proves the exact casing trap the plan calls out:
// sqs's handler method is GetQueueURL (Go), but the SDK's canonical wire name
// is GetQueueUrl. Discover must resolve the Go name against the SDK method
// set case-insensitively and adopt the SDK's casing.
func TestDiscover_SQSCasing(t *testing.T) {
	t.Parallel()

	result := cligen.Discover(service.Services())

	var found *cligen.Action

	for i := range result.Actions {
		a := &result.Actions[i]
		if a.ServiceName == "sqs" && a.GoMethod == "GetQueueURL" {
			found = a

			break
		}
	}

	if found == nil {
		t.Fatal("expected sqs GetQueueURL to be discovered")
	}

	if found.SDKMethod != "GetQueueUrl" {
		t.Errorf("SDKMethod = %q, want %q", found.SDKMethod, "GetQueueUrl")
	}
}

// TestDiscover_RESTOnlyServicesSkipped verifies that services exposing only
// RegisterRoutes (no Query/JSON protocol) are skipped with a diagnostic
// rather than silently ignored or erroring.
func TestDiscover_RESTOnlyServicesSkipped(t *testing.T) {
	t.Parallel()

	result := cligen.Discover(service.Services())

	restOnly := []string{"amplify", "apigateway", "apigatewayv2", "appmesh", "appsync", "backup"}

	for _, name := range restOnly {
		if !hasSkipDiagnostic(result.Diagnostics, name, "", "REST-only") {
			t.Errorf("expected a REST-only skip diagnostic for service %q", name)
		}

		for _, a := range result.Actions {
			if a.ServiceName == name {
				t.Errorf("service %q is REST-only and must not produce discovered actions, got %+v", name, a)
			}
		}
	}
}

// TestDiscover_SeedServicesProduceActions verifies that each JSON-protocol
// seed service (i.e. one with an sdkBindings entry) discovers at least one
// action, and that every discovered action successfully resolved an SDK
// Input/Output type.
func TestDiscover_SeedServicesProduceActions(t *testing.T) {
	t.Parallel()

	result := cligen.Discover(service.Services())

	seeds := []string{"sqs", "dynamodb", "kinesis", "kms", "acm", "athena", "events"}

	counts := make(map[string]int, len(seeds))
	for _, a := range result.Actions {
		counts[a.ServiceName]++

		if a.InputType == nil || a.OutputType == nil {
			t.Errorf("action %s.%s has a nil Input/Output type", a.ServiceName, a.SDKMethod)
		}
	}

	for _, name := range seeds {
		if counts[name] == 0 {
			t.Errorf("expected at least one discovered action for seed service %q", name)
		}
	}
}

// TestDiscover_KMSSignVerifySkipped verifies the compiled-in
// skipAutoGeneration override table takes effect during discovery.
func TestDiscover_KMSSignVerifySkipped(t *testing.T) {
	t.Parallel()

	result := cligen.Discover(service.Services())

	for _, a := range result.Actions {
		if a.ServiceName == "kms" && (a.SDKMethod == "Sign" || a.SDKMethod == "Verify") {
			t.Errorf("kms %s must be skipped (manually overridden), got a discovered action", a.SDKMethod)
		}
	}

	if !hasSkipDiagnostic(result.Diagnostics, "kms", "Sign", "overridden") {
		t.Error("expected a skip diagnostic for kms Sign")
	}
}

func hasSkipDiagnostic(diags []cligen.Diagnostic, service, action, reasonSubstr string) bool {
	for _, d := range diags {
		if d.Service != service || d.Level != cligen.LevelSkip {
			continue
		}

		if action != "" && d.Action != action {
			continue
		}

		if reasonSubstr != "" && !strings.Contains(d.Reason, reasonSubstr) {
			continue
		}

		return true
	}

	return false
}
