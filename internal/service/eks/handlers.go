package eks

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// Error codes.
const (
	errInvalidParameter    = "InvalidParameterException"
	errResourceNotFound    = "ResourceNotFoundException"
	errResourceInUse       = "ResourceInUseException"
	errInternalServerError = "InternalServerError"
)

// Path components.
const (
	pathPrefixEKS      = "eks"
	pathPrefixClusters = "clusters"
	pathPrefixNodeGrps = "node-groups"
)

// CreateCluster handles the CreateCluster operation.
func (s *Service) CreateCluster(w http.ResponseWriter, r *http.Request) {
	var req CreateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidParameter, "Invalid request body")

		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errInvalidParameter, "Cluster name is required")

		return
	}

	if req.RoleArn == "" {
		writeError(w, http.StatusBadRequest, errInvalidParameter, "Role ARN is required")

		return
	}

	cluster, err := s.storage.CreateCluster(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &CreateClusterResponse{Cluster: cluster})
}

// DeleteCluster handles the DeleteCluster operation.
func (s *Service) DeleteCluster(w http.ResponseWriter, r *http.Request) {
	name := extractClusterName(r.URL.Path)
	if name == "" {
		writeError(w, http.StatusBadRequest, errInvalidParameter, "Cluster name is required")

		return
	}

	cluster, err := s.storage.DeleteCluster(r.Context(), name)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &DeleteClusterResponse{Cluster: cluster})
}

// DescribeCluster handles the DescribeCluster operation.
func (s *Service) DescribeCluster(w http.ResponseWriter, r *http.Request) {
	name := extractClusterName(r.URL.Path)
	if name == "" {
		writeError(w, http.StatusBadRequest, errInvalidParameter, "Cluster name is required")

		return
	}

	cluster, err := s.storage.DescribeCluster(r.Context(), name)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &DescribeClusterResponse{Cluster: cluster})
}

// ListClusters handles the ListClusters operation.
func (s *Service) ListClusters(w http.ResponseWriter, r *http.Request) {
	const maxResultsLimit = 100

	maxResults := maxResultsLimit

	if v := r.URL.Query().Get("maxResults"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxResults = n
			// Enforce upper limit per AWS API specification.
			if maxResults > maxResultsLimit {
				maxResults = maxResultsLimit
			}
		}
	}

	nextToken := r.URL.Query().Get("nextToken")

	clusters, next, err := s.storage.ListClusters(r.Context(), maxResults, nextToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errInternalServerError, err.Error())

		return
	}

	resp := &ListClustersResponse{
		Clusters: clusters,
	}
	if next != "" {
		resp.NextToken = &next
	}

	writeJSON(w, resp)
}

// CreateNodegroup handles the CreateNodegroup operation.
func (s *Service) CreateNodegroup(w http.ResponseWriter, r *http.Request) {
	clusterName := extractClusterName(r.URL.Path)
	if clusterName == "" {
		writeError(w, http.StatusBadRequest, errInvalidParameter, "Cluster name is required")

		return
	}

	var req CreateNodegroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidParameter, "Invalid request body")

		return
	}

	req.ClusterName = clusterName

	if req.NodegroupName == "" {
		writeError(w, http.StatusBadRequest, errInvalidParameter, "Nodegroup name is required")

		return
	}

	if req.NodeRole == "" {
		writeError(w, http.StatusBadRequest, errInvalidParameter, "Node role is required")

		return
	}

	if len(req.Subnets) == 0 {
		writeError(w, http.StatusBadRequest, errInvalidParameter, "Subnets are required")

		return
	}

	nodegroup, err := s.storage.CreateNodegroup(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &CreateNodegroupResponse{Nodegroup: nodegroup})
}

// DeleteNodegroup handles the DeleteNodegroup operation.
func (s *Service) DeleteNodegroup(w http.ResponseWriter, r *http.Request) {
	clusterName, nodegroupName := extractClusterAndNodegroupName(r.URL.Path)
	if clusterName == "" || nodegroupName == "" {
		writeError(w, http.StatusBadRequest, errInvalidParameter, "Cluster name and nodegroup name are required")

		return
	}

	nodegroup, err := s.storage.DeleteNodegroup(r.Context(), clusterName, nodegroupName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &DeleteNodegroupResponse{Nodegroup: nodegroup})
}

// DescribeNodegroup handles the DescribeNodegroup operation.
func (s *Service) DescribeNodegroup(w http.ResponseWriter, r *http.Request) {
	clusterName, nodegroupName := extractClusterAndNodegroupName(r.URL.Path)
	if clusterName == "" || nodegroupName == "" {
		writeError(w, http.StatusBadRequest, errInvalidParameter, "Cluster name and nodegroup name are required")

		return
	}

	nodegroup, err := s.storage.DescribeNodegroup(r.Context(), clusterName, nodegroupName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &DescribeNodegroupResponse{Nodegroup: nodegroup})
}

// ListNodegroups handles the ListNodegroups operation.
func (s *Service) ListNodegroups(w http.ResponseWriter, r *http.Request) {
	clusterName := extractClusterName(r.URL.Path)
	if clusterName == "" {
		writeError(w, http.StatusBadRequest, errInvalidParameter, "Cluster name is required")

		return
	}

	const maxResultsLimit = 100

	maxResults := maxResultsLimit

	if v := r.URL.Query().Get("maxResults"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxResults = n
			// Enforce upper limit per AWS API specification.
			if maxResults > maxResultsLimit {
				maxResults = maxResultsLimit
			}
		}
	}

	nextToken := r.URL.Query().Get("nextToken")

	nodegroups, next, err := s.storage.ListNodegroups(r.Context(), clusterName, maxResults, nextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &ListNodegroupsResponse{
		Nodegroups: nodegroups,
	}
	if next != "" {
		resp.NextToken = &next
	}

	writeJSON(w, resp)
}

// extractClusterName extracts the cluster name from the URL path.
// Expected paths: /eks/clusters/{name} or /eks/clusters/{name}/node-groups...
func extractClusterName(path string) string {
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	// Expected: eks/clusters/{name} or eks/clusters/{name}/node-groups...
	if len(parts) >= 3 && parts[0] == pathPrefixEKS && parts[1] == pathPrefixClusters {
		return parts[2]
	}

	return ""
}

// extractClusterAndNodegroupName extracts both cluster and nodegroup names from the URL path.
// Expected path: /eks/clusters/{clusterName}/node-groups/{nodegroupName}.
func extractClusterAndNodegroupName(path string) (string, string) {
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	// Expected: eks/clusters/{clusterName}/node-groups/{nodegroupName}
	if len(parts) >= 5 && parts[0] == pathPrefixEKS && parts[1] == pathPrefixClusters && parts[3] == pathPrefixNodeGrps {
		return parts[2], parts[4]
	}

	return "", ""
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	errResp := struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	}{
		Message: message,
		Code:    code,
	}

	_ = json.NewEncoder(w).Encode(errResp)
}

// handleError handles EKS errors and writes the appropriate response.
func handleError(w http.ResponseWriter, err error) {
	var eksErr *Error
	if errors.As(err, &eksErr) {
		status := http.StatusBadRequest

		switch eksErr.Code {
		case errResourceNotFound:
			status = http.StatusNotFound
		case errResourceInUse:
			status = http.StatusConflict
		}

		writeError(w, status, eksErr.Code, eksErr.Message)

		return
	}

	writeError(w, http.StatusInternalServerError, errInternalServerError, "Internal server error")
}
