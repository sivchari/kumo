package server

import (
	"encoding/json"
	"net/http"

	"github.com/sivchari/kumo/internal/billing"
	"github.com/sivchari/kumo/internal/chaos"
	"github.com/sivchari/kumo/internal/servicecatalog"
)

func registerControlRoutes(
	router *Router,
	catalog *servicecatalog.Catalog,
	engine *chaos.Engine,
	meter *billing.Meter,
) {
	registerChaosControlRoutes(router, catalog, engine)
	registerBillingControlRoutes(router, meter)
}

func registerChaosControlRoutes(
	router *Router,
	catalog *servicecatalog.Catalog,
	engine *chaos.Engine,
) {
	router.HandleFunc("GET", "/kumo/chaos/services", func(w http.ResponseWriter, _ *http.Request) {
		writeControlJSON(w, http.StatusOK, map[string]any{"services": catalog.Services()})
	})

	router.HandleFunc("GET", "/kumo/chaos/rules", func(w http.ResponseWriter, _ *http.Request) {
		writeControlJSON(w, http.StatusOK, map[string]any{"rules": engine.Rules()})
	})

	router.HandleFunc("PUT", "/kumo/chaos/rules/{id}", func(w http.ResponseWriter, r *http.Request) {
		var rule chaos.Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			writeControlError(w, http.StatusBadRequest, "invalid chaos rule JSON")

			return
		}

		if pathID := r.PathValue("id"); pathID != "" {
			rule.ID = pathID
		}

		if err := engine.UpsertRule(&rule); err != nil {
			writeControlError(w, http.StatusBadRequest, err.Error())

			return
		}

		rule.Match.Service = catalog.MustNormalize(rule.Match.Service)
		writeControlJSON(w, http.StatusOK, rule)
	})

	router.HandleFunc("DELETE", "/kumo/chaos/rules/{id}", func(w http.ResponseWriter, r *http.Request) {
		engine.DeleteRule(r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	})

	router.HandleFunc("POST", "/kumo/chaos/reset", func(w http.ResponseWriter, _ *http.Request) {
		engine.Reset()
		w.WriteHeader(http.StatusNoContent)
	})
}

func registerBillingControlRoutes(router *Router, meter *billing.Meter) {
	router.HandleFunc("GET", "/kumo/billing/usage", func(w http.ResponseWriter, _ *http.Request) {
		writeControlJSON(w, http.StatusOK, meter.Usage())
	})

	router.HandleFunc("POST", "/kumo/billing/reset", func(w http.ResponseWriter, _ *http.Request) {
		meter.Reset()
		w.WriteHeader(http.StatusNoContent)
	})

	router.HandleFunc("GET", "/kumo/billing/rate-card", func(w http.ResponseWriter, _ *http.Request) {
		writeControlJSON(w, http.StatusOK, meter.RateCard())
	})

	router.HandleFunc("PUT", "/kumo/billing/rate-card", func(w http.ResponseWriter, r *http.Request) {
		var card billing.RateCard
		if err := json.NewDecoder(r.Body).Decode(&card); err != nil {
			writeControlError(w, http.StatusBadRequest, "invalid rate card JSON")

			return
		}

		meter.SetRateCard(card)
		writeControlJSON(w, http.StatusOK, meter.RateCard())
	})

	router.HandleFunc("GET", "/kumo/billing/cost", func(w http.ResponseWriter, _ *http.Request) {
		writeControlJSON(w, http.StatusOK, meter.Cost())
	})
}

func writeControlJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeControlError(w http.ResponseWriter, status int, message string) {
	writeControlJSON(w, status, map[string]string{"message": message})
}
