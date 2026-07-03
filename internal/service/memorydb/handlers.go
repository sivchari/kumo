// Package memorydb provides AWS MemoryDB service emulation.
package memorydb

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

const errInvalidParam = "InvalidParameterValueException"

// handlerFunc is a type alias for handler functions.
type handlerFunc func(http.ResponseWriter, *http.Request)

// getActionHandlers returns a map of action names to handler functions.
func (s *Service) getActionHandlers() map[string]handlerFunc {
	return map[string]handlerFunc{
		"CreateCluster":    s.CreateCluster,
		"DescribeClusters": s.DescribeClusters,
		"UpdateCluster":    s.UpdateCluster,
		"DeleteCluster":    s.DeleteCluster,
		"CreateUser":       s.CreateUser,
		"DescribeUsers":    s.DescribeUsers,
		"DeleteUser":       s.DeleteUser,
		"CreateACL":        s.CreateACL,
		"DescribeACLs":     s.DescribeACLs,
		"DeleteACL":        s.DeleteACL,
	}
}

// DispatchAction dispatches the request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "AmazonMemoryDB.")

	handlers := s.getActionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeError(w, "InvalidAction", "The action "+action+" is not valid for this endpoint.", http.StatusBadRequest)
}

// CreateCluster handles the CreateCluster API.
func (s *Service) CreateCluster(w http.ResponseWriter, r *http.Request) {
	var req CreateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParam, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ClusterName == "" {
		writeError(w, errInvalidParam, "ClusterName is required", http.StatusBadRequest)

		return
	}

	if req.NodeType == "" {
		writeError(w, errInvalidParam, "NodeType is required", http.StatusBadRequest)

		return
	}

	if req.ACLName == "" {
		writeError(w, errInvalidParam, "ACLName is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.CreateCluster(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeResponse(w, &CreateClusterResponse{Cluster: cluster})
}

// DescribeClusters handles the DescribeClusters API.
func (s *Service) DescribeClusters(w http.ResponseWriter, r *http.Request) {
	var req DescribeClustersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParam, "Invalid request body", http.StatusBadRequest)

		return
	}

	clusters, err := s.storage.DescribeClusters(r.Context(), req.ClusterName)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeResponse(w, &DescribeClustersResponse{Clusters: clusters})
}

// UpdateCluster handles the UpdateCluster API.
func (s *Service) UpdateCluster(w http.ResponseWriter, r *http.Request) {
	var req UpdateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParam, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ClusterName == "" {
		writeError(w, errInvalidParam, "ClusterName is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.UpdateCluster(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeResponse(w, &UpdateClusterResponse{Cluster: cluster})
}

// DeleteCluster handles the DeleteCluster API.
func (s *Service) DeleteCluster(w http.ResponseWriter, r *http.Request) {
	var req DeleteClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParam, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ClusterName == "" {
		writeError(w, errInvalidParam, "ClusterName is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.DeleteCluster(r.Context(), req.ClusterName)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeResponse(w, &DeleteClusterResponse{Cluster: cluster})
}

// CreateUser handles the CreateUser API.
func (s *Service) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParam, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.UserName == "" {
		writeError(w, errInvalidParam, "UserName is required", http.StatusBadRequest)

		return
	}

	if req.AccessString == "" {
		writeError(w, errInvalidParam, "AccessString is required", http.StatusBadRequest)

		return
	}

	user, err := s.storage.CreateUser(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeResponse(w, &CreateUserResponse{User: user})
}

// DescribeUsers handles the DescribeUsers API.
func (s *Service) DescribeUsers(w http.ResponseWriter, r *http.Request) {
	var req DescribeUsersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParam, "Invalid request body", http.StatusBadRequest)

		return
	}

	users, err := s.storage.DescribeUsers(r.Context(), req.UserName)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeResponse(w, &DescribeUsersResponse{Users: users})
}

// DeleteUser handles the DeleteUser API.
func (s *Service) DeleteUser(w http.ResponseWriter, r *http.Request) {
	var req DeleteUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParam, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.UserName == "" {
		writeError(w, errInvalidParam, "UserName is required", http.StatusBadRequest)

		return
	}

	user, err := s.storage.DeleteUser(r.Context(), req.UserName)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeResponse(w, &DeleteUserResponse{User: user})
}

// CreateACL handles the CreateACL API.
func (s *Service) CreateACL(w http.ResponseWriter, r *http.Request) {
	var req CreateACLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParam, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ACLName == "" {
		writeError(w, errInvalidParam, "ACLName is required", http.StatusBadRequest)

		return
	}

	acl, err := s.storage.CreateACL(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeResponse(w, &CreateACLResponse{ACL: acl})
}

// DescribeACLs handles the DescribeACLs API.
func (s *Service) DescribeACLs(w http.ResponseWriter, r *http.Request) {
	var req DescribeACLsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParam, "Invalid request body", http.StatusBadRequest)

		return
	}

	acls, err := s.storage.DescribeACLs(r.Context(), req.ACLName)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeResponse(w, &DescribeACLsResponse{ACLs: acls})
}

// DeleteACL handles the DeleteACL API.
func (s *Service) DeleteACL(w http.ResponseWriter, r *http.Request) {
	var req DeleteACLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParam, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ACLName == "" {
		writeError(w, errInvalidParam, "ACLName is required", http.StatusBadRequest)

		return
	}

	acl, err := s.storage.DeleteACL(r.Context(), req.ACLName)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeResponse(w, &DeleteACLResponse{ACL: acl})
}

// Helper functions.

func writeResponse(w http.ResponseWriter, resp any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeAmzJSON11, code, message, status)
}

func handleServiceError(w http.ResponseWriter, err error) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		writeError(w, svcErr.Code, svcErr.Message, http.StatusBadRequest)

		return
	}

	writeError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)
}
