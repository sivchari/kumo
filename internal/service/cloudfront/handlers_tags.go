package cloudfront

import (
	"encoding/xml"
	"io"
	"net/http"
	"sort"
)

const (
	tagOperationTag   = "Tag"
	tagOperationUntag = "Untag"
)

// CreateDistributionWithTags handles the CreateDistributionWithTags operation.
func (s *Service) CreateDistributionWithTags(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeCloudFrontError(w, errMissingBody, "Request body is missing", http.StatusBadRequest)

		return
	}

	var req DistributionConfigWithTags
	if err := xml.Unmarshal(body, &req); err != nil {
		writeCloudFrontError(w, errInvalidArgument, "Invalid request body", http.StatusBadRequest)

		return
	}

	s.createDistribution(w, r, &req.DistributionConfig, tagsFromXML(req.Tags))
}

// tagging dispatches TagResource and UntagResource, which share the same path.
func (s *Service) tagging(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("Operation") {
	case tagOperationTag:
		s.TagResource(w, r)
	case tagOperationUntag:
		s.UntagResource(w, r)
	default:
		writeCloudFrontError(w, errInvalidArgument, "Operation must be Tag or Untag", http.StatusBadRequest)
	}
}

// TagResource handles the TagResource operation.
func (s *Service) TagResource(w http.ResponseWriter, r *http.Request) {
	arn := r.URL.Query().Get("Resource")
	if arn == "" {
		writeCloudFrontError(w, errInvalidArgument, "Resource is required", http.StatusBadRequest)

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeCloudFrontError(w, errMissingBody, "Request body is missing", http.StatusBadRequest)

		return
	}

	var req TagsXML
	if err := xml.Unmarshal(body, &req); err != nil {
		writeCloudFrontError(w, errInvalidArgument, "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.TagResource(r.Context(), arn, tagsFromXML(req)); err != nil {
		handleStorageError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UntagResource handles the UntagResource operation.
func (s *Service) UntagResource(w http.ResponseWriter, r *http.Request) {
	arn := r.URL.Query().Get("Resource")
	if arn == "" {
		writeCloudFrontError(w, errInvalidArgument, "Resource is required", http.StatusBadRequest)

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeCloudFrontError(w, errMissingBody, "Request body is missing", http.StatusBadRequest)

		return
	}

	var req TagKeysXML
	if err := xml.Unmarshal(body, &req); err != nil {
		writeCloudFrontError(w, errInvalidArgument, "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.UntagResource(r.Context(), arn, req.Items.Keys); err != nil {
		handleStorageError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListTagsForResource handles the ListTagsForResource operation.
func (s *Service) ListTagsForResource(w http.ResponseWriter, r *http.Request) {
	arn := r.URL.Query().Get("Resource")
	if arn == "" {
		writeCloudFrontError(w, errInvalidArgument, "Resource is required", http.StatusBadRequest)

		return
	}

	tags, err := s.storage.ListTagsForResource(r.Context(), arn)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeXMLResponse(w, http.StatusOK, buildTagsXML(tags))
}

func tagsFromXML(t TagsXML) map[string]string {
	tags := make(map[string]string, len(t.Items.Tags))
	for _, tag := range t.Items.Tags {
		tags[tag.Key] = tag.Value
	}

	return tags
}

func buildTagsXML(tags map[string]string) *TagsXML {
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	resp := &TagsXML{Xmlns: cloudfrontXmlns}
	for _, k := range keys {
		resp.Items.Tags = append(resp.Items.Tags, TagXML{Key: k, Value: tags[k]})
	}

	return resp
}
