package cloudfront

import (
	"net/http"
	"strconv"
	"time"
)

const responseHeadersPolicyPath = "/2020-05-31/response-headers-policy/"

// CreateResponseHeadersPolicy handles POST /2020-05-31/response-headers-policy.
func (s *Service) CreateResponseHeadersPolicy(w http.ResponseWriter, r *http.Request) {
	cfg, ok := decodeXMLBody[ResponseHeadersPolicyConfig](w, r)
	if !ok {
		return
	}

	policy, err := s.storage.CreateResponseHeadersPolicy(r.Context(), cfg)
	if err != nil {
		handleResponseHeadersPolicyStorageError(w, err)

		return
	}

	w.Header().Set("ETag", policy.ETag)
	w.Header().Set("Location", responseHeadersPolicyPath+policy.ID)
	writeXMLResponse(w, http.StatusCreated, buildResponseHeadersPolicyXML(policy, cloudfrontXmlns))
}

// GetResponseHeadersPolicy handles GET /2020-05-31/response-headers-policy/{id}.
func (s *Service) GetResponseHeadersPolicy(w http.ResponseWriter, r *http.Request) {
	policy, err := s.storage.GetResponseHeadersPolicy(r.Context(), r.PathValue("id"))
	if err != nil {
		handleResponseHeadersPolicyStorageError(w, err)

		return
	}

	w.Header().Set("ETag", policy.ETag)
	writeXMLResponse(w, http.StatusOK, buildResponseHeadersPolicyXML(policy, cloudfrontXmlns))
}

// GetResponseHeadersPolicyConfig handles GET /2020-05-31/response-headers-policy/{id}/config.
func (s *Service) GetResponseHeadersPolicyConfig(w http.ResponseWriter, r *http.Request) {
	policy, err := s.storage.GetResponseHeadersPolicy(r.Context(), r.PathValue("id"))
	if err != nil {
		handleResponseHeadersPolicyStorageError(w, err)

		return
	}

	cfg := policy.Config
	cfg.Xmlns = cloudfrontXmlns

	w.Header().Set("ETag", policy.ETag)
	writeXMLResponse(w, http.StatusOK, &cfg)
}

// UpdateResponseHeadersPolicy handles PUT /2020-05-31/response-headers-policy/{id}.
func (s *Service) UpdateResponseHeadersPolicy(w http.ResponseWriter, r *http.Request) {
	cfg, ok := decodeXMLBody[ResponseHeadersPolicyConfig](w, r)
	if !ok {
		return
	}

	policy, err := s.storage.UpdateResponseHeadersPolicy(r.Context(), r.PathValue("id"), cfg, r.Header.Get("If-Match"))
	if err != nil {
		handleResponseHeadersPolicyStorageError(w, err)

		return
	}

	w.Header().Set("ETag", policy.ETag)
	writeXMLResponse(w, http.StatusOK, buildResponseHeadersPolicyXML(policy, cloudfrontXmlns))
}

// DeleteResponseHeadersPolicy handles DELETE /2020-05-31/response-headers-policy/{id}.
func (s *Service) DeleteResponseHeadersPolicy(w http.ResponseWriter, r *http.Request) {
	if err := s.storage.DeleteResponseHeadersPolicy(r.Context(), r.PathValue("id"), r.Header.Get("If-Match")); err != nil {
		handleResponseHeadersPolicyStorageError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListResponseHeadersPolicies handles GET /2020-05-31/response-headers-policy.
func (s *Service) ListResponseHeadersPolicies(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	maxItems := responseHeadersPolicyListMaxItems

	if raw := query.Get("MaxItems"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeCloudFrontError(w, errInvalidArgument, "MaxItems must be a positive integer.", http.StatusBadRequest)

			return
		}

		maxItems = n
	}

	policyType := query.Get("Type")
	if policyType != "" && policyType != responseHeadersPolicyTypeCustom && policyType != responseHeadersPolicyTypeManaged {
		writeCloudFrontError(w, errInvalidArgument, "Type must be one of managed, custom.", http.StatusBadRequest)

		return
	}

	items, nextMarker, err := s.storage.ListResponseHeadersPolicies(r.Context(), policyType, query.Get("Marker"), maxItems)
	if err != nil {
		handleResponseHeadersPolicyStorageError(w, err)

		return
	}

	list := &ResponseHeadersPolicyListXML{
		Xmlns:      cloudfrontXmlns,
		NextMarker: nextMarker,
		MaxItems:   maxItems,
		Quantity:   len(items),
	}

	if len(items) > 0 {
		list.Items = &ResponseHeadersPolicySummaryList{}

		for _, policy := range items {
			list.Items.ResponseHeadersPolicySummary = append(list.Items.ResponseHeadersPolicySummary, ResponseHeadersPolicySummaryXML{Type: responseHeadersPolicyTypeCustom, ResponseHeadersPolicy: buildResponseHeadersPolicyXML(policy, "")})
		}
	}

	writeXMLResponse(w, http.StatusOK, list)
}

func buildResponseHeadersPolicyXML(policy *ResponseHeadersPolicy, xmlns string) *ResponseHeadersPolicyXML {
	cfg := policy.Config

	return &ResponseHeadersPolicyXML{
		Xmlns:                       xmlns,
		ID:                          policy.ID,
		LastModifiedTime:            policy.LastModifiedTime.Format(time.RFC3339),
		ResponseHeadersPolicyConfig: &cfg,
	}
}

func handleResponseHeadersPolicyStorageError(w http.ResponseWriter, err error) {
	writePolicyError(w, err, errNoSuchResponseHeadersPolicy, errResponseHeadersPolicyAlreadyExists, errResponseHeadersPolicyInUse)
}
