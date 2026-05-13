package kms

import (
	"encoding/json"
	"net/http"
)

// defaultKeyPolicy is the AWS-default key policy returned for any key when
// no explicit policy has been set. terraform-provider-aws hashes this for
// drift detection so it must be a stable JSON document with the standard
// AccountRootEnable statement.
const defaultKeyPolicy = `{"Version":"2012-10-17","Id":"key-default-1","Statement":[{"Sid":"Enable IAM User Permissions","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::000000000000:root"},"Action":"kms:*","Resource":"*"}]}`

// GetKeyPolicy returns the key policy for an existing key.
//
// terraform-provider-aws calls GetKeyPolicy on every refresh of aws_kms_key
// (and immediately after CreateKey) to populate the `policy` attribute.
// Without it, `tofu apply` errors out before the create call returns.
func (s *Service) GetKeyPolicy(w http.ResponseWriter, r *http.Request) {
	var req getKeyPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.KeyID == "" {
		writeKMSError(w, "ValidationException", "KeyId is required", http.StatusBadRequest)

		return
	}

	key, err := s.storage.GetKey(r.Context(), req.KeyID)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	policy := key.Policy
	if policy == "" {
		policy = defaultKeyPolicy
	}

	writeKMSResponse(w, getKeyPolicyResponse{
		Policy:     policy,
		PolicyName: "default",
	})
}

// PutKeyPolicy stores a key policy for an existing key.
//
// terraform-provider-aws calls PutKeyPolicy after CreateKey when a custom
// policy is specified, then polls GetKeyPolicy until the policy matches.
func (s *Service) PutKeyPolicy(w http.ResponseWriter, r *http.Request) {
	var req putKeyPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.KeyID == "" {
		writeKMSError(w, "ValidationException", "KeyId is required", http.StatusBadRequest)

		return
	}

	key, err := s.storage.GetKey(r.Context(), req.KeyID)
	if err != nil {
		handleKMSError(w, err)

		return
	}

	key.Policy = req.Policy

	writeKMSResponse(w, struct{}{})
}

// ListKeyPolicies reports the single "default" policy name AWS exposes.
func (s *Service) ListKeyPolicies(w http.ResponseWriter, _ *http.Request) {
	writeKMSResponse(w, listKeyPoliciesResponse{PolicyNames: []string{"default"}})
}

// ListResourceTags returns an empty tag list for any key.
//
// terraform-provider-aws calls this on every refresh; the field must be
// present even when empty.
func (s *Service) ListResourceTags(w http.ResponseWriter, _ *http.Request) {
	writeKMSResponse(w, listResourceTagsResponse{Tags: []map[string]string{}})
}

// TagResource accepts and discards tag attachments.
func (s *Service) TagResource(w http.ResponseWriter, _ *http.Request) {
	writeKMSResponse(w, struct{}{})
}

// UntagResource accepts and discards tag detachments.
func (s *Service) UntagResource(w http.ResponseWriter, _ *http.Request) {
	writeKMSResponse(w, struct{}{})
}

// GetKeyRotationStatus reports rotation as disabled for any key.
//
// terraform-provider-aws calls this on every refresh to populate the
// `enable_key_rotation` attribute. Rotation is not modeled in storage.
func (s *Service) GetKeyRotationStatus(w http.ResponseWriter, _ *http.Request) {
	writeKMSResponse(w, getKeyRotationStatusResponse{KeyRotationEnabled: false})
}
