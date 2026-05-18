package sfn

import "net/http"

// ValidateStateMachineDefinition reports the supplied definition as valid.
//
// terraform-provider-aws calls ValidateStateMachineDefinition during the
// plan phase of every aws_sfn_state_machine, before issuing CreateStateMachine.
// Without it, `tofu plan` fails with InvalidAction and the resource never
// reaches the create path.
//
// Definitions are not statically validated in kumo; this stub returns OK
// so the apply pipeline proceeds and CreateStateMachine does the real
// shape check.
func (s *Service) ValidateStateMachineDefinition(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, validateStateMachineDefinitionResponse{
		Result:      "OK",
		Diagnostics: []validateDiagnostic{},
	})
}

// ListTagsForResource returns an empty tag list for any state machine.
//
// terraform-provider-aws calls this on every refresh; the field must be
// present even when empty.
func (s *Service) ListTagsForResource(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, listTagsForResourceResponse{Tags: []map[string]string{}})
}

// TagResource accepts and discards tag attachments.
func (s *Service) TagResource(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, struct{}{})
}

// UntagResource accepts and discards tag detachments.
func (s *Service) UntagResource(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, struct{}{})
}

// ListStateMachineVersions reports no versions for any state machine.
//
// Versions are not modeled in storage. terraform-provider-aws calls this
// on every refresh; the StateMachineVersions field must be present even
// when empty.
func (s *Service) ListStateMachineVersions(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, listStateMachineVersionsResponse{StateMachineVersions: []map[string]string{}})
}

// ListStateMachineAliases reports no aliases for any state machine.
func (s *Service) ListStateMachineAliases(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, listStateMachineAliasesResponse{StateMachineAliases: []map[string]string{}})
}
