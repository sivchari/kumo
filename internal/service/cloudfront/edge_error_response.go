package cloudfront

import (
	"bytes"
	"errors"
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

	cw.wroteHeader = true
	cw.status = status

	if cw.entry = customErrorPageFor(cw.dist, status); cw.entry != nil {
		return
	}

	copyHeader(cw.viewer.Header(), cw.header)
	cw.viewer.WriteHeader(status)
}

func (cw *customErrorWriter) Write(b []byte) (int, error) {
	if !cw.wroteHeader {
		cw.WriteHeader(http.StatusOK)
	}

	if cw.entry != nil {
		_, _ = cw.body.Write(b) // bytes.Buffer.Write never fails.

		return len(b), nil
	}

	return cw.viewer.Write(b) //nolint:wrapcheck // io.Writer passthrough: the caller expects the viewer's own error.
}

// finish writes the custom error page for a held-back response. Per
// https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/HTTPStatusCodes.html
// the page is requested from the origin of "the first cache behavior in your
// distribution that has a path pattern that matches the path of the custom
// error page". If the page isn't available, "CloudFront returns to the viewer
// the status code that CloudFront received from the origin that contains the
// custom error pages"
// (https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/GeneratingCustomErrorResponses.html).
// When the page origin can't be reached at all, the original response is
// served unchanged.
func (cw *customErrorWriter) finish() {
	if cw.entry == nil {
		return
	}

	page, err := cw.svc.fetchErrorPage(cw.r, cw.dist, cw.entry.ResponsePagePath)
	if err != nil {
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
	cw.viewer.Header().Set(headerXCache, errorPageCacheTag)
	cw.viewer.WriteHeader(status)
	_, _ = cw.viewer.Write(page.Body)
}

// fetchErrorPage requests the custom error page from the origin of the cache
// behaviour that matches its path. Viewer-side checks (signed URLs and
// cookies) belong to the viewer's own request and are not applied to it; the
// origin's access control signing is, so a private S3 origin behind an origin
// access control can hold the pages.
func (s *Service) fetchErrorPage(r *http.Request, dist *Distribution, pagePath string) (*originResponse, error) {
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

	signing, err := s.resolveSigning(r.Context(), &origin, behavior)
	if err != nil && !errors.Is(err, errNotSigned) {
		return nil, err
	}

	req := r.Clone(r.Context())
	req.Method = http.MethodGet

	// The page is fetched whole, regardless of the viewer's conditional or
	// range headers.
	for _, h := range []string{"Range", "If-Range", "If-Match", "If-None-Match", "If-Modified-Since", "If-Unmodified-Since"} {
		req.Header.Del(h)
	}

	return fetchOrigin(upstreamTarget{url: pageURL, signing: signing}, req)
}

var errNoErrorPageOrigin = errors.New("no usable origin for the custom error page")

func copyHeader(dst, src http.Header) {
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}
