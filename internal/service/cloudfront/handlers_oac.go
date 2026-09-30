package cloudfront

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strconv"
)

const originAccessControlPath = "/2020-05-31/origin-access-control/"

// CreateOriginAccessControl handles POST /2020-05-31/origin-access-control.
func (s *Service) CreateOriginAccessControl(w http.ResponseWriter, r *http.Request) {
	cfg, ok := readOriginAccessControlConfig(w, r)
	if !ok {
		return
	}

	oac, err := s.storage.CreateOriginAccessControl(r.Context(), cfg)
	if err != nil {
		handleOACStorageError(w, err)

		return
	}

	w.Header().Set("ETag", oac.ETag)
	w.Header().Set("Location", originAccessControlPath+oac.ID)
	writeXMLResponse(w, http.StatusCreated, buildOriginAccessControlXML(oac))
}

// GetOriginAccessControl handles GET /2020-05-31/origin-access-control/{id}.
func (s *Service) GetOriginAccessControl(w http.ResponseWriter, r *http.Request) {
	oac, err := s.storage.GetOriginAccessControl(r.Context(), r.PathValue("id"))
	if err != nil {
		handleOACStorageError(w, err)

		return
	}

	w.Header().Set("ETag", oac.ETag)
	writeXMLResponse(w, http.StatusOK, buildOriginAccessControlXML(oac))
}

// GetOriginAccessControlConfig handles GET /2020-05-31/origin-access-control/{id}/config.
func (s *Service) GetOriginAccessControlConfig(w http.ResponseWriter, r *http.Request) {
	oac, err := s.storage.GetOriginAccessControl(r.Context(), r.PathValue("id"))
	if err != nil {
		handleOACStorageError(w, err)

		return
	}

	cfg := buildOriginAccessControlConfigXML(&oac.Config)
	cfg.Xmlns = cloudfrontXmlns

	w.Header().Set("ETag", oac.ETag)
	writeXMLResponse(w, http.StatusOK, cfg)
}

// UpdateOriginAccessControl handles PUT /2020-05-31/origin-access-control/{id}/config.
func (s *Service) UpdateOriginAccessControl(w http.ResponseWriter, r *http.Request) {
	cfg, ok := readOriginAccessControlConfig(w, r)
	if !ok {
		return
	}

	oac, err := s.storage.UpdateOriginAccessControl(r.Context(), r.PathValue("id"), cfg, r.Header.Get("If-Match"))
	if err != nil {
		handleOACStorageError(w, err)

		return
	}

	w.Header().Set("ETag", oac.ETag)
	writeXMLResponse(w, http.StatusOK, buildOriginAccessControlXML(oac))
}

// DeleteOriginAccessControl handles DELETE /2020-05-31/origin-access-control/{id}.
func (s *Service) DeleteOriginAccessControl(w http.ResponseWriter, r *http.Request) {
	if err := s.storage.DeleteOriginAccessControl(r.Context(), r.PathValue("id"), r.Header.Get("If-Match")); err != nil {
		handleOACStorageError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListOriginAccessControls handles GET /2020-05-31/origin-access-control.
func (s *Service) ListOriginAccessControls(w http.ResponseWriter, r *http.Request) {
	maxItems := oacListMaxItems

	if raw := r.URL.Query().Get("MaxItems"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeCloudFrontError(w, errInvalidArgument, "MaxItems must be a positive integer.", http.StatusBadRequest)

			return
		}

		maxItems = n
	}

	marker := r.URL.Query().Get("Marker")

	items, nextMarker, err := s.storage.ListOriginAccessControls(r.Context(), marker, maxItems)
	if err != nil {
		handleOACStorageError(w, err)

		return
	}

	list := &OriginAccessControlListXML{
		Xmlns:       cloudfrontXmlns,
		Marker:      marker,
		NextMarker:  nextMarker,
		MaxItems:    maxItems,
		IsTruncated: nextMarker != "",
		Quantity:    len(items),
	}

	for _, oac := range items {
		list.Items = append(list.Items, OriginAccessControlSummaryXML{
			ID:                            oac.ID,
			Description:                   oac.Config.Description,
			Name:                          oac.Config.Name,
			SigningProtocol:               oac.Config.SigningProtocol,
			SigningBehavior:               oac.Config.SigningBehavior,
			OriginAccessControlOriginType: oac.Config.OriginAccessControlOriginType,
		})
	}

	writeXMLResponse(w, http.StatusOK, list)
}

// readOriginAccessControlConfig decodes the request body; on failure the
// error response has been written.
func readOriginAccessControlConfig(w http.ResponseWriter, r *http.Request) (*OriginAccessControlConfig, bool) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeCloudFrontError(w, errMissingBody, "Request body is missing", http.StatusBadRequest)

		return nil, false
	}

	var req OriginAccessControlConfigXML
	if err := xml.Unmarshal(body, &req); err != nil {
		writeCloudFrontError(w, errInvalidArgument, "Invalid request body", http.StatusBadRequest)

		return nil, false
	}

	return &OriginAccessControlConfig{
		Name:                          req.Name,
		Description:                   req.Description,
		SigningProtocol:               req.SigningProtocol,
		SigningBehavior:               req.SigningBehavior,
		OriginAccessControlOriginType: req.OriginAccessControlOriginType,
	}, true
}

func buildOriginAccessControlXML(oac *OriginAccessControl) *OriginAccessControlResultXML {
	return &OriginAccessControlResultXML{
		Xmlns:                     cloudfrontXmlns,
		ID:                        oac.ID,
		OriginAccessControlConfig: buildOriginAccessControlConfigXML(&oac.Config),
	}
}

func buildOriginAccessControlConfigXML(cfg *OriginAccessControlConfig) *OriginAccessControlConfigXML {
	return &OriginAccessControlConfigXML{
		Name:                          cfg.Name,
		Description:                   cfg.Description,
		SigningProtocol:               cfg.SigningProtocol,
		SigningBehavior:               cfg.SigningBehavior,
		OriginAccessControlOriginType: cfg.OriginAccessControlOriginType,
	}
}

// handleOACStorageError maps origin access control errors to the statuses
// the API reference documents: 404 for a missing control, 409 for a
// duplicate name or a control still in use, 412 for a stale ETag and 400 for
// a missing If-Match header or an invalid argument.
func handleOACStorageError(w http.ResponseWriter, err error) {
	var cfErr *Error
	if !errors.As(err, &cfErr) {
		writeCloudFrontError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	status := http.StatusBadRequest

	switch cfErr.Code {
	case errNoSuchOriginAccessControl:
		status = http.StatusNotFound
	case errOriginAccessControlAlreadyExists, errOriginAccessControlInUse:
		status = http.StatusConflict
	case errPreconditionFailed:
		status = http.StatusPreconditionFailed
	}

	writeCloudFrontError(w, cfErr.Code, cfErr.Message, status)
}
