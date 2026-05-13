package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sivchari/kumo/internal/billing"
	"github.com/sivchari/kumo/internal/servicecatalog"
)

func TestRouterRecordsBillingForS3Route(t *testing.T) {
	t.Parallel()

	catalog := servicecatalog.NewDefault()
	meter := billing.NewMeter(catalog)
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))
	router.SetCatalog(catalog)
	router.SetBillingMeter(meter)
	router.HandleWithService(http.MethodPut, "/{bucket}/{key...}", "s3", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodPut, "/bucket/key", strings.NewReader("payload"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	usage := meter.Usage()
	if got := usage.Quantity("s3", "PutObject", "requests.put_object"); got != 1 {
		t.Fatalf("requests.put_object = %v, want 1", got)
	}

	if got := usage.Quantity("s3", "PutObject", "response.bytes"); got != 2 {
		t.Fatalf("response.bytes = %v, want 2", got)
	}
}

func TestKumoBillingControlEndpoints(t *testing.T) {
	t.Parallel()

	router := newBillingControlTestRouter()

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

	var card billing.RateCard
	if err := json.Unmarshal(rec.Body.Bytes(), &card); err != nil {
		t.Fatal(err)
	}

	if card.Currency != "USD" || len(card.Rates) != 1 {
		t.Fatalf("unexpected rate card: %#v", card)
	}
}

func TestServerDoesNotExposeLegacyControlEndpoints(t *testing.T) {
	srv := New(Config{LogLevel: slog.LevelError})
	req := httptest.NewRequest(http.MethodGet, "/kumo/chaos/rules", http.NoBody)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("status = %d, want legacy control endpoint to be unavailable", rec.Code)
	}
}

func newBillingControlTestRouter() *Router {
	catalog := servicecatalog.NewDefault()
	meter := billing.NewMeter(catalog)
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))
	router.SetCatalog(catalog)
	router.SetBillingMeter(meter)
	registerBillingControlRoutes(router, meter)

	return router
}
