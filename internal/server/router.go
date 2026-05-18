package server

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Route represents a registered HTTP route.
type Route struct {
	Method  string
	Pattern string
	Handler http.HandlerFunc
}

// Router is the HTTP router for kumo.
type Router struct {
	mux           *http.ServeMux
	routes        []Route
	prefixRouters map[string]*http.ServeMux // Separate routers for services with prefixes
	logger        *slog.Logger
}

// NewRouter creates a new router.
func NewRouter(logger *slog.Logger) *Router {
	r := &Router{
		mux:           http.NewServeMux(),
		routes:        make([]Route, 0),
		prefixRouters: make(map[string]*http.ServeMux),
		logger:        logger,
	}

	return r
}

// Handle registers a handler for the given method and pattern.
func (r *Router) Handle(method, pattern string, handler http.HandlerFunc) {
	r.routes = append(r.routes, Route{
		Method:  method,
		Pattern: pattern,
		Handler: handler,
	})

	// Check if this is a prefixed route (e.g., /lambda/...)
	// Routes with specific prefixes are registered in separate ServeMux instances
	// to avoid conflicts with wildcard routes like /{bucket}/{key...}
	prefix := extractRoutePrefix(pattern)
	if prefix != "" {
		if _, ok := r.prefixRouters[prefix]; !ok {
			r.prefixRouters[prefix] = http.NewServeMux()
		}

		fullPattern := method + " " + pattern
		r.prefixRouters[prefix].HandleFunc(fullPattern, r.wrapHandler(method, pattern, handler))
		r.logger.Debug("registered prefixed route", "method", method, "pattern", pattern, "prefix", prefix)

		return
	}

	// Use Go 1.22+ method pattern
	fullPattern := method + " " + pattern
	r.mux.HandleFunc(fullPattern, r.wrapHandler(method, pattern, handler))
	r.logger.Debug("registered route", "method", method, "pattern", pattern)
}

// extractRoutePrefix extracts service prefixes like "/lambda" from patterns.
// Returns empty string for patterns without service prefixes.
func extractRoutePrefix(pattern string) string {
	// Known service prefixes that need isolation from wildcard routes
	// S3 Tables uses /buckets, /namespaces, /tables, /get-table paths
	// CloudFront uses /2020-05-31 versioned paths
	// /service is for RPC v2 CBOR protocol
	// EventBridge Pipes uses /v1/pipes and /tags paths
	// EMR Serverless uses /applications paths
	prefixes := []string{"/_aws", "/kumo", "/lambda", "/2015-03-31", "/2017-03-31", "/2019-09-25", "/2020-06-30", "/eks", "/iam", "/buckets", "/namespaces", "/tables", "/get-table", "/apigateway", "/restapis", "/ses", "/2020-05-31", "/2013-04-01", "/service", "/appsync", "/v1", "/tags", "/applications", "/v20190125", "/scheduler", "/dlm", "/mq", "/v20180820", "/kx", "/kafka", "/create-app", "/describe-app", "/update-app", "/delete-app", "/list-apps", "/create-resiliency-policy", "/describe-resiliency-policy", "/update-resiliency-policy", "/delete-resiliency-policy", "/list-resiliency-policies", "/start-app-assessment", "/describe-app-assessment", "/delete-app-assessment", "/list-app-assessments", "/tag-resource", "/untag-resource", "/list-tags-for-resource", "/schemas", "/matchingworkflows", "/idmappingworkflows", "/providerservices", "/-", "/snapshots", "/apps", "/backup-vaults", "/backup", "/associations", "/codereviews", "/feedback", "/profilingGroups", "/maps", "/places", "/routes", "/geofencing", "/tracking", "/metadata", "/macie", "/allow-lists", "/jobs", "/custom-data-identifiers", "/findingsfilters", "/findings", "/managed-data-identifiers"}

	for _, prefix := range prefixes {
		if hasPathPrefix(pattern, prefix) {
			return prefix
		}
	}

	return ""
}

// hasPathPrefix reports whether path starts with prefix on a path
// boundary — i.e. either the prefix consumes the entire path or the
// next character is `/`. This stops `/kumo-audit-bad-bucket` from
// being mis-classified as a `/kumo`-prefixed path, which would shadow
// the S3 wildcard route `/{bucket}`.
func hasPathPrefix(path, prefix string) bool {
	if len(path) < len(prefix) {
		return false
	}

	if path[:len(prefix)] != prefix {
		return false
	}

	return len(path) == len(prefix) || path[len(prefix)] == '/'
}

// HandleFunc is an alias for Handle for compatibility with service.Router interface.
func (r *Router) HandleFunc(method, pattern string, handler http.HandlerFunc) {
	r.Handle(method, pattern, handler)
}

// wrapHandler wraps a handler with logging and request ID injection.
func (r *Router) wrapHandler(method, pattern string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		requestID := uuid.New().String()

		// Add AWS-style headers
		w.Header().Set("x-amz-request-id", requestID)
		w.Header().Set("x-amzn-RequestId", requestID)

		// Capture request body for debug logging.
		var requestBody string

		if r.logger.Enabled(req.Context(), slog.LevelDebug) && req.Body != nil {
			body, err := io.ReadAll(req.Body)
			if err == nil {
				requestBody = string(body)
				req.Body = io.NopCloser(bytes.NewReader(body))
			}
		}

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call the actual handler
		handler(wrapped, req)

		// Build log attributes.
		attrs := []any{
			"method", method,
			"path", req.URL.Path,
			"pattern", pattern,
			"status", wrapped.statusCode,
			"duration", time.Since(start),
			"request_id", requestID,
		}

		// Extract service and action from protocol headers.
		if target := req.Header.Get("X-Amz-Target"); target != "" {
			attrs = append(attrs, "target", target)
		}

		if action := req.URL.Query().Get("Action"); action != "" {
			attrs = append(attrs, "action", action)
		}

		if ua := req.Header.Get("User-Agent"); ua != "" {
			attrs = append(attrs, "ua", ua)
		}

		r.logger.Info("request", attrs...)

		// Log request body at debug level.
		if requestBody != "" {
			r.logger.Debug("request body", "request_id", requestID, "body", requestBody)
		}
	}
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Handle health endpoint before ServeMux to avoid route conflicts.
	if req.URL.Path == "/health" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))

		return
	}

	// Rewrite AWS S3 virtual-hosted-style requests so the rest of the
	// router only deals with path-style. terraform / aws-sdk-go-v2
	// default to virtual-hosted-style: the bucket goes in the Host
	// header (`<bucket>.localhost:4566` or `<bucket>.s3.amazonaws.com`)
	// and the URL path is `/` (for bucket-level ops) or `/<key>` (for
	// object ops). Without this rewrite the wildcard `/{bucket}` route
	// can't see the bucket name and `HEAD /` returns 200, which the
	// SDK reads as "bucket exists" → spurious BucketAlreadyExists.
	if bucket := extractBucketFromHost(req.Host); bucket != "" {
		req = req.Clone(req.Context())

		// `/` (bucket-level op) → `/<bucket>`.
		// `/key/path` (object-level op) → `/<bucket>/key/path`.
		if req.URL.Path == "" || req.URL.Path == "/" {
			req.URL.Path = "/" + bucket
		} else {
			req.URL.Path = "/" + bucket + req.URL.Path
		}
	}

	// Clean the URL path to prevent Go's ServeMux from returning 301 redirects
	// for paths with double slashes (e.g., /bucket//key). S3 keys can start with
	// "/" which produces double slashes in path-style URLs. We clean the path
	// ourselves so the mux sees an already-clean path and serves it directly.
	if cleaned := path.Clean(req.URL.Path); cleaned != req.URL.Path {
		req = req.Clone(req.Context())
		req.URL.Path = cleaned
	}

	// Check if the request matches a prefix router first.
	// Use longest prefix match to avoid short prefixes (e.g., "/apps")
	// incorrectly capturing longer ones (e.g., "/appsync").
	bestPrefix := ""

	for prefix := range r.prefixRouters {
		if hasPathPrefix(req.URL.Path, prefix) && len(prefix) > len(bestPrefix) {
			bestPrefix = prefix
		}
	}

	if bestPrefix != "" {
		r.prefixRouters[bestPrefix].ServeHTTP(w, req)

		return
	}

	r.mux.ServeHTTP(w, req)
}

// Routes returns all registered routes.
func (r *Router) Routes() []Route {
	return r.routes
}

// extractBucketFromHost recognises AWS S3 virtual-hosted-style hosts
// and returns the bucket name. Empty result means path-style (or a
// non-S3 host) — caller leaves the URL untouched.
//
// Recognised shapes:
//
//	<bucket>.localhost(:port)          (kumo-style custom endpoint)
//	<bucket>.s3.amazonaws.com
//	<bucket>.s3-<region>.amazonaws.com
//	<bucket>.s3.<region>.amazonaws.com (incl. dualstack subdomains)
func extractBucketFromHost(host string) string {
	if host == "" {
		return ""
	}

	// Strip port — ":4566" etc. We only need the host name.
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		host = host[:idx]
	}

	// Localhost / loopback IPs are never virtual-hosted.
	if host == "localhost" || host == "127.0.0.1" {
		return ""
	}

	dot := strings.Index(host, ".")
	if dot <= 0 {
		return ""
	}

	bucket := host[:dot]
	rest := host[dot+1:]

	// Reject the "no bucket prefix" case: e.g. `s3.amazonaws.com`,
	// `s3.us-east-1.amazonaws.com`. The leftmost label *is* the
	// service marker, not a bucket.
	if bucket == "s3" || strings.HasPrefix(bucket, "s3-") {
		return ""
	}

	switch {
	case rest == "localhost":
		return bucket
	case rest == "s3.amazonaws.com":
		return bucket
	case strings.HasPrefix(rest, "s3.") && strings.HasSuffix(rest, ".amazonaws.com"):
		return bucket
	case strings.HasPrefix(rest, "s3-") && strings.HasSuffix(rest, ".amazonaws.com"):
		return bucket
	}

	return ""
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code.
func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
