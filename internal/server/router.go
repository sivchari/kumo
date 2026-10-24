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

	"github.com/sivchari/kumo/internal/vhost"
)

// localhostHost is the loopback host label used by kumo virtual-hosted URLs.
const localhostHost = "localhost"

// Route represents a registered HTTP route.
type Route struct {
	Method  string
	Pattern string
	Handler http.HandlerFunc
}

// executeAPIDispatch handles a virtual-hosted execute-api request. It returns
// false when apiID is not owned by the handler.
type executeAPIDispatch func(w http.ResponseWriter, r *http.Request, apiID, invokePath string) bool

// functionURLDispatch is the signature of a virtual-hosted function URL
// handler. It returns false when the service does not own urlID.
type functionURLDispatch func(w http.ResponseWriter, r *http.Request, urlID string) bool

// wellKnownJWKSSuffix is the fixed tail of a per-pool JWKS path
// (/{id}/.well-known/jwks.json). Such routes go on a dedicated mux because
// their pattern formally collides with S3's wildcard routes in the main mux:
// GET implies HEAD, so GET /{id}/.well-known/jwks.json overlaps S3's explicit
// HEAD /{bucket}/{key...} with neither pattern more specific — which makes the
// shared http.ServeMux panic at registration.
//
// JWKS is published unsigned, so it cannot live on a SigV4 scope router: a
// token verifier fetches it with a plain GET and never signs.
const wellKnownJWKSSuffix = "/.well-known/jwks.json"

// Router is the HTTP router for kumo.
type Router struct {
	mux                 *http.ServeMux
	routes              []Route
	prefixRouters       map[string]*http.ServeMux // Separate routers for services with prefixes
	scopeRouters        map[string]*http.ServeMux // Routers keyed by SigV4 signing name
	wellKnownMux        *http.ServeMux            // Dedicated mux for /{id}/.well-known/jwks.json
	executeAPIHandlers  []executeAPIDispatch
	functionURLHandlers []functionURLDispatch
	logger              *slog.Logger
}

// AddExecuteAPIHandler registers a handler for virtual-hosted execute-api
// requests ({apiId}.execute-api.<host>).
func (r *Router) AddExecuteAPIHandler(fn executeAPIDispatch) {
	r.executeAPIHandlers = append(r.executeAPIHandlers, fn)
}

// AddFunctionURLHandler registers a handler for virtual-hosted Lambda function
// URL requests ({urlId}.lambda-url.<host>).
func (r *Router) AddFunctionURLHandler(fn functionURLDispatch) {
	r.functionURLHandlers = append(r.functionURLHandlers, fn)
}

// NewRouter creates a new router.
func NewRouter(logger *slog.Logger) *Router {
	r := &Router{
		mux:           http.NewServeMux(),
		routes:        make([]Route, 0),
		prefixRouters: make(map[string]*http.ServeMux),
		scopeRouters:  make(map[string]*http.ServeMux),
		logger:        logger,
	}

	return r
}

// ScopedRouter registers routes matched only for requests whose SigV4
// credential scope carries signingName. Services sharing a signing name
// (API Gateway v1 and v2 both sign as "apigateway") share one scope router,
// so their patterns must not overlap.
type ScopedRouter struct {
	router      *Router
	signingName string
	mux         *http.ServeMux
}

// ScopedRouter returns the registrar for the given SigV4 signing name,
// creating it on first use.
func (r *Router) ScopedRouter(signingName string) *ScopedRouter {
	mux, ok := r.scopeRouters[signingName]
	if !ok {
		mux = http.NewServeMux()
		r.scopeRouters[signingName] = mux
	}

	return &ScopedRouter{router: r, signingName: signingName, mux: mux}
}

// kumoNativePrefixes are kumo's own (non-AWS) namespaces. A ScopedRouter
// path-routes patterns under them: their clients (Lambda Runtime API
// binaries, debug tooling) never sign, and the namespaces are reserved so
// they cannot collide with an AWS path.
var kumoNativePrefixes = []string{"/kumo", "/_runtime"}

// Handle registers a handler for the given method and pattern.
func (s *ScopedRouter) Handle(method, pattern string, handler http.HandlerFunc) {
	for _, prefix := range kumoNativePrefixes {
		if hasPathPrefix(pattern, prefix) {
			s.router.Handle(method, pattern, handler)

			return
		}
	}

	s.mux.HandleFunc(method+" "+pattern, s.router.wrapHandler(method, pattern, handler))
	s.router.logger.Debug("registered scoped route",
		"signing_name", s.signingName, "method", method, "pattern", pattern)
}

// HandleFunc is an alias for Handle for compatibility with service.Router.
func (s *ScopedRouter) HandleFunc(method, pattern string, handler http.HandlerFunc) {
	s.Handle(method, pattern, handler)
}

// Handle registers a handler for the given method and pattern.
func (r *Router) Handle(method, pattern string, handler http.HandlerFunc) {
	r.routes = append(r.routes, Route{
		Method:  method,
		Pattern: pattern,
		Handler: handler,
	})

	// Well-known JWKS routes go on a dedicated mux, isolated from the S3
	// wildcard routes they would otherwise conflict with in the main mux.
	if isWellKnownJWKSPath(pattern) {
		if r.wellKnownMux == nil {
			r.wellKnownMux = http.NewServeMux()
		}

		r.wellKnownMux.HandleFunc(method+" "+pattern, r.wrapHandler(method, pattern, handler))
		r.logger.Debug("registered well-known route", "method", method, "pattern", pattern)

		return
	}

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
	// Amazon Managed Prometheus uses /workspaces paths
	prefixes := []string{"/_aws", "/_runtime", "/kumo", "/buckets", "/namespaces", "/tables", "/get-table", "/2020-05-31", "/2013-04-01", "/service", "/v1", "/tags", "/applications", "/workspaces", "/v20190125", "/v20180820", "/kx", "/create-app", "/describe-app", "/update-app", "/delete-app", "/list-apps", "/create-resiliency-policy", "/describe-resiliency-policy", "/update-resiliency-policy", "/delete-resiliency-policy", "/list-resiliency-policies", "/start-app-assessment", "/describe-app-assessment", "/delete-app-assessment", "/list-app-assessments", "/tag-resource", "/untag-resource", "/list-tags-for-resource", "/schemas", "/matchingworkflows", "/idmappingworkflows", "/providerservices", "/-", "/snapshots", "/apps", "/backup-vaults", "/backup", "/associations", "/codereviews", "/feedback", "/profilingGroups", "/maps", "/places", "/routes", "/geofencing", "/tracking", "/macie", "/allow-lists", "/jobs", "/custom-data-identifiers", "/findingsfilters", "/findings"}

	for _, prefix := range prefixes {
		if hasPathPrefix(pattern, prefix) {
			return prefix
		}
	}

	return ""
}

// isWellKnownJWKSPath reports whether path (or a route pattern) has the exact
// shape /{id}/.well-known/jwks.json — three segments, the last two literal.
// Deeper paths such as /{bucket}/key/.well-known/jwks.json are not matched, so
// only an object stored at a bucket's literal .well-known/jwks.json key is an
// (irreducible) collision with a real JWKS request.
func isWellKnownJWKSPath(path string) bool {
	if !strings.HasSuffix(path, wellKnownJWKSSuffix) {
		return false
	}

	id := strings.TrimSuffix(path, wellKnownJWKSSuffix)

	return len(id) > 1 && id[0] == '/' && !strings.Contains(id[1:], "/")
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

		w.Header().Set("x-amz-request-id", requestID)
		w.Header().Set("x-amzn-RequestId", requestID)

		var requestBody string

		if r.logger.Enabled(req.Context(), slog.LevelDebug) && req.Body != nil {
			body, err := io.ReadAll(req.Body)
			if err == nil {
				requestBody = string(body)
				req.Body = io.NopCloser(bytes.NewReader(body))
			}
		}

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		handler(wrapped, req)

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

		if requestBody != "" {
			r.logger.Debug("request body", "request_id", requestID, "body", requestBody)
		}
	}
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Virtual-hosted Lambda function URLs come first: every path on
	// {urlId}.lambda-url.<host> belongs to the function, including /health,
	// and the S3 host rewrite below must never see these hosts.
	if urlID, ok := extractFunctionURLHost(req.Host); ok {
		r.serveFunctionURL(w, req, urlID)

		return
	}

	// Handle health endpoint before ServeMux to avoid route conflicts.
	if req.URL.Path == "/health" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))

		return
	}

	// Per-pool JWKS lives on a dedicated mux (see wellKnownJWKSSuffix). It is
	// dispatched here, before S3 virtual-host rewriting, because a JWKS request
	// arrives on the plain host rather than a bucket vhost.
	if r.wellKnownMux != nil && isWellKnownJWKSPath(req.URL.Path) {
		r.wellKnownMux.ServeHTTP(w, req)

		return
	}

	// Virtual-hosted execute-api: a deployed API stage is invoked at
	// {apiId}.execute-api.<host>/{stage}/{path}. Dispatch to the service
	// that owns apiID (API Gateway v1 or v2).
	if apiID, ok := extractExecuteAPIHost(req.Host); ok {
		for _, h := range r.executeAPIHandlers {
			if h(w, req, apiID, req.URL.Path) {
				return
			}
		}

		// No service owns this API id: real API Gateway answers 403.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Forbidden"}`))

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

	r.serveMux(w, req)
}

// serveMux dispatches req to the scope, prefix or main mux.
//
// Go's ServeMux redirects a path holding empty, "." or ".." segments to its
// cleaned form, so the route is chosen on the cleaned path. S3 keys such as
// "/lead", "a//b" or "dir/" are distinct objects, though, so the handler gets
// the path as sent whenever that selects the same route.
func (r *Router) serveMux(w http.ResponseWriter, req *http.Request) {
	dispatch := req
	if cleaned := path.Clean(req.URL.Path); cleaned != req.URL.Path {
		dispatch = req.Clone(req.Context())
		dispatch.URL.Path = cleaned
	}

	mux := r.selectMux(dispatch)

	if dispatch != req {
		if preserved, ok := r.preservePath(mux, req, dispatch); ok {
			dispatch = preserved
		}
	}

	mux.ServeHTTP(w, dispatch)
}

// selectMux returns the mux that serves req: its scope router, else its
// prefix router, else the main mux.
func (r *Router) selectMux(req *http.Request) *http.ServeMux {
	if mux := r.scopedMux(req); mux != nil {
		return mux
	}

	if mux := r.prefixedMux(req); mux != nil {
		return mux
	}

	return r.mux
}

// preservePath returns orig, re-escaped by muxEscapedPath, when it selects the
// same mux as cleaned and that mux serves both with the same pattern, so
// r.URL.Path and path wildcards such as {key...} carry the path as sent. It
// reports false when the original path selects another route (e.g.
// "/bucket/" matches /{bucket}/{key...} while "/bucket" matches /{bucket}, or
// "/kumo/cdn/x/../../../bucket/key" selects the /kumo prefix router while its
// cleaned form selects the main mux) or has an empty segment before the
// pattern's trailing wildcard (e.g. "//bucket/key"), keeping the cleaned
// dispatch.
func (r *Router) preservePath(mux *http.ServeMux, orig, cleaned *http.Request) (*http.Request, bool) {
	escaped := orig.URL.EscapedPath()

	raw := muxEscapedPath(escaped)
	if raw == "" {
		return nil, false
	}

	_, want := mux.Handler(cleaned)
	if want == "" {
		return nil, false
	}

	// ServeMux matches the segments before a trailing {name...} wildcard (all
	// segments of a pattern without one) one path segment at a time, and
	// muxEscapedPath merges an empty segment into the next one, so a repeated
	// slash there would turn up as a leading "/" in a value such as {bucket}
	// or {id} instead of being dropped as in the cleaned path.
	if hasEmptySegment(escaped, fixedSegmentCount(want)) {
		return nil, false
	}

	preserved := orig.Clone(orig.Context())
	preserved.URL.RawPath = raw

	if r.selectMux(preserved) != mux {
		return nil, false
	}

	if _, got := mux.Handler(preserved); got != want {
		return nil, false
	}

	return preserved, true
}

// muxEscapedPath re-escapes an escaped request path so that ServeMux finds
// nothing to clean while it still unescapes to the same path: "." and ".."
// segments become "%2E" and "%2E%2E", and a slash that follows another slash
// becomes "%2F". ServeMux treats such escapes as plain characters, not as
// separators or dot segments, and unescapes them in path wildcards. It
// returns "" if the result would still be cleaned.
func muxEscapedPath(escaped string) string {
	if !strings.HasPrefix(escaped, "/") {
		return ""
	}

	segments := strings.Split(escaped, "/")
	for i, seg := range segments {
		switch seg {
		case ".":
			segments[i] = "%2E"
		case "..":
			segments[i] = "%2E%2E"
		}
	}

	joined := strings.Join(segments, "/")

	var b strings.Builder

	b.Grow(len(joined))

	for i := range len(joined) {
		if joined[i] == '/' && i > 0 && joined[i-1] == '/' {
			b.WriteString("%2F")

			continue
		}

		b.WriteByte(joined[i])
	}

	out := b.String()

	// ServeMux cleans with path.Clean but keeps a trailing slash.
	clean := path.Clean(out)
	if strings.HasSuffix(out, "/") && clean != "/" {
		clean += "/"
	}

	if clean != out {
		return ""
	}

	return out
}

// fixedSegmentCount returns how many path segments of a ServeMux pattern
// come before its trailing {name...} wildcard or subtree slash, or the number
// of all its segments if it has neither.
func fixedSegmentCount(pattern string) int {
	if i := strings.IndexByte(pattern, '/'); i >= 0 {
		pattern = pattern[i:] // drop the method and host
	}

	segments := strings.Split(strings.TrimPrefix(pattern, "/"), "/")

	if last := segments[len(segments)-1]; last == "" || strings.HasSuffix(last, "...}") {
		return len(segments) - 1
	}

	return len(segments)
}

// hasEmptySegment reports whether one of the first n segments of an escaped
// path is empty, i.e. the path repeats a slash before or between them. An
// escaped slash ("%2F") sent by the client is part of a segment, not a
// separator, and does not count.
func hasEmptySegment(escaped string, n int) bool {
	segments := strings.SplitN(strings.TrimPrefix(escaped, "/"), "/", n+1)

	for i := 0; i < n && i < len(segments); i++ {
		if segments[i] == "" {
			return true
		}
	}

	return false
}

// prefixedMux returns the router of the longest prefix matching the request
// path, or nil.
//
// Longest prefix wins, so short prefixes (e.g. "/apps") cannot capture
// longer ones (e.g. "/appsync").
func (r *Router) prefixedMux(req *http.Request) *http.ServeMux {
	bestPrefix := ""

	for prefix := range r.prefixRouters {
		if hasPathPrefix(req.URL.Path, prefix) && len(prefix) > len(bestPrefix) {
			bestPrefix = prefix
		}
	}

	if bestPrefix == "" {
		return nil
	}

	return r.prefixRouters[bestPrefix]
}

// scopedMux returns the scope router named by the request's SigV4 credential
// scope when it has a route for the request, or nil.
//
// The signing name is the only request attribute that disambiguates REST
// services sharing identical paths (e.g. GET /tags/{arn}) on a single
// endpoint, where real AWS uses per-service hostnames. Unsigned requests
// fall through to path-based routing: real AWS likewise refuses unsigned API
// calls, and path routing still serves kumo-native namespaces and unmigrated
// services. A nil return (unsigned, unmigrated service, or a path the scope
// router does not know) keeps the request on path-based routing.
func (r *Router) scopedMux(req *http.Request) *http.ServeMux {
	name := sigV4SigningName(req)
	if name == "" {
		return nil
	}

	mux, ok := r.scopeRouters[name]
	if !ok {
		return nil
	}

	if _, pattern := mux.Handler(req); pattern == "" {
		return nil
	}

	return mux
}

// Routes returns all registered routes.
func (r *Router) Routes() []Route {
	return r.routes
}

// serveFunctionURL dispatches a function URL request to the owning service;
// an unknown url id is refused the way AWS does (403 AccessDeniedException).
func (r *Router) serveFunctionURL(w http.ResponseWriter, req *http.Request, urlID string) {
	for _, h := range r.functionURLHandlers {
		if h(w, req, urlID) {
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-ErrorType", "AccessDeniedException")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"Message":null}`))
}

// extractFunctionURLHost recognises Lambda function URL virtual-hosted hosts
// and returns the lower-cased url id (see vhost.FunctionURLID).
func extractFunctionURLHost(host string) (string, bool) {
	return vhost.FunctionURLID(host)
}

// extractExecuteAPIHost recognises API Gateway execute-api virtual-hosted
// hosts and returns the API id (see vhost.ExecuteAPIID).
func extractExecuteAPIHost(host string) (string, bool) {
	return vhost.ExecuteAPIID(host)
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
	if host == localhostHost || host == "127.0.0.1" {
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
	case rest == localhostHost:
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
