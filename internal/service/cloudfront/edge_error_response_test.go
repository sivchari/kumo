package cloudfront

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/textproto"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sivchari/kumo/internal/service/cloudfront/cache"
)

const (
	originContent  = "content"
	originPages    = "pages"
	errorsPattern  = "/errors/*"
	page404Path    = "/errors/404.html"
	page403Path    = "/errors/403.html"
	page404Body    = "<h1>not here</h1>"
	page403Body    = "<h1>sign in first</h1>"
	missingPath    = "/missing.txt"
	contentMissing = "origin 404 body"

	// Request header names the CORS and Vary tests send or vary on.
	headerOrigin         = "Origin"
	headerAcceptEncoding = "Accept-Encoding"
)

// newContentOrigin answers /missing.txt with 404, /boom with 500 and anything
// else with 200 "content".
func newContentOrigin(t *testing.T) *httptest.Server {
	t.Helper()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case missingPath:
			http.Error(w, contentMissing, http.StatusNotFound)
		case "/boom":
			http.Error(w, "origin 500 body", http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte(originContent))
		}
	}))
	t.Cleanup(origin.Close)

	return origin
}

// newPagesOrigin serves the two custom error pages and 404s everything else.
func newPagesOrigin(t *testing.T) *httptest.Server {
	t.Helper()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case page404Path:
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(page404Body))
		case page403Path:
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(page403Body))
		default:
			http.Error(w, "no such page", http.StatusNotFound)
		}
	}))
	t.Cleanup(origin.Close)

	return origin
}

// pageRequest is what a recording pages origin saw of the last page request.
type pageRequest struct {
	method string
	header http.Header
}

// newRecordingPagesOrigin answers every request with page404Body and returns
// a function reporting the last request it got.
func newRecordingPagesOrigin(t *testing.T) (*httptest.Server, func() pageRequest) {
	t.Helper()

	var (
		mu   sync.Mutex
		last pageRequest
	)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		last = pageRequest{method: r.Method, header: r.Header.Clone()}
		mu.Unlock()

		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page404Body))
	}))
	t.Cleanup(origin.Close)

	return origin, func() pageRequest {
		mu.Lock()
		defer mu.Unlock()

		return last
	}
}

// viewerOnlyHeaders are request headers that belong to the viewer's own
// request and must not reach the origin of the custom error page.
func viewerOnlyHeaders() http.Header {
	return http.Header{
		headerAuthorization: {viewerAuthorization},
		"Cookie":            {"session=viewer"},
		"If-None-Match":     {`"viewer-etag"`},
		"Range":             {"bytes=0-3"},
		"X-Viewer":          {"1"},
	}
}

// newErrorPageDistribution routes `/errors/*` to pagesURL and everything else
// to contentURL, with the given default behaviour and custom error responses.
func newErrorPageDistribution(t *testing.T, contentURL, pagesURL string, defaultBehavior *DefaultCacheBehaviorXML, responses ...CustomErrorResponseXML) *Service {
	t.Helper()

	return newErrorPageDistributionWithPagesBehavior(t, contentURL, pagesURL, defaultBehavior, &DefaultCacheBehaviorXML{}, responses...)
}

func newErrorPageDistributionWithPagesBehavior(t *testing.T, contentURL, pagesURL string, defaultBehavior, pagesBehavior *DefaultCacheBehaviorXML, responses ...CustomErrorResponseXML) *Service {
	t.Helper()

	pagesHost, pagesPort := splitHostPort(strings.TrimPrefix(pagesURL, "http://"))
	pagesOrigin := &OriginXML{DomainName: pagesHost, CustomOriginConfig: &CustomOriginConfigXML{HTTPPort: pagesPort, OriginProtocolPolicy: originPolicyHTTP}}

	return createErrorPageDistribution(t, New(NewMemoryStorage()), contentURL, pagesOrigin, defaultBehavior, pagesBehavior, responses...)
}

// createErrorPageDistribution creates in svc's storage a distribution that
// routes `/errors/*` to pagesOrigin and everything else to contentURL.
func createErrorPageDistribution(t *testing.T, svc *Service, contentURL string, pagesOrigin *OriginXML, defaultBehavior, pagesBehavior *DefaultCacheBehaviorXML, responses ...CustomErrorResponseXML) *Service {
	t.Helper()

	contentHost, contentPort := splitHostPort(strings.TrimPrefix(contentURL, "http://"))

	defaultBehavior.TargetOriginID = originContent
	pagesBehavior.TargetOriginID = originPages
	pagesOrigin.ID = originPages

	if _, err := svc.storage.CreateDistribution(t.Context(), &CreateDistributionRequest{
		CallerReference: "custom-error-responses",
		Enabled:         true,
		Origins: &OriginsXML{Quantity: 2, Items: &OriginList{Origin: []OriginXML{
			{ID: originContent, DomainName: contentHost, CustomOriginConfig: &CustomOriginConfigXML{HTTPPort: contentPort, OriginProtocolPolicy: originPolicyHTTP}},
			*pagesOrigin,
		}}},
		DefaultCacheBehavior: defaultBehavior,
		CacheBehaviors: &CacheBehaviorsXML{Quantity: 1, Items: []CacheBehaviorXML{
			{PathPattern: errorsPattern, DefaultCacheBehaviorXML: *pagesBehavior},
		}},
		CustomErrorResponses: &CustomErrorResponsesXML{
			Quantity: len(responses),
			Items:    &CustomErrorResponseListXML{CustomErrorResponse: responses},
		},
	}); err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	return svc
}

// TestEdge_CustomErrorResponseReplacesOriginError — an origin 404 is replaced
// by the page fetched through the `/errors/*` behaviour, with ResponseCode as
// the status.
func TestEdge_CustomErrorResponseReplacesOriginError(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		responseCode string
		want         int
	}{
		{"response code 200", "200", http.StatusOK},
		{"response code 404", "404", http.StatusNotFound},
		{"no response code keeps the origin status", "", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newErrorPageDistribution(t, newContentOrigin(t).URL, newPagesOrigin(t).URL, &DefaultCacheBehaviorXML{},
				CustomErrorResponseXML{ErrorCode: http.StatusNotFound, ResponsePagePath: page404Path, ResponseCode: tc.responseCode})

			w := callEdge(t, svc, http.MethodGet, missingPath, nil)

			if w.Code != tc.want || w.Body.String() != page404Body {
				t.Errorf("got %d %q, want %d %q", w.Code, w.Body.String(), tc.want, page404Body)
			}

			if got := w.Header().Get("Content-Type"); got != "text/html" {
				t.Errorf("Content-Type = %q, want the page's text/html", got)
			}

			if got := w.Header().Get(headerXCache); got != errorPageCacheTag {
				t.Errorf("X-Cache = %q, want %q", got, errorPageCacheTag)
			}
		})
	}
}

// TestEdge_CustomErrorResponseUnsupportedResponseCode — a ResponseCode outside
// the codes CloudFront returns with a custom page keeps the original status,
// and the page body still reaches the viewer over a real HTTP connection.
func TestEdge_CustomErrorResponseUnsupportedResponseCode(t *testing.T) {
	t.Parallel()

	for _, code := range []string{"204", "302", "103"} {
		t.Run(code, func(t *testing.T) {
			t.Parallel()

			svc := newErrorPageDistribution(t, newContentOrigin(t).URL, newPagesOrigin(t).URL, &DefaultCacheBehaviorXML{},
				CustomErrorResponseXML{ErrorCode: http.StatusNotFound, ResponsePagePath: page404Path, ResponseCode: code})

			mem, ok := svc.storage.(*MemoryStorage)
			if !ok {
				t.Fatalf("expected *MemoryStorage, got %T", svc.storage)
			}

			var distID string
			for id := range mem.Distributions {
				distID = id
			}

			mux := http.NewServeMux()
			mux.HandleFunc("GET /kumo/cdn/{distributionId}/{path...}", svc.Edge)

			edge := httptest.NewServer(mux)
			t.Cleanup(edge.Close)

			client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, edge.URL+"/kumo/cdn/"+distID+missingPath, http.NoBody)
			if err != nil {
				t.Fatal(err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}

			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}

			if resp.StatusCode != http.StatusNotFound || string(body) != page404Body {
				t.Errorf("got %q %q, want 404 %q", resp.Status, body, page404Body)
			}
		})
	}
}

// TestEdge_CustomErrorResponseLeavesOtherResponses — successes, codes with no
// entry and caching-only entries reach the viewer unchanged.
func TestEdge_CustomErrorResponseLeavesOtherResponses(t *testing.T) {
	t.Parallel()

	ttl := int64(5)
	svc := newErrorPageDistribution(t, newContentOrigin(t).URL, newPagesOrigin(t).URL, &DefaultCacheBehaviorXML{},
		CustomErrorResponseXML{ErrorCode: http.StatusForbidden, ResponsePagePath: page403Path, ResponseCode: "403"},
		CustomErrorResponseXML{ErrorCode: http.StatusInternalServerError, ErrorCachingMinTTL: &ttl})

	if w := callEdge(t, svc, http.MethodGet, "/index.html", nil); w.Code != http.StatusOK || w.Body.String() != originContent {
		t.Errorf("success: got %d %q", w.Code, w.Body.String())
	}

	if w := callEdge(t, svc, http.MethodGet, missingPath, nil); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), contentMissing) {
		t.Errorf("404 without an entry: got %d %q", w.Code, w.Body.String())
	}

	if w := callEdge(t, svc, http.MethodGet, "/boom", nil); w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "origin 500 body") {
		t.Errorf("500 with a caching-only entry: got %d %q", w.Code, w.Body.String())
	}
}

// TestEdge_CustomErrorResponseForSignedCookie403 — the edge's own 403 for a
// request without signed credentials is replaced too, and the page is fetched
// without the viewer's signed-cookie check.
func TestEdge_CustomErrorResponseForSignedCookie403(t *testing.T) {
	t.Parallel()

	protected := func() *DefaultCacheBehaviorXML {
		return &DefaultCacheBehaviorXML{TrustedKeyGroups: &TrustedKeyGroupsXML{Enabled: true, Quantity: 1, Items: []string{"kg-private"}}}
	}

	for name, pagesBehavior := range map[string]*DefaultCacheBehaviorXML{
		"pages behind an open behaviour":     {},
		"pages behind a protected behaviour": protected(),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc := newErrorPageDistributionWithPagesBehavior(t, newContentOrigin(t).URL, newPagesOrigin(t).URL, protected(), pagesBehavior,
				CustomErrorResponseXML{ErrorCode: http.StatusForbidden, ResponsePagePath: page403Path, ResponseCode: "403"})

			w := callEdge(t, svc, http.MethodGet, "/private.txt", nil)
			if w.Code != http.StatusForbidden || w.Body.String() != page403Body {
				t.Errorf("got %d %q, want 403 %q", w.Code, w.Body.String(), page403Body)
			}
		})
	}
}

// TestEdge_CustomErrorPageRequestLeavesViewerHeadersBehind — the page is
// requested with a GET of its own: the viewer's credentials, cookies,
// conditional and range headers stay with the viewer's request.
func TestEdge_CustomErrorPageRequestLeavesViewerHeadersBehind(t *testing.T) {
	t.Parallel()

	pages, lastPageRequest := newRecordingPagesOrigin(t)
	svc := newErrorPageDistribution(t, newContentOrigin(t).URL, pages.URL, &DefaultCacheBehaviorXML{},
		CustomErrorResponseXML{ErrorCode: http.StatusNotFound, ResponsePagePath: page404Path, ResponseCode: "200"})

	if w := callEdge(t, svc, http.MethodGet, missingPath, viewerOnlyHeaders()); w.Code != http.StatusOK || w.Body.String() != page404Body {
		t.Fatalf("got %d %q, want 200 %q", w.Code, w.Body.String(), page404Body)
	}

	got := lastPageRequest()
	if got.method != http.MethodGet {
		t.Errorf("page request method = %q, want GET", got.method)
	}

	for name := range viewerOnlyHeaders() {
		if v := got.header.Get(name); v != "" {
			t.Errorf("page origin got the viewer's %s: %q", name, v)
		}
	}
}

// TestEdge_CustomErrorPageRequestSignedByOAC — a page origin behind an origin
// access control gets the edge's own signature, never the viewer's, even when
// its behaviour forwards Authorization and the control would let a viewer
// signature through (no-override).
func TestEdge_CustomErrorPageRequestSignedByOAC(t *testing.T) {
	pages, lastPageRequest := newRecordingPagesOrigin(t)
	t.Setenv("KUMO_S3_BACKEND", pages.URL)

	store, ids := newSigningStorage(t)
	pagesOrigin := lambdaOrigin(ids[oacSigningNoOverride])
	forwardsAuthorization := &DefaultCacheBehaviorXML{ForwardedValues: &ForwardedValuesXML{Headers: &HeadersXML{Quantity: 1, Items: []string{headerAuthorization}}}}
	svc := createErrorPageDistribution(t, New(store), newContentOrigin(t).URL, pagesOrigin, &DefaultCacheBehaviorXML{}, forwardsAuthorization,
		CustomErrorResponseXML{ErrorCode: http.StatusNotFound, ResponsePagePath: page404Path, ResponseCode: "200"})

	if w := callEdge(t, svc, http.MethodGet, missingPath, viewerOnlyHeaders()); w.Code != http.StatusOK || w.Body.String() != page404Body {
		t.Fatalf("got %d %q, want 200 %q", w.Code, w.Body.String(), page404Body)
	}

	got := lastPageRequest()
	if auth := got.header.Get(headerAuthorization); !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential="+edgeCredentials().AccessKeyID+"/") {
		t.Errorf("page request Authorization = %q, want the edge's own signature", auth)
	}

	for name := range viewerOnlyHeaders() {
		if v := got.header.Get(name); name != headerAuthorization && v != "" {
			t.Errorf("page origin got the viewer's %s: %q", name, v)
		}
	}
}

// TestEdge_CustomErrorPageKeepsOriginalCORSAndCookies — the response that
// replaces an error keeps the original response's CORS headers, in place of
// the page origin's, and its cookies, next to the page origin's; the rest of
// the headers describe the page.
func TestEdge_CustomErrorPageKeepsOriginalCORSAndCookies(t *testing.T) {
	t.Parallel()

	const viewerOrigin = "https://app.example"

	content := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", viewerOrigin)
		w.Header().Set("Access-Control-Expose-Headers", "X-Trace")
		w.Header().Add("Set-Cookie", "session=1")
		w.Header().Set("ETag", `"original"`)
		http.Error(w, contentMissing, http.StatusNotFound)
	}))
	t.Cleanup(content.Close)

	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Add("Set-Cookie", "page=1")
		w.Header().Set("ETag", `"page"`)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page404Body))
	}))
	t.Cleanup(pages.Close)

	svc := newErrorPageDistribution(t, content.URL, pages.URL, &DefaultCacheBehaviorXML{},
		CustomErrorResponseXML{ErrorCode: http.StatusNotFound, ResponsePagePath: page404Path, ResponseCode: "200"})

	w := callEdge(t, svc, http.MethodGet, missingPath, http.Header{headerOrigin: {viewerOrigin}})
	if w.Code != http.StatusOK || w.Body.String() != page404Body {
		t.Fatalf("got %d %q, want 200 %q", w.Code, w.Body.String(), page404Body)
	}

	for name, want := range map[string][]string{
		"Access-Control-Allow-Origin":   {viewerOrigin},
		"Access-Control-Expose-Headers": {"X-Trace"},
		"Set-Cookie":                    {"page=1", "session=1"},
		"Etag":                          {`"page"`},
		"Content-Type":                  {"text/html"},
		headerXCache:                    {errorPageCacheTag},
	} {
		if got := w.Header().Values(name); !slices.Equal(got, want) {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

// TestEdge_CustomErrorPageCarryingCookiesIsNotStored — a replacement that
// carries the original response's cookies must not be stored by any cache,
// whatever the page origin allowed for the page: the page's caching headers
// are replaced by no-store, and the edge's own cache does not keep it either.
func TestEdge_CustomErrorPageCarryingCookiesIsNotStored(t *testing.T) {
	t.Parallel()

	var contentRequests atomic.Int32

	content := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		contentRequests.Add(1)
		w.Header().Set("Set-Cookie", "session=1")
		w.Header().Set("Cache-Control", "private")
		http.Error(w, contentMissing, http.StatusNotFound)
	}))
	t.Cleanup(content.Close)

	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Header().Set("CDN-Cache-Control", "max-age=3600")
		w.Header().Set("Surrogate-Control", "max-age=3600")
		w.Header().Set("Expires", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat))
		_, _ = w.Write([]byte(page404Body))
	}))
	t.Cleanup(pages.Close)

	// The default behaviour lets the edge cache a response for an hour.
	svc := newErrorPageDistribution(t, content.URL, pages.URL, &DefaultCacheBehaviorXML{DefaultTTL: 3600, MaxTTL: 86400},
		CustomErrorResponseXML{ErrorCode: http.StatusNotFound, ResponsePagePath: page404Path, ResponseCode: "200"})

	for i := range 2 {
		w := callEdge(t, svc, http.MethodGet, missingPath, nil)
		if w.Code != http.StatusOK || w.Body.String() != page404Body || w.Header().Get("Set-Cookie") != "session=1" {
			t.Fatalf("request %d: got %d %q with Set-Cookie %q, want 200 %q with the original's cookie", i, w.Code, w.Body.String(), w.Header().Get("Set-Cookie"), page404Body)
		}

		for name, want := range map[string][]string{
			"Cache-Control":     {"no-store"},
			"Cdn-Cache-Control": nil,
			"Surrogate-Control": nil,
			"Expires":           nil,
		} {
			if got := w.Header().Values(name); !slices.Equal(got, want) {
				t.Errorf("request %d: %s = %q, want %q", i, name, got, want)
			}
		}
	}

	if n := contentRequests.Load(); n != 2 {
		t.Errorf("content origin got %d requests, want 2: the cookie must not be served from the edge cache", n)
	}
}

// TestEdge_CustomErrorPageMergesVary — the replacement's Vary lists the field
// names of the original response's Vary and the page's, each once whatever
// its case; a "*" in either wins.
func TestEdge_CustomErrorPageMergesVary(t *testing.T) {
	t.Parallel()

	const viewerOrigin = "https://app.example"

	origin, acceptEncoding := strings.ToLower(headerOrigin), strings.ToLower(headerAcceptEncoding)

	for _, tc := range []struct {
		name                   string
		originalVary, pageVary []string
		want                   []string // as cache.VaryHeaders reports it
	}{
		{"both", []string{headerOrigin + ", " + acceptEncoding}, []string{headerAcceptEncoding}, []string{acceptEncoding, origin}},
		{"original only", []string{headerOrigin}, nil, []string{origin}},
		{"star in the original", []string{varyWildcard}, []string{headerAcceptEncoding}, []string{varyWildcard}},
		{"star in the page", []string{headerOrigin}, []string{varyWildcard}, []string{varyWildcard}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			content := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				header := w.Header()
				header[headerVary] = tc.originalVary
				header.Set("Access-Control-Allow-Origin", viewerOrigin)
				http.Error(w, contentMissing, http.StatusNotFound)
			}))
			t.Cleanup(content.Close)

			pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header()[headerVary] = tc.pageVary
				_, _ = w.Write([]byte(page404Body))
			}))
			t.Cleanup(pages.Close)

			svc := newErrorPageDistribution(t, content.URL, pages.URL, &DefaultCacheBehaviorXML{},
				CustomErrorResponseXML{ErrorCode: http.StatusNotFound, ResponsePagePath: page404Path, ResponseCode: "200"})

			w := callEdge(t, svc, http.MethodGet, missingPath, http.Header{headerOrigin: {viewerOrigin}})
			if w.Code != http.StatusOK || w.Body.String() != page404Body {
				t.Fatalf("got %d %q, want 200 %q", w.Code, w.Body.String(), page404Body)
			}

			if got := cache.VaryHeaders(w.Header()); !slices.Equal(got, tc.want) {
				t.Errorf("Vary = %q, want %q", w.Header().Values(headerVary), tc.want)
			}
		})
	}
}

// TestEdge_CustomErrorPageRedirectKeepsOriginal — a page origin that answers
// the page request with a 3xx has not served the page: the redirect is not
// followed, and the viewer gets the original response instead of a redirect
// or an empty 304.
func TestEdge_CustomErrorPageRedirectKeepsOriginal(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusNotModified} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			t.Parallel()

			pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/target.html" {
					_, _ = w.Write([]byte("redirect target"))

					return
				}

				w.Header().Set("Location", "/target.html")
				w.WriteHeader(status)
			}))
			t.Cleanup(pages.Close)

			svc := newErrorPageDistribution(t, newContentOrigin(t).URL, pages.URL, &DefaultCacheBehaviorXML{},
				CustomErrorResponseXML{ErrorCode: http.StatusNotFound, ResponsePagePath: page404Path, ResponseCode: "200"})

			w := callEdge(t, svc, http.MethodGet, missingPath, nil)
			if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), contentMissing) || w.Header().Get("Location") != "" {
				t.Errorf("got %d %q (Location %q), want the original 404", w.Code, w.Body.String(), w.Header().Get("Location"))
			}
		})
	}
}

// onlyDistribution returns the distribution the test setup created in svc.
func onlyDistribution(t *testing.T, svc *Service) *Distribution {
	t.Helper()

	mem, ok := svc.storage.(*MemoryStorage)
	if !ok {
		t.Fatalf("expected *MemoryStorage, got %T", svc.storage)
	}

	for _, dist := range mem.Distributions {
		return dist
	}

	t.Fatal("the test setup created no distribution")

	return nil
}

// TestCustomErrorWriter_InformationalStatusIsNotFinal — a 1xx written before
// the final status reaches the viewer, and the final status still decides: a
// 404 after a 103 is replaced by its custom error page.
func TestCustomErrorWriter_InformationalStatusIsNotFinal(t *testing.T) {
	t.Parallel()

	const preload = "</style.css>; rel=preload"

	svc := newErrorPageDistribution(t, newContentOrigin(t).URL, newPagesOrigin(t).URL, &DefaultCacheBehaviorXML{},
		CustomErrorResponseXML{ErrorCode: http.StatusNotFound, ResponsePagePath: page404Path, ResponseCode: "404"})
	dist := onlyDistribution(t, svc)

	edge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w = svc.applyCustomErrorResponses(w, r, dist)

		w.Header().Set("Link", preload)
		w.WriteHeader(http.StatusEarlyHints)
		http.Error(w, contentMissing, http.StatusNotFound)
	}))
	t.Cleanup(edge.Close)

	var hints []string

	trace := &httptrace.ClientTrace{Got1xxResponse: func(code int, header textproto.MIMEHeader) error {
		if code == http.StatusEarlyHints {
			hints = append(hints, header.Values("Link")...)
		}

		return nil
	}}

	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(t.Context(), trace), http.MethodGet, edge.URL, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusNotFound || string(body) != page404Body {
		t.Errorf("got %q %q, want 404 %q", resp.Status, body, page404Body)
	}

	if !slices.Equal(hints, []string{preload}) {
		t.Errorf("103 Link = %q, want [%q]", hints, preload)
	}
}

// TestCustomErrorWriter_ResponseController — http.ResponseController reaches
// the viewer's writer through the wrapper instead of failing with
// http.ErrNotSupported.
func TestCustomErrorWriter_ResponseController(t *testing.T) {
	t.Parallel()

	svc := newErrorPageDistribution(t, newContentOrigin(t).URL, newPagesOrigin(t).URL, &DefaultCacheBehaviorXML{},
		CustomErrorResponseXML{ErrorCode: http.StatusNotFound, ResponsePagePath: page404Path, ResponseCode: "404"})
	dist := onlyDistribution(t, svc)

	// SetWriteDeadline's and Flush's errors, sent by the handler.
	controllerErrs := make(chan [2]error, 1)

	edge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w = svc.applyCustomErrorResponses(w, r, dist)

		rc := http.NewResponseController(w)
		deadlineErr := rc.SetWriteDeadline(time.Now().Add(time.Minute))

		_, _ = w.Write([]byte(originContent))
		controllerErrs <- [2]error{deadlineErr, rc.Flush()}
	}))
	t.Cleanup(edge.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, edge.URL, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = resp.Body.Close() }()

	if body, err := io.ReadAll(resp.Body); err != nil || resp.StatusCode != http.StatusOK || string(body) != originContent {
		t.Fatalf("got %q %q (%v), want 200 %q", resp.Status, body, err, originContent)
	}

	if errs := <-controllerErrs; errs[0] != nil || errs[1] != nil {
		t.Errorf("SetWriteDeadline = %v, Flush = %v; want both to reach the viewer's writer", errs[0], errs[1])
	}
}

// TestCustomErrorWriter_DoesNotHoldBackTheBody — the page is fetched as soon as
// the error status is written, so the original body is never buffered: it is
// passed straight on when the page origin can't be reached, and dropped when
// the page replaces it.
func TestCustomErrorWriter_DoesNotHoldBackTheBody(t *testing.T) {
	t.Parallel()

	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()

	for _, tc := range []struct {
		name     string
		pagesURL string
		want     string // viewer body once the error status and one chunk are written
	}{
		{"page origin unreachable", down.URL, contentMissing},
		{"page served", newPagesOrigin(t).URL, page404Body},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newErrorPageDistribution(t, newContentOrigin(t).URL, tc.pagesURL, &DefaultCacheBehaviorXML{},
				CustomErrorResponseXML{ErrorCode: http.StatusNotFound, ResponsePagePath: page404Path, ResponseCode: "404"})

			viewer := httptest.NewRecorder()
			w := svc.applyCustomErrorResponses(viewer, httptest.NewRequestWithContext(t.Context(), http.MethodGet, missingPath, http.NoBody), onlyDistribution(t, svc))

			w.WriteHeader(http.StatusNotFound)

			if n, err := w.Write([]byte(contentMissing)); n != len(contentMissing) || err != nil {
				t.Fatalf("Write = %d, %v", n, err)
			}

			if viewer.Code != http.StatusNotFound || viewer.Body.String() != tc.want {
				t.Errorf("got %d %q, want 404 %q", viewer.Code, viewer.Body.String(), tc.want)
			}
		})
	}
}

// TestEdge_CustomErrorResponsePageUnavailable — when the page origin answers
// the page request with an error, the viewer gets that status; when the page
// origin can't be reached, the original response is kept.
func TestEdge_CustomErrorResponsePageUnavailable(t *testing.T) {
	t.Parallel()

	t.Run("page origin returns 404", func(t *testing.T) {
		t.Parallel()

		svc := newErrorPageDistribution(t, newContentOrigin(t).URL, newPagesOrigin(t).URL, &DefaultCacheBehaviorXML{},
			CustomErrorResponseXML{ErrorCode: http.StatusInternalServerError, ResponsePagePath: "/errors/500.html", ResponseCode: "200"})

		if w := callEdge(t, svc, http.MethodGet, "/boom", nil); w.Code != http.StatusNotFound {
			t.Errorf("got %d %q, want the page origin's 404", w.Code, w.Body.String())
		}
	})

	t.Run("page origin unreachable", func(t *testing.T) {
		t.Parallel()

		down := httptest.NewServer(http.NotFoundHandler())
		down.Close()

		svc := newErrorPageDistribution(t, newContentOrigin(t).URL, down.URL, &DefaultCacheBehaviorXML{},
			CustomErrorResponseXML{ErrorCode: http.StatusNotFound, ResponsePagePath: page404Path, ResponseCode: "200"})

		if w := callEdge(t, svc, http.MethodGet, missingPath, nil); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), contentMissing) {
			t.Errorf("got %d %q, want the original 404", w.Code, w.Body.String())
		}
	})
}

func TestCustomErrorPageFor(t *testing.T) {
	t.Parallel()

	dist := &Distribution{DistributionConfig: &DistributionConfig{CustomErrorResponses: &CustomErrorResponses{Items: []CustomErrorResponse{
		{ErrorCode: http.StatusNotFound, ResponsePagePath: page404Path, ResponseCode: "200"},
		{ErrorCode: http.StatusServiceUnavailable},
		{ErrorCode: http.StatusGone, ResponsePagePath: "/errors/410.html"},
	}}}}

	for _, tc := range []struct {
		status int
		want   bool
	}{
		{http.StatusNotFound, true},
		{http.StatusServiceUnavailable, false}, // caching only
		{http.StatusGone, false},               // not a code CloudFront returns custom pages for
		{http.StatusForbidden, false},          // no entry
		{http.StatusOK, false},
	} {
		if got := customErrorPageFor(dist, tc.status) != nil; got != tc.want {
			t.Errorf("customErrorPageFor(%d) = %v, want %v", tc.status, got, tc.want)
		}
	}

	if customErrorPageFor(&Distribution{DistributionConfig: &DistributionConfig{}}, http.StatusNotFound) != nil {
		t.Error("a distribution without custom error responses must match nothing")
	}
}

func TestCustomErrorStatus(t *testing.T) {
	t.Parallel()

	for code, want := range map[string]int{
		"200": 200, "404": 404, "416": 416, "504": 504,
		"": 503, "abc": 503, "999": 503,
		// Valid HTTP codes outside the documented set keep the original status.
		"204": 503, "302": 503, "103": 503, "418": 503,
	} {
		if got := customErrorStatus(&CustomErrorResponse{ResponseCode: code}, http.StatusServiceUnavailable); got != want {
			t.Errorf("customErrorStatus(%q) = %d, want %d", code, got, want)
		}
	}
}

// TestCustomErrorResponses_RoundTrip — the request XML is stored and echoed
// back in DistributionConfig and the distribution summary, with the Quantity
// of the items echoed rather than the one the request claimed (3 for 2).
func TestCustomErrorResponses_RoundTrip(t *testing.T) {
	t.Parallel()

	body := `<DistributionConfig><CustomErrorResponses><Quantity>3</Quantity><Items>` +
		`<CustomErrorResponse><ErrorCode>404</ErrorCode><ResponsePagePath>/errors/404.html</ResponsePagePath><ResponseCode>200</ResponseCode><ErrorCachingMinTTL>0</ErrorCachingMinTTL></CustomErrorResponse>` +
		`<CustomErrorResponse><ErrorCode>503</ErrorCode><ErrorCachingMinTTL>5</ErrorCachingMinTTL></CustomErrorResponse>` +
		`</Items></CustomErrorResponses></DistributionConfig>`

	var req CreateDistributionRequest
	if err := xml.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	config := newDistributionConfig(&req)

	out, err := xml.Marshal(buildDistributionConfigXML(config))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, want := range []string{
		"<CustomErrorResponses><Quantity>2</Quantity><Items>",
		"<CustomErrorResponse><ErrorCode>404</ErrorCode><ResponsePagePath>/errors/404.html</ResponsePagePath><ResponseCode>200</ResponseCode><ErrorCachingMinTTL>0</ErrorCachingMinTTL></CustomErrorResponse>",
		"<CustomErrorResponse><ErrorCode>503</ErrorCode><ErrorCachingMinTTL>5</ErrorCachingMinTTL></CustomErrorResponse>",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("config XML lacks %s:\n%s", want, out)
		}
	}

	summary := buildDistributionSummaryXML(&Distribution{DistributionConfig: config})
	if summary.CustomErrorResponses == nil || summary.CustomErrorResponses.Quantity != 2 {
		t.Errorf("summary CustomErrorResponses = %+v, want Quantity 2", summary.CustomErrorResponses)
	}

	if empty := buildDistributionSummaryXML(&Distribution{DistributionConfig: &DistributionConfig{}}); empty.CustomErrorResponses == nil || empty.CustomErrorResponses.Quantity != 0 {
		t.Errorf("summary without custom errors = %+v, want Quantity 0", empty.CustomErrorResponses)
	}
}
