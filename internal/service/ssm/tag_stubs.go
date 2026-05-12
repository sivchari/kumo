package ssm

import "net/http"

// listTagsForResourceResponse is the response for ListTagsForResource.
type listTagsForResourceResponse struct {
	TagList []map[string]string `json:"TagList"` //nolint:tagliatelle // AWS JSON uses PascalCase
}

// ListTagsForResource returns an empty tag list for any resource.
//
// Tags are not modeled in the storage layer yet; this stub exists so reads
// from clients that refresh parameter state after PutParameter (terraform,
// pulumi, CDK) do not fail with ValidationException.
func (s *Service) ListTagsForResource(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, listTagsForResourceResponse{TagList: []map[string]string{}})
}

// AddTagsToResource accepts and discards tag attachments.
func (s *Service) AddTagsToResource(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, struct{}{})
}

// RemoveTagsFromResource accepts and discards tag detachments.
func (s *Service) RemoveTagsFromResource(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, struct{}{})
}
