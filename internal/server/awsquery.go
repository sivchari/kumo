package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
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
	// Parse form data.
	if err := r.ParseForm(); err != nil {
		writeQueryError(w, "InvalidParameterValue", "Failed to parse form data")

		return
	}

	// Get Action parameter.
	action := r.FormValue("Action")
	if action == "" {
		writeQueryError(w, "MissingAction", "Action parameter is required")

		return
	}

	// Identify the target service from User-Agent header.
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
	entry.handler(w, r)
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

		// Convert key from Query format to JSON format.
		// e.g., "Attributes.entry.1.key" -> handled specially
		// Simple values: "Name" -> "Name"
		if len(values) == 1 {
			result[key] = parseFormValue(values[0])
		} else if len(values) > 1 {
			result[key] = values
		}
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

	// Handle nested attributes (like Attributes.entry.N.key/value).
	result = flattenAttributes(result)

	// Handle MessageAttributes.entry.N.Name / Value.DataType / Value.StringValue.
	result = flattenMessageAttributes(result)

	jsonBytes, _ := json.Marshal(result)

	return jsonBytes
}

// parseFormValue converts a form value string to appropriate JSON type.
// Numeric strings are converted to numbers for proper JSON unmarshaling.
func parseFormValue(s string) any {
	// Try to parse as integer.
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}

	// Try to parse as boolean.
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}

	// Return as string.
	return s
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
		return
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
//nolint:funlen // AWS Query form attribute parsing.
func flattenMessageAttributes(data map[string]any) map[string]any {
	const prefix = "MessageAttributes.entry."

	// Collect per-index fields.
	type entryFields struct {
		name        string
		dataType    string
		stringValue string
	}

	entries := make(map[string]*entryFields) // keyed by index (e.g. "1")
	result := make(map[string]any)

	for key, value := range data {
		if !strings.HasPrefix(key, prefix) {
			result[key] = value

			continue
		}

		rest := key[len(prefix):] // e.g. "1.Name", "1.Value.DataType"
		dotIdx := strings.Index(rest, ".")

		if dotIdx < 0 {
			result[key] = value

			continue
		}

		idx := rest[:dotIdx]
		field := rest[dotIdx+1:]
		strValue, _ := value.(string)

		e, ok := entries[idx]
		if !ok {
			e = &entryFields{}
			entries[idx] = e
		}

		switch field {
		case "Name":
			e.name = strValue
		case "Value.DataType":
			e.dataType = strValue
		case "Value.StringValue":
			e.stringValue = strValue
		}
	}

	if len(entries) > 0 {
		msgAttrs := make(map[string]map[string]string, len(entries))

		for _, e := range entries {
			if e.name == "" {
				continue
			}

			attr := make(map[string]string)
			if e.dataType != "" {
				attr["DataType"] = e.dataType
			}

			if e.stringValue != "" {
				attr["StringValue"] = e.stringValue
			}

			msgAttrs[e.name] = attr
		}

		if len(msgAttrs) > 0 {
			result["MessageAttributes"] = msgAttrs
		}
	}

	return result
}

// writeQueryError writes an AWS Query protocol error response.
func writeQueryError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"__type":  code,
		"message": message,
	})
}
