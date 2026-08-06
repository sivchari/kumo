package kafka

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// CreateCluster handles the CreateCluster operation.
func (s *Service) CreateCluster(w http.ResponseWriter, r *http.Request) {
	var req CreateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ClusterName == "" {
		writeError(w, errBadRequest, "Cluster name is required", http.StatusBadRequest)

		return
	}

	if req.BrokerNodeGroupInfo == nil {
		writeError(w, errBadRequest, "Broker node group info is required", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.CreateCluster(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, resp)
}

// ListClusters handles the ListClusters operation.
func (s *Service) ListClusters(w http.ResponseWriter, r *http.Request) {
	const maxResultsLimit = 100

	maxResults := maxResultsLimit

	if v := r.URL.Query().Get("maxResults"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxResults = min(n, maxResultsLimit)
		}
	}

	nextToken := r.URL.Query().Get("nextToken")

	clusters, next, err := s.storage.ListClusters(r.Context(), maxResults, nextToken)
	if err != nil {
		writeError(w, errInternalError, err.Error(), http.StatusInternalServerError)

		return
	}

	resp := &ListClustersResponse{
		ClusterInfoList: clusters,
	}
	if next != "" {
		resp.NextToken = next
	}

	writeJSON(w, resp)
}

// DescribeCluster handles the DescribeCluster operation.
func (s *Service) DescribeCluster(w http.ResponseWriter, r *http.Request) {
	clusterArn := extractClusterArn(r.URL.Path)
	if clusterArn == "" {
		writeError(w, errBadRequest, "Cluster ARN is required", http.StatusBadRequest)

		return
	}

	cluster, err := s.storage.DescribeCluster(r.Context(), clusterArn)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &DescribeClusterResponse{ClusterInfo: cluster})
}

// DeleteCluster handles the DeleteCluster operation.
func (s *Service) DeleteCluster(w http.ResponseWriter, r *http.Request) {
	clusterArn := extractClusterArn(r.URL.Path)
	if clusterArn == "" {
		writeError(w, errBadRequest, "Cluster ARN is required", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.DeleteCluster(r.Context(), clusterArn)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, resp)
}

// GetBootstrapBrokers handles the GetBootstrapBrokers operation.
func (s *Service) GetBootstrapBrokers(w http.ResponseWriter, r *http.Request) {
	clusterArn := extractClusterArn(r.URL.Path)
	if clusterArn == "" {
		writeError(w, errBadRequest, "Cluster ARN is required", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.GetBootstrapBrokers(r.Context(), clusterArn)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, resp)
}

// UpdateClusterConfiguration handles the UpdateClusterConfiguration operation.
func (s *Service) UpdateClusterConfiguration(w http.ResponseWriter, r *http.Request) {
	clusterArn := extractClusterArn(r.URL.Path)
	if clusterArn == "" {
		writeError(w, errBadRequest, "Cluster ARN is required", http.StatusBadRequest)

		return
	}

	var req UpdateClusterConfigurationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.UpdateClusterConfiguration(r.Context(), clusterArn, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, resp)
}

// extractClusterArn extracts the cluster ARN from the URL path.
// The ARN contains slashes (e.g., arn:aws:kafka:us-east-1:123456789012:cluster/name/uuid),
// so we extract everything after /kafka/v1/clusters/ and strip known suffixes.
func extractClusterArn(path string) string {
	const prefix = "/kafka/v1/clusters/"

	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	rest := path[len(prefix):]

	// Strip known path suffixes.
	rest = strings.TrimSuffix(rest, "/bootstrap-brokers")
	rest = strings.TrimSuffix(rest, "/configuration")

	decoded, err := url.PathUnescape(rest)
	if err != nil {
		return ""
	}

	return decoded
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
func writeError(w http.ResponseWriter, code, message string, status int) {
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

// handleError handles MSK errors and writes the appropriate response.
func handleError(w http.ResponseWriter, err error) {
	var mskErr *Error
	if errors.As(err, &mskErr) {
		status := http.StatusBadRequest

		switch mskErr.Code {
		case errNotFound:
			status = http.StatusNotFound
		case errConflict:
			status = http.StatusConflict
		}

		writeError(w, mskErr.Code, mskErr.Message, status)

		return
	}

	writeError(w, errInternalError, "Internal server error", http.StatusInternalServerError)
}
