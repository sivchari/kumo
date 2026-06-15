package athena

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

// Error codes for Athena handlers.
const (
	errInvalidAction           = "InvalidAction"
	errInternalServerException = "InternalServerException"
)

// StartQueryExecution handles the StartQueryExecution action.
func (s *Service) StartQueryExecution(w http.ResponseWriter, r *http.Request) {
	var req StartQueryExecutionRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeAthenaError(w, errInvalidRequestException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.QueryString == "" {
		writeAthenaError(w, errInvalidRequestException, "QueryString is required.", http.StatusBadRequest)

		return
	}

	qe, err := s.storage.StartQueryExecution(r.Context(), req.QueryString, req.WorkGroup, req.QueryExecutionContext, req.ResultConfiguration, req.ExecutionParameters)
	if err != nil {
		handleAthenaError(w, err)

		return
	}

	writeJSONResponse(w, StartQueryExecutionResponse{
		QueryExecutionID: qe.QueryExecutionID,
	})
}

// StopQueryExecution handles the StopQueryExecution action.
func (s *Service) StopQueryExecution(w http.ResponseWriter, r *http.Request) {
	var req StopQueryExecutionRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeAthenaError(w, errInvalidRequestException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.QueryExecutionID == "" {
		writeAthenaError(w, errInvalidRequestException, "QueryExecutionId is required.", http.StatusBadRequest)

		return
	}

	if err := s.storage.StopQueryExecution(r.Context(), req.QueryExecutionID); err != nil {
		handleAthenaError(w, err)

		return
	}

	writeJSONResponse(w, StopQueryExecutionResponse{})
}

// GetQueryExecution handles the GetQueryExecution action.
func (s *Service) GetQueryExecution(w http.ResponseWriter, r *http.Request) {
	var req GetQueryExecutionRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeAthenaError(w, errInvalidRequestException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.QueryExecutionID == "" {
		writeAthenaError(w, errInvalidRequestException, "QueryExecutionId is required.", http.StatusBadRequest)

		return
	}

	qe, err := s.storage.GetQueryExecution(r.Context(), req.QueryExecutionID)
	if err != nil {
		handleAthenaError(w, err)

		return
	}

	writeJSONResponse(w, GetQueryExecutionResponse{
		QueryExecution: convertQueryExecutionToOutput(qe),
	})
}

// GetQueryResults handles the GetQueryResults action.
func (s *Service) GetQueryResults(w http.ResponseWriter, r *http.Request) {
	var req GetQueryResultsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeAthenaError(w, errInvalidRequestException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.QueryExecutionID == "" {
		writeAthenaError(w, errInvalidRequestException, "QueryExecutionId is required.", http.StatusBadRequest)

		return
	}

	rs, nextToken, err := s.storage.GetQueryResults(r.Context(), req.QueryExecutionID, req.NextToken, req.MaxResults)
	if err != nil {
		handleAthenaError(w, err)

		return
	}

	writeJSONResponse(w, GetQueryResultsResponse{
		ResultSet: convertResultSetToOutput(rs),
		NextToken: nextToken,
	})
}

// ListQueryExecutions handles the ListQueryExecutions action.
func (s *Service) ListQueryExecutions(w http.ResponseWriter, r *http.Request) {
	var req ListQueryExecutionsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeAthenaError(w, errInvalidRequestException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	ids, nextToken, err := s.storage.ListQueryExecutions(r.Context(), req.WorkGroup, req.NextToken, req.MaxResults)
	if err != nil {
		handleAthenaError(w, err)

		return
	}

	writeJSONResponse(w, ListQueryExecutionsResponse{
		QueryExecutionIDs: ids,
		NextToken:         nextToken,
	})
}

// CreateWorkGroup handles the CreateWorkGroup action.
func (s *Service) CreateWorkGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateWorkGroupRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeAthenaError(w, errInvalidRequestException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeAthenaError(w, errInvalidRequestException, "Name is required.", http.StatusBadRequest)

		return
	}

	if err := s.storage.CreateWorkGroup(r.Context(), req.Name, req.Configuration, req.Description, req.Tags); err != nil {
		handleAthenaError(w, err)

		return
	}

	writeJSONResponse(w, CreateWorkGroupResponse{})
}

// DeleteWorkGroup handles the DeleteWorkGroup action.
func (s *Service) DeleteWorkGroup(w http.ResponseWriter, r *http.Request) {
	var req DeleteWorkGroupRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeAthenaError(w, errInvalidRequestException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.WorkGroup == "" {
		writeAthenaError(w, errInvalidRequestException, "WorkGroup is required.", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteWorkGroup(r.Context(), req.WorkGroup, req.RecursiveDeleteOption); err != nil {
		handleAthenaError(w, err)

		return
	}

	writeJSONResponse(w, DeleteWorkGroupResponse{})
}

// DispatchAction routes the request to the appropriate handler based on X-Amz-Target header.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "AmazonAthena.")

	switch action {
	case "StartQueryExecution":
		s.StartQueryExecution(w, r)
	case "StopQueryExecution":
		s.StopQueryExecution(w, r)
	case "GetQueryExecution":
		s.GetQueryExecution(w, r)
	case "GetQueryResults":
		s.GetQueryResults(w, r)
	case "ListQueryExecutions":
		s.ListQueryExecutions(w, r)
	case "CreateWorkGroup":
		s.CreateWorkGroup(w, r)
	case "DeleteWorkGroup":
		s.DeleteWorkGroup(w, r)
	default:
		writeAthenaError(w, errInvalidAction, "The action "+action+" is not valid", http.StatusBadRequest)
	}
}

// convertQueryExecutionToOutput converts internal QueryExecution to API output.
func convertQueryExecutionToOutput(qe *QueryExecution) *QueryExecutionOutput {
	output := &QueryExecutionOutput{
		QueryExecutionID:    qe.QueryExecutionID,
		Query:               qe.Query,
		StatementType:       qe.StatementType,
		WorkGroup:           qe.WorkGroup,
		ExecutionParameters: qe.ExecutionParameters,
		SubstatementType:    qe.SubstatementType,
	}

	output.ResultConfiguration = convertResultConfigToOutput(qe.ResultConfiguration)

	if qe.QueryExecutionContext != nil {
		output.QueryExecutionContext = &QueryExecutionContextOutput{
			Database: qe.QueryExecutionContext.Database,
			Catalog:  qe.QueryExecutionContext.Catalog,
		}
	}

	if qe.Status != nil {
		output.Status = &QueryExecutionStatusOutput{
			State:              string(qe.Status.State),
			StateChangeReason:  qe.Status.StateChangeReason,
			SubmissionDateTime: float64(qe.Status.SubmissionDateTime.Unix()),
		}

		if qe.Status.CompletionDateTime != nil {
			completionTime := float64(qe.Status.CompletionDateTime.Unix())
			output.Status.CompletionDateTime = &completionTime
		}
	}

	output.Statistics = convertStatisticsToOutput(qe.Statistics)
	output.EngineVersion = convertEngineVersionToOutput(qe.EngineVersion)

	return output
}

func convertResultConfigToOutput(cfg *ResultConfiguration) *ResultConfigurationOutput {
	if cfg == nil {
		return nil
	}

	output := &ResultConfigurationOutput{
		OutputLocation:      cfg.OutputLocation,
		ExpectedBucketOwner: cfg.ExpectedBucketOwner,
	}

	if cfg.EncryptionConfiguration != nil {
		output.EncryptionConfiguration = &EncryptionConfigurationOutput{
			EncryptionOption: cfg.EncryptionConfiguration.EncryptionOption,
			KmsKey:           cfg.EncryptionConfiguration.KmsKey,
		}
	}

	if cfg.ACLConfiguration != nil {
		output.ACLConfiguration = &ACLConfigurationOutput{
			S3AclOption: cfg.ACLConfiguration.S3AclOption,
		}
	}

	return output
}

func convertStatisticsToOutput(stats *QueryExecutionStatistics) *QueryExecutionStatisticsOutput {
	if stats == nil {
		return nil
	}

	return &QueryExecutionStatisticsOutput{
		EngineExecutionTimeInMillis:      stats.EngineExecutionTimeInMillis,
		DataScannedInBytes:               stats.DataScannedInBytes,
		DataManifestLocation:             stats.DataManifestLocation,
		TotalExecutionTimeInMillis:       stats.TotalExecutionTimeInMillis,
		QueryQueueTimeInMillis:           stats.QueryQueueTimeInMillis,
		ServicePreProcessingTimeInMillis: stats.ServicePreProcessingTimeInMillis,
		QueryPlanningTimeInMillis:        stats.QueryPlanningTimeInMillis,
		ServiceProcessingTimeInMillis:    stats.ServiceProcessingTimeInMillis,
	}
}

func convertEngineVersionToOutput(ev *EngineVersion) *EngineVersionOutput {
	if ev == nil {
		return nil
	}

	return &EngineVersionOutput{
		SelectedEngineVersion:  ev.SelectedEngineVersion,
		EffectiveEngineVersion: ev.EffectiveEngineVersion,
	}
}

// convertResultSetToOutput converts internal ResultSet to API output.
func convertResultSetToOutput(rs *ResultSet) *ResultSetOutput {
	if rs == nil {
		return nil
	}

	output := &ResultSetOutput{
		Rows: make([]RowOutput, 0, len(rs.Rows)),
	}

	for _, row := range rs.Rows {
		rowOutput := RowOutput{
			Data: make([]DatumOutput, 0, len(row.Data)),
		}

		for _, datum := range row.Data {
			rowOutput.Data = append(rowOutput.Data, DatumOutput(datum))
		}

		output.Rows = append(output.Rows, rowOutput)
	}

	if rs.ResultSetMetadata != nil {
		output.ResultSetMetadata = &ResultSetMetadataOutput{
			ColumnInfo: make([]ColumnInfoOutput, 0, len(rs.ResultSetMetadata.ColumnInfo)),
		}

		for i := range rs.ResultSetMetadata.ColumnInfo {
			output.ResultSetMetadata.ColumnInfo = append(output.ResultSetMetadata.ColumnInfo, ColumnInfoOutput(rs.ResultSetMetadata.ColumnInfo[i]))
		}
	}

	return output
}

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	service.WriteJSONResponse(w, service.ContentTypeAmzJSON11, v)
}

// writeAthenaError writes an Athena error response in JSON format.
func writeAthenaError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Type:    code,
		Message: message,
	})
}

// handleAthenaError handles Athena errors and writes the appropriate response.
func handleAthenaError(w http.ResponseWriter, err error) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		status := http.StatusBadRequest

		if svcErr.Code == errInternalServerException {
			status = http.StatusInternalServerError
		}

		writeAthenaError(w, svcErr.Code, svcErr.Message, status)

		return
	}

	writeAthenaError(w, errInternalServerException, "Internal server error", http.StatusInternalServerError)
}
