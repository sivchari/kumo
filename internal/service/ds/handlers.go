package ds

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

// CreateDirectory handles the CreateDirectory API.
func (s *Service) CreateDirectory(w http.ResponseWriter, r *http.Request) {
	var req CreateDirectoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDSError(w, ErrClientException, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeDSError(w, ErrInvalidParameter, "Name is required", http.StatusBadRequest)

		return
	}

	if req.Password == "" {
		writeDSError(w, ErrInvalidParameter, "Password is required", http.StatusBadRequest)

		return
	}

	if req.Size == "" {
		req.Size = DirectorySizeSmall
	}

	directory, err := s.storage.CreateDirectory(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &CreateDirectoryResponse{
		DirectoryID: directory.DirectoryID,
	}

	writeJSONResponse(w, resp)
}

// DescribeDirectories handles the DescribeDirectories API.
func (s *Service) DescribeDirectories(w http.ResponseWriter, r *http.Request) {
	var req DescribeDirectoriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDSError(w, ErrClientException, "Invalid request body", http.StatusBadRequest)

		return
	}

	directories, nextToken, err := s.storage.DescribeDirectories(r.Context(), req.DirectoryIDs, req.Limit, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	descriptions := make([]*DirectoryDescription, len(directories))

	for i, d := range directories {
		desc := &DirectoryDescription{
			DirectoryID:        d.DirectoryID,
			Name:               d.Name,
			ShortName:          d.ShortName,
			Size:               d.Size,
			Description:        d.Description,
			DNSIPAddrs:         d.DNSIPAddrs,
			Stage:              d.Stage,
			LaunchTime:         float64(d.LaunchTime.Unix()),
			StageLastUpdatedAt: float64(d.StageLastUpdatedAt.Unix()),
			Type:               d.Type,
			SSOEnabled:         d.SSOEnabled,
			DesiredNumberOfDCs: d.DesiredNumberOfDCs,
		}

		if d.VPCSettings != nil {
			desc.VPCSettings = &DirectoryVPCSettingsResp{
				VPCID:             d.VPCSettings.VPCID,
				SubnetIDs:         d.VPCSettings.SubnetIDs,
				SecurityGroupID:   d.VPCSettings.SecurityGroupID,
				AvailabilityZones: d.VPCSettings.AvailabilityZones,
			}
		}

		descriptions[i] = desc
	}

	resp := &DescribeDirectoriesResponse{
		DirectoryDescriptions: descriptions,
		NextToken:             nextToken,
	}

	writeJSONResponse(w, resp)
}

// DeleteDirectory handles the DeleteDirectory API.
func (s *Service) DeleteDirectory(w http.ResponseWriter, r *http.Request) {
	var req DeleteDirectoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDSError(w, ErrClientException, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.DirectoryID == "" {
		writeDSError(w, ErrInvalidParameter, "DirectoryId is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteDirectory(r.Context(), req.DirectoryID)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &DeleteDirectoryResponse{
		DirectoryID: req.DirectoryID,
	}

	writeJSONResponse(w, resp)
}

// CreateSnapshot handles the CreateSnapshot API.
func (s *Service) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	var req CreateSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDSError(w, ErrClientException, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.DirectoryID == "" {
		writeDSError(w, ErrInvalidParameter, "DirectoryId is required", http.StatusBadRequest)

		return
	}

	snapshot, err := s.storage.CreateSnapshot(r.Context(), req.DirectoryID, req.Name)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &CreateSnapshotResponse{
		SnapshotID: snapshot.SnapshotID,
	}

	writeJSONResponse(w, resp)
}

// DescribeSnapshots handles the DescribeSnapshots API.
func (s *Service) DescribeSnapshots(w http.ResponseWriter, r *http.Request) {
	var req DescribeSnapshotsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDSError(w, ErrClientException, "Invalid request body", http.StatusBadRequest)

		return
	}

	snapshots, nextToken, err := s.storage.DescribeSnapshots(r.Context(), req.DirectoryID, req.SnapshotIDs, req.Limit, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	descriptions := make([]*SnapshotDescription, len(snapshots))
	for i, snap := range snapshots {
		descriptions[i] = &SnapshotDescription{
			SnapshotID:  snap.SnapshotID,
			DirectoryID: snap.DirectoryID,
			Name:        snap.Name,
			Type:        snap.Type,
			Status:      snap.Status,
			StartTime:   float64(snap.StartTime.Unix()),
		}
	}

	resp := &DescribeSnapshotsResponse{
		Snapshots: descriptions,
		NextToken: nextToken,
	}

	writeJSONResponse(w, resp)
}

// DeleteSnapshot handles the DeleteSnapshot API.
func (s *Service) DeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	var req DeleteSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDSError(w, ErrClientException, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.SnapshotID == "" {
		writeDSError(w, ErrInvalidParameter, "SnapshotId is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteSnapshot(r.Context(), req.SnapshotID)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &DeleteSnapshotResponse{
		SnapshotID: req.SnapshotID,
	}

	writeJSONResponse(w, resp)
}

// handleError handles Error and writes appropriate response.
func handleError(w http.ResponseWriter, err error) {
	var dsErr *Error
	if errors.As(err, &dsErr) {
		status := http.StatusBadRequest

		switch dsErr.Type {
		case ErrEntityDoesNotExist:
			status = http.StatusBadRequest
		case ErrEntityAlreadyExists:
			status = http.StatusConflict
		case ErrServiceException:
			status = http.StatusInternalServerError
		}

		writeDSError(w, dsErr.Type, dsErr.Message, status)

		return
	}

	writeDSError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)
}

// writeJSONResponse writes a JSON response with status 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	service.WriteJSONResponse(w, service.ContentTypeAmzJSON11, v)
}

// writeDSError writes a Directory Service error response.
func writeDSError(w http.ResponseWriter, errType, message string, status int) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("X-Amzn-Requestid", uuid.New().String())
	w.Header().Set("X-Amzn-Errortype", errType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&Error{
		Type:    errType,
		Message: message,
	})
}
