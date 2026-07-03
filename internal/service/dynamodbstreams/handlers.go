package dynamodbstreams

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// DispatchAction routes the request to the appropriate handler based on X-Amz-Target header.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, s.TargetPrefix()+".")

	switch action {
	case "DescribeStream":
		s.DescribeStream(w, r)
	case "GetShardIterator":
		s.GetShardIterator(w, r)
	case "GetRecords":
		s.GetRecords(w, r)
	default:
		writeError(w, "UnknownOperationException", "The action "+action+" is not valid")
	}
}

// DescribeStream handles the DescribeStream action.
func (s *Service) DescribeStream(w http.ResponseWriter, r *http.Request) {
	var req DescribeStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "SerializationException", "Failed to parse request body")

		return
	}

	if req.StreamArn == "" {
		writeError(w, "ValidationException", "StreamArn is required")

		return
	}

	desc, err := s.storage.DescribeStream(req.StreamArn, req.Limit, req.ExclusiveStartShardID)
	if err != nil {
		writeError(w, "ResourceNotFoundException", "Requested resource not found: "+req.StreamArn)

		return
	}

	writeResponse(w, DescribeStreamResponse{
		StreamDescription: *desc,
	})
}

// GetShardIterator handles the GetShardIterator action.
func (s *Service) GetShardIterator(w http.ResponseWriter, r *http.Request) {
	var req GetShardIteratorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "SerializationException", "Failed to parse request body")

		return
	}

	if req.StreamArn == "" || req.ShardID == "" || req.ShardIteratorType == "" {
		writeError(w, "ValidationException", "StreamArn, ShardId, and ShardIteratorType are required")

		return
	}

	iterator, err := s.storage.GetShardIterator(req.StreamArn, req.ShardID, req.ShardIteratorType, req.SequenceNumber)
	if err != nil {
		writeError(w, "ResourceNotFoundException", "Requested resource not found")

		return
	}

	writeResponse(w, GetShardIteratorResponse{
		ShardIterator: iterator,
	})
}

// GetRecords handles the GetRecords action.
func (s *Service) GetRecords(w http.ResponseWriter, r *http.Request) {
	var req GetRecordsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "SerializationException", "Failed to parse request body")

		return
	}

	if req.ShardIterator == "" {
		writeError(w, "ValidationException", "ShardIterator is required")

		return
	}

	records, nextIterator, err := s.storage.GetRecords(req.ShardIterator, req.Limit)
	if err != nil {
		writeError(w, "ExpiredIteratorException", err.Error())

		return
	}

	writeResponse(w, GetRecordsResponse{
		Records:           records,
		NextShardIterator: nextIterator,
	})
}

// writeResponse writes a JSON response.
func writeResponse(w http.ResponseWriter, resp any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(&ErrorResponse{
		Type:    code,
		Message: message,
	})
}
