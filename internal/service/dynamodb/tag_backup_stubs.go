package dynamodb

import (
	"encoding/json"
	"net/http"
)

const continuousBackupsDisabled = "DISABLED"

// updateTableRequest is the wire shape of UpdateTable.
type updateTableRequest struct {
	TableName string `json:"TableName"` //nolint:tagliatelle // AWS JSON uses PascalCase
}

// UpdateTable is a no-op stub that returns the current table description.
//
// terraform-provider-aws calls UpdateTable during terraform destroy to
// remove GSIs before deleting the table. Without this stub, kumo returns
// UnknownOperationException and destroy fails.
func (s *Service) UpdateTable(w http.ResponseWriter, r *http.Request) {
	var req updateTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TableName == "" {
		writeDynamoDBError(w, "ValidationException", "TableName is required", http.StatusBadRequest)

		return
	}

	table, err := s.storage.DescribeTable(r.Context(), req.TableName)
	if err != nil {
		writeDynamoDBError(w, "ResourceNotFoundException", "Table not found: "+req.TableName, http.StatusBadRequest)

		return
	}

	writeJSONResponse(w, DescribeTableResponse{
		Table: tableToDescription(table),
	})
}

// ListTagsOfResource returns an empty tag list for any resource.
//
// Tags are not modeled in the storage layer yet; this stub exists so reads
// from clients that refresh table state after CreateTable (terraform, pulumi,
// CDK) do not fail with UnknownOperationException.
func (s *Service) ListTagsOfResource(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, listTagsOfResourceResponse{Tags: []map[string]string{}})
}

// TagResource accepts and discards tag attachments.
func (s *Service) TagResource(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, struct{}{})
}

// UntagResource accepts and discards tag detachments.
func (s *Service) UntagResource(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, struct{}{})
}

// DescribeContinuousBackups reports continuous backups as DISABLED for any
// existing table, returning TableNotFoundException for missing tables to
// match AWS semantics that terraform refresh paths depend on.
func (s *Service) DescribeContinuousBackups(w http.ResponseWriter, r *http.Request) {
	var req describeContinuousBackupsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TableName == "" {
		writeDynamoDBError(w, "ValidationException", "TableName is required", http.StatusBadRequest)

		return
	}

	if _, err := s.storage.DescribeTable(r.Context(), req.TableName); err != nil {
		writeDynamoDBError(w, "TableNotFoundException", "Table not found: "+req.TableName, http.StatusBadRequest)

		return
	}

	writeJSONResponse(w, describeContinuousBackupsResponse{
		ContinuousBackupsDescription: continuousBackupsDescription{
			ContinuousBackupsStatus: continuousBackupsDisabled,
			PointInTimeRecoveryDescription: pointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: continuousBackupsDisabled,
			},
		},
	})
}
