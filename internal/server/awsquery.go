package server

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

// QueryServiceHandler handles Query protocol requests for a specific service.
type QueryServiceHandler func(w http.ResponseWriter, r *http.Request)

// queryHandlerEntry holds a handler and its associated metadata.
type queryHandlerEntry struct {
	handler       QueryServiceHandler
	servicePrefix string
}

// QueryProtocolDispatcher routes AWS Query protocol requests to the appropriate service
// based on the Action parameter and User-Agent header.
// The User-Agent header must contain an api/{service} token (set by AWS SDK v2)
// to identify the target service.
type QueryProtocolDispatcher struct {
	// handlers maps service prefix to service handler.
	handlers map[string]QueryServiceHandler
	// actionHandlers maps serviceIdentifier -> action -> handler entry.
	actionHandlers map[string]map[string]queryHandlerEntry
}

// NewQueryProtocolDispatcher creates a new Query protocol dispatcher.
func NewQueryProtocolDispatcher() *QueryProtocolDispatcher {
	return &QueryProtocolDispatcher{
		handlers:       make(map[string]QueryServiceHandler),
		actionHandlers: make(map[string]map[string]queryHandlerEntry),
	}
}

// Register registers a service handler.
func (d *QueryProtocolDispatcher) Register(serviceName string, handler QueryServiceHandler) {
	d.handlers[serviceName] = handler
}

// RegisterAction registers a handler for a specific action under a service identifier.
func (d *QueryProtocolDispatcher) RegisterAction(action, servicePrefix, serviceIdentifier string, handler QueryServiceHandler) {
	if d.actionHandlers[serviceIdentifier] == nil {
		d.actionHandlers[serviceIdentifier] = make(map[string]queryHandlerEntry)
	}

	d.actionHandlers[serviceIdentifier][action] = queryHandlerEntry{
		handler:       handler,
		servicePrefix: servicePrefix,
	}
}

// ServeHTTP implements http.Handler and dispatches to the appropriate service.
func (d *QueryProtocolDispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeQueryError(w, "InvalidParameterValue", "Failed to parse form data")

		return
	}

	action := r.FormValue("Action")
	if action == "" {
		writeQueryError(w, "MissingAction", "Action parameter is required")

		return
	}

	svcID := parseServiceFromUserAgent(r.Header.Get("User-Agent"))

	entry, err := d.resolveHandler(svcID, action)
	if err != nil {
		writeQueryError(w, err.code, err.message)

		return
	}

	// Convert form data to JSON and dispatch.
	jsonBody := formToJSON(r.Form)
	r.Body = io.NopCloser(bytes.NewReader(jsonBody))
	r.ContentLength = int64(len(jsonBody))
	r.Header.Set("Content-Type", "application/x-amz-json-1.0")
	r.Header.Set("X-Amz-Target", entry.servicePrefix+"."+action)
	entry.handler(w, service.WithQueryProtocol(r))
}

// queryResolveError represents an error during handler resolution.
type queryResolveError struct {
	code    string
	message string
}

// resolveHandler finds the handler for the given service ID and action.
func (d *QueryProtocolDispatcher) resolveHandler(svcID, action string) (queryHandlerEntry, *queryResolveError) {
	if svcID != "" {
		return d.resolveByService(svcID, action)
	}

	return d.resolveByFallback(action)
}

func (d *QueryProtocolDispatcher) resolveByService(svcID, action string) (queryHandlerEntry, *queryResolveError) {
	actions, ok := d.actionHandlers[svcID]
	if !ok {
		return queryHandlerEntry{}, &queryResolveError{"UnknownService", "Unknown service: " + svcID}
	}

	entry, ok := actions[action]
	if !ok {
		return queryHandlerEntry{}, &queryResolveError{"UnknownAction", "Action " + action + " is not supported for service " + svcID}
	}

	return entry, nil
}

func (d *QueryProtocolDispatcher) resolveByFallback(action string) (queryHandlerEntry, *queryResolveError) {
	var entry queryHandlerEntry

	var found bool

	for _, actions := range d.actionHandlers {
		if e, ok := actions[action]; ok {
			if found {
				return queryHandlerEntry{}, &queryResolveError{"AmbiguousAction", "Action " + action + " matches multiple services; send a User-Agent with api/ identifier"}
			}

			entry = e
			found = true
		}
	}

	if !found {
		return queryHandlerEntry{}, &queryResolveError{"UnknownAction", "Action " + action + " is not supported"}
	}

	return entry, nil
}

// parseServiceFromUserAgent extracts the service identifier from the AWS SDK v2 User-Agent header.
// The User-Agent contains a token like "api/rds#1.5.0"; this function returns "rds".
// Returns empty string if no api/ token is found.
//
// terraform-provider-aws (and a few other SDK consumers) sometimes use "/"
// instead of "#" as the version separator (e.g. "api/monitoring/1.0"), so
// both are accepted.
func parseServiceFromUserAgent(userAgent string) string {
	for _, token := range strings.Split(userAgent, " ") {
		after, found := strings.CutPrefix(token, "api/")
		if !found {
			continue
		}

		// Strip version suffix: "rds#1.5.0" / "rds/1.5.0" -> "rds"
		name, _, _ := strings.Cut(after, "#")
		name, _, _ = strings.Cut(name, "/")
		name = strings.ToLower(name)

		return name
	}

	return ""
}

// indexedEntry holds a value with its original index from the query parameter.
type indexedEntry struct {
	index int
	value string
}

// formToJSON converts form values to JSON.
func formToJSON(form map[string][]string) []byte {
	result := make(map[string]any)
	indexedArrays := make(map[string][]indexedEntry)

	for key, values := range form {
		if key == "Action" || key == "Version" {
			continue
		}

		// Check for indexed array pattern: Name.1, Name.2, etc.
		if idx := strings.LastIndex(key, "."); idx > 0 {
			suffix := key[idx+1:]

			n, err := strconv.Atoi(suffix)
			if err == nil {
				baseName := key[:idx]
				if len(values) == 1 {
					indexedArrays[baseName] = append(indexedArrays[baseName], indexedEntry{index: n, value: values[0]})
				}

				continue
			}
		}

		assignFormValue(result, key, values)
	}

	// Add indexed arrays to result, sorted by original index.
	// Handle two patterns:
	// 1. List.member.N pattern (AWS Query Protocol lists) -> strip ".member" to get "List"
	// 2. Simple InstanceId.N pattern -> pluralize to "InstanceIds"
	for baseName, entries := range indexedArrays {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].index < entries[j].index
		})

		arr := make([]string, 0, len(entries))
		for _, e := range entries {
			arr = append(arr, e.value)
		}

		var keyName string

		if stripped, found := strings.CutSuffix(baseName, ".member"); found {
			// AWS Query Protocol list pattern: Subnets.member.1 -> Subnets
			keyName = stripped
		} else {
			// Simple indexed pattern: InstanceId.1 -> InstanceIds
			keyName = baseName + "s"
		}

		result[keyName] = arr
	}

	// Handle struct-list patterns like Tags.member.N.Key/Value.
	result = flattenMemberStructs(result)

	// Handle nested attributes (like Attributes.entry.N.key/value).
	result = flattenAttributes(result)

	// Handle MessageAttributes.entry.N.Name / Value.DataType / Value.StringValue / Value.BinaryValue.
	result = flattenMessageAttributes(result)

	jsonBytes, _ := json.Marshal(result)

	return jsonBytes
}

// messageAttributesPrefix is the Query-protocol prefix of SNS/SQS message
// attribute entries, whose values are kept verbatim.
const messageAttributesPrefix = "MessageAttributes.entry."

// assignFormValue stores a non-indexed form field. Message attribute values
// are opaque strings ("42" or "true" must stay strings); every other single
// value goes through parseFormValue and repeated values become a list.
func assignFormValue(result map[string]any, key string, values []string) {
	switch {
	case len(values) == 1 && strings.HasPrefix(key, messageAttributesPrefix):
		result[key] = values[0]
	case len(values) == 1:
		result[key] = parseFormValue(values[0])
	case len(values) > 1:
		result[key] = values
	}
}

// parseFormValue converts a form value string to appropriate JSON type.
// Numeric strings are converted to numbers for proper JSON unmarshaling.
func parseFormValue(s string) any {
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}

	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}

	return s
}

// flattenMemberStructs converts "<Base>.member.<N>.<Field>" form keys into an
// array of objects, e.g. Tags.member.1.Key / Tags.member.1.Value becomes
// {"Tags": [{"Key": "...", "Value": "..."}]}.
//
// Plain scalar member lists (e.g. Subnets.member.1) are already consumed by
// the numeric indexed-array pass in formToJSON and never reach this
// function. Entry-based patterns (Attributes.entry.N, MessageAttributes.entry.N)
// use ".entry." rather than ".member." and are handled by their own flatten
// functions, so there is no overlap.
func flattenMemberStructs(data map[string]any) map[string]any {
	result := make(map[string]any)
	groups := make(map[string]map[int]map[string]any) // base -> index -> field -> value

	for key, value := range data {
		base, idx, field, ok := parseMemberStructKey(key)
		if !ok {
			result[key] = value

			continue
		}

		if groups[base] == nil {
			groups[base] = make(map[int]map[string]any)
		}

		if groups[base][idx] == nil {
			groups[base][idx] = make(map[string]any)
		}

		groups[base][idx][field] = value
	}

	for base, byIndex := range groups {
		result[base] = membersInOrder(byIndex)
	}

	return result
}

// membersInOrder returns the members sorted by index, each with its dotted
// field paths expanded by nestFields.
func membersInOrder(byIndex map[int]map[string]any) []map[string]any {
	indices := slices.Sorted(maps.Keys(byIndex))

	arr := make([]map[string]any, 0, len(indices))
	for _, i := range indices {
		arr = append(arr, nestFields(byIndex[i]))
	}

	return arr
}

// nestFields expands the dotted field paths of a member into nested objects,
// and "<Name>.member.<N>.<Field>" paths into lists of objects, e.g.
// MetricStat.Metric.Dimensions.member.1.Name becomes
// {"MetricStat": {"Metric": {"Dimensions": [{"Name": ...}]}}}.
func nestFields(fields map[string]any) map[string]any {
	result := make(map[string]any, len(fields))
	children := make(map[string]map[string]any)

	for key, value := range fields {
		head, rest, found := strings.Cut(key, ".")
		if !found {
			result[key] = value

			continue
		}

		if children[head] == nil {
			children[head] = make(map[string]any)
		}

		children[head][rest] = value
	}

	for head, child := range children {
		if members, ok := groupMembers(child); ok {
			result[head] = membersInOrder(members)
		} else {
			result[head] = nestFields(child)
		}
	}

	return result
}

// groupMembers groups "member.<N>.<Field>" keys by index. ok is false unless
// every key has that shape.
func groupMembers(fields map[string]any) (map[int]map[string]any, bool) {
	byIndex := make(map[int]map[string]any)

	for key, value := range fields {
		rest, found := strings.CutPrefix(key, "member.")
		if !found {
			return nil, false
		}

		idxStr, field, found := strings.Cut(rest, ".")

		n, err := strconv.Atoi(idxStr)
		if !found || err != nil {
			return nil, false
		}

		if byIndex[n] == nil {
			byIndex[n] = make(map[string]any)
		}

		byIndex[n][field] = value
	}

	return byIndex, true
}

// parseMemberStructKey parses a "<Base>.member.<N>.<Field>" key. Field may be
// a dotted path into nested structures, which nestFields expands. ok is false
// for any other shape of key.
func parseMemberStructKey(key string) (base string, index int, field string, ok bool) {
	const marker = ".member."

	memberIdx := strings.Index(key, marker)
	if memberIdx < 0 {
		return "", 0, "", false
	}

	base = key[:memberIdx]
	rest := key[memberIdx+len(marker):] // "<N>.<Field>"

	dotIdx := strings.Index(rest, ".")
	if dotIdx < 0 {
		return "", 0, "", false
	}

	idxStr := rest[:dotIdx]
	field = rest[dotIdx+1:]

	n, err := strconv.Atoi(idxStr)
	if err != nil {
		return "", 0, "", false
	}

	return base, n, field, true
}

// flattenAttributes converts nested Query protocol attributes to JSON format.
func flattenAttributes(data map[string]any) map[string]any {
	result := make(map[string]any)
	attrs := make(map[string]string)

	for key, value := range data {
		if !strings.HasPrefix(key, "Attributes.entry.") {
			result[key] = value

			continue
		}

		parseAttributeEntry(key, value, attrs)
	}

	buildAttributesMap(attrs, result)

	return result
}

// parseAttributeEntry parses an Attributes.entry.N.key/value pattern.
func parseAttributeEntry(key string, value any, attrs map[string]string) {
	parts := strings.Split(key, ".")
	if len(parts) != 4 {
		return
	}

	idx := parts[2]
	field := parts[3]

	strValue, ok := value.(string)
	if !ok {
		strValue = fmt.Sprint(value)
	}

	switch field {
	case "key":
		attrs[idx+"_key"] = strValue
	case "value":
		attrs[idx+"_value"] = strValue
	}
}

// buildAttributesMap builds the Attributes map from parsed key/value pairs.
func buildAttributesMap(attrs map[string]string, result map[string]any) {
	if len(attrs) == 0 {
		return
	}

	attrMap := make(map[string]string)

	for k, v := range attrs {
		if !strings.HasSuffix(k, "_key") {
			continue
		}

		idx := strings.TrimSuffix(k, "_key")

		if val, ok := attrs[idx+"_value"]; ok {
			attrMap[v] = val
		}
	}

	if len(attrMap) > 0 {
		result["Attributes"] = attrMap
	}
}

// flattenMessageAttributes converts MessageAttributes.entry.N.* form keys
// into a nested JSON map that matches the SNS PublishRequest wire shape:
//
//	MessageAttributes.entry.1.Name          = event_type
//	MessageAttributes.entry.1.Value.DataType    = String
//	MessageAttributes.entry.1.Value.StringValue = billing
//
// becomes:
//
//	{"MessageAttributes": {"event_type": {"DataType":"String","StringValue":"billing"}}}
//
// Binary attributes carry their base64 text in Value.BinaryValue.
func flattenMessageAttributes(data map[string]any) map[string]any {
	entries := make(map[string]*messageAttributeEntry) // keyed by index (e.g. "1")
	result := make(map[string]any)

	for key, value := range data {
		rest, isEntry := strings.CutPrefix(key, messageAttributesPrefix)
		idx, field, hasField := strings.Cut(rest, ".") // e.g. "1", "Value.DataType"

		if !isEntry || !hasField {
			result[key] = value

			continue
		}

		entry, ok := entries[idx]
		if !ok {
			entry = &messageAttributeEntry{}
			entries[idx] = entry
		}

		strValue, _ := value.(string)
		entry.set(field, strValue)
	}

	msgAttrs := make(map[string]map[string]string, len(entries))

	for _, entry := range entries {
		if entry.name != "" {
			msgAttrs[entry.name] = entry.attribute()
		}
	}

	if len(msgAttrs) > 0 {
		result["MessageAttributes"] = msgAttrs
	}

	return result
}

// messageAttributeEntry collects the fields of one MessageAttributes.entry.N group.
type messageAttributeEntry struct {
	name        string
	dataType    string
	stringValue string
	binaryValue string
}

func (e *messageAttributeEntry) set(field, value string) {
	switch field {
	case "Name":
		e.name = value
	case "Value.DataType":
		e.dataType = value
	case "Value.StringValue":
		e.stringValue = value
	case "Value.BinaryValue":
		e.binaryValue = value
	}
}

// attribute renders the entry as the JSON object the service handlers decode.
func (e *messageAttributeEntry) attribute() map[string]string {
	attr := make(map[string]string)

	if e.dataType != "" {
		attr["DataType"] = e.dataType
	}

	if e.stringValue != "" {
		attr["StringValue"] = e.stringValue
	}

	if e.binaryValue != "" {
		attr["BinaryValue"] = e.binaryValue
	}

	return attr
}

// writeQueryError writes an AWS Query protocol ErrorResponse. It is written
// before any service is chosen, so it carries no service namespace.
func writeQueryError(w http.ResponseWriter, code, message string) {
	requestID := uuid.New().String()

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", requestID)
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(queryErrorResponse{
		Error:     queryError{Type: "Sender", Code: code, Message: message},
		RequestID: requestID,
	})
}

type queryErrorResponse struct {
	XMLName   xml.Name   `xml:"ErrorResponse"`
	Error     queryError `xml:"Error"`
	RequestID string     `xml:"RequestId"`
}

type queryError struct {
	Type    string `xml:"Type"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}
