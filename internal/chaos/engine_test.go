package chaos

import (
	"net/http"
	"testing"
	"time"

	"github.com/sivchari/kumo/internal/awsapi"
	"github.com/sivchari/kumo/internal/servicecatalog"
)

func TestLatencyProfileSamplesPercentileAnchors(t *testing.T) {
	profile := LatencyProfile{
		P50Ms: 20,
		P95Ms: 300,
		P99Ms: 1500,
		MaxMs: 3000,
	}

	tests := map[float64]time.Duration{
		0.00: 0,
		0.50: 20 * time.Millisecond,
		0.95: 300 * time.Millisecond,
		0.99: 1500 * time.Millisecond,
		1.00: 3000 * time.Millisecond,
	}

	for q, want := range tests {
		got := profile.DurationAt(q)
		if got != want {
			t.Fatalf("DurationAt(%v) = %v, want %v", q, got, want)
		}
	}
}

func TestEngineReturnsConfiguredErrorForMatchingServiceAlias(t *testing.T) {
	engine := NewEngine(servicecatalog.NewDefault())
	if err := engine.UpsertRule(&Rule{
		ID:      "ddb-down",
		Enabled: true,
		Match: Match{
			Service: "DynamoDB_20120810",
			Action:  "PutItem",
		},
		Fault: Fault{
			Type:    FaultError,
			Status:  http.StatusServiceUnavailable,
			Code:    "ServiceUnavailable",
			Message: "DynamoDB is unavailable",
		},
	}); err != nil {
		t.Fatal(err)
	}

	decision := engine.Evaluate(&awsapi.RequestInfo{Service: "dynamodb", Action: "PutItem"})
	if decision == nil {
		t.Fatal("expected a chaos decision")
	}

	if decision.Error == nil || decision.Error.Status != http.StatusServiceUnavailable {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestEngineRateLimitReturnsErrorAfterBurst(t *testing.T) {
	engine := NewEngine(servicecatalog.NewDefault())
	engine.SetClock(func() time.Time { return time.Unix(100, 0) })

	if err := engine.UpsertRule(&Rule{
		ID:      "s3-put-limit",
		Enabled: true,
		Match: Match{
			Service: "s3",
			Action:  "PutObject",
		},
		Fault: Fault{
			Type: FaultRateLimit,
			Limit: &RateLimit{
				RPS:   1,
				Burst: 1,
			},
			OnLimit: &FaultErrorSpec{
				Status:  http.StatusTooManyRequests,
				Code:    "SlowDown",
				Message: "Please reduce your request rate.",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	first := engine.Evaluate(&awsapi.RequestInfo{Service: "s3", Action: "PutObject"})
	if first != nil {
		t.Fatalf("first request should be allowed, got %#v", first)
	}

	second := engine.Evaluate(&awsapi.RequestInfo{Service: "s3", Action: "PutObject"})
	if second == nil || second.Error == nil || second.Error.Code != "SlowDown" {
		t.Fatalf("expected SlowDown decision, got %#v", second)
	}
}
