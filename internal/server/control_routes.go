package server

import (
	"encoding/json"
	"net/http"

	"github.com/sivchari/kumo/internal/billing"
)

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
