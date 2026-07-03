package kinesis

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

// handlerFunc is a type alias for handler functions.
type handlerFunc func(http.ResponseWriter, *http.Request)

// getActionHandlers returns a map of action names to handler functions.
func (s *Service) getActionHandlers() map[string]handlerFunc {
	return map[string]handlerFunc{
		"CreateStream":          s.CreateStream,
		"DeleteStream":          s.DeleteStream,
		"DescribeStream":        s.DescribeStream,
		"ListStreams":           s.ListStreams,
		"ListShards":            s.ListShards,
		"PutRecord":             s.PutRecord,
		"PutRecords":            s.PutRecords,
		"GetShardIterator":      s.GetShardIterator,
		"GetRecords":            s.GetRecords,
		"DescribeStreamSummary": s.DescribeStreamSummary,
	}
}

// DispatchAction dispatches the request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "Kinesis_20131202.")

	handlers := s.getActionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeError(w, "InvalidAction", "The action "+action+" is not valid for this endpoint.", http.StatusBadRequest)
}

// CreateStream handles the CreateStream API.
func (s *Service) CreateStream(w http.ResponseWriter, r *http.Request) {
	var req CreateStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.CreateStream(r.Context(), &req); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &CreateStreamResponse{})
}

// DeleteStream handles the DeleteStream API.
func (s *Service) DeleteStream(w http.ResponseWriter, r *http.Request) {
	var req DeleteStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	streamName := req.StreamName
	if streamName == "" && req.StreamARN != "" {
		// Extract stream name from ARN.
		parts := strings.Split(req.StreamARN, "/")
		if len(parts) >= 2 {
			streamName = parts[len(parts)-1]
		}
	}

	if err := s.storage.DeleteStream(r.Context(), streamName); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DeleteStreamResponse{})
}

// DescribeStream handles the DescribeStream API.
func (s *Service) DescribeStream(w http.ResponseWriter, r *http.Request) {
	var req DescribeStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	streamName := req.StreamName
	if streamName == "" && req.StreamARN != "" {
		parts := strings.Split(req.StreamARN, "/")
		if len(parts) >= 2 {
			streamName = parts[len(parts)-1]
		}
	}

	stream, shards, hasMoreShards, err := s.storage.DescribeStream(r.Context(), streamName, req.Limit, req.ExclusiveStartShardID)
	if err != nil {
		handleError(w, err)

		return
	}

	shardOutputs := make([]ShardOutput, len(shards))
	for i, shard := range shards {
		shardOutputs[i] = ShardOutput{
			ShardID:               shard.ShardID,
			ParentShardID:         shard.ParentShardID,
			AdjacentParentShardID: shard.AdjacentParentShardID,
			HashKeyRange:          shard.HashKeyRange,
			SequenceNumberRange:   shard.SequenceNumberRange,
		}
	}

	resp := &DescribeStreamResponse{
		StreamDescription: StreamDescription{
			StreamName:              stream.StreamName,
			StreamARN:               stream.StreamARN,
			StreamStatus:            string(stream.StreamStatus),
			StreamModeDetails:       stream.StreamModeDetails,
			Shards:                  shardOutputs,
			HasMoreShards:           hasMoreShards,
			RetentionPeriodHours:    stream.RetentionPeriodHours,
			StreamCreationTimestamp: float64(stream.StreamCreationTimestamp.Unix()),
			EnhancedMonitoring:      stream.EnhancedMonitoring,
			EncryptionType:          stream.EncryptionType,
			KeyID:                   stream.KeyID,
			OpenShardCount:          stream.OpenShardCount,
			ConsumerCount:           stream.ConsumerCount,
		},
	}

	writeResponse(w, resp)
}

// DescribeStreamSummary handles the DescribeStreamSummary API.
func (s *Service) DescribeStreamSummary(w http.ResponseWriter, r *http.Request) {
	var req DescribeStreamSummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	streamName := req.StreamName
	if streamName == "" && req.StreamARN != "" {
		parts := strings.Split(req.StreamARN, "/")
		if len(parts) >= 2 {
			streamName = parts[len(parts)-1]
		}
	}

	stream, _, _, err := s.storage.DescribeStream(r.Context(), streamName, 0, "")
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DescribeStreamSummaryResponse{
		StreamDescriptionSummary: StreamDescriptionSummary{
			StreamName:              stream.StreamName,
			StreamARN:               stream.StreamARN,
			StreamStatus:            string(stream.StreamStatus),
			StreamModeDetails:       stream.StreamModeDetails,
			RetentionPeriodHours:    stream.RetentionPeriodHours,
			StreamCreationTimestamp: float64(stream.StreamCreationTimestamp.Unix()),
			EnhancedMonitoring:      stream.EnhancedMonitoring,
			EncryptionType:          stream.EncryptionType,
			KeyID:                   stream.KeyID,
			OpenShardCount:          stream.OpenShardCount,
			ConsumerCount:           stream.ConsumerCount,
		},
	})
}

// ListStreams handles the ListStreams API.
func (s *Service) ListStreams(w http.ResponseWriter, r *http.Request) {
	var req ListStreamsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	// Use NextToken as ExclusiveStartStreamName if provided.
	// NextToken takes precedence over ExclusiveStartStreamName per AWS API behavior.
	exclusiveStartStreamName := req.ExclusiveStartStreamName
	if req.NextToken != "" {
		exclusiveStartStreamName = req.NextToken
	}

	streams, hasMoreStreams, err := s.storage.ListStreams(r.Context(), exclusiveStartStreamName, req.Limit)
	if err != nil {
		handleError(w, err)

		return
	}

	streamNames := make([]string, len(streams))
	streamSummaries := make([]StreamSummary, len(streams))

	for i, stream := range streams {
		streamNames[i] = stream.StreamName
		streamSummaries[i] = StreamSummary{
			StreamName:              stream.StreamName,
			StreamARN:               stream.StreamARN,
			StreamStatus:            string(stream.StreamStatus),
			StreamModeDetails:       stream.StreamModeDetails,
			StreamCreationTimestamp: float64(stream.StreamCreationTimestamp.Unix()),
		}
	}

	resp := &ListStreamsResponse{
		StreamNames:     streamNames,
		HasMoreStreams:  hasMoreStreams,
		StreamSummaries: streamSummaries,
	}

	// Set NextToken for pagination if there are more streams.
	if hasMoreStreams && len(streamNames) > 0 {
		resp.NextToken = streamNames[len(streamNames)-1]
	}

	writeResponse(w, resp)
}

// ListShards handles the ListShards API.
func (s *Service) ListShards(w http.ResponseWriter, r *http.Request) {
	var req ListShardsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	streamName := req.StreamName
	if streamName == "" && req.StreamARN != "" {
		parts := strings.Split(req.StreamARN, "/")
		if len(parts) >= 2 {
			streamName = parts[len(parts)-1]
		}
	}

	shards, nextToken, err := s.storage.ListShards(r.Context(), streamName, req.NextToken, req.MaxResults)
	if err != nil {
		handleError(w, err)

		return
	}

	shardOutputs := make([]ShardOutput, len(shards))
	for i, shard := range shards {
		shardOutputs[i] = ShardOutput{
			ShardID:               shard.ShardID,
			ParentShardID:         shard.ParentShardID,
			AdjacentParentShardID: shard.AdjacentParentShardID,
			HashKeyRange:          shard.HashKeyRange,
			SequenceNumberRange:   shard.SequenceNumberRange,
		}
	}

	resp := &ListShardsResponse{
		Shards:    shardOutputs,
		NextToken: nextToken,
	}

	writeResponse(w, resp)
}

// PutRecord handles the PutRecord API.
func (s *Service) PutRecord(w http.ResponseWriter, r *http.Request) {
	var req PutRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	streamName := req.StreamName
	if streamName == "" && req.StreamARN != "" {
		parts := strings.Split(req.StreamARN, "/")
		if len(parts) >= 2 {
			streamName = parts[len(parts)-1]
		}
	}

	shardID, seqNum, err := s.storage.PutRecord(r.Context(), streamName, req.Data, req.PartitionKey, req.ExplicitHashKey)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &PutRecordResponse{
		ShardID:        shardID,
		SequenceNumber: seqNum,
	}

	writeResponse(w, resp)
}

// PutRecords handles the PutRecords API.
func (s *Service) PutRecords(w http.ResponseWriter, r *http.Request) {
	var req PutRecordsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	streamName := req.StreamName
	if streamName == "" && req.StreamARN != "" {
		parts := strings.Split(req.StreamARN, "/")
		if len(parts) >= 2 {
			streamName = parts[len(parts)-1]
		}
	}

	results, failedCount, err := s.storage.PutRecords(r.Context(), streamName, req.Records)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &PutRecordsResponse{
		FailedRecordCount: failedCount,
		Records:           results,
	}

	writeResponse(w, resp)
}

// GetShardIterator handles the GetShardIterator API.
func (s *Service) GetShardIterator(w http.ResponseWriter, r *http.Request) {
	var req GetShardIteratorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	streamName := req.StreamName
	if streamName == "" && req.StreamARN != "" {
		parts := strings.Split(req.StreamARN, "/")
		if len(parts) >= 2 {
			streamName = parts[len(parts)-1]
		}
	}

	iterator, err := s.storage.GetShardIterator(r.Context(), streamName, req.ShardID, req.ShardIteratorType, req.StartingSequenceNumber, req.Timestamp)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &GetShardIteratorResponse{
		ShardIterator: iterator,
	}

	writeResponse(w, resp)
}

// GetRecords handles the GetRecords API.
func (s *Service) GetRecords(w http.ResponseWriter, r *http.Request) {
	var req GetRecordsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	records, nextIterator, millisBehind, err := s.storage.GetRecords(r.Context(), req.ShardIterator, req.Limit)
	if err != nil {
		handleError(w, err)

		return
	}

	recordOutputs := make([]RecordOutput, len(records))
	for i, record := range records {
		recordOutputs[i] = RecordOutput{
			Data:                        base64.StdEncoding.EncodeToString(record.Data),
			PartitionKey:                record.PartitionKey,
			SequenceNumber:              record.SequenceNumber,
			ApproximateArrivalTimestamp: float64(record.ApproximateArrivalTimestamp.Unix()),
			EncryptionType:              record.EncryptionType,
		}
	}

	resp := &GetRecordsResponse{
		Records:            recordOutputs,
		NextShardIterator:  nextIterator,
		MillisBehindLatest: millisBehind,
	}

	writeResponse(w, resp)
}

// writeResponse writes a JSON response.
func writeResponse(w http.ResponseWriter, resp any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeAmzJSON11, code, message, status)
}

// handleError handles service errors.
func handleError(w http.ResponseWriter, err error) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		status := getErrorStatus(svcErr.Code)
		writeError(w, svcErr.Code, svcErr.Message, status)

		return
	}

	writeError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)
}

// getErrorStatus returns the HTTP status code for a given error code.
func getErrorStatus(code string) int {
	switch code {
	case errResourceNotFound:
		return http.StatusNotFound
	case errResourceInUse:
		return http.StatusConflict
	case errInvalidArgument:
		return http.StatusBadRequest
	case errExpiredIterator:
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}
