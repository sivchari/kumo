package cloudfront

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"slices"
)

// Helpers shared by the cache policy and response headers policy APIs.

// requireIfMatch rejects a write without an If-Match header (InvalidIfMatchVersion).
func requireIfMatch(ifMatch string) error {
	if ifMatch == "" {
		return &Error{Code: errInvalidIfMatchVersion, Message: "The If-Match version is missing or not valid for the resource."}
	}

	return nil
}

// checkIfMatch rejects an If-Match header that is not the current ETag (PreconditionFailed).
func checkIfMatch(current, ifMatch string) error {
	if current != ifMatch {
		return &Error{Code: errPreconditionFailed, Message: "The precondition in one or more of the request fields evaluated to false."}
	}

	return nil
}

// pageByID returns the items whose id sorts after marker, ordered by id and
// limited to maxItems; nextMarker is the last id returned when more remain.
func pageByID[T any](items []T, id func(T) string, marker string, maxItems int) ([]T, string) {
	out := slices.DeleteFunc(slices.Clone(items), func(item T) bool { return id(item) <= marker })

	sortByID(out, id)

	if len(out) <= maxItems {
		return out, ""
	}

	page := out[:maxItems]

	return page, id(page[len(page)-1])
}

// distributionReferencingLocked returns the id of a distribution, enabled or
// not, with a default or ordered cache behavior for which refers is true.
func (s *MemoryStorage) distributionReferencingLocked(refers func(*DefaultCacheBehavior) bool) (string, bool) {
	for _, d := range s.Distributions {
		if slices.ContainsFunc(d.DistributionConfig.behaviors(), refers) {
			return d.ID, true
		}
	}

	return "", false
}

// validatePolicyReferencesLocked rejects a distribution whose default or
// ordered cache behavior names a cache policy or response headers policy that
// is neither stored nor AWS-managed, as CloudFront does (NoSuchCachePolicy,
// NoSuchResponseHeadersPolicy). The caller holds the lock the policy APIs'
// in-use scan shares, so a policy cannot be deleted between check and write.
func (s *MemoryStorage) validatePolicyReferencesLocked(config *DistributionConfig) error {
	for _, b := range config.behaviors() {
		if id := b.CachePolicyID; id != "" && !s.cachePolicyExistsLocked(id) {
			return &Error{Code: errNoSuchCachePolicy, Message: "The cache policy does not exist."}
		}

		if id := b.ResponseHeadersPolicyID; id != "" && !s.responseHeadersPolicyExistsLocked(id) {
			return &Error{Code: errNoSuchResponseHeadersPolicy, Message: "The response headers policy does not exist."}
		}
	}

	return nil
}

func (s *MemoryStorage) cachePolicyExistsLocked(id string) bool {
	if _, ok := s.CachePolicies[id]; ok {
		return true
	}

	_, managed := managedCachePolicies[id]

	return managed
}

func (s *MemoryStorage) responseHeadersPolicyExistsLocked(id string) bool {
	if _, ok := s.ResponseHeadersPolicies[id]; ok {
		return true
	}

	_, managed := managedResponseHeadersPolicies[id]

	return managed
}

// decodeXMLBody decodes the request body; on failure the error response has
// been written.
func decodeXMLBody[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeCloudFrontError(w, errMissingBody, "Request body is missing", http.StatusBadRequest)

		return nil, false
	}

	var v T
	if err := xml.Unmarshal(body, &v); err != nil {
		writeCloudFrontError(w, errInvalidArgument, "Invalid request body", http.StatusBadRequest)

		return nil, false
	}

	return &v, true
}

// writePolicyError maps a policy storage error to the status the API
// reference documents: 404 for notFound, 409 for the conflict codes, 412 for
// a stale ETag and 400 for everything else (a missing If-Match header, an
// invalid argument, inconsistent quantities).
func writePolicyError(w http.ResponseWriter, err error, notFound string, conflicts ...string) {
	var cfErr *Error
	if !errors.As(err, &cfErr) {
		writeCloudFrontError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	status := http.StatusBadRequest

	switch {
	case cfErr.Code == notFound:
		status = http.StatusNotFound
	case slices.Contains(conflicts, cfErr.Code):
		status = http.StatusConflict
	case cfErr.Code == errPreconditionFailed:
		status = http.StatusPreconditionFailed
	}

	writeCloudFrontError(w, cfErr.Code, cfErr.Message, status)
}
