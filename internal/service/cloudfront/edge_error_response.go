package cloudfront

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/sivchari/kumo/internal/service/cloudfront/cache"
)

// errorPageCacheTag marks a response the edge answered with a custom error
// page, after CloudFront's "X-Cache: Error from cloudfront" (see the sample
// responses in the blog post cited on customErrorWriter).
const errorPageCacheTag = "Error from kumo"

// customErrorPageCodes are the HTTP status codes CloudFront can return a
// custom error page for:
// https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/creating-custom-error-pages.html
var customErrorPageCodes = map[int]bool{
	http.StatusBadRequest:                   true,
	http.StatusForbidden:                    true,
	http.StatusNotFound:                     true,
	http.StatusMethodNotAllowed:             true,
	http.StatusRequestURITooLong:            true,
	http.StatusRequestedRangeNotSatisfiable: true,
	http.StatusInternalServerError:          true,
	http.StatusNotImplemented:               true,
	http.StatusBadGateway:                   true,
	http.StatusServiceUnavailable:           true,
	http.StatusGatewayTimeout:               true,
}

// customErrorPageFor returns the custom error response that replaces a
// response with the given status, or nil. Entries without a ResponsePagePath
// only set an error caching duration and never replace the response.
func customErrorPageFor(dist *Distribution, status int) *CustomErrorResponse {
	if !customErrorPageCodes[status] || dist == nil || dist.DistributionConfig == nil || dist.DistributionConfig.CustomErrorResponses == nil {
		return nil
	}

	for i := range dist.DistributionConfig.CustomErrorResponses.Items {
		entry := &dist.DistributionConfig.CustomErrorResponses.Items[i]
		if entry.ErrorCode == status && entry.ResponsePagePath != "" {
			return entry
		}
	}

	return nil
}

// customErrorStatus is the status the viewer gets with the custom error page:
// ResponseCode when it is one of the codes CloudFront can return along with a
// custom error page, which are 200 and the customErrorPageCodes:
// https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/custom-error-pages-response-code.html
// The original status otherwise, including when ResponseCode is unset or
// invalid.
func customErrorStatus(entry *CustomErrorResponse, original int) int {
	code, err := strconv.Atoi(strings.TrimSpace(entry.ResponseCode))
	if err != nil || (code != http.StatusOK && !customErrorPageCodes[code]) {
		return original
	}

	return code
}

// hasCustomErrorPages reports whether any custom error response carries a page.
func hasCustomErrorPages(dist *Distribution) bool {
	if dist == nil || dist.DistributionConfig == nil || dist.DistributionConfig.CustomErrorResponses == nil {
		return false
	}

	for _, entry := range dist.DistributionConfig.CustomErrorResponses.Items {
		if entry.ResponsePagePath != "" {
			return true
		}
	}

	return false
}

// customErrorWriter sits between the edge handler and the viewer. When the
// handler writes a status that has a custom error page, the page is fetched
// right away and written in place of the handler's response, whose body is
// then dropped; every other response is passed straight through. Nothing is
// held back, so there is no body to buffer.
//
// CloudFront "serves them to the user anytime that the edge location receives
// an HTTP status code that matches the error code. It does not differentiate
// errors from the edge location from those from the origin."
// (https://aws.amazon.com/blogs/networking-and-content-delivery/generating-dynamic-error-responses-in-amazon-cloudfront-with-lambdaedge/)
// so errors the edge produces itself, such as a 403 for a missing signed
// cookie, are replaced as well.
//
// TODO: apply ErrorCachingMinTTL. It is stored and returned by the API, but
// the edge caches an error response only by the cache behavior's TTLs and the
// origin's headers, like any other response, and never caches the custom
// error page. CloudFront caches error responses "for a default duration of 10
// seconds", or for the Error Caching Minimum TTL configured for the status
// code
// (https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/custom-error-pages-expiration.html),
// and caches the custom error page with them (HTTPStatusCodes.html, cited on
// writeErrorPage).
type customErrorWriter struct {
	svc    *Service
	viewer http.ResponseWriter
	r      *http.Request
	dist   *Distribution

	header      http.Header
	wroteHeader bool
	// replaced is set once the viewer got the custom error page, or the page
	// origin's error, in place of the handler's response.
	replaced bool
}

// applyCustomErrorResponses returns the writer for the edge handler: w
// wrapped in a customErrorWriter when the distribution has custom error
// pages, w itself otherwise.
func (s *Service) applyCustomErrorResponses(w http.ResponseWriter, r *http.Request, dist *Distribution) http.ResponseWriter {
	if !hasCustomErrorPages(dist) {
		return w
	}

	return &customErrorWriter{svc: s, viewer: w, r: r, dist: dist, header: http.Header{}}
}

func (cw *customErrorWriter) Header() http.Header {
	return cw.header
}

// Unwrap returns the viewer's writer, for http.ResponseController.
func (cw *customErrorWriter) Unwrap() http.ResponseWriter {
	return cw.viewer
}

func (cw *customErrorWriter) WriteHeader(status int) {
	if cw.wroteHeader {
		return
	}

	if isInformational(status) {
		cw.writeInformational(status)

		return
	}

	cw.wroteHeader = true

	if entry := customErrorPageFor(cw.dist, status); entry != nil && cw.writeErrorPage(entry, status) {
		cw.replaced = true

		return
	}

	copyHeader(cw.viewer.Header(), cw.header)
	cw.viewer.WriteHeader(status)
}

// isInformational reports whether status is a 1xx that net/http sends ahead
// of the final response; it takes 101 Switching Protocols as final.
func isInformational(status int) bool {
	return status >= http.StatusContinue && status < http.StatusOK && status != http.StatusSwitchingProtocols
}

// writeInformational passes a 1xx on to the viewer with the handler's header,
// as net/http does, without taking it for the final status. The viewer's
// header is restored afterwards, since the final response copies the
// handler's header again.
func (cw *customErrorWriter) writeInformational(status int) {
	h := cw.viewer.Header()
	before := h.Clone()

	copyHeader(h, cw.header)
	cw.viewer.WriteHeader(status)

	clear(h)
	copyHeader(h, before)
}

func (cw *customErrorWriter) Write(b []byte) (int, error) {
	if !cw.wroteHeader {
		cw.WriteHeader(http.StatusOK)
	}

	// The custom error page has taken the place of this body.
	if cw.replaced {
		return len(b), nil
	}

	n, err := cw.viewer.Write(b)
	if err != nil {
		return n, fmt.Errorf("write response: %w", err)
	}

	return n, nil
}

// FlushError flushes the viewer's writer, for http.ResponseController, which
// would otherwise reach it through Unwrap and bypass WriteHeader. As with
// net/http, flushing before WriteHeader sends a 200.
func (cw *customErrorWriter) FlushError() error {
	if !cw.wroteHeader {
		cw.WriteHeader(http.StatusOK)
	}

	if err := http.NewResponseController(cw.viewer).Flush(); err != nil {
		return fmt.Errorf("flush response: %w", err)
	}

	return nil
}

// Flush implements http.Flusher with FlushError.
func (cw *customErrorWriter) Flush() {
	_ = cw.FlushError()
}

// writeErrorPage writes the custom error page of entry to the viewer in place
// of the handler's response with the given status, and reports whether it
// did. Per
// https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/HTTPStatusCodes.html
// the page is requested from the origin of "the first cache behavior in your
// distribution that has a path pattern that matches the path of the custom
// error page". If the page isn't available, "CloudFront returns to the viewer
// the status code that CloudFront received from the origin that contains the
// custom error pages"
// (https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/GeneratingCustomErrorResponses.html).
//
// When the page origin can't be reached, or answers with a 3xx, nothing is
// written and the handler's response goes out unchanged. A 3xx does not serve
// the page either, but passing it on would turn the error into a redirect, or
// into a 304 the viewer did not ask for. The docs describe an unavailable page
// only through an error status (their example is the 404 for a deleted page),
// so keeping the original response for a 3xx is kumo's choice.
func (cw *customErrorWriter) writeErrorPage(entry *CustomErrorResponse, status int) bool {
	page, err := cw.svc.fetchErrorPage(cw.r.Context(), cw.dist, entry.ResponsePagePath)
	if err != nil || (page.StatusCode >= http.StatusMultipleChoices && page.StatusCode < http.StatusBadRequest) {
		return false
	}

	viewerStatus := page.StatusCode
	if viewerStatus >= http.StatusOK && viewerStatus < http.StatusMultipleChoices {
		viewerStatus = customErrorStatus(entry, status)
	}

	h := cw.viewer.Header()
	copyHeader(h, page.Header)
	keepOriginalHeaders(h, cw.header)
	h.Set(headerXCache, errorPageCacheTag)
	cw.viewer.WriteHeader(viewerStatus)
	_, _ = cw.viewer.Write(page.Body)

	return true
}

// Headers that keepOriginalHeaders carries over from the original response or
// adjusts on the response that replaces it.
const (
	corsHeaderPrefix = "Access-Control-"
	headerSetCookie  = "Set-Cookie"

	headerCacheControl         = "Cache-Control"
	headerExpires              = "Expires"
	headerSurrogateControl     = "Surrogate-Control"
	targetedCacheControlSuffix = "-Cache-Control"
	cacheDirectiveNoStore      = "no-store"

	headerVary   = "Vary"
	varyWildcard = "*"
)

// keepOriginalHeaders carries headers of the original response over to the
// response that replaces it, whose other headers come from the page origin
// and describe the page. The CORS headers (Access-Control-*) replace the page
// origin's, so a cross-origin request that ended in the page can still read
// it; Set-Cookie is added to the page origin's; Vary lists the field names
// of both responses' Vary (mergeVary). CloudFront's documentation of custom
// error pages specifies no response header beyond Connection
// (https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/creating-custom-error-pages.html),
// so which headers are kept is kumo's choice.
//
// A carried-over cookie comes without the caching headers of the response
// that set it, and "the Set-Cookie response header field does not inhibit
// caching" (https://www.rfc-editor.org/rfc/rfc9111.html#section-7.3): under
// the page's caching headers a shared cache could hand the viewer's cookie to
// others. So once cookies are carried over, the response is kept out of every
// cache (forbidStoring), which is never more cacheable than the original
// allowed. The docs are silent here too, so this is kumo's choice as well.
// The edge's own cache never stores the response that replaces an error: it
// stores origin responses only.
func keepOriginalHeaders(dst, original http.Header) {
	carriedCookies := false

	for name, values := range original {
		name = http.CanonicalHeaderKey(name)

		switch {
		case strings.HasPrefix(name, corsHeaderPrefix):
			dst.Del(name)
		case name == headerSetCookie:
			carriedCookies = carriedCookies || len(values) > 0
		default:
			continue
		}

		for _, v := range values {
			dst.Add(name, v)
		}
	}

	mergeVary(dst, original)

	if carriedCookies {
		forbidStoring(dst)
	}
}

// mergeVary makes dst's Vary list the field names of its own Vary and of
// original's. The CORS headers carried over, and whether the page is served
// at all, depend on the request fields the original response varied on, and a
// cache must not reuse a stored response for a request that differs in them
// (https://www.rfc-editor.org/rfc/rfc9111.html#section-4.1). Field names are
// case-insensitive (RFC 9110 Section 5.1), so each is listed once, lower-cased
// as cache.VaryHeaders reports it; a "*" in either, which never matches a
// later request, replaces the list.
func mergeVary(dst, original http.Header) {
	if cache.VaryDisablesCache(dst) || cache.VaryDisablesCache(original) {
		dst.Set(headerVary, varyWildcard)

		return
	}

	names := cache.VaryHeaders(dst)
	for _, name := range cache.VaryHeaders(original) {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}

	if len(names) > 0 {
		dst.Set(headerVary, strings.Join(names, ", "))
	}
}

// forbidStoring makes h the header of a response that no cache may store:
// with no-store "a cache MUST NOT store any part of either the immediate
// request or the response", and the directive "applies to both private and
// shared caches"
// (https://www.rfc-editor.org/rfc/rfc9111.html#section-5.2.2.5). It replaces
// Cache-Control and drops Expires, the targeted cache-control fields such as
// CDN-Cache-Control, which a CDN that uses them follows instead of
// Cache-Control and Expires
// (https://www.rfc-editor.org/rfc/rfc9213.html#section-2.2), and the older
// Surrogate-Control.
func forbidStoring(h http.Header) {
	for name := range h {
		canonical := http.CanonicalHeaderKey(name)
		if canonical == headerExpires || canonical == headerSurrogateControl || strings.HasSuffix(canonical, targetedCacheControlSuffix) {
			delete(h, name)
		}
	}

	h.Set(headerCacheControl, cacheDirectiveNoStore)
}

// fetchErrorPage requests the custom error page from the origin of the cache
// behaviour that matches its path, with a GET of its own. None of the
// viewer's headers are sent: its credentials, cookies and conditional or
// range headers belong to the viewer's request, and the page origin may not
// be the origin that request went to. Viewer-side checks (signed URLs and
// cookies) are not applied to the page request either; the page origin's
// access control signing is, so a private origin behind an origin access
// control can hold the pages.
//
// The page is not cached: every error that is replaced fetches it from its
// origin again. That is intentional for now, while CloudFront "also caches
// the custom error page" for the error caching minimum TTL or the max-age of
// the error response, whichever is longer (HTTPStatusCodes.html, cited on
// writeErrorPage); see the TODO on customErrorWriter.
func (s *Service) fetchErrorPage(ctx context.Context, dist *Distribution, pagePath string) (*originResponse, error) {
	path := strings.TrimPrefix(pagePath, "/")
	behavior := resolveBehavior(dist, path)

	origin, ok := selectOrigin(dist, behavior)
	if !ok {
		return nil, errNoErrorPageOrigin
	}

	pageURL, ok := edgeOriginURL(&origin, path, "")
	if !ok {
		return nil, errNoErrorPageOrigin
	}

	signing, err := s.resolveSigning(ctx, &origin, behavior)
	if err != nil && !errors.Is(err, errNotSigned) {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build custom error page request: %w", err)
	}

	return fetchOrigin(upstreamTarget{url: pageURL, signing: signing, client: errorPageClient}, req)
}

// errorPageClient requests custom error pages without following redirects:
// when an origin returns one, "CloudFront doesn't follow the redirect"
// (https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/http-3xx-status-codes.html),
// and writeErrorPage handles the 3xx itself.
var errorPageClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

var errNoErrorPageOrigin = errors.New("no usable origin for the custom error page")

func copyHeader(dst, src http.Header) {
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}
