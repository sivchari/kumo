package glue

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/sivchari/kumo/internal/service"
)

// DispatchAction routes Glue requests based on the X-Amz-Target header.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	if target == "" {
		writeError(w, errInvalidInput, "Missing X-Amz-Target header", http.StatusBadRequest)

		return
	}

	// Extract operation from target (e.g., "AWSGlue.CreateDatabase").
	parts := strings.Split(target, ".")
	if len(parts) != 2 {
		writeError(w, errInvalidInput, "Invalid X-Amz-Target header", http.StatusBadRequest)

		return
	}

	operation := parts[1]

	handler, ok := s.actionHandlers()[operation]
	if !ok {
		writeError(w, errInvalidInput, fmt.Sprintf("Unknown operation: %s", operation), http.StatusBadRequest)

		return
	}

	handler(w, r)
}

// actionHandlers returns the operation → handler map.
func (s *Service) actionHandlers() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"CreateDatabase": s.CreateDatabase,
		"GetDatabase":    s.GetDatabase,
		"GetDatabases":   s.GetDatabases,
		"DeleteDatabase": s.DeleteDatabase,
		"CreateTable":    s.CreateTable,
		"GetTable":       s.GetTable,
		"GetTables":      s.GetTables,
		"DeleteTable":    s.DeleteTable,
		"CreateJob":      s.CreateJob,
		"DeleteJob":      s.DeleteJob,
		"StartJobRun":    s.StartJobRun,
		"GetTags":        s.GetTags,
		"TagResource":    s.TagResource,
		"UntagResource":  s.UntagResource,
	}
}

// CreateDatabase handles the CreateDatabase operation.
func (s *Service) CreateDatabase(w http.ResponseWriter, r *http.Request) {
	var req CreateDatabaseInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidInput, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.DatabaseInput == nil {
		writeError(w, errInvalidInput, "DatabaseInput is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.CreateDatabase(r.Context(), req.CatalogID, req.DatabaseInput); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// GetDatabase handles the GetDatabase operation.
func (s *Service) GetDatabase(w http.ResponseWriter, r *http.Request) {
	var req GetDatabaseInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidInput, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, errInvalidInput, "Name is required", http.StatusBadRequest)

		return
	}

	db, err := s.storage.GetDatabase(r.Context(), req.CatalogID, req.Name)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, GetDatabaseOutput{
		Database: &DatabaseResponse{
			Name:            db.Name,
			Description:     db.Description,
			LocationURI:     db.LocationURI,
			Parameters:      db.Parameters,
			CreateTime:      ToAWSTimestamp(db.CreateTime).Ptr(),
			CatalogID:       db.CatalogID,
			CreateTableMode: db.CreateTableMode,
		},
	})
}

// GetDatabases handles the GetDatabases operation.
func (s *Service) GetDatabases(w http.ResponseWriter, r *http.Request) {
	var req GetDatabasesInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidInput, "Invalid request body", http.StatusBadRequest)

		return
	}

	databases, nextToken, err := s.storage.GetDatabases(r.Context(), req.CatalogID, req.MaxResults, req.NextToken)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	dbResponses := make([]*DatabaseResponse, 0, len(databases))

	for _, db := range databases {
		dbResponses = append(dbResponses, &DatabaseResponse{
			Name:            db.Name,
			Description:     db.Description,
			LocationURI:     db.LocationURI,
			Parameters:      db.Parameters,
			CreateTime:      ToAWSTimestamp(db.CreateTime).Ptr(),
			CatalogID:       db.CatalogID,
			CreateTableMode: db.CreateTableMode,
		})
	}

	writeJSONResponse(w, GetDatabasesOutput{
		DatabaseList: dbResponses,
		NextToken:    nextToken,
	})
}

// DeleteDatabase handles the DeleteDatabase operation.
func (s *Service) DeleteDatabase(w http.ResponseWriter, r *http.Request) {
	var req DeleteDatabaseInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidInput, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, errInvalidInput, "Name is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteDatabase(r.Context(), req.CatalogID, req.Name); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// CreateTable handles the CreateTable operation.
func (s *Service) CreateTable(w http.ResponseWriter, r *http.Request) {
	var req CreateTableInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidInput, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.DatabaseName == "" {
		writeError(w, errInvalidInput, "DatabaseName is required", http.StatusBadRequest)

		return
	}

	if req.TableInput == nil {
		writeError(w, errInvalidInput, "TableInput is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.CreateTable(r.Context(), req.CatalogID, req.DatabaseName, req.TableInput); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// GetTable handles the GetTable operation.
func (s *Service) GetTable(w http.ResponseWriter, r *http.Request) {
	var req GetTableInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidInput, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.DatabaseName == "" {
		writeError(w, errInvalidInput, "DatabaseName is required", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, errInvalidInput, "Name is required", http.StatusBadRequest)

		return
	}

	table, err := s.storage.GetTable(r.Context(), req.CatalogID, req.DatabaseName, req.Name)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, GetTableOutput{
		Table: &TableResponse{
			Name:              table.Name,
			DatabaseName:      table.DatabaseName,
			Description:       table.Description,
			Owner:             table.Owner,
			CreateTime:        ToAWSTimestamp(table.CreateTime).Ptr(),
			UpdateTime:        ToAWSTimestamp(table.UpdateTime).Ptr(),
			LastAccessTime:    ToAWSTimestampPtr(table.LastAccessTime),
			LastAnalyzedTime:  ToAWSTimestampPtr(table.LastAnalyzedTime),
			Retention:         table.Retention,
			StorageDescriptor: table.StorageDescriptor,
			PartitionKeys:     table.PartitionKeys,
			ViewOriginalText:  table.ViewOriginalText,
			ViewExpandedText:  table.ViewExpandedText,
			TableType:         table.TableType,
			Parameters:        table.Parameters,
			CatalogID:         table.CatalogID,
		},
	})
}

// GetTables handles the GetTables operation.
func (s *Service) GetTables(w http.ResponseWriter, r *http.Request) {
	var req GetTablesInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidInput, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.DatabaseName == "" {
		writeError(w, errInvalidInput, "DatabaseName is required", http.StatusBadRequest)

		return
	}

	tables, nextToken, err := s.storage.GetTables(r.Context(), req.CatalogID, req.DatabaseName, req.MaxResults, req.NextToken)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	tableResponses := make([]*TableResponse, 0, len(tables))

	for _, table := range tables {
		tableResponses = append(tableResponses, &TableResponse{
			Name:              table.Name,
			DatabaseName:      table.DatabaseName,
			Description:       table.Description,
			Owner:             table.Owner,
			CreateTime:        ToAWSTimestamp(table.CreateTime).Ptr(),
			UpdateTime:        ToAWSTimestamp(table.UpdateTime).Ptr(),
			LastAccessTime:    ToAWSTimestampPtr(table.LastAccessTime),
			LastAnalyzedTime:  ToAWSTimestampPtr(table.LastAnalyzedTime),
			Retention:         table.Retention,
			StorageDescriptor: table.StorageDescriptor,
			PartitionKeys:     table.PartitionKeys,
			ViewOriginalText:  table.ViewOriginalText,
			ViewExpandedText:  table.ViewExpandedText,
			TableType:         table.TableType,
			Parameters:        table.Parameters,
			CatalogID:         table.CatalogID,
		})
	}

	writeJSONResponse(w, GetTablesOutput{
		TableList: tableResponses,
		NextToken: nextToken,
	})
}

// DeleteTable handles the DeleteTable operation.
func (s *Service) DeleteTable(w http.ResponseWriter, r *http.Request) {
	var req DeleteTableInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidInput, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.DatabaseName == "" {
		writeError(w, errInvalidInput, "DatabaseName is required", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, errInvalidInput, "Name is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteTable(r.Context(), req.CatalogID, req.DatabaseName, req.Name); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// CreateJob handles the CreateJob operation.
func (s *Service) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidInput, "Invalid request body", http.StatusBadRequest)

		return
	}

	job, err := s.storage.CreateJob(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, CreateJobOutput{
		Name: job.Name,
	})
}

// DeleteJob handles the DeleteJob operation.
func (s *Service) DeleteJob(w http.ResponseWriter, r *http.Request) {
	var req DeleteJobInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidInput, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.JobName == "" {
		writeError(w, errInvalidInput, "JobName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteJob(r.Context(), req.JobName); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, DeleteJobOutput(req))
}

// StartJobRun handles the StartJobRun operation.
func (s *Service) StartJobRun(w http.ResponseWriter, r *http.Request) {
	var req StartJobRunInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidInput, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.JobName == "" {
		writeError(w, errInvalidInput, "JobName is required", http.StatusBadRequest)

		return
	}

	jobRun, err := s.storage.StartJobRun(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, StartJobRunOutput{
		JobRunID: jobRun.ID,
	})
}

// Helper functions.

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	service.WriteJSONResponse(w, service.ContentTypeJSON, v)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeJSON, code, message, status)
}

// handleStorageError handles storage errors and writes appropriate response.
func handleStorageError(w http.ResponseWriter, err error) {
	var glueErr *Error
	if errors.As(err, &glueErr) {
		status := http.StatusBadRequest
		if glueErr.Code == errEntityNotFound {
			status = http.StatusNotFound
		}

		writeError(w, glueErr.Code, glueErr.Message, status)

		return
	}

	writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)
}

// GetTags handles the GetTags operation.
func (s *Service) GetTags(w http.ResponseWriter, r *http.Request) {
	var req GetTagsInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidInput, "Invalid request body", http.StatusBadRequest)

		return
	}

	tags, err := s.storage.GetTags(r.Context(), req.ResourceArn)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, GetTagsOutput{Tags: tags})
}

// TagResource handles the TagResource operation.
func (s *Service) TagResource(w http.ResponseWriter, r *http.Request) {
	var req TagResourceInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidInput, "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.TagResource(r.Context(), req.ResourceArn, req.TagsToAdd); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// UntagResource handles the UntagResource operation.
func (s *Service) UntagResource(w http.ResponseWriter, r *http.Request) {
	var req UntagResourceInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidInput, "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.UntagResource(r.Context(), req.ResourceArn, req.TagsToRemove); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}
