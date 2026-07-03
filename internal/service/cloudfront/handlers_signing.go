package cloudfront

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"time"
)

// CreatePublicKey handles POST /2020-05-31/public-key.
func (s *Service) CreatePublicKey(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeCloudFrontError(w, errMissingBody, "Request body is missing", http.StatusBadRequest)

		return
	}

	var req PublicKeyConfigXML
	if err := xml.Unmarshal(body, &req); err != nil {
		writeCloudFrontError(w, errInvalidArgument, "Invalid request body", http.StatusBadRequest)

		return
	}

	cfg := &PublicKeyConfig{
		CallerReference: req.CallerReference,
		Name:            req.Name,
		EncodedKey:      req.EncodedKey,
		Comment:         req.Comment,
	}

	key, err := s.storage.CreatePublicKey(r.Context(), cfg)
	if err != nil {
		handleSigningStorageError(w, err)

		return
	}

	w.Header().Set("ETag", key.ETag)
	writeXMLResponse(w, http.StatusCreated, buildPublicKeyResultXML(key))
}

// GetPublicKey handles GET /2020-05-31/public-key/{id}.
func (s *Service) GetPublicKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	key, err := s.storage.GetPublicKey(r.Context(), id)
	if err != nil {
		handleSigningStorageError(w, err)

		return
	}

	w.Header().Set("ETag", key.ETag)
	writeXMLResponse(w, http.StatusOK, buildPublicKeyResultXML(key))
}

// ListPublicKeys handles GET /2020-05-31/public-key.
func (s *Service) ListPublicKeys(w http.ResponseWriter, r *http.Request) {
	keys := s.storage.ListPublicKeys(r.Context())

	items := make([]PublicKeySummaryXML, len(keys))
	for i, k := range keys {
		items[i] = PublicKeySummaryXML{
			ID:          k.ID,
			Name:        k.PublicKeyConfig.Name,
			CreatedTime: k.CreatedTime.UTC().Format(time.RFC3339),
			EncodedKey:  k.PublicKeyConfig.EncodedKey,
			Comment:     k.PublicKeyConfig.Comment,
		}
	}

	writeXMLResponse(w, http.StatusOK, &PublicKeyListXML{
		Xmlns:    cloudfrontXmlns,
		MaxItems: 100,
		Quantity: len(items),
		Items:    items,
	})
}

// DeletePublicKey handles DELETE /2020-05-31/public-key/{id}.
func (s *Service) DeletePublicKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.storage.DeletePublicKey(r.Context(), id); err != nil {
		handleSigningStorageError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateKeyGroup handles POST /2020-05-31/key-group.
func (s *Service) CreateKeyGroup(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeCloudFrontError(w, errMissingBody, "Request body is missing", http.StatusBadRequest)

		return
	}

	var req KeyGroupConfigXML
	if err := xml.Unmarshal(body, &req); err != nil {
		writeCloudFrontError(w, errInvalidArgument, "Invalid request body", http.StatusBadRequest)

		return
	}

	cfg := &KeyGroupConfig{Name: req.Name, Items: req.Items, Comment: req.Comment}

	group, err := s.storage.CreateKeyGroup(r.Context(), cfg)
	if err != nil {
		handleSigningStorageError(w, err)

		return
	}

	w.Header().Set("ETag", group.ETag)
	writeXMLResponse(w, http.StatusCreated, buildKeyGroupResultXML(group))
}

// GetKeyGroup handles GET /2020-05-31/key-group/{id}.
func (s *Service) GetKeyGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	group, err := s.storage.GetKeyGroup(r.Context(), id)
	if err != nil {
		handleSigningStorageError(w, err)

		return
	}

	w.Header().Set("ETag", group.ETag)
	writeXMLResponse(w, http.StatusOK, buildKeyGroupResultXML(group))
}

// ListKeyGroups handles GET /2020-05-31/key-group.
func (s *Service) ListKeyGroups(w http.ResponseWriter, r *http.Request) {
	groups := s.storage.ListKeyGroups(r.Context())

	items := make([]KeyGroupSummaryXML, len(groups))
	for i, g := range groups {
		items[i] = KeyGroupSummaryXML{KeyGroup: *buildKeyGroupResultXML(g)}
	}

	writeXMLResponse(w, http.StatusOK, &KeyGroupListXML{
		Xmlns:    cloudfrontXmlns,
		MaxItems: 100,
		Quantity: len(items),
		Items:    items,
	})
}

// DeleteKeyGroup handles DELETE /2020-05-31/key-group/{id}.
func (s *Service) DeleteKeyGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.storage.DeleteKeyGroup(r.Context(), id); err != nil {
		handleSigningStorageError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func buildPublicKeyResultXML(k *PublicKey) *PublicKeyResultXML {
	return &PublicKeyResultXML{
		Xmlns:       cloudfrontXmlns,
		ID:          k.ID,
		CreatedTime: k.CreatedTime.UTC().Format(time.RFC3339),
		PublicKeyConfig: &PublicKeyConfigXML{
			CallerReference: k.PublicKeyConfig.CallerReference,
			Name:            k.PublicKeyConfig.Name,
			EncodedKey:      k.PublicKeyConfig.EncodedKey,
			Comment:         k.PublicKeyConfig.Comment,
		},
	}
}

func buildKeyGroupResultXML(g *KeyGroup) *KeyGroupResultXML {
	return &KeyGroupResultXML{
		Xmlns:        cloudfrontXmlns,
		ID:           g.ID,
		LastModified: g.LastModified.UTC().Format(time.RFC3339),
		KeyGroupConfig: &KeyGroupConfigXML{
			Name:    g.KeyGroupConfig.Name,
			Items:   g.KeyGroupConfig.Items,
			Comment: g.KeyGroupConfig.Comment,
		},
	}
}

// handleSigningStorageError maps signing-store errors to HTTP status
// codes. CloudFront uses 409 for already-exists / in-use conflicts and
// 404 for not-found.
func handleSigningStorageError(w http.ResponseWriter, err error) {
	var cfErr *Error
	if errors.As(err, &cfErr) {
		status := http.StatusBadRequest

		switch cfErr.Code {
		case errNoSuchPublicKey, errNoSuchKeyGroup:
			status = http.StatusNotFound
		case errPublicKeyAlreadyExists, errKeyGroupAlreadyExists:
			status = http.StatusConflict
		case errPublicKeyInUse, errKeyGroupReferencedError:
			status = http.StatusConflict
		}

		writeCloudFrontError(w, cfErr.Code, cfErr.Message, status)

		return
	}

	writeCloudFrontError(w, "InternalError", "Internal server error", http.StatusInternalServerError)
}
