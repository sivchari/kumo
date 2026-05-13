package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sivchari/kumo/internal/awsapi"
	"github.com/sivchari/kumo/internal/billing"
	"github.com/sivchari/kumo/internal/chaos"
	"github.com/sivchari/kumo/internal/servicecatalog"
)

func TestRouterAppliesChaosAndRecordsBillingForS3Route(t *testing.T) {
	catalog := servicecatalog.NewDefault()
	engine := chaos.NewEngine(catalog)
	meter := billing.NewMeter()
	router := NewRouter(slog.New(slog.NewTextHandler(ioDiscard{}, nil)))
	router.SetCatalog(catalog)
	router.SetChaosEngine(engine)
	router.SetBillingMeter(meter)

	if err := engine.UpsertRule(&chaos.Rule{
		ID:      "s3-put-down",
		Enabled: true,
		Match: chaos.Match{
			Service: "s3",
			Action:  "PutObject",
		},
		Fault: chaos.Fault{
			Type:    chaos.FaultError,
			Status:  http.StatusServiceUnavailable,
			Code:    "ServiceUnavailable",
			Message: "S3 is unavailable",
		},
	}); err != nil {
		t.Fatal(err)
	}

	called := false

	router.HandleWithService("PUT", "/{bucket}/{key...}", "s3", func(w http.ResponseWriter, _ *http.Request) {
		called = true

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPut, "/bucket/key", strings.NewReader("payload"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if called {
		t.Fatal("handler should not be called when chaos error is applied")
	}

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	if got := meter.Usage().Quantity("s3", "PutObject", "requests.put_object"); got != 1 {
		t.Fatalf("metered requests.put_object = %v, want 1", got)
	}
}

func TestKumoChaosControlEndpoints(t *testing.T) {
	router := newControlTestRouter()

	ruleBody := `{
		"id": "ddb-down",
		"enabled": true,
		"match": {"service": "DynamoDB_20120810", "action": "PutItem"},
		"fault": {"type": "error", "status": 503, "code": "ServiceUnavailable"}
	}`
	req := httptest.NewRequest(http.MethodPut, "/kumo/chaos/rules/ddb-down", strings.NewReader(ruleBody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("put chaos rule status = %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/kumo/chaos/services", http.NoBody)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("services status = %d", rec.Code)
	}

	var services struct {
		Services []servicecatalog.Identity `json:"services"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &services); err != nil {
		t.Fatal(err)
	}

	foundEventBridge := false

	for _, svc := range services.Services {
		if svc.Canonical == "eventbridge" {
			foundEventBridge = true
		}
	}

	if !foundEventBridge {
		t.Fatal("eventbridge should be listed as a canonical chaos service")
	}
}

func TestKumoBillingControlEndpoints(t *testing.T) {
	router := newControlTestRouter()

	rateCard := `{
		"currency": "USD",
		"rates": [{"service":"s3","dimension":"requests.put_object","unit":"1000_requests","price":0.005}]
	}`
	req := httptest.NewRequest(http.MethodPut, "/kumo/billing/rate-card", strings.NewReader(rateCard))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("put rate-card status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func newControlTestRouter() *Router {
	catalog := servicecatalog.NewDefault()
	engine := chaos.NewEngine(catalog)
	meter := billing.NewMeter()
	router := NewRouter(slog.New(slog.NewTextHandler(ioDiscard{}, nil)))
	router.SetCatalog(catalog)
	router.SetChaosEngine(engine)
	router.SetBillingMeter(meter)
	registerControlRoutes(router, catalog, engine, meter)

	return router
}

func TestRouterAppliesChaosAndBillingToJSONProtocolRequest(t *testing.T) {
	catalog := servicecatalog.NewDefault()
	engine := chaos.NewEngine(catalog)
	meter := billing.NewMeter(catalog)
	router := NewRouter(slog.New(slog.NewTextHandler(ioDiscard{}, nil)))
	router.SetCatalog(catalog)
	router.SetChaosEngine(engine)
	router.SetBillingMeter(meter)
	router.RegisterJSONPrefix("AmazonSQS", "sqs")

	err := engine.UpsertRule(&chaos.Rule{
		ID:      "sqs-send-throttle",
		Enabled: true,
		Match: chaos.Match{
			Service: "AmazonSQS",
			Action:  "send_message",
		},
		Fault: chaos.Fault{
			Type: chaos.FaultRateLimit,
			Limit: &chaos.RateLimit{
				RPS:   1,
				Burst: 1,
			},
			OnLimit: &chaos.FaultErrorSpec{
				Status: http.StatusTooManyRequests,
				Code:   "ThrottlingException",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	router.HandleFunc("POST", "/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		_, _ = w.Write([]byte(`{}`))
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/x-amz-json-1.0")
		req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if i == 0 && rec.Code != http.StatusOK {
			t.Fatalf("first status = %d, want %d", rec.Code, http.StatusOK)
		}

		if i == 1 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("second status = %d, want %d", rec.Code, http.StatusTooManyRequests)
		}
	}

	if got := meter.Usage().Quantity("sqs", "SendMessage", "requests.send_message"); got != 2 {
		t.Fatalf("metered requests.send_message = %v, want 2", got)
	}
}

func TestBillingRateCardNormalizesServiceAliases(t *testing.T) {
	catalog := servicecatalog.NewDefault()
	meter := billing.NewMeter(catalog)
	meter.SetRateCard(billing.RateCard{
		Currency: "USD",
		Rates: []billing.Rate{
			{
				Service:   "events",
				Dimension: "requests.put_events",
				Unit:      billing.UnitRequest,
				Price:     0.01,
			},
		},
	})

	meter.Record(&billing.RequestUsage{
		Info: awsapi.RequestInfo{
			Service: "eventbridge",
			Action:  "PutEvents",
		},
		Status: 200,
	})

	cost := meter.Cost()
	if cost.Total != 0.01 {
		t.Fatalf("cost = %v, want 0.01", cost.Total)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
