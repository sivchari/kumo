// Package sts implements the AWS Security Token Service handlers.
package sts

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

const (
	stsXMLNS        = "https://sts.amazonaws.com/doc/2011-06-15/"
	errInvalidParam = "InvalidParameterValue"
	errMissingParam = "MissingParameter"
)

// DispatchAction routes the request to the appropriate handler based on Action parameter.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	action := extractAction(r)

	switch action {
	case "AssumeRole":
		s.handleAssumeRole(w, r)
	case "AssumeRoleWithSAML":
		s.handleAssumeRoleWithSAML(w, r)
	case "AssumeRoleWithWebIdentity":
		s.handleAssumeRoleWithWebIdentity(w, r)
	case "GetCallerIdentity":
		s.handleGetCallerIdentity(w, r)
	case "GetSessionToken":
		s.handleGetSessionToken(w, r)
	case "GetFederationToken":
		s.handleGetFederationToken(w, r)
	default:
		writeError(w, errInvalidParam, fmt.Sprintf("The action '%s' is not valid", action), http.StatusBadRequest)
	}
}

// handleAssumeRole handles the AssumeRole action.
func (s *Service) handleAssumeRole(w http.ResponseWriter, r *http.Request) {
	var req AssumeRoleInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParam, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.RoleArn == "" {
		writeError(w, errMissingParam, "RoleArn is required", http.StatusBadRequest)

		return
	}

	if req.RoleSessionName == "" {
		writeError(w, errMissingParam, "RoleSessionName is required", http.StatusBadRequest)

		return
	}

	result, err := s.storage.AssumeRole(r.Context(), &req)
	if err != nil {
		writeError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)

		return
	}

	writeXMLResponse(w, XMLAssumeRoleResponse{
		Xmlns:            stsXMLNS,
		AssumedRoleUser:  result.AssumedRoleUser,
		Credentials:      result.Credentials,
		PackedPolicySize: result.PackedPolicySize,
		RequestID:        uuid.New().String(),
	})
}

// handleAssumeRoleWithSAML handles the AssumeRoleWithSAML action.
func (s *Service) handleAssumeRoleWithSAML(w http.ResponseWriter, r *http.Request) {
	var req AssumeRoleWithSAMLInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParam, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.RoleArn == "" {
		writeError(w, errMissingParam, "RoleArn is required", http.StatusBadRequest)

		return
	}

	if req.PrincipalArn == "" {
		writeError(w, errMissingParam, "PrincipalArn is required", http.StatusBadRequest)

		return
	}

	if req.SAMLAssertion == "" {
		writeError(w, errMissingParam, "SAMLAssertion is required", http.StatusBadRequest)

		return
	}

	result, err := s.storage.AssumeRoleWithSAML(r.Context(), &req)
	if err != nil {
		writeError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)

		return
	}

	writeXMLResponse(w, XMLAssumeRoleWithSAMLResponse{
		Xmlns:            stsXMLNS,
		AssumedRoleUser:  result.AssumedRoleUser,
		Credentials:      result.Credentials,
		PackedPolicySize: result.PackedPolicySize,
		RequestID:        uuid.New().String(),
	})
}

// handleAssumeRoleWithWebIdentity handles the AssumeRoleWithWebIdentity action.
func (s *Service) handleAssumeRoleWithWebIdentity(w http.ResponseWriter, r *http.Request) {
	var req AssumeRoleWithWebIdentityInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParam, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.RoleArn == "" {
		writeError(w, errMissingParam, "RoleArn is required", http.StatusBadRequest)

		return
	}

	if req.RoleSessionName == "" {
		writeError(w, errMissingParam, "RoleSessionName is required", http.StatusBadRequest)

		return
	}

	if req.WebIdentityToken == "" {
		writeError(w, errMissingParam, "WebIdentityToken is required", http.StatusBadRequest)

		return
	}

	result, err := s.storage.AssumeRoleWithWebIdentity(r.Context(), &req)
	if err != nil {
		writeError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)

		return
	}

	writeXMLResponse(w, XMLAssumeRoleWithWebIdentityResponse{
		Xmlns:            stsXMLNS,
		AssumedRoleUser:  result.AssumedRoleUser,
		Credentials:      result.Credentials,
		PackedPolicySize: result.PackedPolicySize,
		RequestID:        uuid.New().String(),
	})
}

// handleGetCallerIdentity handles the GetCallerIdentity action.
func (s *Service) handleGetCallerIdentity(w http.ResponseWriter, r *http.Request) {
	identity, err := s.storage.GetCallerIdentity(r.Context())
	if err != nil {
		writeError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)

		return
	}

	writeXMLResponse(w, XMLGetCallerIdentityResponse{
		Xmlns:     stsXMLNS,
		Account:   identity.Account,
		Arn:       identity.Arn,
		UserID:    identity.UserID,
		RequestID: uuid.New().String(),
	})
}

// handleGetSessionToken handles the GetSessionToken action.
func (s *Service) handleGetSessionToken(w http.ResponseWriter, r *http.Request) {
	var req GetSessionTokenInput

	_ = service.ReadJSONRequest(r, &req)

	creds, err := s.storage.GetSessionToken(r.Context(), &req)
	if err != nil {
		writeError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)

		return
	}

	writeXMLResponse(w, XMLGetSessionTokenResponse{
		Xmlns:       stsXMLNS,
		Credentials: creds,
		RequestID:   uuid.New().String(),
	})
}

// handleGetFederationToken handles the GetFederationToken action.
func (s *Service) handleGetFederationToken(w http.ResponseWriter, r *http.Request) {
	var req GetFederationTokenInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParam, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, errMissingParam, "Name is required", http.StatusBadRequest)

		return
	}

	result, err := s.storage.GetFederationToken(r.Context(), &req)
	if err != nil {
		writeError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)

		return
	}

	writeXMLResponse(w, XMLGetFederationTokenResponse{
		Xmlns:            stsXMLNS,
		Credentials:      result.Credentials,
		FederatedUser:    result.FederatedUser,
		PackedPolicySize: result.PackedPolicySize,
		RequestID:        uuid.New().String(),
	})
}

// Helper functions.

func extractAction(r *http.Request) string {
	target := r.Header.Get("X-Amz-Target")
	if target != "" {
		if idx := strings.LastIndex(target, "."); idx >= 0 {
			return target[idx+1:]
		}
	}

	return r.URL.Query().Get("Action")
}

func writeXMLResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code, message string, status int) {
	requestID := uuid.New().String()

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", requestID)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(XMLErrorResponse{
		Error: XMLError{
			Code:    code,
			Message: message,
		},
		RequestID: requestID,
	})
}
