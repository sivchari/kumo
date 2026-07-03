// Package ecs implements the Amazon ECS service emulator.
package ecs

import (
	"errors"
	"net/http"
	"strings"

	"github.com/sivchari/kumo/internal/service"
)

// DispatchAction routes the request to the appropriate handler based on X-Amz-Target header.
// This method implements the JSONProtocolService interface.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "AmazonEC2ContainerServiceV20141113.")

	switch action {
	case "CreateCluster":
		s.CreateCluster(w, r)
	case "DeleteCluster":
		s.DeleteCluster(w, r)
	case "DescribeClusters":
		s.DescribeClusters(w, r)
	case "ListClusters":
		s.ListClusters(w, r)
	case "RegisterTaskDefinition":
		s.RegisterTaskDefinition(w, r)
	case "DeregisterTaskDefinition":
		s.DeregisterTaskDefinition(w, r)
	case "RunTask":
		s.RunTask(w, r)
	case "StopTask":
		s.StopTask(w, r)
	case "DescribeTasks":
		s.DescribeTasks(w, r)
	case "CreateService":
		s.CreateService(w, r)
	case "DeleteService":
		s.DeleteService(w, r)
	case "UpdateService":
		s.UpdateService(w, r)
	default:
		writeECSError(w, "UnknownOperationException", "The action "+action+" is not valid", http.StatusBadRequest)
	}
}

// CreateCluster handles the CreateCluster action.
func (s *Service) CreateCluster(w http.ResponseWriter, r *http.Request) {
	var req CreateClusterRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeECSError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.CreateCluster(r.Context(), &req)
	if err != nil {
		var ecsErr *Error
		if errors.As(err, &ecsErr) {
			writeECSError(w, ecsErr.Code, ecsErr.Message, http.StatusBadRequest)

			return
		}

		writeECSError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, CreateClusterResponse{
		Cluster: cluster,
	})
}

// DeleteCluster handles the DeleteCluster action.
func (s *Service) DeleteCluster(w http.ResponseWriter, r *http.Request) {
	var req DeleteClusterRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeECSError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Cluster == "" {
		writeECSError(w, "InvalidParameterException", "Cluster is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.DeleteCluster(r.Context(), req.Cluster)
	if err != nil {
		var ecsErr *Error
		if errors.As(err, &ecsErr) {
			writeECSError(w, ecsErr.Code, ecsErr.Message, http.StatusBadRequest)

			return
		}

		writeECSError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, DeleteClusterResponse{
		Cluster: cluster,
	})
}

// DescribeClusters handles the DescribeClusters action.
func (s *Service) DescribeClusters(w http.ResponseWriter, r *http.Request) {
	var req DescribeClustersRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeECSError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	clusters, failures, err := s.storage.DescribeClusters(r.Context(), req.Clusters)
	if err != nil {
		writeECSError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, DescribeClustersResponse{
		Clusters: clusters,
		Failures: failures,
	})
}

// ListClusters handles the ListClusters action.
func (s *Service) ListClusters(w http.ResponseWriter, r *http.Request) {
	var req ListClustersRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeECSError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	// Validate maxResults: must be between 1 and 100 if specified
	if req.MaxResults != 0 && (req.MaxResults < 1 || req.MaxResults > 100) {
		writeECSError(w, "InvalidParameterException", "maxResults must be between 1 and 100", http.StatusBadRequest)

		return
	}

	clusterArns, nextToken, err := s.storage.ListClusters(r.Context(), req.MaxResults, req.NextToken)
	if err != nil {
		writeECSError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, ListClustersResponse{
		ClusterArns: clusterArns,
		NextToken:   nextToken,
	})
}

// RegisterTaskDefinition handles the RegisterTaskDefinition action.
func (s *Service) RegisterTaskDefinition(w http.ResponseWriter, r *http.Request) {
	var req RegisterTaskDefinitionRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeECSError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Family == "" {
		writeECSError(w, "InvalidParameterException", "Family is required", http.StatusBadRequest)

		return
	}

	if len(req.ContainerDefinitions) == 0 {
		writeECSError(w, "InvalidParameterException", "ContainerDefinitions are required", http.StatusBadRequest)

		return
	}

	taskDef, err := s.storage.RegisterTaskDefinition(r.Context(), &req)
	if err != nil {
		var ecsErr *Error
		if errors.As(err, &ecsErr) {
			writeECSError(w, ecsErr.Code, ecsErr.Message, http.StatusBadRequest)

			return
		}

		writeECSError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, RegisterTaskDefinitionResponse{
		TaskDefinition: taskDef,
		Tags:           taskDef.Tags,
	})
}

// DeregisterTaskDefinition handles the DeregisterTaskDefinition action.
func (s *Service) DeregisterTaskDefinition(w http.ResponseWriter, r *http.Request) {
	var req DeregisterTaskDefinitionRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeECSError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TaskDefinition == "" {
		writeECSError(w, "InvalidParameterException", "TaskDefinition is required", http.StatusBadRequest)

		return
	}

	taskDef, err := s.storage.DeregisterTaskDefinition(r.Context(), req.TaskDefinition)
	if err != nil {
		var ecsErr *Error
		if errors.As(err, &ecsErr) {
			writeECSError(w, ecsErr.Code, ecsErr.Message, http.StatusBadRequest)

			return
		}

		writeECSError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, DeregisterTaskDefinitionResponse{
		TaskDefinition: taskDef,
	})
}

// RunTask handles the RunTask action.
func (s *Service) RunTask(w http.ResponseWriter, r *http.Request) {
	var req RunTaskRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeECSError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TaskDefinition == "" {
		writeECSError(w, "InvalidParameterException", "TaskDefinition is required", http.StatusBadRequest)

		return
	}

	tasks, failures, err := s.storage.RunTask(r.Context(), &req)
	if err != nil {
		var ecsErr *Error
		if errors.As(err, &ecsErr) {
			writeECSError(w, ecsErr.Code, ecsErr.Message, http.StatusBadRequest)

			return
		}

		writeECSError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, RunTaskResponse{
		Tasks:    tasks,
		Failures: failures,
	})
}

// StopTask handles the StopTask action.
func (s *Service) StopTask(w http.ResponseWriter, r *http.Request) {
	var req StopTaskRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeECSError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Task == "" {
		writeECSError(w, "InvalidParameterException", "Task is required", http.StatusBadRequest)

		return
	}

	task, err := s.storage.StopTask(r.Context(), req.Cluster, req.Task, req.Reason)
	if err != nil {
		var ecsErr *Error
		if errors.As(err, &ecsErr) {
			writeECSError(w, ecsErr.Code, ecsErr.Message, http.StatusBadRequest)

			return
		}

		writeECSError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, StopTaskResponse{
		Task: task,
	})
}

// DescribeTasks handles the DescribeTasks action.
func (s *Service) DescribeTasks(w http.ResponseWriter, r *http.Request) {
	var req DescribeTasksRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeECSError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.Tasks) == 0 {
		writeECSError(w, "InvalidParameterException", "Tasks are required", http.StatusBadRequest)

		return
	}

	tasks, failures, err := s.storage.DescribeTasks(r.Context(), req.Cluster, req.Tasks)
	if err != nil {
		writeECSError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, DescribeTasksResponse{
		Tasks:    tasks,
		Failures: failures,
	})
}

// CreateService handles the CreateService action.
func (s *Service) CreateService(w http.ResponseWriter, r *http.Request) {
	var req CreateServiceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeECSError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ServiceName == "" {
		writeECSError(w, "InvalidParameterException", "ServiceName is required", http.StatusBadRequest)

		return
	}

	if req.TaskDefinition == "" {
		writeECSError(w, "InvalidParameterException", "TaskDefinition is required", http.StatusBadRequest)

		return
	}

	svc, err := s.storage.CreateService(r.Context(), &req)
	if err != nil {
		var ecsErr *Error
		if errors.As(err, &ecsErr) {
			writeECSError(w, ecsErr.Code, ecsErr.Message, http.StatusBadRequest)

			return
		}

		writeECSError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, CreateServiceResponse{
		Service: svc,
	})
}

// DeleteService handles the DeleteService action.
func (s *Service) DeleteService(w http.ResponseWriter, r *http.Request) {
	var req DeleteServiceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeECSError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Service == "" {
		writeECSError(w, "InvalidParameterException", "Service is required", http.StatusBadRequest)

		return
	}

	svc, err := s.storage.DeleteService(r.Context(), req.Cluster, req.Service, req.Force)
	if err != nil {
		var ecsErr *Error
		if errors.As(err, &ecsErr) {
			writeECSError(w, ecsErr.Code, ecsErr.Message, http.StatusBadRequest)

			return
		}

		writeECSError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, DeleteServiceResponse{
		Service: svc,
	})
}

// UpdateService handles the UpdateService action.
func (s *Service) UpdateService(w http.ResponseWriter, r *http.Request) {
	var req UpdateServiceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeECSError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Service == "" {
		writeECSError(w, "InvalidParameterException", "Service is required", http.StatusBadRequest)

		return
	}

	svc, err := s.storage.UpdateService(r.Context(), &req)
	if err != nil {
		var ecsErr *Error
		if errors.As(err, &ecsErr) {
			writeECSError(w, ecsErr.Code, ecsErr.Message, http.StatusBadRequest)

			return
		}

		writeECSError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, UpdateServiceResponse{
		Service: svc,
	})
}

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	service.WriteJSONResponse(w, service.ContentTypeAmzJSON11, v)
}

// writeECSError writes an ECS error response in JSON format.
func writeECSError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeAmzJSON11, code, message, status)
}
