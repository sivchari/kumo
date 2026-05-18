package eventbridge

import "net/http"

// ListTagsForResource returns an empty tag list for any resource.
//
// Tags are not modeled in the storage layer yet; this stub exists so reads
// from clients that refresh state after CreateEventBus / PutRule (terraform,
// pulumi, CDK) do not fail with InvalidAction.
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
