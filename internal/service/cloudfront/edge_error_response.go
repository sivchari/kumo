package cloudfront

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

// customErrorWriter sits between the edge handler and the viewer. Responses
// whose status has a custom error page are held back and replaced by that
// page in finish; every other response is passed straight through.
//
// CloudFront "serves them to the user anytime that the edge location receives
// an HTTP status code that matches the error code. It does not differentiate
// errors from the edge location from those from the origin."
// (https://aws.amazon.com/blogs/networking-and-content-delivery/generating-dynamic-error-responses-in-amazon-cloudfront-with-lambdaedge/)
// so errors the edge produces itself, such as a 403 for a missing signed
// cookie, are replaced as well.
type customErrorWriter struct {
	svc    *Service
	viewer http.ResponseWriter
	r      *http.Request
	dist   *Distribution

	header      http.Header
	wroteHeader bool
	status      int
	entry       *CustomErrorResponse
	body        bytes.Buffer
}

// applyCustomErrorResponses wraps w in a customErrorWriter when the
// distribution has custom error pages, and returns the writer to use with the
// function that writes the page once the edge handler is done.
func (s *Service) applyCustomErrorResponses(w http.ResponseWriter, r *http.Request, dist *Distribution) (http.ResponseWriter, func()) {
	if !hasCustomErrorPages(dist) {
		return w, func() {}
	}

	cw := &customErrorWriter{svc: s, viewer: w, r: r, dist: dist, header: http.Header{}}

	return cw, cw.finish
}

func (cw *customErrorWriter) Header() http.Header {
	return cw.header
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
	cw.status = status

	if cw.entry = customErrorPageFor(cw.dist, status); cw.entry != nil {
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

	if cw.entry != nil {
		_, _ = cw.body.Write(b) // bytes.Buffer.Write never fails.

		return len(b), nil
	}

	n, err := cw.viewer.Write(b)
	if err != nil {
		return n, fmt.Errorf("write response: %w", err)
	}

	return n, nil
}

// finish writes the custom error page for a held-back response. Per
// https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/HTTPStatusCodes.html
// the page is requested from the origin of "the first cache behavior in your
// distribution that has a path pattern that matches the path of the custom
// error page". If the page isn't available, "CloudFront returns to the viewer
// the status code that CloudFront received from the origin that contains the
// custom error pages"
// (https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/GeneratingCustomErrorResponses.html).
//
// When the page origin can't be reached, or answers with a 3xx, the original
// response is served unchanged. A 3xx does not serve the page either, but
// passing it on would turn the error into a redirect, or into a 304 the
// viewer did not ask for. The docs describe an unavailable page only through
// an error status (their example is the 404 for a deleted page), so keeping
// the original response for a 3xx is kumo's choice.
func (cw *customErrorWriter) finish() {
	if cw.entry == nil {
		return
	}

	page, err := cw.svc.fetchErrorPage(cw.r.Context(), cw.dist, cw.entry.ResponsePagePath)
	if err != nil || (page.StatusCode >= http.StatusMultipleChoices && page.StatusCode < http.StatusBadRequest) {
		copyHeader(cw.viewer.Header(), cw.header)
		cw.viewer.WriteHeader(cw.status)
		_, _ = cw.viewer.Write(cw.body.Bytes())

		return
	}

	status := page.StatusCode
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		status = customErrorStatus(cw.entry, cw.status)
	}

	copyHeader(cw.viewer.Header(), page.Header)
	keepOriginalHeaders(cw.viewer.Header(), cw.header)
	cw.viewer.Header().Set(headerXCache, errorPageCacheTag)
	cw.viewer.WriteHeader(status)
	_, _ = cw.viewer.Write(page.Body)
}

// Headers of the original response that keepOriginalHeaders carries over.
const (
	corsHeaderPrefix = "Access-Control-"
	headerSetCookie  = "Set-Cookie"
)

// keepOriginalHeaders carries headers of the original response over to the
// response that replaces it, whose other headers come from the page origin
// and describe the page. The CORS headers (Access-Control-*) replace the page
// origin's, so a cross-origin request that ended in the page can still read
// it; Set-Cookie is added to the page origin's. CloudFront's documentation of
// custom error pages specifies no response header beyond Connection
// (https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/creating-custom-error-pages.html),
// so which headers are kept is kumo's choice.
func keepOriginalHeaders(dst, original http.Header) {
	for name, values := range original {
		name = http.CanonicalHeaderKey(name)

		switch {
		case strings.HasPrefix(name, corsHeaderPrefix):
			dst.Del(name)
		case name != headerSetCookie:
			continue
		}

		for _, v := range values {
			dst.Add(name, v)
		}
	}
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
// the custom error page" for the error caching minimum TTL or the origin's
// max-age, whichever is longer (HTTPStatusCodes.html, cited on finish).
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
// and finish handles the 3xx itself.
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
