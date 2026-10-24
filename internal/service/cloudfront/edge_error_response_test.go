package cloudfront

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

// newErrorPageDistribution routes `/errors/*` to pagesURL and everything else
// to contentURL, with the given default behaviour and custom error responses.
func newErrorPageDistribution(t *testing.T, contentURL, pagesURL string, defaultBehavior *DefaultCacheBehaviorXML, responses ...CustomErrorResponseXML) *Service {
	t.Helper()

	return newErrorPageDistributionWithPagesBehavior(t, contentURL, pagesURL, defaultBehavior, &DefaultCacheBehaviorXML{}, responses...)
}

func newErrorPageDistributionWithPagesBehavior(t *testing.T, contentURL, pagesURL string, defaultBehavior, pagesBehavior *DefaultCacheBehaviorXML, responses ...CustomErrorResponseXML) *Service {
	t.Helper()

	svc := New(NewMemoryStorage())
	contentHost, contentPort := splitHostPort(strings.TrimPrefix(contentURL, "http://"))
	pagesHost, pagesPort := splitHostPort(strings.TrimPrefix(pagesURL, "http://"))

	defaultBehavior.TargetOriginID = originContent
	pagesBehavior.TargetOriginID = originPages

	if _, err := svc.storage.CreateDistribution(t.Context(), &CreateDistributionRequest{
		CallerReference: "custom-error-responses",
		Enabled:         true,
		Origins: &OriginsXML{Quantity: 2, Items: &OriginList{Origin: []OriginXML{
			{ID: originContent, DomainName: contentHost, CustomOriginConfig: &CustomOriginConfigXML{HTTPPort: contentPort, OriginProtocolPolicy: originPolicyHTTP}},
			{ID: originPages, DomainName: pagesHost, CustomOriginConfig: &CustomOriginConfigXML{HTTPPort: pagesPort, OriginProtocolPolicy: originPolicyHTTP}},
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
// back in DistributionConfig and the distribution summary.
func TestCustomErrorResponses_RoundTrip(t *testing.T) {
	t.Parallel()

	body := `<DistributionConfig><CustomErrorResponses><Quantity>2</Quantity><Items>` +
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
