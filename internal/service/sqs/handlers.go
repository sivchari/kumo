package sqs

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/sivchari/kumo/internal/service"
)

// CreateQueue handles the CreateQueue action.
func (s *Service) CreateQueue(w http.ResponseWriter, r *http.Request) {
	var req CreateQueueRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueName, "QueueName") {
		return
	}

	queue, err := s.storage.CreateQueue(r.Context(), req.QueueName, req.Attributes, req.Tags)
	if err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, CreateQueueResponse{
		QueueURL: queue.URL,
	})
}

// ListQueueTags handles the ListQueueTags action.
func (s *Service) ListQueueTags(w http.ResponseWriter, r *http.Request) {
	var req ListQueueTagsRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	tags, err := s.storage.ListQueueTags(r.Context(), req.QueueURL)
	if err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, ListQueueTagsResponse{
		Tags: tags,
	})
}

// TagQueue handles the TagQueue action.
func (s *Service) TagQueue(w http.ResponseWriter, r *http.Request) {
	var req TagQueueRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	if err := s.storage.TagQueue(r.Context(), req.QueueURL, req.Tags); err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// UntagQueue handles the UntagQueue action.
func (s *Service) UntagQueue(w http.ResponseWriter, r *http.Request) {
	var req UntagQueueRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	if err := s.storage.UntagQueue(r.Context(), req.QueueURL, req.TagKeys); err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// DeleteQueue handles the DeleteQueue action.
func (s *Service) DeleteQueue(w http.ResponseWriter, r *http.Request) {
	var req DeleteQueueRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	if err := s.storage.DeleteQueue(r.Context(), req.QueueURL); err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// ListQueues handles the ListQueues action.
func (s *Service) ListQueues(w http.ResponseWriter, r *http.Request) {
	var req ListQueuesRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	urls, err := s.storage.ListQueues(r.Context(), req.QueueNamePrefix)
	if err != nil {
		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, ListQueuesResponse{
		QueueUrls: urls,
	})
}

// GetQueueURL handles the GetQueueURL action.
func (s *Service) GetQueueURL(w http.ResponseWriter, r *http.Request) {
	var req GetQueueURLRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueName, "QueueName") {
		return
	}

	url, err := s.storage.GetQueueURL(r.Context(), req.QueueName)
	if err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, GetQueueURLResponse{
		QueueURL: url,
	})
}

// SendMessage handles the SendMessage action.
func (s *Service) SendMessage(w http.ResponseWriter, r *http.Request) {
	var req SendMessageRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	if !requireSQSParameter(w, req.MessageBody, "MessageBody") {
		return
	}

	msg, err := s.storage.SendMessage(r.Context(), req.QueueURL, req.MessageBody, req.DelaySeconds, req.MessageAttributes, req.MessageGroupID, req.MessageDeduplicationID)
	if err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, SendMessageResponse{
		MessageID:        msg.MessageID,
		MD5OfMessageBody: msg.MD5OfBody,
		SequenceNumber:   msg.SequenceNumber,
	})
}

// SendMessageBatch handles the SendMessageBatch action.
func (s *Service) SendMessageBatch(w http.ResponseWriter, r *http.Request) {
	var req SendMessageBatchRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	if len(req.Entries) == 0 {
		writeSQSError(w, "EmptyBatchRequest", "There should be at least one SendMessageBatchRequestEntry in the request", http.StatusBadRequest)

		return
	}

	if len(req.Entries) > 10 {
		writeSQSError(w, "TooManyEntriesInBatchRequest", "Maximum number of entries per request are 10", http.StatusBadRequest)

		return
	}

	seen := make(map[string]struct{}, len(req.Entries))
	for _, entry := range req.Entries {
		if _, exists := seen[entry.ID]; exists {
			writeSQSError(w, "BatchEntryIdsNotDistinct", "Two or more batch entries in the request have the same Id", http.StatusBadRequest)

			return
		}

		seen[entry.ID] = struct{}{}
	}

	resp := s.processBatchEntries(r.Context(), req.QueueURL, req.Entries)

	writeJSONResponse(w, resp)
}

// processBatchEntries processes individual entries in a SendMessageBatch request.
func (s *Service) processBatchEntries(ctx context.Context, queueURL string, entries []SendMessageBatchRequestEntry) SendMessageBatchResponse {
	var resp SendMessageBatchResponse

	for _, entry := range entries {
		if entry.MessageBody == "" {
			resp.Failed = append(resp.Failed, BatchResultErrorEntry{
				ID:          entry.ID,
				SenderFault: true,
				Code:        errCodeMissingParameter,
				Message:     "The request must contain the parameter MessageBody",
			})

			continue
		}

		msg, err := s.storage.SendMessage(ctx, queueURL, entry.MessageBody, entry.DelaySeconds, entry.MessageAttributes, entry.MessageGroupID, entry.MessageDeduplicationID)
		if err != nil {
			resp.Failed = append(resp.Failed, s.batchEntryError(entry.ID, err))

			continue
		}

		resp.Successful = append(resp.Successful, SendMessageBatchResultEntry{
			ID:               entry.ID,
			MessageID:        msg.MessageID,
			MD5OfMessageBody: msg.MD5OfBody,
			SequenceNumber:   msg.SequenceNumber,
		})
	}

	return resp
}

// batchEntryError converts an error to a BatchResultErrorEntry.
func (s *Service) batchEntryError(id string, err error) BatchResultErrorEntry {
	var qErr *QueueError
	if errors.As(err, &qErr) {
		return BatchResultErrorEntry{
			ID:          id,
			SenderFault: true,
			Code:        qErr.Code,
			Message:     qErr.Message,
		}
	}

	return BatchResultErrorEntry{
		ID:          id,
		SenderFault: false,
		Code:        "InternalError",
		Message:     "Internal server error",
	}
}

// ReceiveMessage handles the ReceiveMessage action.
func (s *Service) ReceiveMessage(w http.ResponseWriter, r *http.Request) {
	var req ReceiveMessageRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	maxMessages := req.MaxNumberOfMessages
	if maxMessages < 1 {
		maxMessages = 1
	}

	if maxMessages > 10 {
		maxMessages = 10
	}

	messages, err := s.storage.ReceiveMessage(r.Context(), req.QueueURL, maxMessages, req.VisibilityTimeout, req.WaitTimeSeconds)
	if err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, ReceiveMessageResponse{
		Messages: convertMessagesToResponse(messages),
	})
}

// convertMessagesToResponse converts Message slice to MessageResponse slice.
func convertMessagesToResponse(messages []*Message) []MessageResponse {
	result := make([]MessageResponse, len(messages))

	for i, msg := range messages {
		result[i] = MessageResponse{
			MessageID:         msg.MessageID,
			ReceiptHandle:     msg.ReceiptHandle,
			MD5OfBody:         msg.MD5OfBody,
			Body:              msg.Body,
			Attributes:        msg.Attributes,
			MessageAttributes: msg.MessageAttributes,
			SequenceNumber:    msg.SequenceNumber,
		}
	}

	return result
}

// DeleteMessage handles the DeleteMessage action.
func (s *Service) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	var req DeleteMessageRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	if !requireSQSParameter(w, req.ReceiptHandle, "ReceiptHandle") {
		return
	}

	if err := s.storage.DeleteMessage(r.Context(), req.QueueURL, req.ReceiptHandle); err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// DeleteMessageBatch handles the DeleteMessageBatch action.
func (s *Service) DeleteMessageBatch(w http.ResponseWriter, r *http.Request) {
	var req DeleteMessageBatchRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	if len(req.Entries) == 0 {
		writeSQSError(w, "EmptyBatchRequest", "There should be at least one DeleteMessageBatchRequestEntry in the request", http.StatusBadRequest)

		return
	}

	if len(req.Entries) > 10 {
		writeSQSError(w, "TooManyEntriesInBatchRequest", "Maximum number of entries per request are 10", http.StatusBadRequest)

		return
	}

	seen := make(map[string]struct{}, len(req.Entries))
	for _, entry := range req.Entries {
		if _, exists := seen[entry.ID]; exists {
			writeSQSError(w, "BatchEntryIdsNotDistinct", "Two or more batch entries in the request have the same Id", http.StatusBadRequest)

			return
		}

		seen[entry.ID] = struct{}{}
	}

	resp := s.processDeleteBatchEntries(r.Context(), req.QueueURL, req.Entries)

	writeJSONResponse(w, resp)
}

// processDeleteBatchEntries processes individual entries in a DeleteMessageBatch request.
func (s *Service) processDeleteBatchEntries(ctx context.Context, queueURL string, entries []DeleteMessageBatchRequestEntry) DeleteMessageBatchResponse {
	var resp DeleteMessageBatchResponse

	for _, entry := range entries {
		if entry.ReceiptHandle == "" {
			resp.Failed = append(resp.Failed, BatchResultErrorEntry{
				ID:          entry.ID,
				SenderFault: true,
				Code:        errCodeMissingParameter,
				Message:     "The request must contain the parameter ReceiptHandle",
			})

			continue
		}

		if err := s.storage.DeleteMessage(ctx, queueURL, entry.ReceiptHandle); err != nil {
			resp.Failed = append(resp.Failed, s.batchEntryError(entry.ID, err))

			continue
		}

		resp.Successful = append(resp.Successful, DeleteMessageBatchResultEntry{
			ID: entry.ID,
		})
	}

	return resp
}

// ChangeMessageVisibility handles the ChangeMessageVisibility action.
func (s *Service) ChangeMessageVisibility(w http.ResponseWriter, r *http.Request) {
	var req ChangeMessageVisibilityRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	if !requireSQSParameter(w, req.ReceiptHandle, "ReceiptHandle") {
		return
	}

	if err := s.storage.ChangeMessageVisibility(r.Context(), req.QueueURL, req.ReceiptHandle, req.VisibilityTimeout); err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// ChangeMessageVisibilityBatch handles the ChangeMessageVisibilityBatch action.
func (s *Service) ChangeMessageVisibilityBatch(w http.ResponseWriter, r *http.Request) {
	var req ChangeMessageVisibilityBatchRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	if len(req.Entries) == 0 {
		writeSQSError(w, "EmptyBatchRequest", "There should be at least one entry in the request", http.StatusBadRequest)

		return
	}

	var resp ChangeMessageVisibilityBatchResponse

	for _, entry := range req.Entries {
		if err := s.storage.ChangeMessageVisibility(r.Context(), req.QueueURL, entry.ReceiptHandle, entry.VisibilityTimeout); err != nil {
			resp.Failed = append(resp.Failed, s.batchEntryError(entry.ID, err))

			continue
		}

		resp.Successful = append(resp.Successful, ChangeMessageVisibilityBatchResultEntry{
			ID: entry.ID,
		})
	}

	writeJSONResponse(w, resp)
}

// PurgeQueue handles the PurgeQueue action.
func (s *Service) PurgeQueue(w http.ResponseWriter, r *http.Request) {
	var req PurgeQueueRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	if err := s.storage.PurgeQueue(r.Context(), req.QueueURL); err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// GetQueueAttributes handles the GetQueueAttributes action.
func (s *Service) GetQueueAttributes(w http.ResponseWriter, r *http.Request) {
	var req GetQueueAttributesRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	attrs, err := s.storage.GetQueueAttributes(r.Context(), req.QueueURL, req.AttributeNames)
	if err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, GetQueueAttributesResponse{
		Attributes: attrs,
	})
}

// SetQueueAttributes handles the SetQueueAttributes action.
func (s *Service) SetQueueAttributes(w http.ResponseWriter, r *http.Request) {
	var req SetQueueAttributesRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	if err := s.storage.SetQueueAttributes(r.Context(), req.QueueURL, req.Attributes); err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// AddPermission handles the AddPermission action.
func (s *Service) AddPermission(w http.ResponseWriter, r *http.Request) {
	var req AddPermissionRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	if !requireSQSParameter(w, req.Label, "Label") {
		return
	}

	if err := s.storage.AddPermission(r.Context(), req.QueueURL, req.Label, req.AWSAccountIDs, req.Actions); err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// RemovePermission handles the RemovePermission action.
func (s *Service) RemovePermission(w http.ResponseWriter, r *http.Request) {
	var req RemovePermissionRequest
	if !decodeSQSRequest(w, r, &req) {
		return
	}

	if !requireSQSParameter(w, req.QueueURL, "QueueUrl") {
		return
	}

	if !requireSQSParameter(w, req.Label, "Label") {
		return
	}

	if err := s.storage.RemovePermission(r.Context(), req.QueueURL, req.Label); err != nil {
		var qErr *QueueError
		if errors.As(err, &qErr) {
			writeSQSError(w, qErr.Code, qErr.Message, http.StatusBadRequest)

			return
		}

		writeSQSError(w, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	service.WriteJSONResponse(w, service.ContentTypeAmzJSON10, v)
}

// writeSQSError writes an SQS error response in JSON format.
func writeSQSError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeAmzJSON10, code, message, status)
}

// decodeSQSRequest decodes the JSON request body into req, writing the
// standard SQS decode-failure error and returning false on failure.
func decodeSQSRequest(w http.ResponseWriter, r *http.Request, req any) bool {
	if err := service.ReadJSONRequest(r, req); err != nil {
		writeSQSError(w, errCodeInvalidParameterValue, "Failed to parse request body", http.StatusBadRequest)

		return false
	}

	return true
}

// requireSQSParameter writes the standard SQS MissingParameter error and
// returns false if value is empty.
func requireSQSParameter(w http.ResponseWriter, value, name string) bool {
	if value == "" {
		writeSQSError(w, errCodeMissingParameter, name+" is required", http.StatusBadRequest)

		return false
	}

	return true
}

// sqsActions maps an X-Amz-Target action name to its handler method.
var sqsActions = map[string]func(*Service, http.ResponseWriter, *http.Request){
	"AddPermission":                (*Service).AddPermission,
	"RemovePermission":             (*Service).RemovePermission,
	"CreateQueue":                  (*Service).CreateQueue,
	"ListQueueTags":                (*Service).ListQueueTags,
	"TagQueue":                     (*Service).TagQueue,
	"UntagQueue":                   (*Service).UntagQueue,
	"DeleteQueue":                  (*Service).DeleteQueue,
	"ListQueues":                   (*Service).ListQueues,
	actionGetQueueURL:              (*Service).GetQueueURL,
	actionSendMessage:              (*Service).SendMessage,
	"SendMessageBatch":             (*Service).SendMessageBatch,
	actionReceiveMessage:           (*Service).ReceiveMessage,
	actionDeleteMessage:            (*Service).DeleteMessage,
	"DeleteMessageBatch":           (*Service).DeleteMessageBatch,
	actionPurgeQueue:               (*Service).PurgeQueue,
	actionGetQueueAttributes:       (*Service).GetQueueAttributes,
	"SetQueueAttributes":           (*Service).SetQueueAttributes,
	actionChangeMessageVisibility:  (*Service).ChangeMessageVisibility,
	"ChangeMessageVisibilityBatch": (*Service).ChangeMessageVisibilityBatch,
}

// DispatchAction routes the request to the appropriate handler based on X-Amz-Target header.
// This method implements the JSONProtocolService interface.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "AmazonSQS.")

	handler, ok := sqsActions[action]
	if !ok {
		writeSQSError(w, "InvalidAction", "The action "+action+" is not valid", http.StatusBadRequest)

		return
	}

	handler(s, w, r)
}
