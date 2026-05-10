package secretsmanager

import (
	"errors"
	"net/http"
)

// GetResourcePolicy returns an empty resource policy for an existing secret.
//
// terraform-provider-aws calls GetResourcePolicy on every refresh of
// aws_secretsmanager_secret. Resource policies are not modeled in storage
// yet; this stub exists so the refresh path completes. The response shape
// matches AWS: ARN + Name + null ResourcePolicy.
func (s *Service) GetResourcePolicy(w http.ResponseWriter, r *http.Request) {
	var req getResourcePolicyRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeSecretsManagerError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.SecretID == "" {
		writeSecretsManagerError(w, errInvalidParameter, "You must provide a value for the SecretId parameter.", http.StatusBadRequest)

		return
	}

	secret, err := s.storage.DescribeSecret(r.Context(), req.SecretID)
	if err != nil {
		var sErr *SecretError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errResourceNotFound {
				status = http.StatusNotFound
			}

			writeSecretsManagerError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeSecretsManagerError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, getResourcePolicyResponse{
		ARN:  secret.ARN,
		Name: secret.Name,
	})
}

// PutResourcePolicy accepts and discards a policy attachment.
func (s *Service) PutResourcePolicy(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, struct{}{})
}

// DeleteResourcePolicy accepts and discards a policy detachment.
func (s *Service) DeleteResourcePolicy(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, struct{}{})
}
