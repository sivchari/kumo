// Package kms provides AWS KMS service emulation.
package kms

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// handlerFunc is a type alias for handler functions.
type handlerFunc func(http.ResponseWriter, *http.Request)

// getActionHandlers returns a map of action names to handler functions.
func (s *Service) getActionHandlers() map[string]handlerFunc {
	return map[string]handlerFunc{
		"CreateKey":           s.CreateKey,
		"DescribeKey":         s.DescribeKey,
		"ListKeys":            s.ListKeys,
		"EnableKey":           s.EnableKey,
		"DisableKey":          s.DisableKey,
		"ScheduleKeyDeletion": s.ScheduleKeyDeletion,
		"Encrypt":             s.Encrypt,
		"Decrypt":             s.Decrypt,
		"GenerateDataKey":     s.GenerateDataKey,
		"CreateAlias":         s.CreateAlias,
		"DeleteAlias":         s.DeleteAlias,
		"ListAliases":         s.ListAliases,
		// Key policy + tag operations.
		"GetKeyPolicy":         s.GetKeyPolicy,
		"PutKeyPolicy":         s.PutKeyPolicy,
		"ListKeyPolicies":      s.ListKeyPolicies,
		"ListResourceTags":     s.ListResourceTags,
		"TagResource":          s.TagResource,
		"UntagResource":        s.UntagResource,
		"GetKeyRotationStatus": s.GetKeyRotationStatus,
	}
}

// DispatchAction dispatches the request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "TrentService.")

	handlers := s.getActionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeKMSError(w, "InvalidAction", "The action "+action+" is not valid for this endpoint.", http.StatusBadRequest)
}

// CreateKey handles the CreateKey API.
func (s *Service) CreateKey(w http.ResponseWriter, r *http.Request) {
	var req CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeKMSError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	key, err := s.storage.CreateKey(r.Context(), &req)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	resp := &CreateKeyResponse{
		KeyMetadata: keyToMetadata(key),
	}

	writeKMSResponse(w, resp)
}

// DescribeKey handles the DescribeKey API.
func (s *Service) DescribeKey(w http.ResponseWriter, r *http.Request) {
	var req DescribeKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeKMSError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	key, err := s.storage.GetKey(r.Context(), req.KeyID)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	resp := &DescribeKeyResponse{
		KeyMetadata: keyToMetadata(key),
	}

	writeKMSResponse(w, resp)
}

// ListKeys handles the ListKeys API.
func (s *Service) ListKeys(w http.ResponseWriter, r *http.Request) {
	var req ListKeysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeKMSError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	keys, nextMarker, err := s.storage.ListKeys(r.Context(), req.Limit, req.Marker)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	keyEntries := make([]KeyListEntry, 0, len(keys))
	for _, key := range keys {
		keyEntries = append(keyEntries, KeyListEntry{
			KeyID:  key.KeyID,
			KeyArn: key.Arn,
		})
	}

	resp := &ListKeysResponse{
		Keys:       keyEntries,
		NextMarker: nextMarker,
		Truncated:  nextMarker != "",
	}

	writeKMSResponse(w, resp)
}

// EnableKey handles the EnableKey API.
func (s *Service) EnableKey(w http.ResponseWriter, r *http.Request) {
	var req EnableKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeKMSError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.EnableKey(r.Context(), req.KeyID); err != nil {
		handleKMSError(w, err)

		return
	}

	writeKMSResponse(w, &EnableKeyResponse{})
}

// DisableKey handles the DisableKey API.
func (s *Service) DisableKey(w http.ResponseWriter, r *http.Request) {
	var req DisableKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeKMSError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.DisableKey(r.Context(), req.KeyID); err != nil {
		handleKMSError(w, err)

		return
	}

	writeKMSResponse(w, &DisableKeyResponse{})
}

// ScheduleKeyDeletion handles the ScheduleKeyDeletion API.
func (s *Service) ScheduleKeyDeletion(w http.ResponseWriter, r *http.Request) {
	var req ScheduleKeyDeletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeKMSError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	key, err := s.storage.ScheduleKeyDeletion(r.Context(), req.KeyID, req.PendingWindowInDays)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	resp := &ScheduleKeyDeletionResponse{
		KeyID:               key.KeyID,
		DeletionDate:        float64(key.DeletionDate.Unix()),
		KeyState:            string(key.KeyState),
		PendingWindowInDays: key.PendingDeletionWindow,
	}

	writeKMSResponse(w, resp)
}

// Encrypt handles the Encrypt API.
func (s *Service) Encrypt(w http.ResponseWriter, r *http.Request) {
	var req EncryptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeKMSError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	key, err := s.storage.GetKey(r.Context(), req.KeyID)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	ciphertext, err := s.storage.Encrypt(r.Context(), req.KeyID, req.Plaintext, req.EncryptionContext)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	resp := &EncryptResponse{
		CiphertextBlob:      ciphertext,
		KeyID:               key.Arn,
		EncryptionAlgorithm: "SYMMETRIC_DEFAULT",
	}

	writeKMSResponse(w, resp)
}

// Decrypt handles the Decrypt API.
func (s *Service) Decrypt(w http.ResponseWriter, r *http.Request) {
	var req DecryptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeKMSError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	plaintext, keyID, err := s.storage.Decrypt(r.Context(), req.CiphertextBlob, req.EncryptionContext, req.KeyID)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	key, err := s.storage.GetKey(r.Context(), keyID)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	resp := &DecryptResponse{
		KeyID:               key.Arn,
		Plaintext:           plaintext,
		EncryptionAlgorithm: "SYMMETRIC_DEFAULT",
	}

	writeKMSResponse(w, resp)
}

// GenerateDataKey handles the GenerateDataKey API.
func (s *Service) GenerateDataKey(w http.ResponseWriter, r *http.Request) {
	var req GenerateDataKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeKMSError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	key, err := s.storage.GetKey(r.Context(), req.KeyID)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	plaintext, ciphertext, err := s.storage.GenerateDataKey(r.Context(), req.KeyID, req.KeySpec, req.NumberOfBytes, req.EncryptionContext)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	resp := &GenerateDataKeyResponse{
		CiphertextBlob: ciphertext,
		Plaintext:      plaintext,
		KeyID:          key.Arn,
	}

	writeKMSResponse(w, resp)
}

// CreateAlias handles the CreateAlias API.
func (s *Service) CreateAlias(w http.ResponseWriter, r *http.Request) {
	var req CreateAliasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeKMSError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.CreateAlias(r.Context(), req.AliasName, req.TargetKeyID); err != nil {
		handleKMSError(w, err)

		return
	}

	writeKMSResponse(w, &CreateAliasResponse{})
}

// DeleteAlias handles the DeleteAlias API.
func (s *Service) DeleteAlias(w http.ResponseWriter, r *http.Request) {
	var req DeleteAliasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeKMSError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteAlias(r.Context(), req.AliasName); err != nil {
		handleKMSError(w, err)

		return
	}

	writeKMSResponse(w, &DeleteAliasResponse{})
}

// ListAliases handles the ListAliases API.
func (s *Service) ListAliases(w http.ResponseWriter, r *http.Request) {
	var req ListAliasesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeKMSError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	aliases, nextMarker, err := s.storage.ListAliases(r.Context(), req.KeyID, req.Limit, req.Marker)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	aliasEntries := make([]AliasListEntry, 0, len(aliases))
	for _, alias := range aliases {
		aliasEntries = append(aliasEntries, AliasListEntry{
			AliasName:       alias.AliasName,
			AliasArn:        alias.AliasArn,
			TargetKeyID:     alias.TargetKeyID,
			CreationDate:    float64(alias.CreationDate.Unix()),
			LastUpdatedDate: float64(alias.LastUpdatedDate.Unix()),
		})
	}

	resp := &ListAliasesResponse{
		Aliases:    aliasEntries,
		NextMarker: nextMarker,
		Truncated:  nextMarker != "",
	}

	writeKMSResponse(w, resp)
}

// keyToMetadata converts a Key to KeyMetadata.
func keyToMetadata(key *Key) *KeyMetadata {
	metadata := &KeyMetadata{
		AWSAccountID: defaultAccountID,
		KeyID:        key.KeyID,
		Arn:          key.Arn,
		CreationDate: float64(key.CreationDate.Unix()),
		Enabled:      key.Enabled,
		Description:  key.Description,
		KeyUsage:     string(key.KeyUsage),
		KeyState:     string(key.KeyState),
		Origin:       key.Origin,
		KeyManager:   string(key.KeyManager),
		KeySpec:      string(key.KeySpec),
		MultiRegion:  key.MultiRegion,
	}

	if key.KeyUsage == KeyUsageEncryptDecrypt {
		metadata.EncryptionAlgorithms = []string{"SYMMETRIC_DEFAULT"}
	}

	if key.DeletionDate != nil {
		deletionDate := float64(key.DeletionDate.Unix())
		metadata.DeletionDate = &deletionDate
		metadata.PendingDeletionWindow = &key.PendingDeletionWindow
	}

	return metadata
}

// writeKMSResponse writes a JSON response.
func writeKMSResponse(w http.ResponseWriter, resp any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// writeKMSError writes an error response.
func writeKMSError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&ErrorResponse{
		Type:    code,
		Message: message,
	})
}

// handleKMSError handles KMS errors.
func handleKMSError(w http.ResponseWriter, err error) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		status := getErrorStatus(svcErr.Code)
		writeKMSError(w, svcErr.Code, svcErr.Message, status)

		return
	}

	writeKMSError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)
}

// getErrorStatus returns the HTTP status code for a given error code.
func getErrorStatus(code string) int {
	switch code {
	case errNotFound:
		return http.StatusNotFound
	case errAlreadyExists:
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}

// GetKeyPolicy handles the GetKeyPolicy API.
func (s *Service) GetKeyPolicy(w http.ResponseWriter, r *http.Request) {
	var req GetKeyPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.KeyID == "" {
		writeKMSError(w, "ValidationException", "KeyId is required", http.StatusBadRequest)

		return
	}

	policy, err := s.storage.GetKeyPolicy(r.Context(), req.KeyID)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	writeKMSResponse(w, &GetKeyPolicyResponse{
		Policy:     policy,
		PolicyName: "default",
	})
}

// PutKeyPolicy handles the PutKeyPolicy API.
func (s *Service) PutKeyPolicy(w http.ResponseWriter, r *http.Request) {
	var req PutKeyPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.KeyID == "" {
		writeKMSError(w, "ValidationException", "KeyId is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.PutKeyPolicy(r.Context(), req.KeyID, req.Policy); err != nil {
		handleKMSError(w, err)

		return
	}

	writeKMSResponse(w, &PutKeyPolicyResponse{})
}

// ListKeyPolicies handles the ListKeyPolicies API.
// AWS always returns a single policy named "default".
func (s *Service) ListKeyPolicies(w http.ResponseWriter, r *http.Request) {
	var req ListKeyPoliciesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.KeyID == "" {
		writeKMSError(w, "ValidationException", "KeyId is required", http.StatusBadRequest)

		return
	}

	// Verify the key exists.
	if _, err := s.storage.GetKey(r.Context(), req.KeyID); err != nil {
		handleKMSError(w, err)

		return
	}

	writeKMSResponse(w, &ListKeyPoliciesResponse{
		PolicyNames: []string{"default"},
		Truncated:   false,
	})
}

// ListResourceTags handles the ListResourceTags API.
func (s *Service) ListResourceTags(w http.ResponseWriter, r *http.Request) {
	var req ListResourceTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.KeyID == "" {
		writeKMSError(w, "ValidationException", "KeyId is required", http.StatusBadRequest)

		return
	}

	tags, err := s.storage.ListResourceTags(r.Context(), req.KeyID)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	writeKMSResponse(w, &ListResourceTagsResponse{
		Tags:      tags,
		Truncated: false,
	})
}

// TagResource handles the TagResource API.
func (s *Service) TagResource(w http.ResponseWriter, r *http.Request) {
	var req TagResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.KeyID == "" {
		writeKMSError(w, "ValidationException", "KeyId is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.TagResource(r.Context(), req.KeyID, req.Tags); err != nil {
		handleKMSError(w, err)

		return
	}

	writeKMSResponse(w, &TagResourceResponse{})
}

// UntagResource handles the UntagResource API.
func (s *Service) UntagResource(w http.ResponseWriter, r *http.Request) {
	var req UntagResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.KeyID == "" {
		writeKMSError(w, "ValidationException", "KeyId is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.UntagResource(r.Context(), req.KeyID, req.TagKeys); err != nil {
		handleKMSError(w, err)

		return
	}

	writeKMSResponse(w, &UntagResourceResponse{})
}

// GetKeyRotationStatus handles the GetKeyRotationStatus API.
func (s *Service) GetKeyRotationStatus(w http.ResponseWriter, r *http.Request) {
	var req GetKeyRotationStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.KeyID == "" {
		writeKMSError(w, "ValidationException", "KeyId is required", http.StatusBadRequest)

		return
	}

	rotationEnabled, err := s.storage.GetKeyRotationStatus(r.Context(), req.KeyID)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	writeKMSResponse(w, &GetKeyRotationStatusResponse{
		KeyRotationEnabled: rotationEnabled,
	})
}
