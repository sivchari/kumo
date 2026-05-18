package route53

import (
	"net/http"
)

// ListTagsForResource returns an empty tag list for any resource.
//
// terraform-provider-aws calls this on every refresh of aws_route53_zone
// (URL `/2013-04-01/tags/hostedzone/{Id}`). Without it, `tofu apply` of
// any Route53 zone fails with a 404 right after the zone is created.
//
// Tags are not modeled in the storage layer yet; this stub exists so the
// refresh path completes. Same shape as the ecr / logs / dynamodb /
// eventbridge tag stubs — wire-level no-op with a future-extensible door.
func (s *Service) ListTagsForResource(w http.ResponseWriter, _ *http.Request) {
	writeXMLResponse(w, http.StatusOK, ListTagsForResourceXMLResponse{
		XMLNS: xmlns,
		ResourceTagSet: ResourceTagSet{
			ResourceType: "hostedzone",
			Tags:         TagList{},
		},
	})
}

// ChangeTagsForResource accepts and discards tag mutations.
func (s *Service) ChangeTagsForResource(w http.ResponseWriter, _ *http.Request) {
	writeXMLResponse(w, http.StatusOK, ChangeTagsForResourceXMLResponse{
		XMLNS: xmlns,
	})
}
