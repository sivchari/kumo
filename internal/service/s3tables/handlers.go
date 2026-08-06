package s3tables

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Path component constants.
const (
	pathPrefixBuckets    = "buckets"
	pathPrefixNamespaces = "namespaces"
	pathPrefixTables     = "tables"
)

// CreateTableBucket handles the CreateTableBucket operation.
func (s *Service) CreateTableBucket(w http.ResponseWriter, r *http.Request) {
	var req CreateTableBucketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, errBadRequest, "Table bucket name is required", http.StatusBadRequest)

		return
	}

	bucket, err := s.storage.CreateTableBucket(r.Context(), req.Name)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &CreateTableBucketResponse{Arn: bucket.Arn})
}

// DeleteTableBucket handles the DeleteTableBucket operation.
func (s *Service) DeleteTableBucket(w http.ResponseWriter, r *http.Request) {
	arn := extractTableBucketARN(getURLPath(r))
	if arn == "" {
		writeError(w, errBadRequest, "Table bucket ARN is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteTableBucket(r.Context(), arn); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetTableBucket handles the GetTableBucket operation.
func (s *Service) GetTableBucket(w http.ResponseWriter, r *http.Request) {
	arn := extractTableBucketARN(getURLPath(r))
	if arn == "" {
		writeError(w, errBadRequest, "Table bucket ARN is required", http.StatusBadRequest)

		return
	}

	bucket, err := s.storage.GetTableBucket(r.Context(), arn)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &GetTableBucketResponse{
		Arn:       bucket.Arn,
		Name:      bucket.Name,
		OwnerID:   bucket.OwnerID,
		CreatedAt: bucket.CreatedAt,
	})
}

// ListTableBuckets handles the ListTableBuckets operation.
func (s *Service) ListTableBuckets(w http.ResponseWriter, r *http.Request) {
	maxBuckets := defaultMaxItems

	if v := r.URL.Query().Get("maxBuckets"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxBuckets = n
		}
	}

	prefix := r.URL.Query().Get("prefix")
	continuationToken := r.URL.Query().Get("continuationToken")

	buckets, nextToken, err := s.storage.ListTableBuckets(r.Context(), prefix, continuationToken, maxBuckets)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &ListTableBucketsResponse{
		TableBuckets:      buckets,
		ContinuationToken: nextToken,
	})
}

// CreateNamespace handles the CreateNamespace operation.
func (s *Service) CreateNamespace(w http.ResponseWriter, r *http.Request) {
	tableBucketArn := extractTableBucketARNFromNamespacePath(getURLPath(r))
	if tableBucketArn == "" {
		writeError(w, errBadRequest, "Table bucket ARN is required", http.StatusBadRequest)

		return
	}

	var req CreateNamespaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if len(req.Namespace) == 0 {
		writeError(w, errBadRequest, "Namespace is required", http.StatusBadRequest)

		return
	}

	ns, err := s.storage.CreateNamespace(r.Context(), tableBucketArn, req.Namespace)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &CreateNamespaceResponse{
		Namespace:      ns.Namespace,
		TableBucketArn: ns.TableBucketArn,
	})
}

// DeleteNamespace handles the DeleteNamespace operation.
func (s *Service) DeleteNamespace(w http.ResponseWriter, r *http.Request) {
	tableBucketArn, namespace := extractNamespaceParams(getURLPath(r))
	if tableBucketArn == "" || namespace == "" {
		writeError(w, errBadRequest, "Table bucket ARN and namespace are required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteNamespace(r.Context(), tableBucketArn, namespace); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetNamespace handles the GetNamespace operation.
func (s *Service) GetNamespace(w http.ResponseWriter, r *http.Request) {
	tableBucketArn, namespace := extractNamespaceParams(getURLPath(r))
	if tableBucketArn == "" || namespace == "" {
		writeError(w, errBadRequest, "Table bucket ARN and namespace are required", http.StatusBadRequest)

		return
	}

	ns, err := s.storage.GetNamespace(r.Context(), tableBucketArn, namespace)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &GetNamespaceResponse{
		Namespace:      ns.Namespace,
		TableBucketArn: ns.TableBucketArn,
		OwnerID:        ns.OwnerID,
		CreatedAt:      ns.CreatedAt,
		CreatedBy:      ns.CreatedBy,
	})
}

// ListNamespaces handles the ListNamespaces operation.
func (s *Service) ListNamespaces(w http.ResponseWriter, r *http.Request) {
	tableBucketArn := extractTableBucketARNFromNamespacePath(getURLPath(r))
	if tableBucketArn == "" {
		writeError(w, errBadRequest, "Table bucket ARN is required", http.StatusBadRequest)

		return
	}

	maxNamespaces := defaultMaxItems

	if v := r.URL.Query().Get("maxNamespaces"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxNamespaces = n
		}
	}

	prefix := r.URL.Query().Get("prefix")

	namespaces, err := s.storage.ListNamespaces(r.Context(), tableBucketArn, prefix, maxNamespaces)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &ListNamespacesResponse{
		Namespaces: namespaces,
	})
}

// CreateTable handles the CreateTable operation.
func (s *Service) CreateTable(w http.ResponseWriter, r *http.Request) {
	tableBucketArn, namespace := extractTablePathParams(getURLPath(r))
	if tableBucketArn == "" || namespace == "" {
		writeError(w, errBadRequest, "Table bucket ARN and namespace are required", http.StatusBadRequest)

		return
	}

	var req CreateTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, errBadRequest, "Table name is required", http.StatusBadRequest)

		return
	}

	if req.Format == "" {
		writeError(w, errBadRequest, "Table format is required", http.StatusBadRequest)

		return
	}

	table, err := s.storage.CreateTable(r.Context(), tableBucketArn, namespace, req.Name, req.Format)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &CreateTableResponse{
		TableArn:     table.Arn,
		VersionToken: table.VersionToken,
	})
}

// DeleteTable handles the DeleteTable operation.
func (s *Service) DeleteTable(w http.ResponseWriter, r *http.Request) {
	tableBucketArn, namespace, tableName := extractFullTableParams(getURLPath(r))
	if tableBucketArn == "" || namespace == "" || tableName == "" {
		writeError(w, errBadRequest, "Table bucket ARN, namespace, and table name are required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteTable(r.Context(), tableBucketArn, namespace, tableName); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetTable handles the GetTable operation.
// SDK sends: GET /get-table?tableBucketARN=...&namespace=...&name=...
func (s *Service) GetTable(w http.ResponseWriter, r *http.Request) {
	tableBucketArn := r.URL.Query().Get("tableBucketARN")
	namespace := r.URL.Query().Get("namespace")
	tableName := r.URL.Query().Get("name")

	if tableBucketArn == "" || namespace == "" || tableName == "" {
		writeError(w, errBadRequest, "Table bucket ARN, namespace, and table name are required", http.StatusBadRequest)

		return
	}

	table, err := s.storage.GetTable(r.Context(), tableBucketArn, namespace, tableName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &GetTableResponse{
		Arn:               table.Arn,
		Name:              table.Name,
		Namespace:         []string{table.Namespace},
		TableBucketArn:    table.TableBucketArn,
		Type:              table.Type,
		Format:            table.Format,
		VersionToken:      table.VersionToken,
		MetadataLocation:  table.MetadataLocation,
		WarehouseLocation: table.WarehouseLocation,
		CreatedAt:         table.CreatedAt,
		CreatedBy:         table.CreatedBy,
		ModifiedAt:        table.ModifiedAt,
		ModifiedBy:        table.ModifiedBy,
		OwnerID:           table.OwnerID,
	})
}

// ListTables handles the ListTables operation.
func (s *Service) ListTables(w http.ResponseWriter, r *http.Request) {
	tableBucketArn, namespace := extractTablePathParams(getURLPath(r))
	if tableBucketArn == "" {
		writeError(w, errBadRequest, "Table bucket ARN is required", http.StatusBadRequest)

		return
	}

	maxTables := defaultMaxItems

	if v := r.URL.Query().Get("maxTables"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxTables = n
		}
	}

	prefix := r.URL.Query().Get("prefix")

	tables, err := s.storage.ListTables(r.Context(), tableBucketArn, namespace, prefix, maxTables)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &ListTablesResponse{
		Tables: tables,
	})
}

// getURLPath returns the raw URL path if available, otherwise the decoded path.
// RawPath preserves URL encoding which is important for ARNs containing '/'.
func getURLPath(r *http.Request) string {
	if r.URL.RawPath != "" {
		return r.URL.RawPath
	}

	return r.URL.Path
}

// extractTableBucketARN extracts the table bucket ARN from the URL path.
func extractTableBucketARN(path string) string {
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) >= 2 && parts[0] == pathPrefixBuckets {
		arn, err := url.PathUnescape(parts[1])
		if err != nil {
			return ""
		}

		return arn
	}

	return ""
}

// extractTableBucketARNFromNamespacePath extracts the table bucket ARN from a namespace path.
func extractTableBucketARNFromNamespacePath(path string) string {
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) >= 2 && parts[0] == pathPrefixNamespaces {
		arn, err := url.PathUnescape(parts[1])
		if err != nil {
			return ""
		}

		return arn
	}

	return ""
}

// extractNamespaceParams extracts table bucket ARN and namespace from the URL path.
func extractNamespaceParams(path string) (string, string) {
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) >= 3 && parts[0] == pathPrefixNamespaces {
		arn, err := url.PathUnescape(parts[1])
		if err != nil {
			return "", ""
		}

		namespace, err := url.PathUnescape(parts[2])
		if err != nil {
			return "", ""
		}

		return arn, namespace
	}

	return "", ""
}

// extractTablePathParams extracts table bucket ARN and namespace from the tables path.
func extractTablePathParams(path string) (string, string) {
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) >= 3 && parts[0] == pathPrefixTables {
		arn, err := url.PathUnescape(parts[1])
		if err != nil {
			return "", ""
		}

		namespace, err := url.PathUnescape(parts[2])
		if err != nil {
			return "", ""
		}

		return arn, namespace
	}

	if len(parts) >= 2 && parts[0] == pathPrefixTables {
		arn, err := url.PathUnescape(parts[1])
		if err != nil {
			return "", ""
		}

		return arn, ""
	}

	return "", ""
}

// extractFullTableParams extracts table bucket ARN, namespace, and table name from the URL path.
func extractFullTableParams(path string) (string, string, string) {
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) >= 4 && parts[0] == pathPrefixTables {
		arn, err := url.PathUnescape(parts[1])
		if err != nil {
			return "", "", ""
		}

		namespace, err := url.PathUnescape(parts[2])
		if err != nil {
			return "", "", ""
		}

		tableName, err := url.PathUnescape(parts[3])
		if err != nil {
			return "", "", ""
		}

		return arn, namespace, tableName
	}

	return "", "", ""
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

// handleError handles S3 Tables errors and writes the appropriate response.
func handleError(w http.ResponseWriter, err error) {
	var s3tablesErr *Error
	if errors.As(err, &s3tablesErr) {
		status := http.StatusBadRequest

		switch s3tablesErr.Code {
		case errNotFound:
			status = http.StatusNotFound
		case errConflict:
			status = http.StatusConflict
		case errInternalError:
			status = http.StatusInternalServerError
		}

		writeError(w, s3tablesErr.Code, s3tablesErr.Message, status)

		return
	}

	writeError(w, errInternalError, "Internal server error", http.StatusInternalServerError)
}
