package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// Escaped request paths used by several tests below.
const (
	escapedSlashKey   = "/bucket/a%2Fb"
	objectPathAB      = "/bucket/a/b"
	keyAB             = "a/b"
	keyDir            = "dir/"
	escapedSpaceKey   = "/bucket/my%20key"
	unicodeKeyPath    = "/bucket/日"
	escapedUnicodeKey = "/bucket/%E6%97%A5"
	plusKeyPath       = "/bucket/a+b"
	escapedPlusKey    = "/bucket/a%2Bb"
	escapedPercentKey = "/bucket/100%25"
	spacelessPlusKey  = "/bucket/my+key"
	escapedDotKey     = "/bucket/a/%2E%2E/b"
	subtreeRoute      = "subtree"
	subtreeBase       = "/static/"
	prefixSubtree     = "/kumo/files/"
)

// seen records what a handler saw for a request.
type seen struct {
	route   string
	path    string
	rawPath string
	value   string
}

// recordSeen returns a handler that stores what it saw in got. wildcard
// names the path value to record; an empty name records none.
func recordSeen(got *seen, route, wildcard string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		*got = seen{route: route, path: req.URL.Path, rawPath: req.URL.RawPath}
		if wildcard != "" {
			got.value = req.PathValue(wildcard)
		}

		w.WriteHeader(http.StatusOK)
	}
}

type muxPathCase struct {
	name         string
	path         string
	rawPath      string
	wantStatus   int
	wantLocation string
	wantPath     string
	wantValue    string
}

var muxPathCases = []muxPathCase{
	{
		name:         "double slash without RawPath is redirected",
		path:         objectPathDoubleSlash,
		wantStatus:   http.StatusTemporaryRedirect,
		wantLocation: objectPathAB,
	},
	{
		name:         "dot segment without RawPath is redirected",
		path:         objectPathDotDot,
		wantStatus:   http.StatusTemporaryRedirect,
		wantLocation: "/bucket/b",
	},
	{
		name:       "escaped slash for the empty segment is served as sent",
		path:       objectPathDoubleSlash,
		rawPath:    "/bucket/a/%2Fb",
		wantStatus: http.StatusOK,
		wantPath:   objectPathDoubleSlash,
		wantValue:  "a//b",
	},
	{
		name:       "escaped dot segments are served as sent",
		path:       objectPathDotDot,
		rawPath:    escapedDotKey,
		wantStatus: http.StatusOK,
		wantPath:   objectPathDotDot,
		wantValue:  "a/../b",
	},
	{
		name:       "escaped slash is not a separator but unescapes in the wildcard",
		path:       objectPathAB,
		rawPath:    escapedSlashKey,
		wantStatus: http.StatusOK,
		wantPath:   objectPathAB,
		wantValue:  keyAB,
	},
	{
		name:       "RawPath that is not an encoding of Path is ignored",
		path:       objectPathAB,
		rawPath:    "/bucket/x%2Fy",
		wantStatus: http.StatusOK,
		wantPath:   objectPathAB,
		wantValue:  keyAB,
	},
}

// TestServeMux_PathAssumptions pins the net/http.ServeMux behaviour that
// Router.serveMux relies on: the mux cleans and redirects on the escaped
// path, and it does not touch a path whose empty and dot segments are
// escaped, unescaping them for path wildcards only.
func TestServeMux_PathAssumptions(t *testing.T) {
	t.Parallel()

	cases := muxPathCases

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got seen

			mux := http.NewServeMux()
			mux.HandleFunc("GET /{bucket}/{key...}", recordSeen(&got, routeObject, "key"))

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			req.URL.Path = tc.path
			req.URL.RawPath = tc.rawPath

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d (Location %q), want %d", rec.Code, rec.Header().Get("Location"), tc.wantStatus)
			}

			if loc := rec.Header().Get("Location"); loc != tc.wantLocation {
				t.Errorf("Location = %q, want %q", loc, tc.wantLocation)
			}

			if got.path != tc.wantPath || got.value != tc.wantValue {
				t.Errorf("handler saw path=%q value=%q, want path=%q value=%q", got.path, got.value, tc.wantPath, tc.wantValue)
			}
		})
	}
}

// TestURL_EscapedPathContract pins the net/url behaviour the router uses to
// recover the path as sent: RawPath is kept only when it is a valid encoding
// of Path, and EscapedPath returns it in that case, else the default
// encoding of Path.
func TestURL_EscapedPathContract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		target      string
		wantPath    string
		wantRawPath string
		wantEscaped string
	}{
		{"default encoding leaves RawPath empty", escapedSpaceKey, "/bucket/my key", "", escapedSpaceKey},
		{"unicode in default encoding leaves RawPath empty", escapedUnicodeKey, unicodeKeyPath, "", escapedUnicodeKey},
		{"raw unicode is kept in RawPath but re-encoded by EscapedPath", unicodeKeyPath, unicodeKeyPath, unicodeKeyPath, escapedUnicodeKey},
		{"plus stays a plus in the path", plusKeyPath, plusKeyPath, "", plusKeyPath},
		{"escaped plus is kept as sent", escapedPlusKey, plusKeyPath, escapedPlusKey, escapedPlusKey},
		{"escaped slash is kept as sent", escapedSlashKey, objectPathAB, escapedSlashKey, escapedSlashKey},
		{"double slash needs no RawPath", objectPathDoubleSlash, objectPathDoubleSlash, "", objectPathDoubleSlash},
		{"escaped dots are kept as sent", escapedDotKey, objectPathDotDot, escapedDotKey, escapedDotKey},
		{"escaped percent is kept", escapedPercentKey, "/bucket/100%", "", escapedPercentKey},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			u, err := url.ParseRequestURI(tc.target)
			if err != nil {
				t.Fatalf("ParseRequestURI(%q): %v", tc.target, err)
			}

			if u.Path != tc.wantPath {
				t.Errorf("Path = %q, want %q", u.Path, tc.wantPath)
			}

			if u.RawPath != tc.wantRawPath {
				t.Errorf("RawPath = %q, want %q", u.RawPath, tc.wantRawPath)
			}

			if got := u.EscapedPath(); got != tc.wantEscaped {
				t.Errorf("EscapedPath() = %q, want %q", got, tc.wantEscaped)
			}
		})
	}
}

type keyEncodingCase struct {
	name      string
	target    string
	wantPath  string
	wantValue string
}

var keyEncodingCases = []keyEncodingCase{
	{"unicode", "/bucket/%E6%97%A5%E6%9C%AC%E8%AA%9E/%D0%BA%D0%BB%D1%8E%D1%87", "/bucket/日本語/ключ", "日本語/ключ"},
	{"raw unicode in request target", "/bucket/日本語", "/bucket/日本語", "日本語"},
	{"emoji outside the BMP", "/bucket/%F0%9F%90%A7", "/bucket/\U0001f427", "\U0001f427"},
	{"percent-encoded space", escapedSpaceKey, "/bucket/my key", "my key"},
	{"plus is not a space", spacelessPlusKey, spacelessPlusKey, "my+key"},
	{"escaped plus is a plus", "/bucket/my%2Bkey", spacelessPlusKey, "my+key"},
	{"space and plus together", "/bucket/a%20b+c", "/bucket/a b+c", "a b+c"},
	{"escaped percent stays literal", escapedPercentKey, "/bucket/100%", "100%"},
	{"escaped percent followed by hex digits", "/bucket/a%252Fb", "/bucket/a%2Fb", "a%2Fb"},
	{"escaped question mark", "/bucket/what%3F", "/bucket/what?", "what?"},
	{"escaped slash", escapedSlashKey, objectPathAB, keyAB},
	{"double slash with spaces", "/bucket/my%20dir//a%20b", "/bucket/my dir//a b", "my dir//a b"},
	{"double slash with unicode", "/bucket/%E6%97%A5//%E6%9C%AC", "/bucket/日//本", "日//本"},
	{"double slash with plus", "/bucket/a+b//c%2Bd", "/bucket/a+b//c+d", "a+b//c+d"},
	{"leading slash key with space", "/bucket//my%20key", "/bucket//my key", "/my key"},
	{"trailing slash key with unicode", "/bucket/%E6%97%A5/", "/bucket/日/", "日/"},
	{"dot segment with space", "/bucket/a%20b/./c", "/bucket/a b/./c", "a b/./c"},
	{"escaped dot-dot with unicode", "/bucket/%E6%97%A5/%2E%2E/x", "/bucket/日/../x", "日/../x"},
}

// TestRouter_ObjectKeyEncodingsReachHandler verifies that S3-style keys with
// unicode, spaces, plus signs and repeated slashes reach the handler with the
// key exactly as sent, whether or not the path also needs cleaning, and that
// no redirect is issued.
func TestRouter_ObjectKeyEncodingsReachHandler(t *testing.T) {
	t.Parallel()

	cases := keyEncodingCases

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got seen

			r := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))
			r.Handle(http.MethodGet, "/{bucket}", recordSeen(&got, routeBucket, "bucket"))
			r.Handle(http.MethodGet, "/{bucket}/{key...}", recordSeen(&got, routeObject, "key"))

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, http.NoBody))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d (Location %q), want 200", rec.Code, rec.Header().Get("Location"))
			}

			if got.route != routeObject {
				t.Errorf("route = %q, want %q", got.route, routeObject)
			}

			if got.path != tc.wantPath {
				t.Errorf("r.URL.Path = %q, want %q", got.path, tc.wantPath)
			}

			if got.value != tc.wantValue {
				t.Errorf("key = %q, want %q", got.value, tc.wantValue)
			}

			// Whatever RawPath the router left must still be an encoding of Path.
			u := url.URL{Path: got.path, RawPath: got.rawPath}
			if unescaped, err := url.PathUnescape(u.EscapedPath()); err != nil || unescaped != got.path {
				t.Errorf("EscapedPath() = %q does not unescape to Path %q (err %v)", u.EscapedPath(), got.path, err)
			}
		})
	}
}

// newSubtreeRouter registers trailing-slash (subtree) patterns on the main
// mux, next to the S3 object wildcard, and under the /kumo prefix router, next
// to a single wildcard route. It has no "/{bucket}" route, which would take
// over the root of a main-mux subtree (see
// TestRouter_SubtreeRootShadowedByBucketRoute).
func newSubtreeRouter(got *seen) *Router {
	r := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	r.Handle(http.MethodGet, "/{bucket}/{key...}", recordSeen(got, routeObject, "key"))
	r.Handle(http.MethodGet, subtreeBase, recordSeen(got, subtreeRoute, ""))
	r.Handle(http.MethodGet, prefixSubtree, recordSeen(got, subtreeRoute, ""))
	r.Handle(http.MethodGet, "/kumo/other/{id}", recordSeen(got, "other", "id"))

	return r
}

// TestRouter_TrailingSlashSubtreePattern verifies that a pattern ending in
// "/" (ServeMux's subtree form) matches its own root, deeper paths and
// paths with repeated slashes, never redirects its root to itself, and hands
// the handler the path as sent.
func TestRouter_TrailingSlashSubtreePattern(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		method    string
		target    string
		wantRoute string
		wantPath  string
	}{
		{"main root", http.MethodGet, subtreeBase, subtreeRoute, subtreeBase},
		{"main deeper", http.MethodGet, "/static/css/app.css", subtreeRoute, "/static/css/app.css"},
		{"main deeper trailing slash", http.MethodGet, "/static/css/", subtreeRoute, "/static/css/"},
		{"main empty segment", http.MethodGet, "/static/css//app.css", subtreeRoute, "/static/css//app.css"},
		{"main leading empty segment under subtree", http.MethodGet, "/static//app.css", subtreeRoute, "/static//app.css"},
		{"main dot-dot segment", http.MethodGet, "/static/a/../b", subtreeRoute, "/static/a/../b"},
		{"main HEAD", http.MethodHead, subtreeBase, subtreeRoute, subtreeBase},
		{"main empty segment before the pattern", http.MethodGet, "//static/app.css", subtreeRoute, "/static/app.css"},
		{"main escaped unicode", http.MethodGet, "/static/%E6%97%A5%20x/", subtreeRoute, "/static/日 x/"},
		{"prefix root", http.MethodGet, prefixSubtree, subtreeRoute, prefixSubtree},
		{"prefix deeper", http.MethodGet, "/kumo/files/a/b", subtreeRoute, "/kumo/files/a/b"},
		{"prefix empty segment", http.MethodGet, "/kumo/files/a//b", subtreeRoute, "/kumo/files/a//b"},
		{"prefix empty segment before the pattern", http.MethodGet, "/kumo//files/a", subtreeRoute, "/kumo/files/a"},
		{"prefix sibling single wildcard", http.MethodGet, "/kumo/other/x", "other", "/kumo/other/x"},
		{"prefix sibling is not captured by the subtree", http.MethodGet, "/kumo/files2/x", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got seen

			r := newSubtreeRouter(&got)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), tc.method, tc.target, http.NoBody))

			if tc.wantRoute == "" {
				if rec.Code != http.StatusNotFound {
					t.Fatalf("status = %d, want 404", rec.Code)
				}

				return
			}

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

// TestRouter_SubtreePatternWithoutSlashRedirects pins ServeMux's own redirect
// from a subtree root without its trailing slash, which the router leaves to
// the mux because the path needs no cleaning.
func TestRouter_SubtreePatternWithoutSlashRedirects(t *testing.T) {
	t.Parallel()

	cases := []struct {
		target       string
		wantLocation string
	}{
		{"/static", subtreeBase},
		{"/kumo/files", prefixSubtree},
	}

	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()

			var got seen

			r := newSubtreeRouter(&got)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, http.NoBody))

			if rec.Code != http.StatusTemporaryRedirect {
				t.Fatalf("status = %d, want 307", rec.Code)
			}

			if loc := rec.Header().Get("Location"); loc != tc.wantLocation {
				t.Errorf("Location = %q, want %q", loc, tc.wantLocation)
			}
		})
	}
}

// TestRouter_SubtreeRootShadowedByBucketRoute pins that on the main mux the
// root of a subtree pattern is served by a "/{bucket}" route, as it was
// before the path was preserved: the cleaned path "/static" selects
// "/{bucket}", and "/static/" selects another pattern, so the cleaned
// dispatch is kept. Deeper paths still reach the subtree handler.
func TestRouter_SubtreeRootShadowedByBucketRoute(t *testing.T) {
	t.Parallel()

	cases := []struct {
		target    string
		wantRoute string
		wantPath  string
	}{
		{subtreeBase, routeBucket, "/static"},
		{"/static/a", subtreeRoute, "/static/a"},
	}

	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()

			var got seen

			r := newSubtreeRouter(&got)
			r.Handle(http.MethodGet, "/{bucket}", recordSeen(&got, routeBucket, "bucket"))

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, http.NoBody))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d (Location %q), want 200", rec.Code, rec.Header().Get("Location"))
			}

			if got.route != tc.wantRoute || got.path != tc.wantPath {
				t.Errorf("route=%q path=%q, want route=%q path=%q", got.route, got.path, tc.wantRoute, tc.wantPath)
			}
		})
	}
}

// TestRouter_VirtualHostedKeyEncodingsReachHandler verifies that keys sent to
// a virtual-hosted-style endpoint, where the router prefixes the bucket to
// the path, reach the handler as sent even though the request's RawPath no
// longer matches its rewritten Path.
func TestRouter_VirtualHostedKeyEncodingsReachHandler(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		target    string
		wantPath  string
		wantValue string
	}{
		{"space and unicode", "/my%20dir/%E6%97%A5", "/my-bucket/my dir/日", "my dir/日"},
		{"plus", "/a+b", "/my-bucket/a+b", "a+b"},
		{"escaped slash", "/a%2Fb", "/my-bucket/a/b", keyAB},
		{"double slash", "/a//b", "/my-bucket/a//b", "a//b"},
		{"leading slash key", "//lead", "/my-bucket//lead", "/lead"},
		{"trailing slash key", "/dir/", "/my-bucket/dir/", keyDir},
		{"escaped dot-dot", "/a/%2E%2E/b", "/my-bucket/a/../b", "a/../b"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got seen

			r := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))
			r.Handle(http.MethodGet, "/{bucket}", recordSeen(&got, routeBucket, "bucket"))
			r.Handle(http.MethodGet, "/{bucket}/{key...}", recordSeen(&got, routeObject, "key"))

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, http.NoBody)
			req.Host = myBucketHost

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d (Location %q), want 200", rec.Code, rec.Header().Get("Location"))
			}

			if got.route != routeObject || got.path != tc.wantPath || got.value != tc.wantValue {
				t.Errorf("route=%q path=%q key=%q, want route=%q path=%q key=%q",
					got.route, got.path, got.value, routeObject, tc.wantPath, tc.wantValue)
			}
		})
	}
}
