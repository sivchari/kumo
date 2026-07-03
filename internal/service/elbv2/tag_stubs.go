package elbv2

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// DescribeTags returns tag descriptions for requested ResourceArns.
func (s *Service) DescribeTags(w http.ResponseWriter, r *http.Request) {
	arns := collectResourceArns(r)
	tagsByARN, err := s.storage.DescribeTags(r.Context(), arns)
	if err != nil {
		handleELBError(w, err)

		return
	}

	items := make([]XMLTagDescription, 0, len(arns))
	for _, arn := range arns {
		items = append(items, XMLTagDescription{
			ResourceArn: arn,
			Tags:        xmlTags(tagsByARN[arn]),
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
		arns := parseMemberListFromForm(r.Form, "ResourceArns")
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

// AddTags attaches or updates tags on ELBv2 resources.
func (s *Service) AddTags(w http.ResponseWriter, r *http.Request) {
	arns, tags, err := readAddTagsRequest(r)
	if err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.AddTags(r.Context(), arns, tags); err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLAddTagsResponse{
		Xmlns:            elbXMLNS,
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// RemoveTags detaches tags from ELBv2 resources.
func (s *Service) RemoveTags(w http.ResponseWriter, r *http.Request) {
	arns, keys, err := readRemoveTagsRequest(r)
	if err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.RemoveTags(r.Context(), arns, keys); err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLRemoveTagsResponse{
		Xmlns:            elbXMLNS,
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

func readAddTagsRequest(r *http.Request) ([]string, map[string]string, error) {
	if err := r.ParseForm(); err == nil {
		arns := parseMemberListFromForm(r.Form, "ResourceArns")
		tags := parseELBTagPairsFromForm(r.Form)
		if len(arns) > 0 || len(tags) > 0 {
			return arns, tags, nil
		}
	}

	var req addTagsJSONRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		return nil, nil, err
	}

	return req.ResourceArns, jsonTagsToMap(req.Tags), nil
}

func readRemoveTagsRequest(r *http.Request) ([]string, []string, error) {
	if err := r.ParseForm(); err == nil {
		arns := parseMemberListFromForm(r.Form, "ResourceArns")
		keys := parseMemberListFromForm(r.Form, "TagKeys")
		if len(arns) > 0 || len(keys) > 0 {
			return arns, keys, nil
		}
	}

	var req removeTagsJSONRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		return nil, nil, err
	}

	return req.ResourceArns, req.TagKeys, nil
}

type addTagsJSONRequest struct {
	ResourceArns []string      `json:"ResourceArns"`
	Tags         []tagJSONPair `json:"Tags"`
}

type removeTagsJSONRequest struct {
	ResourceArns []string `json:"ResourceArns"`
	TagKeys      []string `json:"TagKeys"`
}

type tagJSONPair struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

type tagPairAcc struct {
	key   string
	value string
}

func parseELBTagPairsFromForm(form map[string][]string) map[string]string {
	byIdx := make(map[int]*tagPairAcc)

	for key, values := range form {
		applyELBTagPairFormEntry(byIdx, key, values)
	}

	out := make(map[string]string)
	for _, entry := range byIdx {
		if entry.key != "" {
			out[entry.key] = entry.value
		}
	}

	return out
}

func applyELBTagPairFormEntry(byIdx map[int]*tagPairAcc, key string, values []string) {
	suffix, ok := strings.CutPrefix(key, "Tags.member.")
	if !ok || len(values) == 0 {
		return
	}

	dot := strings.Index(suffix, ".")
	if dot < 0 {
		return
	}

	n, err := strconv.Atoi(suffix[:dot])
	if err != nil {
		return
	}

	entry, exists := byIdx[n]
	if !exists {
		entry = &tagPairAcc{}
		byIdx[n] = entry
	}

	switch suffix[dot+1:] {
	case "Key":
		entry.key = values[0]
	case "Value":
		entry.value = values[0]
	}
}

func jsonTagsToMap(tags []tagJSONPair) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		if tag.Key != "" {
			out[tag.Key] = tag.Value
		}
	}

	return out
}

func xmlTags(tags map[string]string) XMLTagList {
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	items := make([]XMLTagMember, 0, len(keys))
	for _, key := range keys {
		items = append(items, XMLTagMember{Key: key, Value: tags[key]})
	}

	return XMLTagList{Items: items}
}
