// Package route53 provides an implementation of AWS Route 53 service.
package route53

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// readXMLBody reads and XML-decodes the request body into v, writing an
// InvalidInput error response and returning false on failure.
func readXMLBody(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "InvalidInput", "Failed to read request body")

		return false
	}

	if err := xml.Unmarshal(body, v); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "InvalidInput", "Failed to parse request body")

		return false
	}

	return true
}

// defaultDelegationSet returns the fixed name servers kumo reports for a zone.
func defaultDelegationSet() DelegationSet {
	return DelegationSet{
		NameServers: []string{
			"ns-1.kumo.local",
			"ns-2.kumo.local",
			"ns-3.kumo.local",
			"ns-4.kumo.local",
		},
	}
}

// recordInSyncChange creates an INSYNC ChangeInfo, persists it, and returns it.
func (s *Service) recordInSyncChange(comment string) (ChangeInfo, error) {
	ci := ChangeInfo{
		ID:          "/change/" + uuid.New().String(),
		Status:      "INSYNC",
		SubmittedAt: time.Now().UTC().Format(time.RFC3339),
		Comment:     comment,
	}

	if err := s.storage.PutChange(&ci); err != nil {
		return ChangeInfo{}, fmt.Errorf("put change: %w", err)
	}

	return ci, nil
}

// parseMaxItems reads a maxitems query value, clamped to (0,100], default 100.
func parseMaxItems(maxItemsStr string) int {
	maxItems := 100

	if maxItemsStr != "" {
		if parsed, err := strconv.Atoi(maxItemsStr); err == nil && parsed > 0 && parsed <= 100 {
			maxItems = parsed
		}
	}

	return maxItems
}

// pageHostedZones returns the page [startIdx, startIdx+maxItems) of zones as
// values, the index just past the page (len(zones) when not truncated), and
// whether more zones remain.
func pageHostedZones(zones []*HostedZone, startIdx, maxItems int) ([]HostedZone, int, bool) {
	endIdx := startIdx + maxItems

	truncated := endIdx < len(zones)
	if !truncated {
		endIdx = len(zones)
	}

	page := make([]HostedZone, 0, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		page = append(page, *zones[i])
	}

	return page, endIdx, truncated
}

// CreateHostedZone handles the CreateHostedZone API.
func (s *Service) CreateHostedZone(w http.ResponseWriter, r *http.Request) {
	var req CreateHostedZoneRequest
	if !readXMLBody(w, r, &req) {
		return
	}

	if req.Name == "" {
		writeErrorResponse(w, http.StatusBadRequest, "InvalidInput", "Name is required")

		return
	}

	if req.CallerReference == "" {
		writeErrorResponse(w, http.StatusBadRequest, "InvalidInput", "CallerReference is required")

		return
	}

	// Ensure name ends with a dot
	name := req.Name
	if !strings.HasSuffix(name, ".") {
		name += "."
	}

	zoneID := uuid.New().String()
	zone := &HostedZone{
		ID:                     "/hostedzone/" + zoneID,
		Name:                   name,
		CallerReference:        req.CallerReference,
		Config:                 req.HostedZoneConfig,
		ResourceRecordSetCount: 0,
	}

	if err := s.storage.CreateHostedZone(zone); err != nil {
		if errors.Is(err, ErrHostedZoneAlreadyExists) {
			writeErrorResponse(w, http.StatusConflict, "HostedZoneAlreadyExists", "Hosted zone already exists")

			return
		}

		writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())

		return
	}

	ci, err := s.recordInSyncChange("")
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())

		return
	}

	resp := CreateHostedZoneResponse{
		XMLNS:         xmlns,
		HostedZone:    *zone,
		ChangeInfo:    ci,
		DelegationSet: defaultDelegationSet(),
	}

	w.Header().Set("Location", fmt.Sprintf("https://route53.amazonaws.com/2013-04-01%s", zone.ID))
	writeXMLResponse(w, http.StatusCreated, resp)
}

// GetHostedZone handles the GetHostedZone API.
func (s *Service) GetHostedZone(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErrorResponse(w, http.StatusBadRequest, "InvalidInput", "Hosted zone ID is required")

		return
	}

	zoneID := "/hostedzone/" + id

	zone, err := s.storage.GetHostedZone(zoneID)
	if err != nil {
		if errors.Is(err, ErrHostedZoneNotFound) {
			writeErrorResponse(w, http.StatusNotFound, "NoSuchHostedZone", "Hosted zone not found")

			return
		}

		writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())

		return
	}

	resp := GetHostedZoneResponse{
		XMLNS:         xmlns,
		HostedZone:    *zone,
		DelegationSet: defaultDelegationSet(),
	}

	writeXMLResponse(w, http.StatusOK, resp)
}

// ListHostedZones handles the ListHostedZones API.
func (s *Service) ListHostedZones(w http.ResponseWriter, r *http.Request) {
	marker := r.URL.Query().Get("marker")
	maxItems := parseMaxItems(r.URL.Query().Get("maxitems"))

	zones, err := s.storage.ListHostedZones()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())

		return
	}

	// Sort zones by ID for consistent pagination.
	sort.Slice(zones, func(i, j int) bool {
		return zones[i].ID < zones[j].ID
	})

	// Find starting position based on marker.
	startIdx := 0

	if marker != "" {
		markerID := "/hostedzone/" + marker
		for i, z := range zones {
			if z.ID == markerID {
				startIdx = i + 1

				break
			}
		}
	}

	hostedZones, endIdx, isTruncated := pageHostedZones(zones, startIdx, maxItems)

	resp := ListHostedZonesResponse{
		XMLNS:       xmlns,
		HostedZones: hostedZones,
		IsTruncated: isTruncated,
		MaxItems:    strconv.Itoa(maxItems),
	}

	if marker != "" {
		resp.Marker = marker
	}

	if isTruncated {
		resp.NextMarker = strings.TrimPrefix(zones[endIdx].ID, "/hostedzone/")
	}

	writeXMLResponse(w, http.StatusOK, resp)
}

// ListHostedZonesByName handles the ListHostedZonesByName API. Hosted zones
// are sorted ASCII-ascending on Name (matching the AWS contract used by data
// sources such as aws_route53_zone). The dnsname query parameter, when
// supplied, filters to zones with Name >= dnsname; an exact-match dnsname
// yields a single-zone response with that zone first.
func (s *Service) ListHostedZonesByName(w http.ResponseWriter, r *http.Request) {
	dnsname := normalizeDNSName(r.URL.Query().Get("dnsname"))
	hostedZoneID := r.URL.Query().Get("hostedzoneid")
	maxItems := parseMaxItems(r.URL.Query().Get("maxitems"))

	zones, err := s.storage.ListHostedZones()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())

		return
	}

	sort.Slice(zones, func(i, j int) bool {
		return zones[i].Name < zones[j].Name
	})

	startIdx := 0

	if dnsname != "" {
		for i, z := range zones {
			if z.Name >= dnsname {
				startIdx = i

				break
			}

			startIdx = len(zones) // beyond range
		}
	}

	hostedZones, endIdx, isTruncated := pageHostedZones(zones, startIdx, maxItems)

	resp := ListHostedZonesByNameResponse{
		XMLNS:        xmlns,
		HostedZones:  hostedZones,
		DNSName:      dnsname,
		HostedZoneID: hostedZoneID,
		IsTruncated:  isTruncated,
		MaxItems:     strconv.Itoa(maxItems),
	}

	if isTruncated {
		resp.NextDNSName = zones[endIdx].Name
		resp.NextHostedZoneID = strings.TrimPrefix(zones[endIdx].ID, "/hostedzone/")
	}

	writeXMLResponse(w, http.StatusOK, resp)
}

// normalizeDNSName makes the DNS-name query value compare-friendly: AWS
// canonicalizes hosted-zone names with a trailing dot, but clients may send
// either form. Lowercase to make comparison case-insensitive.
func normalizeDNSName(name string) string {
	if name == "" {
		return ""
	}

	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".") {
		lower += "."
	}

	return lower
}

// DeleteHostedZone handles the DeleteHostedZone API.
func (s *Service) DeleteHostedZone(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErrorResponse(w, http.StatusBadRequest, "InvalidInput", "Hosted zone ID is required")

		return
	}

	zoneID := "/hostedzone/" + id

	if err := s.storage.DeleteHostedZone(zoneID); err != nil {
		if errors.Is(err, ErrHostedZoneNotFound) {
			writeErrorResponse(w, http.StatusNotFound, "NoSuchHostedZone", "Hosted zone not found")

			return
		}

		if errors.Is(err, ErrHostedZoneNotEmpty) {
			writeErrorResponse(w, http.StatusBadRequest, "HostedZoneNotEmpty", "Hosted zone is not empty")

			return
		}

		writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())

		return
	}

	ci, err := s.recordInSyncChange("")
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())

		return
	}

	resp := DeleteHostedZoneResponse{
		XMLNS:      xmlns,
		ChangeInfo: ci,
	}

	writeXMLResponse(w, http.StatusOK, resp)
}

// ChangeResourceRecordSets handles the ChangeResourceRecordSets API.
//
// changeRecordSetsErrors maps storage sentinels from ChangeRecordSets to their
// HTTP error response, checked in order.
var changeRecordSetsErrors = []struct {
	err     error
	status  int
	code    string
	message string
}{
	{ErrHostedZoneNotFound, http.StatusNotFound, "NoSuchHostedZone", "Hosted zone not found"},
	{ErrRecordSetAlreadyExists, http.StatusBadRequest, "ResourceRecordAlreadyExists", "Resource record already exists"},
	{ErrRecordSetNotFound, http.StatusBadRequest, "InvalidChangeBatch", "Resource record not found"},
	{ErrInvalidInput, http.StatusBadRequest, "InvalidInput", "Invalid change action"},
}

// writeChangeRecordSetsError maps a ChangeRecordSets error to its response,
// falling back to InternalError for an unrecognized error.
func writeChangeRecordSetsError(w http.ResponseWriter, err error) {
	for _, m := range changeRecordSetsErrors {
		if errors.Is(err, m.err) {
			writeErrorResponse(w, m.status, m.code, m.message)

			return
		}
	}

	writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())
}

// ChangeResourceRecordSets handles the ChangeResourceRecordSets API.
func (s *Service) ChangeResourceRecordSets(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErrorResponse(w, http.StatusBadRequest, "InvalidInput", "Hosted zone ID is required")

		return
	}

	var req ChangeResourceRecordSetsRequest
	if !readXMLBody(w, r, &req) {
		return
	}

	if len(req.ChangeBatch.Changes) == 0 {
		writeErrorResponse(w, http.StatusBadRequest, "InvalidInput", "At least one change is required")

		return
	}

	zoneID := "/hostedzone/" + id

	if err := s.storage.ChangeRecordSets(zoneID, req.ChangeBatch.Changes); err != nil {
		writeChangeRecordSetsError(w, err)

		return
	}

	ci, err := s.recordInSyncChange(req.ChangeBatch.Comment)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())

		return
	}

	resp := ChangeResourceRecordSetsResponse{
		XMLNS:      xmlns,
		ChangeInfo: ci,
	}

	writeXMLResponse(w, http.StatusOK, resp)
}

// ListResourceRecordSets handles the ListResourceRecordSets API.
func (s *Service) ListResourceRecordSets(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErrorResponse(w, http.StatusBadRequest, "InvalidInput", "Hosted zone ID is required")

		return
	}

	zoneID := "/hostedzone/" + id

	records, err := s.storage.GetRecordSets(zoneID)
	if err != nil {
		if errors.Is(err, ErrHostedZoneNotFound) {
			writeErrorResponse(w, http.StatusNotFound, "NoSuchHostedZone", "Hosted zone not found")

			return
		}

		writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())

		return
	}

	resp := ListResourceRecordSetsResponse{
		XMLNS:              xmlns,
		ResourceRecordSets: records,
		IsTruncated:        false,
		MaxItems:           "100",
	}

	writeXMLResponse(w, http.StatusOK, resp)
}

// GetChange handles the GetChange API.
func (s *Service) GetChange(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErrorResponse(w, http.StatusBadRequest, "InvalidInput", "Change ID is required")

		return
	}

	change, err := s.storage.GetChange(id)
	if err != nil {
		if errors.Is(err, ErrChangeNotFound) {
			writeErrorResponse(w, http.StatusNotFound, "NoSuchChange", "Change not found")

			return
		}

		writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())

		return
	}

	resp := GetChangeResponse{XMLNS: xmlns, ChangeInfo: *change}
	writeXMLResponse(w, http.StatusOK, resp)
}

// ListTagsForResource handles the ListTagsForResource API.
func (s *Service) ListTagsForResource(w http.ResponseWriter, r *http.Request) {
	resourceType := r.PathValue("type")
	resourceID := r.PathValue("id")

	if resourceType == "" || resourceID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "InvalidInput", "Resource type and ID are required")

		return
	}

	tags, err := s.storage.ListTagsForResource(resourceType, resourceID)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())

		return
	}

	writeXMLResponse(w, http.StatusOK, ListTagsForResourceXMLResponse{
		XMLNS: xmlns,
		ResourceTagSet: ResourceTagSet{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Tags:         TagList{Tag: tags},
		},
	})
}

// ChangeTagsForResource handles the ChangeTagsForResource API.
func (s *Service) ChangeTagsForResource(w http.ResponseWriter, r *http.Request) {
	resourceType := r.PathValue("type")
	resourceID := r.PathValue("id")

	if resourceType == "" || resourceID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "InvalidInput", "Resource type and ID are required")

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "InvalidInput", "Failed to read request body")

		return
	}

	var req ChangeTagsForResourceRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "InvalidInput", "Failed to parse request body")

		return
	}

	var addTags []Tag
	if req.AddTags != nil {
		addTags = req.AddTags.Tags
	}

	if err := s.storage.ChangeTagsForResource(resourceType, resourceID, addTags, req.RemoveTagKeys); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())

		return
	}

	writeXMLResponse(w, http.StatusOK, ChangeTagsForResourceXMLResponse{
		XMLNS: xmlns,
	})
}

// writeXMLResponse writes an XML response.
func writeXMLResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("x-amzn-RequestID", uuid.New().String())
	w.WriteHeader(status)
	_, _ = io.WriteString(w, xml.Header)
	_ = xml.NewEncoder(w).Encode(v)
}

// writeErrorResponse writes an error response.
func writeErrorResponse(w http.ResponseWriter, status int, code, message string) {
	resp := ErrorResponse{
		XMLNS: xmlns,
		Error: Error{
			Type:    "Sender",
			Code:    code,
			Message: message,
		},
		RequestID: uuid.New().String(),
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("x-amzn-RequestID", resp.RequestID)
	w.WriteHeader(status)
	_, _ = io.WriteString(w, xml.Header)
	_ = xml.NewEncoder(w).Encode(resp)
}
