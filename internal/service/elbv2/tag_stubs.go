package elbv2

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// DescribeTags returns an empty TagDescription per requested ResourceArn.
//
// Required for terraform compatibility — terraform-provider-aws calls
// DescribeTags on every refresh of aws_lb / aws_lb_target_group /
// aws_lb_listener. Without it, `tofu apply` of any ELBv2 resource fails
// with InvalidAction immediately after the resource is created.
//
// AWS returns one TagDescription per requested ARN even when no tags are
// attached (the provider crashes if a requested ARN is missing from the
// response), so this stub echoes each ResourceArns.member.N entry back
// with an empty Tags list. Tags are not modeled in storage yet; same
// shape as the ecr / logs / dynamodb / route53 stubs — wire-level no-op
// with the door open for real persistence later.
func (s *Service) DescribeTags(w http.ResponseWriter, r *http.Request) {
	arns := collectResourceArns(r)

	items := make([]XMLTagDescription, 0, len(arns))
	for _, arn := range arns {
		items = append(items, XMLTagDescription{
			ResourceArn: arn,
			Tags:        XMLTagList{Items: []XMLTagMember{}},
		})
	}

	writeELBXMLResponse(w, XMLDescribeTagsResponse{
		Xmlns: elbXMLNS,
		DescribeTagsResult: XMLDescribeTagsResult{
			TagDescriptions: XMLTagDescriptionList{Items: items},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// collectResourceArns reads ResourceArns.member.N from the form, then
// falls back to a JSON ResourceArns array if the request body has been
// converted by the unified Query→JSON dispatcher.
func collectResourceArns(r *http.Request) []string {
	if err := r.ParseForm(); err == nil {
		var arns []string

		for i := 1; ; i++ {
			v := r.Form.Get(fmt.Sprintf("ResourceArns.member.%d", i))
			if v == "" {
				break
			}

			arns = append(arns, v)
		}

		if len(arns) > 0 {
			return arns
		}
	}

	var req describeTagsJSONRequest
	if err := readELBJSONRequest(r, &req); err == nil {
		return req.ResourceArns
	}

	return nil
}

// AddTags accepts and discards tag attachments.
func (s *Service) AddTags(w http.ResponseWriter, _ *http.Request) {
	writeELBXMLResponse(w, XMLAddTagsResponse{
		Xmlns:            elbXMLNS,
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// RemoveTags accepts and discards tag detachments.
func (s *Service) RemoveTags(w http.ResponseWriter, _ *http.Request) {
	writeELBXMLResponse(w, XMLRemoveTagsResponse{
		Xmlns:            elbXMLNS,
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}
