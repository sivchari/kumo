package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"time"
)

// startPprofServer launches a separate HTTP server exposing /debug/pprof/*
// when KUMO_PPROF is set to a truthy value (1/true/on). KUMO_PPROF_ADDR
// overrides the default :6060 listen address.
//
// Kept on its own ServeMux so we don't pollute the main service router.
func startPprofServer(logger *slog.Logger) {
	v := os.Getenv("KUMO_PPROF")
	if v == "" {
		return
	}

	on, err := strconv.ParseBool(v)

	if err != nil || !on {
		return
	}

	addr := os.Getenv("KUMO_PPROF_ADDR")
	if addr == "" {
		addr = ":6060"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		ln, lerr := (&net.ListenConfig{}).Listen(context.Background(), "tcp", addr)
		if lerr != nil {
			logger.Error("pprof listen failed", "addr", addr, "err", lerr)

			return
		}

		logger.Info(fmt.Sprintf("pprof endpoint enabled on %s", addr))

		if serr := srv.Serve(ln); serr != nil && serr != http.ErrServerClosed {
			logger.Error("pprof server failed", "err", serr)
		}
	}()
}
