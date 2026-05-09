package s3

import (
	"net/http"
	"strings"
	"time"
)

// preconditionResult is the outcome of evaluating RFC 9110 conditional
// request headers against an object's ETag + Last-Modified.
type preconditionResult int

const (
	preconditionPass         preconditionResult = iota // request should proceed normally
	preconditionNotModified                            // GET/HEAD → 304 Not Modified
	preconditionFailed                                 // any → 412 Precondition Failed
)

// evalGetObjectPreconditions implements RFC 9110 §13.1 evaluation order
// for GET/HEAD: If-Match → If-Unmodified-Since → If-None-Match →
// If-Modified-Since.
//
// Precondition headers are commonly used for cache revalidation
// (`If-None-Match`) and optimistic concurrency (`If-Match`).
func evalGetObjectPreconditions(h http.Header, etag string, lastModified time.Time) preconditionResult {
	if v := h.Get("If-Match"); v != "" {
		if !matchesAnyETag(v, etag) {
			return preconditionFailed
		}
	} else if v := h.Get("If-Unmodified-Since"); v != "" {
		if t, err := http.ParseTime(v); err == nil && lastModified.After(t) {
			return preconditionFailed
		}
	}

	if v := h.Get("If-None-Match"); v != "" {
		if matchesAnyETag(v, etag) {
			return preconditionNotModified
		}
	} else if v := h.Get("If-Modified-Since"); v != "" {
		if t, err := http.ParseTime(v); err == nil && !lastModified.After(t) {
			return preconditionNotModified
		}
	}

	return preconditionPass
}

// evalCopySourcePreconditions implements the AWS S3 CopyObject
// `x-amz-copy-source-if-*` family (semantically the same as the
// HTTP-level conditionals above, just on the *source* object). All
// failures collapse to 412 — there's no "304" path on copy.
func evalCopySourcePreconditions(h http.Header, etag string, lastModified time.Time) bool {
	if v := h.Get("X-Amz-Copy-Source-If-Match"); v != "" {
		if !matchesAnyETag(v, etag) {
			return false
		}
	}

	if v := h.Get("X-Amz-Copy-Source-If-None-Match"); v != "" {
		if matchesAnyETag(v, etag) {
			return false
		}
	}

	if v := h.Get("X-Amz-Copy-Source-If-Unmodified-Since"); v != "" {
		if t, err := http.ParseTime(v); err == nil && lastModified.After(t) {
			return false
		}
	}

	if v := h.Get("X-Amz-Copy-Source-If-Modified-Since"); v != "" {
		if t, err := http.ParseTime(v); err == nil && !lastModified.After(t) {
			return false
		}
	}

	return true
}

// matchesAnyETag returns true if `etag` matches any token in a
// comma-separated If-*-Match header value, or if the header is `*`
// (RFC 9110 §13.1.1: `*` matches if any selected representation
// exists, which for a successful GetObject lookup is always true).
//
// ETag comparison is the *strong* variant per RFC 9110 §8.8.3.2 —
// kumo doesn't emit `W/"..."` weak ETags, and AWS S3 doesn't either,
// so quoted-string equality is sufficient.
func matchesAnyETag(headerValue, etag string) bool {
	v := strings.TrimSpace(headerValue)
	if v == "*" {
		return true
	}

	for _, raw := range strings.Split(v, ",") {
		if strings.TrimSpace(raw) == etag {
			return true
		}
	}

	return false
}
