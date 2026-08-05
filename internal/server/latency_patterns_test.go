package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sivchari/kumo/internal/latency"
	"github.com/sivchari/kumo/internal/servicecatalog"
)

func TestLatencyPatternAppliesToRESTResourceMatch(t *testing.T) {
	t.Parallel()

	router := newLatencyPatternRouter(t, latency.Config{
		Rules: []latency.Rule{
			{
				ID:      "s3-object",
				Enabled: true,
				Match: latency.Match{
					Service:  "s3",
					Action:   "PutObject",
					Resource: "bucket/key",
				},
				Latency: latency.Latency{FixedMs: 30},
			},
		},
	})
	router.HandleWithService(http.MethodPut, "/{bucket}/{key...}", "s3", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	elapsed := measureRequest(router, rec, httptest.NewRequest(http.MethodPut, "/bucket/key", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	assertLatencyApplied(t, elapsed)
}

func TestLatencyPatternAppliesToRESTRouteWithFormURLEncodedBody(t *testing.T) {
	t.Parallel()

	router := newLatencyPatternRouter(t, latency.Config{
		Rules: []latency.Rule{
			{
				ID:      "s3-form-object",
				Enabled: true,
				Match: latency.Match{
					Service: "s3",
					Action:  "PutObject",
				},
				Latency: latency.Latency{FixedMs: 30},
			},
		},
	})
	router.HandleWithService(http.MethodPut, "/{bucket}/{key...}", "s3", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPut, "/bucket/key", strings.NewReader("field=value"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	elapsed := measureRequest(router, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	assertLatencyApplied(t, elapsed)
}

func TestLatencyPatternAppliesToJSONProtocolTarget(t *testing.T) {
	t.Parallel()

	router := newLatencyPatternRouter(t, latency.Config{
		Rules: []latency.Rule{
			{
				ID:      "sqs-json",
				Enabled: true,
				Match: latency.Match{
					Service: "sqs",
					Action:  "SendMessage",
				},
				Latency: latency.Latency{FixedMs: 30},
			},
		},
	})
	router.RegisterJSONPrefix("AmazonSQS", "sqs")
	router.HandleFunc(http.MethodPost, "/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rec := httptest.NewRecorder()
	elapsed := measureRequest(router, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	assertLatencyApplied(t, elapsed)
}

func TestLatencyPatternAppliesToQueryProtocolWithoutConsumingBody(t *testing.T) {
	t.Parallel()

	router := newLatencyPatternRouter(t, latency.Config{
		Rules: []latency.Rule{
			{
				ID:      "ec2-query",
				Enabled: true,
				Match: latency.Match{
					Service: "ec2",
					Action:  "DescribeInstances",
				},
				Latency: latency.Latency{FixedMs: 30},
			},
		},
	})
	router.HandleFunc(http.MethodPost, "/", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		if !strings.Contains(string(body), "Action=DescribeInstances") {
			t.Errorf("request body was not restored after latency request inspection: %q", string(body))
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("Action=DescribeInstances&Version=2016-11-15"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "aws-sdk-go-v2/1.36.3 api/ec2#1.0.0 os/linux")

	rec := httptest.NewRecorder()
	elapsed := measureRequest(router, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	assertLatencyApplied(t, elapsed)
}

func TestLatencyPatternSkipsControlPathEvenWhenRuleMatches(t *testing.T) {
	t.Parallel()

	router := newLatencyPatternRouter(t, latency.Config{
		Rules: []latency.Rule{
			{
				ID:      "control-path",
				Enabled: true,
				Match: latency.Match{
					Service: "kumo",
					Method:  http.MethodGet,
					Path:    "/kumo/probe",
				},
				Latency: latency.Latency{FixedMs: 30},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/kumo/probe", http.NoBody)
	info := router.requestInfo("kumo", http.MethodGet, "/kumo/probe", req)

	if !info.isControl {
		t.Fatal("expected /kumo path to be classified as control")
	}

	if decision := router.evaluateLatency(&info); decision != nil {
		t.Fatalf("control path should not receive latency decision: %#v", decision)
	}
}

func newLatencyPatternRouter(t *testing.T, config latency.Config) *Router {
	t.Helper()

	catalog := servicecatalog.NewDefault()
	engine := latency.NewEngine(catalog)

	if err := engine.LoadConfig(config); err != nil {
		t.Fatal(err)
	}

	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))
	router.SetCatalog(catalog)
	router.SetLatencyEngine(engine)

	return router
}

func measureRequest(handler http.Handler, rec *httptest.ResponseRecorder, req *http.Request) time.Duration {
	start := time.Now()

	handler.ServeHTTP(rec, req)

	return time.Since(start)
}

func assertLatencyApplied(t *testing.T, got time.Duration) {
	t.Helper()

	want := 25 * time.Millisecond
	if got < want {
		t.Fatalf("elapsed = %v, want at least %v", got, want)
	}
}
