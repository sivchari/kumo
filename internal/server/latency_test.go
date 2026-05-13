package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sivchari/kumo/internal/latency"
	"github.com/sivchari/kumo/internal/servicecatalog"
)

func TestRouterAppliesConfiguredLatency(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	catalog := servicecatalog.NewDefault()
	engine := latency.NewEngine(catalog)

	if err := engine.LoadConfig(latency.Config{
		Rules: []latency.Rule{
			{
				ID:      "s3-put-latency",
				Enabled: true,
				Match: latency.Match{
					Service: "s3",
					Action:  "PutObject",
				},
				Latency: latency.Latency{FixedMs: 20},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	router := NewRouter(logger)
	router.SetCatalog(catalog)
	router.SetLatencyEngine(engine)
	router.HandleWithService(http.MethodPut, "/{bucket}/{key...}", "s3", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPut, "/bucket/key", http.NoBody)
	rec := httptest.NewRecorder()
	start := time.Now()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Fatalf("elapsed = %v, want configured latency to be applied", elapsed)
	}
}

func TestServerDoesNotExposeChaosControlEndpoints(t *testing.T) {
	t.Parallel()

	srv := New(Config{LogLevel: slog.LevelError})
	req := httptest.NewRequest(http.MethodGet, "/kumo/chaos/rules", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("status = %d, want chaos control endpoint to be unavailable", rec.Code)
	}
}

func TestServerLoadsLatencyConfigFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "latency.json")
	body := []byte(`{
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

	srv := New(Config{LogLevel: slog.LevelError, LatencyConfig: path})
	info := srv.router.requestInfo("s3", http.MethodPut, "/{bucket}/{key...}",
		httptest.NewRequest(http.MethodPut, "/bucket/key", http.NoBody),
	)
	decision := srv.router.latencyEngine.Evaluate(&info.RequestInfo)

	if decision == nil || decision.Delay != 10*time.Millisecond {
		t.Fatalf("unexpected latency decision: %#v", decision)
	}
}
