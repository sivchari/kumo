package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Routes registered by newPathTestRouter.
const (
	routeBucket     = "bucket"
	routeBucketPost = "bucket-post"
	routeObject     = "object"
	routePreflight  = "preflight"
	routeEdge       = "edge"
	routeTags       = "tags"
)

const (
	signingLambda         = "lambda"
	keyDoubleSlash        = "a//b"
	objectPathDoubleSlash = "/bucket/a//b"
	objectPathDir         = "/bucket/dir/"
	edgePathDir           = "/kumo/cdn/E123/browser/"
	bucketPathSlash       = "/bucket/"
	cleanedBucketPath     = "/bucket"
	cleanedObjectPath     = "/bucket/key"
	cleanedObjectKey      = "key"
)

// observed records what a handler saw for a request.
type observed struct {
	route string
	path  string
	value string
}

// newPathTestRouter registers S3-style bucket (GET, POST) and object (GET,
// PUT, OPTIONS) routes, a CloudFront-style edge route under a prefix router,
// a fixed route without a trailing slash and a SigV4-scoped tags route. Each
// handler records the route it serves, r.URL.Path and the value of its
// trailing wildcard.
func newPathTestRouter(got *observed) *Router {
	r := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	record := func(route, wildcard string) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			*got = observed{route: route, path: req.URL.Path}
			if wildcard != "" {
				got.value = req.PathValue(wildcard)
			}

			w.WriteHeader(http.StatusOK)
		}
	}

	r.Handle(http.MethodGet, "/{bucket}", record(routeBucket, "bucket"))
	r.Handle(http.MethodPost, "/{bucket}", record(routeBucketPost, "bucket"))
	r.Handle(http.MethodGet, "/{bucket}/{key...}", record(routeObject, "key"))
	r.Handle(http.MethodPut, "/{bucket}/{key...}", record(routeObject, "key"))
	r.Handle(http.MethodOptions, "/{bucket}/{key...}", record(routePreflight, "key"))
	r.Handle(http.MethodGet, "/kumo/cdn/{id}/{path...}", record(routeEdge, "path"))
	r.Handle(http.MethodGet, "/2020-05-31/distribution", record("fixed", ""))
	r.ScopedRouter(signingLambda).Handle(http.MethodGet, "/2017-03-31/tags/{arn...}", record(routeTags, "arn"))

	return r
}

// TestRouter_UncleanPathAcrossMuxesKeepsCleanedDispatch verifies that the
// path as sent is only preserved when it selects the same mux as its cleaned
// form. Otherwise the cleaned dispatch is used, as before, even if the
// original path happens to match the same pattern on the cleaned path's mux.
func TestRouter_UncleanPathAcrossMuxesKeepsCleanedDispatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		target    string
		signing   string
		wantRoute string
		wantPath  string
		wantValue string
	}{
		{"prefixed to main", "/kumo/cdn/E123/../../../bucket/key", "", routeObject, cleanedObjectPath, cleanedObjectKey},
		{"fixed prefix to main", "/2020-05-31/distribution/../../bucket/key", "", routeObject, cleanedObjectPath, cleanedObjectKey},
		{"main to prefixed", "/bucket/../kumo/cdn/E123/x", "", routeEdge, "/kumo/cdn/E123/x", "x"},
		{"scoped to main", "/2017-03-31/tags/x/../../../bucket/key", signingLambda, routeObject, cleanedObjectPath, cleanedObjectKey},
		{"main to scoped", "/bucket/../2017-03-31/tags/arn", signingLambda, routeTags, "/2017-03-31/tags/arn", "arn"},
		{"within scoped", "/2017-03-31/tags/a//b", signingLambda, routeTags, "/2017-03-31/tags/a//b", keyDoubleSlash},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got observed

			r := newPathTestRouter(&got)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, http.NoBody)

			if tc.signing != "" {
				req.Header.Set("Authorization", authHeader(tc.signing))
			}

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d (Location %q), want 200", rec.Code, rec.Header().Get("Location"))
			}

			if got != (observed{route: tc.wantRoute, path: tc.wantPath, value: tc.wantValue}) {
				t.Errorf("observed %+v, want route=%q path=%q value=%q", got, tc.wantRoute, tc.wantPath, tc.wantValue)
			}
		})
	}
}

// TestRouter_UncleanPathPreservedForHandler verifies that a request whose
// path holds empty, "." or ".." segments or a trailing slash is served
// directly (no ServeMux redirect) and that the handler sees the path as
// sent, so an S3 key or a CloudFront edge path is not rewritten.
func TestRouter_UncleanPathPreservedForHandler(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		method    string
		target    string
		wantRoute string
		wantPath  string
		wantValue string
	}{
		{"trailing slash key", http.MethodPut, objectPathDir, routeObject, objectPathDir, "dir/"},
		{"leading slash key", http.MethodPut, "/bucket//lead", routeObject, "/bucket//lead", "/lead"},
		{"double slash in key", http.MethodGet, objectPathDoubleSlash, routeObject, objectPathDoubleSlash, keyDoubleSlash},
		{"triple slash in key", http.MethodGet, "/bucket/a///b", routeObject, "/bucket/a///b", "a///b"},
		{"dot segment in key", http.MethodGet, "/bucket/a/./b", routeObject, "/bucket/a/./b", "a/./b"},
		{"dot-dot segment in key", http.MethodGet, "/bucket/a/../b", routeObject, "/bucket/a/../b", "a/../b"},
		{"trailing double slash key", http.MethodGet, "/bucket/a//", routeObject, "/bucket/a//", "a//"},
		{"escaped slash kept", http.MethodGet, "/bucket/a%2F/b//c", routeObject, "/bucket/a//b//c", "a//b//c"},
		{"clean key unchanged", http.MethodGet, "/bucket/a/b", routeObject, "/bucket/a/b", "a/b"},
		{"edge trailing slash", http.MethodGet, edgePathDir, routeEdge, edgePathDir, "browser/"},
		{"edge double slash", http.MethodGet, "/kumo/cdn/E123/a//b", routeEdge, "/kumo/cdn/E123/a//b", keyDoubleSlash},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got observed

			r := newPathTestRouter(&got)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), tc.method, tc.target, http.NoBody))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d (Location %q), want 200", rec.Code, rec.Header().Get("Location"))
			}

			if got.route != tc.wantRoute {
				t.Errorf("route = %q, want %q", got.route, tc.wantRoute)
			}

			if got.path != tc.wantPath {
				t.Errorf("r.URL.Path = %q, want %q", got.path, tc.wantPath)
			}

			if got.value != tc.wantValue {
				t.Errorf("path value = %q, want %q", got.value, tc.wantValue)
			}
		})
	}
}

// TestRouter_UncleanPathKeepsCleanedRoute verifies that when the path as sent
// would select a different route than its cleaned form (here: a trailing
// slash), the request is still dispatched on the cleaned path, as before.
func TestRouter_UncleanPathKeepsCleanedRoute(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		target    string
		wantRoute string
		wantPath  string
	}{
		{"bucket with trailing slash", bucketPathSlash, routeBucket, cleanedBucketPath},
		{"fixed route with trailing slash", "/2020-05-31/distribution/", "fixed", "/2020-05-31/distribution"},
		{"fixed route with double slash", "/2020-05-31//distribution", "fixed", "/2020-05-31/distribution"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got observed

			r := newPathTestRouter(&got)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, http.NoBody))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d (Location %q), want 200", rec.Code, rec.Header().Get("Location"))
			}

			if got.route != tc.wantRoute {
				t.Errorf("route = %q, want %q", got.route, tc.wantRoute)
			}

			if got.path != tc.wantPath {
				t.Errorf("r.URL.Path = %q, want %q", got.path, tc.wantPath)
			}
		})
	}
}

// TestRouter_UncleanPathMethodResolution verifies the route comparison under
// ServeMux's method resolution: HEAD served by a GET route, an OPTIONS
// route, a method that has a route only for the cleaned path (the original
// path alone would get 405), and a method that has a route for neither, which
// still gets ServeMux's 405 for the cleaned path.
func TestRouter_UncleanPathMethodResolution(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantAllow  string
		wantRoute  string
		wantPath   string
		wantValue  string
	}{
		{"HEAD served by GET object route", http.MethodHead, objectPathDoubleSlash, http.StatusOK, "", routeObject, objectPathDoubleSlash, keyDoubleSlash},
		{"HEAD served by GET edge route", http.MethodHead, edgePathDir, http.StatusOK, "", routeEdge, edgePathDir, "browser/"},
		{"HEAD bucket with trailing slash", http.MethodHead, bucketPathSlash, http.StatusOK, "", routeBucket, cleanedBucketPath, "bucket"},
		{"OPTIONS object route", http.MethodOptions, objectPathDir, http.StatusOK, "", routePreflight, objectPathDir, "dir/"},
		{"method routed only for cleaned path", http.MethodPost, bucketPathSlash, http.StatusOK, "", routeBucketPost, cleanedBucketPath, "bucket"},
		{"method routed for neither path", http.MethodDelete, objectPathDoubleSlash, http.StatusMethodNotAllowed, "GET, HEAD, OPTIONS, PUT", "", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got observed

			r := newPathTestRouter(&got)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), tc.method, tc.target, http.NoBody))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d (Location %q), want %d", rec.Code, rec.Header().Get("Location"), tc.wantStatus)
			}

			if allow := rec.Header().Get("Allow"); allow != tc.wantAllow {
				t.Errorf("Allow = %q, want %q", allow, tc.wantAllow)
			}

			if got != (observed{route: tc.wantRoute, path: tc.wantPath, value: tc.wantValue}) {
				t.Errorf("observed %+v, want route=%q path=%q value=%q", got, tc.wantRoute, tc.wantPath, tc.wantValue)
			}
		})
	}
}

func TestMuxEscapedPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"/", "/"},
		{"/b/a/b", "/b/a/b"},
		{"/b/dir/", "/b/dir/"},
		{"/b//lead", "/b/%2Flead"},
		{"/b/a///b", "/b/a/%2F%2Fb"},
		{"/b/a//", "/b/a/%2F"},
		{"/b/./a/..", "/b/%2E/a/%2E%2E"},
		{"/b/a//../c", "/b/a/%2F%2E%2E/c"},
		{"/b/..x/.y", "/b/..x/.y"},
		{"/b/a%2Fb", "/b/a%2Fb"},
	}

	for _, tc := range cases {
		if got := muxEscapedPath(tc.in); got != tc.want {
			t.Errorf("muxEscapedPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
