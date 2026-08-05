package latency

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sivchari/kumo/internal/awsapi"
	"github.com/sivchari/kumo/internal/servicecatalog"
)

func TestLatencySamplesPercentileAnchors(t *testing.T) {
	t.Parallel()

	latency := Latency{
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

	for quantile, want := range tests {
		got := latency.DurationAt(quantile)
		if got != want {
			t.Fatalf("DurationAt(%v) = %v, want %v", quantile, got, want)
		}
	}
}

func TestEngineLoadsConfigAndMatchesServiceAlias(t *testing.T) {
	t.Parallel()

	engine := NewEngine(servicecatalog.NewDefault())

	err := engine.LoadConfig(Config{
		Rules: []Rule{
			{
				ID:      "ddb-latency",
				Enabled: true,
				Match: Match{
					Service: "DynamoDB_20120810",
					Action:  "PutItem",
				},
				Latency: Latency{FixedMs: 25},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	decision := engine.Evaluate(&awsapi.RequestInfo{Service: "dynamodb", Action: "PutItem"})
	if decision == nil {
		t.Fatal("expected latency decision")

		return
	}

	if decision.Delay != 25*time.Millisecond {
		t.Fatalf("Delay = %v, want 25ms", decision.Delay)
	}
}

func TestEngineLoadFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "latency.json")
	body := []byte(`{
  "seed": 12345,
  "rules": [
    {
      "id": "s3-put-latency",
      "enabled": true,
      "match": {"service": "s3", "action": "PutObject"},
      "latency": {"fixedMs": 10}
    }
  ]
}`)

	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(servicecatalog.NewDefault())
	if err := engine.LoadFile(path); err != nil {
		t.Fatal(err)
	}

	decision := engine.Evaluate(&awsapi.RequestInfo{Service: "s3", Action: "PutObject"})
	if decision == nil || decision.Delay != 10*time.Millisecond {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestEngineRejectsMixedLatencyModes(t *testing.T) {
	t.Parallel()

	engine := NewEngine(servicecatalog.NewDefault())

	err := engine.LoadConfig(Config{
		Rules: []Rule{
			{
				ID:      "mixed",
				Enabled: true,
				Match:   Match{Service: "s3"},
				Latency: Latency{FixedMs: 10, P50Ms: 1, P95Ms: 2, P99Ms: 3, MaxMs: 4},
			},
		},
	})
	if err == nil {
		t.Fatal("expected mixed latency mode validation error")
	}
}
