package cloudfront

import (
	"net/http"
	"strconv"
	"time"
)

const cachePolicyPath = "/2020-05-31/cache-policy/"

// CreateCachePolicy handles POST /2020-05-31/cache-policy.
func (s *Service) CreateCachePolicy(w http.ResponseWriter, r *http.Request) {
	cfg, ok := readCachePolicyConfig(w, r)
	if !ok {
		return
	}

	policy, err := s.storage.CreateCachePolicy(r.Context(), cfg)
	if err != nil {
		handleCachePolicyStorageError(w, err)

		return
	}

	w.Header().Set("ETag", policy.ETag)
	w.Header().Set("Location", cachePolicyPath+policy.ID)
	writeXMLResponse(w, http.StatusCreated, buildCachePolicyXML(policy, cloudfrontXmlns))
}

// GetCachePolicy handles GET /2020-05-31/cache-policy/{id}.
func (s *Service) GetCachePolicy(w http.ResponseWriter, r *http.Request) {
	policy, err := s.storage.GetCachePolicy(r.Context(), r.PathValue("id"))
	if err != nil {
		handleCachePolicyStorageError(w, err)

		return
	}

	w.Header().Set("ETag", policy.ETag)
	writeXMLResponse(w, http.StatusOK, buildCachePolicyXML(policy, cloudfrontXmlns))
}

// GetCachePolicyConfig handles GET /2020-05-31/cache-policy/{id}/config.
func (s *Service) GetCachePolicyConfig(w http.ResponseWriter, r *http.Request) {
	policy, err := s.storage.GetCachePolicy(r.Context(), r.PathValue("id"))
	if err != nil {
		handleCachePolicyStorageError(w, err)

		return
	}

	cfg := policy.Config
	cfg.Xmlns = cloudfrontXmlns

	w.Header().Set("ETag", policy.ETag)
	writeXMLResponse(w, http.StatusOK, &cfg)
}

// UpdateCachePolicy handles PUT /2020-05-31/cache-policy/{id}.
func (s *Service) UpdateCachePolicy(w http.ResponseWriter, r *http.Request) {
	cfg, ok := readCachePolicyConfig(w, r)
	if !ok {
		return
	}

	policy, err := s.storage.UpdateCachePolicy(r.Context(), r.PathValue("id"), cfg, r.Header.Get("If-Match"))
	if err != nil {
		handleCachePolicyStorageError(w, err)

		return
	}

	w.Header().Set("ETag", policy.ETag)
	writeXMLResponse(w, http.StatusOK, buildCachePolicyXML(policy, cloudfrontXmlns))
}

// DeleteCachePolicy handles DELETE /2020-05-31/cache-policy/{id}.
func (s *Service) DeleteCachePolicy(w http.ResponseWriter, r *http.Request) {
	if err := s.storage.DeleteCachePolicy(r.Context(), r.PathValue("id"), r.Header.Get("If-Match")); err != nil {
		handleCachePolicyStorageError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListCachePolicies handles GET /2020-05-31/cache-policy.
func (s *Service) ListCachePolicies(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	maxItems := cachePolicyListMaxItems

	if raw := query.Get("MaxItems"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeCloudFrontError(w, errInvalidArgument, "MaxItems must be a positive integer.", http.StatusBadRequest)

			return
		}

		maxItems = n
	}

	policyType := query.Get("Type")
	if policyType != "" && policyType != cachePolicyTypeCustom && policyType != cachePolicyTypeManaged {
		writeCloudFrontError(w, errInvalidArgument, "Type must be one of managed, custom.", http.StatusBadRequest)

		return
	}

	items, nextMarker, err := s.storage.ListCachePolicies(r.Context(), policyType, query.Get("Marker"), maxItems)
	if err != nil {
		handleCachePolicyStorageError(w, err)

		return
	}

	list := &CachePolicyListXML{
		Xmlns:      cloudfrontXmlns,
		NextMarker: nextMarker,
		MaxItems:   maxItems,
		Quantity:   len(items),
	}

	if len(items) > 0 {
		list.Items = &CachePolicySummaryList{}

		for _, policy := range items {
			list.Items.CachePolicySummary = append(list.Items.CachePolicySummary, CachePolicySummaryXML{Type: cachePolicyTypeCustom, CachePolicy: buildCachePolicyXML(policy, "")})
		}
	}

	writeXMLResponse(w, http.StatusOK, list)
}

// readCachePolicyConfig decodes the request body; on failure the error
// response has been written.
func readCachePolicyConfig(w http.ResponseWriter, r *http.Request) (*CachePolicyConfig, bool) {
	return decodeXMLBody[CachePolicyConfig](w, r)
}

func buildCachePolicyXML(policy *CachePolicy, xmlns string) *CachePolicyXML {
	cfg := policy.Config

	return &CachePolicyXML{
		Xmlns:             xmlns,
		ID:                policy.ID,
		LastModifiedTime:  policy.LastModifiedTime.Format(time.RFC3339),
		CachePolicyConfig: &cfg,
	}
}

func handleCachePolicyStorageError(w http.ResponseWriter, err error) {
	writePolicyError(w, err, errNoSuchCachePolicy, errCachePolicyAlreadyExists, errCachePolicyInUse)
}
