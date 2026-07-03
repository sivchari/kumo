package cloudwatchlogs

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

// Error codes for CloudWatch Logs.
const (
	errInvalidParameter     = "InvalidParameterException"
	errInternalServiceError = "ServiceUnavailableException"
	errInvalidAction        = "UnrecognizedClientException"
)

// CreateLogGroup handles the CreateLogGroup action.
func (s *Service) CreateLogGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateLogGroupRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LogGroupName == "" {
		writeLogsError(w, errInvalidParameter, "1 validation error detected: Value at 'logGroupName' failed to satisfy constraint: Member must not be null", http.StatusBadRequest)

		return
	}

	if err := s.storage.CreateLogGroup(r.Context(), &req); err != nil {
		handleLogsError(w, err)

		return
	}

	writeEmptyResponse(w)
}

// DeleteLogGroup handles the DeleteLogGroup action.
func (s *Service) DeleteLogGroup(w http.ResponseWriter, r *http.Request) {
	var req DeleteLogGroupRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LogGroupName == "" {
		writeLogsError(w, errInvalidParameter, "1 validation error detected: Value at 'logGroupName' failed to satisfy constraint: Member must not be null", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteLogGroup(r.Context(), req.LogGroupName); err != nil {
		handleLogsError(w, err)

		return
	}

	writeEmptyResponse(w)
}

// CreateLogStream handles the CreateLogStream action.
func (s *Service) CreateLogStream(w http.ResponseWriter, r *http.Request) {
	var req CreateLogStreamRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LogGroupName == "" || req.LogStreamName == "" {
		writeLogsError(w, errInvalidParameter, "1 validation error detected: Value at 'logGroupName' or 'logStreamName' failed to satisfy constraint: Member must not be null", http.StatusBadRequest)

		return
	}

	if err := s.storage.CreateLogStream(r.Context(), req.LogGroupName, req.LogStreamName); err != nil {
		handleLogsError(w, err)

		return
	}

	writeEmptyResponse(w)
}

// DeleteLogStream handles the DeleteLogStream action.
func (s *Service) DeleteLogStream(w http.ResponseWriter, r *http.Request) {
	var req DeleteLogStreamRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LogGroupName == "" || req.LogStreamName == "" {
		writeLogsError(w, errInvalidParameter, "1 validation error detected: Value at 'logGroupName' or 'logStreamName' failed to satisfy constraint: Member must not be null", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteLogStream(r.Context(), req.LogGroupName, req.LogStreamName); err != nil {
		handleLogsError(w, err)

		return
	}

	writeEmptyResponse(w)
}

// PutLogEvents handles the PutLogEvents action.
func (s *Service) PutLogEvents(w http.ResponseWriter, r *http.Request) {
	var req PutLogEventsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LogGroupName == "" || req.LogStreamName == "" {
		writeLogsError(w, errInvalidParameter, "1 validation error detected: Value at 'logGroupName' or 'logStreamName' failed to satisfy constraint: Member must not be null", http.StatusBadRequest)

		return
	}

	if len(req.LogEvents) == 0 {
		writeLogsError(w, errInvalidParameter, "1 validation error detected: Value at 'logEvents' failed to satisfy constraint: Member must not be empty", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.PutLogEvents(r.Context(), req.LogGroupName, req.LogStreamName, req.LogEvents, req.SequenceToken)
	if err != nil {
		handleLogsError(w, err)

		return
	}

	writeJSONResponse(w, resp)
}

// GetLogEvents handles the GetLogEvents action.
func (s *Service) GetLogEvents(w http.ResponseWriter, r *http.Request) {
	var req GetLogEventsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LogGroupName == "" || req.LogStreamName == "" {
		writeLogsError(w, errInvalidParameter, "1 validation error detected: Value at 'logGroupName' or 'logStreamName' failed to satisfy constraint: Member must not be null", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.GetLogEvents(r.Context(), &req)
	if err != nil {
		handleLogsError(w, err)

		return
	}

	writeJSONResponse(w, resp)
}

// FilterLogEvents handles the FilterLogEvents action.
func (s *Service) FilterLogEvents(w http.ResponseWriter, r *http.Request) {
	var req FilterLogEventsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LogGroupName == "" && req.LogGroupIdentifier == "" {
		writeLogsError(w, errInvalidParameter, "1 validation error detected: Value at 'logGroupName' or 'logGroupIdentifier' failed to satisfy constraint: Member must not be null", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.FilterLogEvents(r.Context(), &req)
	if err != nil {
		handleLogsError(w, err)

		return
	}

	writeJSONResponse(w, resp)
}

// DescribeLogGroups handles the DescribeLogGroups action.
func (s *Service) DescribeLogGroups(w http.ResponseWriter, r *http.Request) {
	var req DescribeLogGroupsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.DescribeLogGroups(r.Context(), &req)
	if err != nil {
		handleLogsError(w, err)

		return
	}

	writeJSONResponse(w, resp)
}

// DescribeLogStreams handles the DescribeLogStreams action.
func (s *Service) DescribeLogStreams(w http.ResponseWriter, r *http.Request) {
	var req DescribeLogStreamsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LogGroupName == "" && req.LogGroupIdentifier == "" {
		writeLogsError(w, errInvalidParameter, "1 validation error detected: Value at 'logGroupName' or 'logGroupIdentifier' failed to satisfy constraint: Member must not be null", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.DescribeLogStreams(r.Context(), &req)
	if err != nil {
		handleLogsError(w, err)

		return
	}

	writeJSONResponse(w, resp)
}

// PutRetentionPolicy handles the PutRetentionPolicy action.
func (s *Service) PutRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	var req PutRetentionPolicyRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LogGroupName == "" {
		writeLogsError(w, errInvalidParameter, "1 validation error detected: Value at 'logGroupName' failed to satisfy constraint: Member must not be null", http.StatusBadRequest)

		return
	}

	if req.RetentionInDays <= 0 {
		writeLogsError(w, errInvalidParameter, "1 validation error detected: Value at 'retentionInDays' failed to satisfy constraint: Member must be greater than 0", http.StatusBadRequest)

		return
	}

	if err := s.storage.PutRetentionPolicy(r.Context(), req.LogGroupName, req.RetentionInDays); err != nil {
		handleLogsError(w, err)

		return
	}

	writeEmptyResponse(w)
}

// DeleteRetentionPolicy handles the DeleteRetentionPolicy action.
func (s *Service) DeleteRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	var req DeleteRetentionPolicyRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LogGroupName == "" {
		writeLogsError(w, errInvalidParameter, "1 validation error detected: Value at 'logGroupName' failed to satisfy constraint: Member must not be null", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteRetentionPolicy(r.Context(), req.LogGroupName); err != nil {
		handleLogsError(w, err)

		return
	}

	writeEmptyResponse(w)
}

// ListTagsLogGroup handles the ListTagsLogGroup action.
func (s *Service) ListTagsLogGroup(w http.ResponseWriter, r *http.Request) {
	var req ListTagsLogGroupRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	tags, err := s.storage.ListTags(r.Context(), req.LogGroupName)
	if err != nil {
		handleLogsError(w, err)

		return
	}

	writeJSONResponse(w, TagsResponse{Tags: tags})
}

// ListTagsForResource handles the ListTagsForResource action.
func (s *Service) ListTagsForResource(w http.ResponseWriter, r *http.Request) {
	var req ListTagsForResourceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	tags, err := s.storage.ListTags(r.Context(), req.ResourceARN)
	if err != nil {
		handleLogsError(w, err)

		return
	}

	writeJSONResponse(w, TagsResponse{Tags: tags})
}

// TagLogGroup handles the TagLogGroup action.
func (s *Service) TagLogGroup(w http.ResponseWriter, r *http.Request) {
	var req TagLogGroupRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.Tag(r.Context(), req.LogGroupName, req.Tags); err != nil {
		handleLogsError(w, err)

		return
	}

	writeEmptyResponse(w)
}

// TagResource handles the TagResource action.
func (s *Service) TagResource(w http.ResponseWriter, r *http.Request) {
	var req TagResourceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.Tag(r.Context(), req.ResourceARN, req.Tags); err != nil {
		handleLogsError(w, err)

		return
	}

	writeEmptyResponse(w)
}

// UntagLogGroup handles the UntagLogGroup action.
func (s *Service) UntagLogGroup(w http.ResponseWriter, r *http.Request) {
	var req UntagLogGroupRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.Untag(r.Context(), req.LogGroupName, req.Tags); err != nil {
		handleLogsError(w, err)

		return
	}

	writeEmptyResponse(w)
}

// UntagResource handles the UntagResource action.
func (s *Service) UntagResource(w http.ResponseWriter, r *http.Request) {
	var req UntagResourceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeLogsError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.Untag(r.Context(), req.ResourceARN, req.TagKeys); err != nil {
		handleLogsError(w, err)

		return
	}

	writeEmptyResponse(w)
}

// DispatchAction routes the request to the appropriate handler based on X-Amz-Target header.
// This method implements the JSONProtocolService interface.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "Logs_20140328.")

	handler, ok := s.actionHandlers()[action]
	if !ok {
		writeLogsError(w, errInvalidAction, "The action "+action+" is not valid for this web service", http.StatusBadRequest)

		return
	}

	handler(w, r)
}

func (s *Service) actionHandlers() map[string]func(http.ResponseWriter, *http.Request) {
	return map[string]func(http.ResponseWriter, *http.Request){
		"CreateLogGroup":        s.CreateLogGroup,
		"DeleteLogGroup":        s.DeleteLogGroup,
		"CreateLogStream":       s.CreateLogStream,
		"DeleteLogStream":       s.DeleteLogStream,
		"PutLogEvents":          s.PutLogEvents,
		"GetLogEvents":          s.GetLogEvents,
		"FilterLogEvents":       s.FilterLogEvents,
		"DescribeLogGroups":     s.DescribeLogGroups,
		"DescribeLogStreams":    s.DescribeLogStreams,
		"PutRetentionPolicy":    s.PutRetentionPolicy,
		"DeleteRetentionPolicy": s.DeleteRetentionPolicy,
		"ListTagsForResource":   s.ListTagsForResource,
		"ListTagsLogGroup":      s.ListTagsLogGroup,
		"TagResource":           s.TagResource,
		"UntagResource":         s.UntagResource,
		"TagLogGroup":           s.TagLogGroup,
		"UntagLogGroup":         s.UntagLogGroup,
	}
}

// handleLogsError handles CloudWatch Logs errors.
func handleLogsError(w http.ResponseWriter, err error) {
	var logsErr *LogsError
	if errors.As(err, &logsErr) {
		writeLogsError(w, logsErr.Code, logsErr.Message, http.StatusBadRequest)

		return
	}

	writeLogsError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)
}

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	service.WriteJSONResponse(w, service.ContentTypeAmzJSON11, v)
}

// writeEmptyResponse writes an empty response with HTTP 200 OK.
func writeEmptyResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
}

// writeLogsError writes a CloudWatch Logs error response in JSON format.
func writeLogsError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeAmzJSON11, code, message, status)
}
