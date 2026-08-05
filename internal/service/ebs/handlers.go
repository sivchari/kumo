// Package ebs provides AWS EBS direct API service emulation.
package ebs

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

const errValidation = "ValidationException"

// StartSnapshotHandler handles POST /snapshots.
func (s *Service) StartSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	var req StartSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.VolumeSize <= 0 {
		writeError(w, errValidation, "VolumeSize is required and must be greater than 0", http.StatusBadRequest)

		return
	}

	snapshot, err := s.storage.StartSnapshot(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(snapshot)
}

// CompleteSnapshotHandler handles POST /snapshots/completion/{snapshotId}.
func (s *Service) CompleteSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.PathValue("snapshotId")
	if snapshotID == "" {
		writeError(w, errValidation, "SnapshotId is required", http.StatusBadRequest)

		return
	}

	snapshot, err := s.storage.CompleteSnapshot(r.Context(), snapshotID)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeJSON(w, snapshot)
}

// ListSnapshotBlocksHandler handles GET /snapshots/{snapshotId}/blocks.
func (s *Service) ListSnapshotBlocksHandler(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.PathValue("snapshotId")
	if snapshotID == "" {
		writeError(w, errValidation, "SnapshotId is required", http.StatusBadRequest)

		return
	}

	result, err := s.storage.ListSnapshotBlocks(r.Context(), snapshotID)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeJSON(w, result)
}

// PutSnapshotBlockHandler handles PUT /snapshots/{snapshotId}/blocks/{blockIndex}.
func (s *Service) PutSnapshotBlockHandler(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.PathValue("snapshotId")
	if snapshotID == "" {
		writeError(w, errValidation, "SnapshotId is required", http.StatusBadRequest)

		return
	}

	blockIndexStr := r.PathValue("blockIndex")

	blockIndexInt, err := strconv.ParseInt(blockIndexStr, 10, 32)
	if err != nil {
		writeError(w, errValidation, "BlockIndex must be a valid integer", http.StatusBadRequest)

		return
	}

	blockIndex := int32(blockIndexInt)

	checksumAlgorithm := r.Header.Get("x-amz-Checksum-Algorithm")
	if checksumAlgorithm != "SHA256" {
		writeError(w, errValidation, "ChecksumAlgorithm must be SHA256", http.StatusBadRequest)

		return
	}

	providedChecksum := r.Header.Get("x-amz-Checksum")

	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, "InternalException", "Failed to read request body", http.StatusInternalServerError)

		return
	}

	// Compute SHA256 checksum of the block data.
	sum := sha256.Sum256(data)
	computedChecksum := base64.StdEncoding.EncodeToString(sum[:])

	checksum := providedChecksum
	if checksum == "" {
		checksum = computedChecksum
	}

	if err := s.storage.PutSnapshotBlock(r.Context(), snapshotID, blockIndex, data, checksum); err != nil {
		handleServiceError(w, err)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.Header().Set("x-amz-Checksum", checksum)
	w.Header().Set("x-amz-Checksum-Algorithm", checksumAlgorithm)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(&PutSnapshotBlockResponse{
		Checksum:          checksum,
		ChecksumAlgorithm: checksumAlgorithm,
	})
}

// GetSnapshotBlockHandler handles GET /snapshots/{snapshotId}/blocks/{blockIndex}.
func (s *Service) GetSnapshotBlockHandler(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.PathValue("snapshotId")
	if snapshotID == "" {
		writeError(w, errValidation, "SnapshotId is required", http.StatusBadRequest)

		return
	}

	blockIndexStr := r.PathValue("blockIndex")

	blockIndexInt, err := strconv.ParseInt(blockIndexStr, 10, 32)
	if err != nil {
		writeError(w, errValidation, "BlockIndex must be a valid integer", http.StatusBadRequest)

		return
	}

	blockIndex := int32(blockIndexInt)

	data, checksum, err := s.storage.GetSnapshotBlock(r.Context(), snapshotID, blockIndex)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	// Explicitly set content type to avoid XSS risk.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.Header().Set("x-amz-Data-Length", strconv.Itoa(len(data)))
	w.Header().Set("x-amz-Checksum", checksum)
	w.Header().Set("x-amz-Checksum-Algorithm", "SHA256")

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ListChangedBlocksHandler handles GET /snapshots/{secondSnapshotId}/changedblocks.
func (s *Service) ListChangedBlocksHandler(w http.ResponseWriter, r *http.Request) {
	secondSnapshotID := r.PathValue("secondSnapshotId")
	if secondSnapshotID == "" {
		writeError(w, errValidation, "SecondSnapshotId is required", http.StatusBadRequest)

		return
	}

	firstSnapshotID := r.URL.Query().Get("firstSnapshotId")

	result, err := s.storage.ListChangedBlocks(r.Context(), firstSnapshotID, secondSnapshotID)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeJSON(w, result)
}

// Helper functions.

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&ErrorResponse{
		Message: message,
		Reason:  code,
	})
}

func handleServiceError(w http.ResponseWriter, err error) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		status := http.StatusBadRequest

		if svcErr.Code == errSnapshotNotFound {
			status = http.StatusNotFound
		}

		writeError(w, svcErr.Code, svcErr.Message, status)

		return
	}

	writeError(w, "InternalException", err.Error(), http.StatusInternalServerError)
}
