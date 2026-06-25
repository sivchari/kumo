// Package kumo provides a public API for running an in-process AWS service emulator.
//
// Usage:
//
//	srv := kumo.NewServer()
//	defer srv.Close()
//
//	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
//	    o.BaseEndpoint = aws.String(srv.URL)
//	})
package kumo

import (
	"net/http/httptest"

	// Register all services via init(). See internal/registry for the
	// single canonical list shared with the CLI and the README generator.
	_ "github.com/sivchari/kumo/internal/registry"
	"github.com/sivchari/kumo/internal/server"
)

// Server is an in-process AWS service emulator.
// It wraps httptest.Server to provide a familiar API for Go testing.
type Server struct {
	// URL is the base URL of the server in the form "http://host:port".
	URL string

	httpServer *httptest.Server
}

// NewServer creates and starts a new in-process AWS emulator server.
// The server listens on a random available port on localhost.
// Use srv.URL as the BaseEndpoint for AWS SDK clients.
func NewServer() *Server {
	cfg := server.DefaultConfig()
	cfg.LogLevel = 100 // Suppress all logs in test mode.
	cfg.Version = Version
	internalSrv := server.New(cfg)

	ts := httptest.NewServer(internalSrv.Handler())

	return &Server{
		URL:        ts.URL,
		httpServer: ts,
	}
}

// Close shuts down the server.
func (s *Server) Close() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
}
