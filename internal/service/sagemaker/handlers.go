package sagemaker

import (
	"errors"
	"net/http"
	"strings"

	"github.com/sivchari/kumo/internal/service"
)

// Error codes for SageMaker handlers.
const (
	errInvalidAction = "UnknownOperationException"
)

// DispatchAction routes the request to the appropriate handler based on X-Amz-Target header.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "SageMaker.")

	switch action {
	case "CreateNotebookInstance":
		s.CreateNotebookInstance(w, r)
	case "DeleteNotebookInstance":
		s.DeleteNotebookInstance(w, r)
	case "DescribeNotebookInstance":
		s.DescribeNotebookInstance(w, r)
	case "ListNotebookInstances":
		s.ListNotebookInstances(w, r)
	case "CreateTrainingJob":
		s.CreateTrainingJob(w, r)
	case "DescribeTrainingJob":
		s.DescribeTrainingJob(w, r)
	case "CreateModel":
		s.CreateModel(w, r)
	case "DeleteModel":
		s.DeleteModel(w, r)
	case "CreateEndpoint":
		s.CreateEndpoint(w, r)
	case "DeleteEndpoint":
		s.DeleteEndpoint(w, r)
	case "DescribeEndpoint":
		s.DescribeEndpoint(w, r)
	default:
		writeError(w, errInvalidAction, "Unknown operation: "+action, http.StatusBadRequest)
	}
}

// CreateNotebookInstance handles the CreateNotebookInstance action.
func (s *Service) CreateNotebookInstance(w http.ResponseWriter, r *http.Request) {
	var req CreateNotebookInstanceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errValidationException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.NotebookInstanceName == "" {
		writeError(w, errValidationException, "NotebookInstanceName is required", http.StatusBadRequest)

		return
	}

	if req.InstanceType == "" {
		writeError(w, errValidationException, "InstanceType is required", http.StatusBadRequest)

		return
	}

	if req.RoleArn == "" {
		writeError(w, errValidationException, "RoleArn is required", http.StatusBadRequest)

		return
	}

	instance, err := s.storage.CreateNotebookInstance(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSONResponse(w, CreateNotebookInstanceResponse{
		NotebookInstanceArn: instance.NotebookInstanceArn,
	})
}

// DeleteNotebookInstance handles the DeleteNotebookInstance action.
func (s *Service) DeleteNotebookInstance(w http.ResponseWriter, r *http.Request) {
	var req DeleteNotebookInstanceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errValidationException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.NotebookInstanceName == "" {
		writeError(w, errValidationException, "NotebookInstanceName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteNotebookInstance(r.Context(), req.NotebookInstanceName); err != nil {
		handleError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// DescribeNotebookInstance handles the DescribeNotebookInstance action.
func (s *Service) DescribeNotebookInstance(w http.ResponseWriter, r *http.Request) {
	var req DescribeNotebookInstanceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errValidationException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.NotebookInstanceName == "" {
		writeError(w, errValidationException, "NotebookInstanceName is required", http.StatusBadRequest)

		return
	}

	instance, err := s.storage.DescribeNotebookInstance(r.Context(), req.NotebookInstanceName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSONResponse(w, DescribeNotebookInstanceResponse{
		NotebookInstanceName:   instance.NotebookInstanceName,
		NotebookInstanceArn:    instance.NotebookInstanceArn,
		NotebookInstanceStatus: instance.NotebookInstanceStatus,
		URL:                    instance.URL,
		InstanceType:           instance.InstanceType,
		RoleArn:                instance.RoleArn,
		KmsKeyID:               instance.KmsKeyID,
		SubnetID:               instance.SubnetID,
		SecurityGroups:         instance.SecurityGroups,
		DirectInternetAccess:   instance.DirectInternetAccess,
		VolumeSizeInGB:         instance.VolumeSizeInGB,
		RootAccess:             instance.RootAccess,
		CreationTime:           float64(instance.CreationTime.Unix()),
		LastModifiedTime:       float64(instance.LastModifiedTime.Unix()),
	})
}

// ListNotebookInstances handles the ListNotebookInstances action.
func (s *Service) ListNotebookInstances(w http.ResponseWriter, r *http.Request) {
	var req ListNotebookInstancesRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errValidationException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	instances, nextToken, err := s.storage.ListNotebookInstances(r.Context(), req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	summaries := make([]NotebookInstanceSummary, 0, len(instances))

	for _, instance := range instances {
		summaries = append(summaries, NotebookInstanceSummary{
			NotebookInstanceName:   instance.NotebookInstanceName,
			NotebookInstanceArn:    instance.NotebookInstanceArn,
			NotebookInstanceStatus: instance.NotebookInstanceStatus,
			URL:                    instance.URL,
			InstanceType:           instance.InstanceType,
			CreationTime:           float64(instance.CreationTime.Unix()),
			LastModifiedTime:       float64(instance.LastModifiedTime.Unix()),
		})
	}

	writeJSONResponse(w, ListNotebookInstancesResponse{
		NotebookInstances: summaries,
		NextToken:         nextToken,
	})
}

// CreateTrainingJob handles the CreateTrainingJob action.
func (s *Service) CreateTrainingJob(w http.ResponseWriter, r *http.Request) {
	var req CreateTrainingJobRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errValidationException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TrainingJobName == "" {
		writeError(w, errValidationException, "TrainingJobName is required", http.StatusBadRequest)

		return
	}

	if req.RoleArn == "" {
		writeError(w, errValidationException, "RoleArn is required", http.StatusBadRequest)

		return
	}

	job, err := s.storage.CreateTrainingJob(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSONResponse(w, CreateTrainingJobResponse{
		TrainingJobArn: job.TrainingJobArn,
	})
}

// DescribeTrainingJob handles the DescribeTrainingJob action.
func (s *Service) DescribeTrainingJob(w http.ResponseWriter, r *http.Request) {
	var req DescribeTrainingJobRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errValidationException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TrainingJobName == "" {
		writeError(w, errValidationException, "TrainingJobName is required", http.StatusBadRequest)

		return
	}

	job, err := s.storage.DescribeTrainingJob(r.Context(), req.TrainingJobName)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := DescribeTrainingJobResponse{
		TrainingJobName:   job.TrainingJobName,
		TrainingJobArn:    job.TrainingJobArn,
		TrainingJobStatus: job.TrainingJobStatus,
		SecondaryStatus:   job.SecondaryStatus,
		AlgorithmSpec:     job.AlgorithmSpec,
		RoleArn:           job.RoleArn,
		InputDataConfig:   job.InputDataConfig,
		OutputDataConfig:  job.OutputDataConfig,
		ResourceConfig:    job.ResourceConfig,
		StoppingCondition: job.StoppingCondition,
		CreationTime:      float64(job.CreationTime.Unix()),
		FailureReason:     job.FailureReason,
	}

	if job.TrainingStartTime != nil {
		startTime := float64(job.TrainingStartTime.Unix())
		resp.TrainingStartTime = &startTime
	}

	if job.TrainingEndTime != nil {
		endTime := float64(job.TrainingEndTime.Unix())
		resp.TrainingEndTime = &endTime
	}

	writeJSONResponse(w, resp)
}

// CreateModel handles the CreateModel action.
func (s *Service) CreateModel(w http.ResponseWriter, r *http.Request) {
	var req CreateModelRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errValidationException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ModelName == "" {
		writeError(w, errValidationException, "ModelName is required", http.StatusBadRequest)

		return
	}

	if req.ExecutionRoleArn == "" {
		writeError(w, errValidationException, "ExecutionRoleArn is required", http.StatusBadRequest)

		return
	}

	model, err := s.storage.CreateModel(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSONResponse(w, CreateModelResponse{
		ModelArn: model.ModelArn,
	})
}

// DeleteModel handles the DeleteModel action.
func (s *Service) DeleteModel(w http.ResponseWriter, r *http.Request) {
	var req DeleteModelRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errValidationException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ModelName == "" {
		writeError(w, errValidationException, "ModelName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteModel(r.Context(), req.ModelName); err != nil {
		handleError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// CreateEndpoint handles the CreateEndpoint action.
func (s *Service) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	var req CreateEndpointRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errValidationException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.EndpointName == "" {
		writeError(w, errValidationException, "EndpointName is required", http.StatusBadRequest)

		return
	}

	if req.EndpointConfigName == "" {
		writeError(w, errValidationException, "EndpointConfigName is required", http.StatusBadRequest)

		return
	}

	endpoint, err := s.storage.CreateEndpoint(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSONResponse(w, CreateEndpointResponse{
		EndpointArn: endpoint.EndpointArn,
	})
}

// DeleteEndpoint handles the DeleteEndpoint action.
func (s *Service) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	var req DeleteEndpointRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errValidationException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.EndpointName == "" {
		writeError(w, errValidationException, "EndpointName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteEndpoint(r.Context(), req.EndpointName); err != nil {
		handleError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// DescribeEndpoint handles the DescribeEndpoint action.
func (s *Service) DescribeEndpoint(w http.ResponseWriter, r *http.Request) {
	var req DescribeEndpointRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errValidationException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.EndpointName == "" {
		writeError(w, errValidationException, "EndpointName is required", http.StatusBadRequest)

		return
	}

	endpoint, err := s.storage.DescribeEndpoint(r.Context(), req.EndpointName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSONResponse(w, DescribeEndpointResponse{
		EndpointName:       endpoint.EndpointName,
		EndpointArn:        endpoint.EndpointArn,
		EndpointConfigName: endpoint.EndpointConfigName,
		EndpointStatus:     endpoint.EndpointStatus,
		CreationTime:       float64(endpoint.CreationTime.Unix()),
		LastModifiedTime:   float64(endpoint.LastModifiedTime.Unix()),
		FailureReason:      endpoint.FailureReason,
	})
}

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	service.WriteJSONResponse(w, service.ContentTypeAmzJSON11, v)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeAmzJSON11, code, message, status)
}

// handleError handles service errors.
func handleError(w http.ResponseWriter, err error) {
	var sErr *Error
	if errors.As(err, &sErr) {
		status := http.StatusBadRequest

		switch sErr.Code {
		case errResourceNotFound:
			status = http.StatusNotFound
		case errResourceInUse:
			status = http.StatusConflict
		case errInternalFailure:
			status = http.StatusInternalServerError
		}

		writeError(w, sErr.Code, sErr.Message, status)

		return
	}

	writeError(w, errInternalFailure, "Internal server error", http.StatusInternalServerError)
}
