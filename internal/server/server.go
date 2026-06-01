// Package server provides the HTTP server for kumo.
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sivchari/kumo/internal/initdir"
	"github.com/sivchari/kumo/internal/service"
)

// Config holds the server configuration.
type Config struct {
	Host     string
	Port     int
	LogLevel slog.Level
	InitDir  string // Directory containing init scripts to execute on startup
}

// DefaultConfig returns the default server configuration.
// KUMO_HOST and KUMO_PORT override the bind address when set; an
// unparseable KUMO_PORT is ignored and the default port is kept.
// KUMO_LOG_LEVEL (debug|info|warn|error) overrides the default INFO level —
// useful when benchmarking, where per-request INFO logs dominate CPU.
func DefaultConfig() Config {
	cfg := Config{
		Host:     "0.0.0.0",
		Port:     4566,
		LogLevel: parseLogLevel(os.Getenv("KUMO_LOG_LEVEL"), slog.LevelInfo),
		InitDir:  os.Getenv("KUMO_INIT_DIR"),
	}

	if host := os.Getenv("KUMO_HOST"); host != "" {
		cfg.Host = host
	}

	if portStr := os.Getenv("KUMO_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			cfg.Port = p
		}
	}

	return cfg
}

func parseLogLevel(s string, def slog.Level) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return def
	}
}

// Server is the main HTTP server for kumo.
type Server struct {
	config          Config
	router          *Router
	registry        *service.Registry
	jsonDispatcher  *JSONProtocolDispatcher
	queryDispatcher *QueryProtocolDispatcher
	cborDispatcher  *CBORProtocolDispatcher
	logger          *slog.Logger
	server          *http.Server
}

// New creates a new server with the given configuration.
// Services registered via init() are automatically loaded.
func New(config Config) *Server {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.LogLevel,
	}))

	registry := service.NewRegistry()
	router := NewRouter(logger)
	jsonDispatcher := NewJSONProtocolDispatcher()
	queryDispatcher := NewQueryProtocolDispatcher()
	cborDispatcher := NewCBORProtocolDispatcher()

	srv := &Server{
		config:          config,
		router:          router,
		registry:        registry,
		jsonDispatcher:  jsonDispatcher,
		queryDispatcher: queryDispatcher,
		cborDispatcher:  cborDispatcher,
		logger:          logger,
	}

	// Auto-register services from global registry
	for _, svc := range service.Services() {
		srv.RegisterService(svc)
	}

	// Cross-service wiring (must happen after all services are registered).
	wireSNStoSQS(registry)
	wireS3toSQS(registry)
	wireCloudWatchToSNS(registry)

	// Register unified protocol dispatcher for POST /
	hasJSONServices := len(jsonDispatcher.handlers) > 0
	hasQueryServices := len(queryDispatcher.handlers) > 0

	if hasJSONServices || hasQueryServices {
		router.HandleFunc("POST", "/", srv.unifiedDispatcher)
		logger.Debug("registered unified protocol dispatcher for POST /")
	}

	// Register CBOR protocol dispatcher for /service/{serviceName}/operation/{operationName}
	hasCBORServices := len(cborDispatcher.handlers) > 0
	if hasCBORServices {
		router.HandleFunc("POST", "/service/{serviceName}/operation/{operationName}", srv.cborDispatcher.ServeHTTP)
		logger.Debug("registered CBOR protocol dispatcher for POST /service/*/operation/*")
	}

	return srv
}

// unifiedDispatcher routes requests to JSON or Query protocol handlers based on Content-Type.
func (s *Server) unifiedDispatcher(w http.ResponseWriter, r *http.Request) {
	// Parse media type per RFC 2045 (e.g. "application/x-www-form-urlencoded; charset=utf-8").
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))

	// Query protocol uses form-urlencoded.
	if mediaType == "application/x-www-form-urlencoded" {
		s.queryDispatcher.ServeHTTP(w, r)

		return
	}

	// Default to JSON protocol.
	s.jsonDispatcher.ServeHTTP(w, r)
}

// Registry returns the service registry.
func (s *Server) Registry() *service.Registry {
	return s.registry
}

// Router returns the router.
func (s *Server) Router() *Router {
	return s.router
}

// RegisterService registers a service with the server.
func (s *Server) RegisterService(svc service.Service) {
	s.registry.Register(svc)
	svc.RegisterRoutes(s.router)

	// Check if service implements JSON protocol.
	if jsonSvc, ok := svc.(service.JSONProtocolService); ok {
		s.jsonDispatcher.Register(jsonSvc.TargetPrefix(), jsonSvc.DispatchAction)
		s.logger.Debug("registered JSON protocol service", "name", svc.Name(), "prefix", jsonSvc.TargetPrefix())
	}

	// Check if service exposes an execute-api invoke surface.
	if execSvc, ok := svc.(service.ExecuteAPIHandler); ok {
		s.router.AddExecuteAPIHandler(execSvc.HandleExecuteAPI)
		s.logger.Debug("registered execute-api handler", "name", svc.Name())
	}

	// Check if service implements Query protocol.
	if querySvc, ok := svc.(service.QueryProtocolService); ok {
		s.queryDispatcher.Register(querySvc.TargetPrefix(), querySvc.DispatchAction)

		// Register each action for proper routing.
		for _, action := range querySvc.Actions() {
			s.queryDispatcher.RegisterAction(action, querySvc.TargetPrefix(), querySvc.ServiceIdentifier(), querySvc.DispatchAction)
		}

		s.logger.Debug("registered Query protocol service",
			"name", svc.Name(),
			"prefix", querySvc.TargetPrefix(),
			"identifier", querySvc.ServiceIdentifier(),
		)
	}

	// Check if service implements CBOR protocol.
	if cborSvc, ok := svc.(service.CBORProtocolService); ok {
		s.cborDispatcher.Register(cborSvc.ServiceName(), cborSvc.DispatchCBORAction)
		s.logger.Debug("registered CBOR protocol service", "name", svc.Name(), "serviceName", cborSvc.ServiceName())
	}

	s.logger.Info("registered service", "name", svc.Name())
}

// Addr returns the server address.
func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}

// Handler returns the HTTP handler for the server.
// This can be used with httptest.NewServer for in-process testing.
func (s *Server) Handler() http.Handler {
	return s.router
}

// Start starts the HTTP server. It accepts an optional readyCh channel that will be
// closed once the server is listening and ready to accept connections.
func (s *Server) Start(readyCh ...chan struct{}) error {
	s.server = &http.Server{
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	s.logger.Info("starting kumo server", "addr", s.Addr())

	// Optional pprof endpoint (KUMO_PPROF=1).
	startPprofServer(s.logger)

	// List registered services
	for _, name := range s.registry.Names() {
		s.logger.Info("service available", "name", name)
	}

	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", s.Addr())
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.Addr(), err)
	}

	// Signal that the server is ready to accept connections.
	if len(readyCh) > 0 && readyCh[0] != nil {
		close(readyCh[0])
	}

	if err := s.server.Serve(ln); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down server")

	// Save snapshots for services that implement io.Closer.
	for _, svc := range s.registry.All() {
		if c, ok := svc.(io.Closer); ok {
			if err := c.Close(); err != nil {
				s.logger.Error("failed to save snapshot", "service", svc.Name(), "error", err)
			}
		}
	}

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	return nil
}

// Run starts the server and handles graceful shutdown.
func (s *Server) Run() error {
	// Channel to receive OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Channel to receive server errors
	errChan := make(chan error, 1)

	// Channel to signal server readiness
	readyCh := make(chan struct{})

	// Start server in a goroutine
	go func() {
		if err := s.Start(readyCh); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	// Wait for server to be ready, then execute init scripts
	select {
	case <-readyCh:
		s.runInitScripts()
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	}

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		s.logger.Info("received signal", "signal", sig)
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.Shutdown(ctx)
}

// runInitScripts executes init scripts from the configured directory.
func (s *Server) runInitScripts() {
	if s.config.InitDir == "" {
		return
	}

	s.logger.Info("running init scripts", "dir", s.config.InitDir)

	go func() {
		ctx := context.Background()
		if err := initdir.Run(ctx, s.config.InitDir, s.logger); err != nil {
			s.logger.Error("failed to run init scripts", "error", err)
		}
	}()
}
