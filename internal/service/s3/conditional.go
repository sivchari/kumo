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
	preconditionPass        preconditionResult = iota // request should proceed normally
	preconditionNotModified                           // GET/HEAD → 304 Not Modified
	preconditionFailed                                // any → 412 Precondition Failed
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
// kumo doesn't emit `W/"..."` weak ETags, and AWS S3 doesn't either.
// Tokens are compared with surrounding quotes stripped so an unquoted
// If-Match/If-None-Match value (real clients and the AWS CLI send
// either form) still matches the quoted ETag kumo stores, matching AWS
// S3's lenient behavior.
func matchesAnyETag(headerValue, etag string) bool {
	v := strings.TrimSpace(headerValue)
	if v == "*" {
		return true
	}

	want := strings.Trim(etag, `"`)

	for _, raw := range strings.Split(v, ",") {
		if strings.Trim(strings.TrimSpace(raw), `"`) == want {
			return true
		}
	}

	return false
}

// parsePutCondition parses the If-Match / If-None-Match request headers
// into a PutCondition for PutObject / CompleteMultipartUpload. AWS S3
// only supports `If-None-Match: *` on writes (any other value returns
// 501 NotImplemented instead of being evaluated), reported via the
// second return value being false.
func parsePutCondition(h http.Header) (PutCondition, bool) {
	var cond PutCondition

	if v := strings.TrimSpace(h.Get("If-None-Match")); v != "" {
		if v != "*" {
			return PutCondition{}, false
		}

		cond.IfNoneMatchAny = true
	}

	cond.IfMatch = strings.TrimSpace(h.Get("If-Match"))

	return cond, true
}

// objectErrorStatus maps an *ObjectError Code produced by
// checkPutCondition to its S3 HTTP status: PreconditionFailed is 412,
// NoSuchKey (and anything else) is 404.
func objectErrorStatus(code string) int {
	if code == "PreconditionFailed" {
		return http.StatusPreconditionFailed
	}

	return http.StatusNotFound
}

// checkPutCondition evaluates cond against bucket b's current object at
// key, in RFC 9110 order (If-Match before If-None-Match). The caller
// must hold s.mu (write lock) across both this check and the insert
// that follows, so the check is atomic with the write.
//
// A key whose current version is a delete marker is treated as
// non-existent (mirrors GetObject: b.Objects[key] stays populated with
// the delete marker after a versioned delete, it isn't removed), so
// `If-None-Match: *` succeeds and `If-Match` reports NoSuchKey just as
// it would for a truly absent key.
func checkPutCondition(b *MemoryBucket, key string, cond PutCondition) error {
	current, ok := b.Objects[key]
	exists := ok && !current.IsDeleteMarker

	if cond.IfMatch != "" {
		if !exists {
			return &ObjectError{Code: "NoSuchKey", Message: "The specified key does not exist.", Key: key}
		}

		if !matchesAnyETag(cond.IfMatch, current.ETag) {
			return &ObjectError{Code: "PreconditionFailed", Message: "At least one of the preconditions you specified did not hold.", Key: key}
		}
	}

	if cond.IfNoneMatchAny && exists {
		return &ObjectError{Code: "PreconditionFailed", Message: "At least one of the preconditions you specified did not hold.", Key: key}
	}

	return nil
}
