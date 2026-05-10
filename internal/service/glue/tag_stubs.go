package glue

import "net/http"

// GetTags returns an empty Tags map for any resource ARN.
//
// Required for terraform compatibility — terraform-provider-aws calls
// GetTags on every refresh of aws_glue_catalog_database /
// aws_glue_catalog_table / aws_glue_job. Without it, `tofu apply` of any
// Glue resource fails with `Unknown operation` immediately after the
// resource is created and destroy is also blocked.
//
// Tags are not modeled in the storage layer yet; this stub exists so the
// refresh path completes. Same shape as the ecr / logs / dynamodb /
// route53 / elbv2 stubs — wire-level no-op with the door open for real
// persistence later.
func (s *Service) GetTags(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, getTagsResponse{Tags: map[string]string{}})
}

// TagResource accepts and discards tag attachments.
func (s *Service) TagResource(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, struct{}{})
}

// UntagResource accepts and discards tag detachments.
func (s *Service) UntagResource(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, struct{}{})
}
