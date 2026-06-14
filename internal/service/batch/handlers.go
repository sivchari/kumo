package batch

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

// CreateComputeEnvironment handles the CreateComputeEnvironment operation.
func (s *Service) CreateComputeEnvironment(w http.ResponseWriter, r *http.Request) {
	var req CreateComputeEnvironmentInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ComputeEnvironmentName == "" {
		writeError(w, errInvalidRequest, "computeEnvironmentName is required", http.StatusBadRequest)

		return
	}

	if req.Type == "" {
		writeError(w, errInvalidRequest, "type is required", http.StatusBadRequest)

		return
	}

	ce, err := s.storage.CreateComputeEnvironment(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, CreateComputeEnvironmentOutput{
		ComputeEnvironmentARN:  ce.ComputeEnvironmentARN,
		ComputeEnvironmentName: ce.ComputeEnvironmentName,
	})
}

// DeleteComputeEnvironment handles the DeleteComputeEnvironment operation.
func (s *Service) DeleteComputeEnvironment(w http.ResponseWriter, r *http.Request) {
	var req DeleteComputeEnvironmentInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ComputeEnvironment == "" {
		writeError(w, errInvalidRequest, "computeEnvironment is required", http.StatusBadRequest)

		return
	}

	// Extract name from ARN if provided.
	name := extractResourceName(req.ComputeEnvironment)

	if err := s.storage.DeleteComputeEnvironment(r.Context(), name); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// DescribeComputeEnvironments handles the DescribeComputeEnvironments operation.
func (s *Service) DescribeComputeEnvironments(w http.ResponseWriter, r *http.Request) {
	var req DescribeComputeEnvironmentsInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	// Extract names from ARNs if provided.
	names := make([]string, 0, len(req.ComputeEnvironments))

	for _, ce := range req.ComputeEnvironments {
		names = append(names, extractResourceName(ce))
	}

	ces, err := s.storage.DescribeComputeEnvironments(r.Context(), names)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, DescribeComputeEnvironmentsOutput{
		ComputeEnvironments: ces,
	})
}

// CreateJobQueue handles the CreateJobQueue operation.
func (s *Service) CreateJobQueue(w http.ResponseWriter, r *http.Request) {
	var req CreateJobQueueInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.JobQueueName == "" {
		writeError(w, errInvalidRequest, "jobQueueName is required", http.StatusBadRequest)

		return
	}

	if len(req.ComputeEnvironmentOrder) == 0 {
		writeError(w, errInvalidRequest, "computeEnvironmentOrder is required", http.StatusBadRequest)

		return
	}

	jq, err := s.storage.CreateJobQueue(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, CreateJobQueueOutput{
		JobQueueARN:  jq.JobQueueARN,
		JobQueueName: jq.JobQueueName,
	})
}

// DeleteJobQueue handles the DeleteJobQueue operation.
func (s *Service) DeleteJobQueue(w http.ResponseWriter, r *http.Request) {
	var req DeleteJobQueueInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.JobQueue == "" {
		writeError(w, errInvalidRequest, "jobQueue is required", http.StatusBadRequest)

		return
	}

	name := extractResourceName(req.JobQueue)

	if err := s.storage.DeleteJobQueue(r.Context(), name); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// DescribeJobQueues handles the DescribeJobQueues operation.
func (s *Service) DescribeJobQueues(w http.ResponseWriter, r *http.Request) {
	var req DescribeJobQueuesInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	names := make([]string, 0, len(req.JobQueues))

	for _, jq := range req.JobQueues {
		names = append(names, extractResourceName(jq))
	}

	jqs, err := s.storage.DescribeJobQueues(r.Context(), names)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, DescribeJobQueuesOutput{
		JobQueues: jqs,
	})
}

// RegisterJobDefinition handles the RegisterJobDefinition operation.
func (s *Service) RegisterJobDefinition(w http.ResponseWriter, r *http.Request) {
	var req RegisterJobDefinitionInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.JobDefinitionName == "" {
		writeError(w, errInvalidRequest, "jobDefinitionName is required", http.StatusBadRequest)

		return
	}

	if req.Type == "" {
		writeError(w, errInvalidRequest, "type is required", http.StatusBadRequest)

		return
	}

	jd, err := s.storage.RegisterJobDefinition(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, RegisterJobDefinitionOutput{
		JobDefinitionARN:  jd.JobDefinitionARN,
		JobDefinitionName: jd.JobDefinitionName,
		Revision:          jd.Revision,
	})
}

// SubmitJob handles the SubmitJob operation.
func (s *Service) SubmitJob(w http.ResponseWriter, r *http.Request) {
	var req SubmitJobInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.JobName == "" {
		writeError(w, errInvalidRequest, "jobName is required", http.StatusBadRequest)

		return
	}

	if req.JobDefinition == "" {
		writeError(w, errInvalidRequest, "jobDefinition is required", http.StatusBadRequest)

		return
	}

	if req.JobQueue == "" {
		writeError(w, errInvalidRequest, "jobQueue is required", http.StatusBadRequest)

		return
	}

	job, err := s.storage.SubmitJob(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, SubmitJobOutput{
		JobARN:  job.JobARN,
		JobID:   job.JobID,
		JobName: job.JobName,
	})
}

// DescribeJobs handles the DescribeJobs operation.
func (s *Service) DescribeJobs(w http.ResponseWriter, r *http.Request) {
	var req DescribeJobsInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if len(req.Jobs) == 0 {
		writeError(w, errInvalidRequest, "jobs is required", http.StatusBadRequest)

		return
	}

	jobs, err := s.storage.DescribeJobs(r.Context(), req.Jobs)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, DescribeJobsOutput{
		Jobs: jobs,
	})
}

// TerminateJob handles the TerminateJob operation.
func (s *Service) TerminateJob(w http.ResponseWriter, r *http.Request) {
	var req TerminateJobInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.JobID == "" {
		writeError(w, errInvalidRequest, "jobId is required", http.StatusBadRequest)

		return
	}

	if req.Reason == "" {
		writeError(w, errInvalidRequest, "reason is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.TerminateJob(r.Context(), req.JobID, req.Reason); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// Helper functions.

// extractResourceName extracts resource name from ARN or returns as-is.
func extractResourceName(arnOrName string) string {
	if strings.HasPrefix(arnOrName, "arn:") {
		parts := strings.Split(arnOrName, "/")
		if len(parts) > 1 {
			return parts[len(parts)-1]
		}
	}

	return arnOrName
}

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)

	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Type:    code,
		Message: message,
	})
}

// handleStorageError handles storage errors and writes appropriate response.
func handleStorageError(w http.ResponseWriter, err error) {
	var batchErr *Error
	if errors.As(err, &batchErr) {
		writeError(w, batchErr.Code, batchErr.Message, http.StatusBadRequest)

		return
	}

	writeError(w, "ServerException", "Internal server error", http.StatusInternalServerError)
}
