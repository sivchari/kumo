package dynamodb

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// CreateTable handles the CreateTable action.
func (s *Service) CreateTable(w http.ResponseWriter, r *http.Request) {
	var req CreateTableRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TableName == "" {
		writeDynamoDBError(w, "ValidationException", "TableName is required", http.StatusBadRequest)

		return
	}

	if len(req.KeySchema) == 0 {
		writeDynamoDBError(w, "ValidationException", "KeySchema is required", http.StatusBadRequest)

		return
	}

	table, err := s.storage.CreateTable(r.Context(), &req)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			writeDynamoDBError(w, tErr.Code, tErr.Message, http.StatusBadRequest)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, CreateTableResponse{
		TableDescription: tableToDescription(table),
	})
}

// DeleteTable handles the DeleteTable action.
func (s *Service) DeleteTable(w http.ResponseWriter, r *http.Request) {
	var req DeleteTableRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TableName == "" {
		writeDynamoDBError(w, "ValidationException", "TableName is required", http.StatusBadRequest)

		return
	}

	table, err := s.storage.DeleteTable(r.Context(), req.TableName)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			writeDynamoDBError(w, tErr.Code, tErr.Message, http.StatusBadRequest)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, DeleteTableResponse{
		TableDescription: tableToDescription(table),
	})
}

// ListTables handles the ListTables action.
func (s *Service) ListTables(w http.ResponseWriter, r *http.Request) {
	var req ListTablesRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	names, lastEvaluated, err := s.storage.ListTables(r.Context(), req.ExclusiveStartTableName, req.Limit)
	if err != nil {
		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, ListTablesResponse{
		TableNames:             names,
		LastEvaluatedTableName: lastEvaluated,
	})
}

// DescribeTable handles the DescribeTable action.
func (s *Service) DescribeTable(w http.ResponseWriter, r *http.Request) {
	var req DescribeTableRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TableName == "" {
		writeDynamoDBError(w, "ValidationException", "TableName is required", http.StatusBadRequest)

		return
	}

	table, err := s.storage.DescribeTable(r.Context(), req.TableName)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			writeDynamoDBError(w, tErr.Code, tErr.Message, http.StatusBadRequest)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, DescribeTableResponse{
		Table: tableToDescription(table),
	})
}

// PutItem handles the PutItem action.
func (s *Service) PutItem(w http.ResponseWriter, r *http.Request) {
	var req PutItemRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TableName == "" {
		writeDynamoDBError(w, "ValidationException", "TableName is required", http.StatusBadRequest)

		return
	}

	if len(req.Item) == 0 {
		writeDynamoDBError(w, "ValidationException", "Item is required", http.StatusBadRequest)

		return
	}

	returnOld := req.ReturnValues == ReturnValuesAllOld

	cond := ConditionInput{
		Expression: req.ConditionExpression,
		ExprNames:  req.ExpressionAttributeNames,
		ExprValues: req.ExpressionAttributeValues,
	}

	oldItem, err := s.storage.PutItem(r.Context(), req.TableName, req.Item, returnOld, cond)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			status := http.StatusBadRequest

			writeDynamoDBError(w, tErr.Code, tErr.Message, status)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, PutItemResponse{
		Attributes: oldItem,
	})
}

// GetItem handles the GetItem action.
func (s *Service) GetItem(w http.ResponseWriter, r *http.Request) {
	var req GetItemRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TableName == "" {
		writeDynamoDBError(w, "ValidationException", "TableName is required", http.StatusBadRequest)

		return
	}

	if len(req.Key) == 0 {
		writeDynamoDBError(w, "ValidationException", "Key is required", http.StatusBadRequest)

		return
	}

	item, err := s.storage.GetItem(r.Context(), req.TableName, req.Key)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			writeDynamoDBError(w, tErr.Code, tErr.Message, http.StatusBadRequest)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	item = projectItemForExpression(item, req.ProjectionExpression, req.ExpressionAttributeNames)

	writeJSONResponse(w, GetItemResponse{
		Item: item,
	})
}

// DeleteItem handles the DeleteItem action.
func (s *Service) DeleteItem(w http.ResponseWriter, r *http.Request) {
	var req DeleteItemRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TableName == "" {
		writeDynamoDBError(w, "ValidationException", "TableName is required", http.StatusBadRequest)

		return
	}

	if len(req.Key) == 0 {
		writeDynamoDBError(w, "ValidationException", "Key is required", http.StatusBadRequest)

		return
	}

	returnOld := req.ReturnValues == ReturnValuesAllOld

	cond := ConditionInput{
		Expression: req.ConditionExpression,
		ExprNames:  req.ExpressionAttributeNames,
		ExprValues: req.ExpressionAttributeValues,
	}

	oldItem, err := s.storage.DeleteItem(r.Context(), req.TableName, req.Key, returnOld, cond)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			status := http.StatusBadRequest

			writeDynamoDBError(w, tErr.Code, tErr.Message, status)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, DeleteItemResponse{
		Attributes: oldItem,
	})
}

// UpdateItem handles the UpdateItem action.
func (s *Service) UpdateItem(w http.ResponseWriter, r *http.Request) {
	var req UpdateItemRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TableName == "" {
		writeDynamoDBError(w, "ValidationException", "TableName is required", http.StatusBadRequest)

		return
	}

	if len(req.Key) == 0 {
		writeDynamoDBError(w, "ValidationException", "Key is required", http.StatusBadRequest)

		return
	}

	// Convert legacy AttributeUpdates to UpdateExpression if needed.
	if req.UpdateExpression == "" && len(req.AttributeUpdates) > 0 {
		convertAttributeUpdates(&req)
	}

	cond := ConditionInput{
		Expression: req.ConditionExpression,
		ExprNames:  req.ExpressionAttributeNames,
		ExprValues: req.ExpressionAttributeValues,
	}

	result, err := s.storage.UpdateItem(
		r.Context(),
		req.TableName,
		req.Key,
		req.UpdateExpression,
		req.ExpressionAttributeNames,
		req.ExpressionAttributeValues,
		req.ReturnValues,
		cond,
	)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			status := http.StatusBadRequest

			writeDynamoDBError(w, tErr.Code, tErr.Message, status)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, UpdateItemResponse{
		Attributes: result,
	})
}

// Query handles the Query action.
func (s *Service) Query(w http.ResponseWriter, r *http.Request) {
	var req QueryRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TableName == "" {
		writeDynamoDBError(w, "ValidationException", "TableName is required", http.StatusBadRequest)

		return
	}

	scanForward := true
	if req.ScanIndexForward != nil {
		scanForward = *req.ScanIndexForward
	}

	items, lastKey, scannedCount, err := s.storage.Query(
		r.Context(),
		req.TableName,
		req.IndexName,
		req.KeyConditionExpression,
		req.FilterExpression,
		req.ExpressionAttributeNames,
		req.ExpressionAttributeValues,
		req.Limit,
		req.ExclusiveStartKey,
		scanForward,
	)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			writeDynamoDBError(w, tErr.Code, tErr.Message, http.StatusBadRequest)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	items = projectItemsForExpression(items, req.ProjectionExpression, req.ExpressionAttributeNames)

	writeJSONResponse(w, QueryResponse{
		Items:            items,
		Count:            len(items),
		ScannedCount:     scannedCount,
		LastEvaluatedKey: lastKey,
	})
}

// Scan handles the Scan action.
func (s *Service) Scan(w http.ResponseWriter, r *http.Request) {
	var req ScanRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TableName == "" {
		writeDynamoDBError(w, "ValidationException", "TableName is required", http.StatusBadRequest)

		return
	}

	items, lastKey, scannedCount, err := s.storage.Scan(
		r.Context(),
		req.TableName,
		req.FilterExpression,
		req.ExpressionAttributeNames,
		req.ExpressionAttributeValues,
		req.Limit,
		req.ExclusiveStartKey,
		req.Segment,
		req.TotalSegments,
	)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			writeDynamoDBError(w, tErr.Code, tErr.Message, http.StatusBadRequest)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	items = projectItemsForExpression(items, req.ProjectionExpression, req.ExpressionAttributeNames)

	writeJSONResponse(w, ScanResponse{
		Items:            items,
		Count:            len(items),
		ScannedCount:     scannedCount,
		LastEvaluatedKey: lastKey,
	})
}

// tableToDescription converts a Table to TableDescription.
//
//nolint:funlen // Struct initialization with GSI/LSI conversion requires many statements.
func tableToDescription(table *Table) TableDescription {
	desc := TableDescription{
		TableName:                 table.Name,
		TableStatus:               table.TableStatus,
		TableARN:                  table.TableARN,
		TableID:                   uuid.New().String(),
		CreationDateTime:          float64(table.CreationDateTime.Unix()),
		KeySchema:                 table.KeySchema,
		AttributeDefinitions:      table.AttributeDefinitions,
		ItemCount:                 table.ItemCount,
		TableSizeBytes:            table.TableSizeBytes,
		DeletionProtectionEnabled: table.DeletionProtection,
		LatestStreamArn:           table.LatestStreamArn,
	}

	if table.StreamEnabled {
		desc.StreamSpecification = &StreamSpecification{
			StreamEnabled:  true,
			StreamViewType: table.StreamViewType,
		}
	}

	if table.ProvisionedThroughput != nil {
		desc.ProvisionedThroughput = &ProvisionedThroughputDescription{
			ReadCapacityUnits:  table.ProvisionedThroughput.ReadCapacityUnits,
			WriteCapacityUnits: table.ProvisionedThroughput.WriteCapacityUnits,
		}
	}

	if table.BillingMode != "" {
		desc.BillingModeSummary = &BillingModeSummary{
			BillingMode: table.BillingMode,
		}
	}

	for _, gsi := range table.GlobalSecondaryIndexes {
		gsiDesc := GlobalSecondaryIndexDescription{
			IndexName:      gsi.IndexName,
			KeySchema:      gsi.KeySchema,
			Projection:     gsi.Projection,
			IndexStatus:    "ACTIVE",
			IndexArn:       fmt.Sprintf("%s/index/%s", table.TableARN, gsi.IndexName),
			ItemCount:      table.ItemCount,
			IndexSizeBytes: table.TableSizeBytes,
		}

		if gsi.ProvisionedThroughput != nil {
			gsiDesc.ProvisionedThroughput = &ProvisionedThroughputDescription{
				ReadCapacityUnits:  gsi.ProvisionedThroughput.ReadCapacityUnits,
				WriteCapacityUnits: gsi.ProvisionedThroughput.WriteCapacityUnits,
			}
		}

		desc.GlobalSecondaryIndexes = append(desc.GlobalSecondaryIndexes, gsiDesc)
	}

	for _, lsi := range table.LocalSecondaryIndexes {
		lsiDesc := LocalSecondaryIndexDescription{
			IndexName:      lsi.IndexName,
			KeySchema:      lsi.KeySchema,
			Projection:     lsi.Projection,
			IndexArn:       fmt.Sprintf("%s/index/%s", table.TableARN, lsi.IndexName),
			ItemCount:      table.ItemCount,
			IndexSizeBytes: table.TableSizeBytes,
		}

		desc.LocalSecondaryIndexes = append(desc.LocalSecondaryIndexes, lsiDesc)
	}

	return desc
}

// readJSONRequest reads and decodes JSON request body.
func readJSONRequest(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	if len(body) == 0 {
		return nil
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// writeDynamoDBError writes a DynamoDB error response in JSON format.
func writeDynamoDBError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Type:    code,
		Message: message,
	})
}

// UpdateTimeToLive handles the UpdateTimeToLive action.
func (s *Service) UpdateTimeToLive(w http.ResponseWriter, r *http.Request) {
	var req UpdateTimeToLiveRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TableName == "" {
		writeDynamoDBError(w, "ValidationException", "TableName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.UpdateTimeToLive(r.Context(), req.TableName, req.TimeToLiveSpecification.AttributeName, req.TimeToLiveSpecification.Enabled); err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			writeDynamoDBError(w, tErr.Code, tErr.Message, http.StatusBadRequest)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, UpdateTimeToLiveResponse{
		TimeToLiveSpecification: req.TimeToLiveSpecification,
	})
}

// DescribeTimeToLive handles the DescribeTimeToLive action.
func (s *Service) DescribeTimeToLive(w http.ResponseWriter, r *http.Request) {
	var req DescribeTimeToLiveRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TableName == "" {
		writeDynamoDBError(w, "ValidationException", "TableName is required", http.StatusBadRequest)

		return
	}

	attrName, enabled, err := s.storage.DescribeTimeToLive(r.Context(), req.TableName)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			writeDynamoDBError(w, tErr.Code, tErr.Message, http.StatusBadRequest)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	status := "DISABLED"
	if enabled {
		status = "ENABLED"
	}

	writeJSONResponse(w, DescribeTimeToLiveResponse{
		TimeToLiveDescription: TimeToLiveDescription{
			AttributeName:    attrName,
			TimeToLiveStatus: status,
		},
	})
}

// TransactWriteItems handles the TransactWriteItems action.
func (s *Service) TransactWriteItems(w http.ResponseWriter, r *http.Request) {
	var req TransactWriteItemsRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.TransactItems) == 0 {
		writeDynamoDBError(w, "ValidationException", "TransactItems is required", http.StatusBadRequest)

		return
	}

	if len(req.TransactItems) > 100 {
		writeDynamoDBError(w, "ValidationException", "Member must have length less than or equal to 100", http.StatusBadRequest)

		return
	}

	reasons, err := s.storage.TransactWriteItems(r.Context(), req.TransactItems)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			if tErr.Code == "TransactionCanceledException" && reasons != nil {
				w.Header().Set("Content-Type", "application/x-amz-json-1.0")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(TransactionCanceledResponse{
					Type:                "TransactionCanceledException",
					Message:             tErr.Message,
					CancellationReasons: reasons,
				})

				return
			}

			writeDynamoDBError(w, tErr.Code, tErr.Message, http.StatusBadRequest)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, TransactWriteItemsResponse{})
}

// TransactGetItems handles the TransactGetItems action.
func (s *Service) TransactGetItems(w http.ResponseWriter, r *http.Request) {
	var req TransactGetItemsRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.TransactItems) == 0 {
		writeDynamoDBError(w, "ValidationException", "TransactItems is required", http.StatusBadRequest)

		return
	}

	if len(req.TransactItems) > 100 {
		writeDynamoDBError(w, "ValidationException", "Member must have length less than or equal to 100", http.StatusBadRequest)

		return
	}

	items, err := s.storage.TransactGetItems(r.Context(), req.TransactItems)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			writeDynamoDBError(w, tErr.Code, tErr.Message, http.StatusBadRequest)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	responses := make([]TransactGetItemResponse, len(items))

	for i, item := range items {
		if req.TransactItems[i].Get != nil {
			item = projectItemForExpression(
				item,
				req.TransactItems[i].Get.ProjectionExpression,
				req.TransactItems[i].Get.ExpressionAttributeNames,
			)
		}

		responses[i] = TransactGetItemResponse{Item: item}
	}

	writeJSONResponse(w, TransactGetItemsResponse{Responses: responses})
}

// actionHandlers returns a map of action names to handler functions.
func (s *Service) actionHandlers() map[string]func(http.ResponseWriter, *http.Request) {
	return map[string]func(http.ResponseWriter, *http.Request){
		"CreateTable":               s.CreateTable,
		"DeleteTable":               s.DeleteTable,
		"ListTables":                s.ListTables,
		"DescribeTable":             s.DescribeTable,
		"PutItem":                   s.PutItem,
		"GetItem":                   s.GetItem,
		"DeleteItem":                s.DeleteItem,
		"UpdateItem":                s.UpdateItem,
		"Query":                     s.Query,
		"Scan":                      s.Scan,
		"UpdateTimeToLive":          s.UpdateTimeToLive,
		"DescribeTimeToLive":        s.DescribeTimeToLive,
		"TransactWriteItems":        s.TransactWriteItems,
		"TransactGetItems":          s.TransactGetItems,
		"BatchWriteItem":            s.BatchWriteItem,
		"BatchGetItem":              s.BatchGetItem,
		"UpdateTable":               s.UpdateTable,
		"ListTagsOfResource":        s.ListTagsOfResource,
		"TagResource":               s.TagResource,
		"UntagResource":             s.UntagResource,
		"DescribeContinuousBackups": s.DescribeContinuousBackups,
	}
}

// BatchWriteItem handles the BatchWriteItem action.
func (s *Service) BatchWriteItem(w http.ResponseWriter, r *http.Request) {
	var req BatchWriteItemRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.RequestItems) == 0 {
		writeDynamoDBError(w, "ValidationException", "RequestItems is required", http.StatusBadRequest)

		return
	}

	totalItems := 0
	for _, reqs := range req.RequestItems {
		totalItems += len(reqs)
	}

	if totalItems > 25 {
		writeDynamoDBError(w, "ValidationException", "Too many items requested for the BatchWriteItem call", http.StatusBadRequest)

		return
	}

	unprocessed, err := s.storage.BatchWriteItem(r.Context(), req.RequestItems)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			writeDynamoDBError(w, tErr.Code, tErr.Message, http.StatusBadRequest)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, BatchWriteItemResponse{UnprocessedItems: unprocessed})
}

// BatchGetItem handles the BatchGetItem action.
func (s *Service) BatchGetItem(w http.ResponseWriter, r *http.Request) {
	var req BatchGetItemRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.RequestItems) == 0 {
		writeDynamoDBError(w, "ValidationException", "RequestItems is required", http.StatusBadRequest)

		return
	}

	totalKeys := 0
	for _, ka := range req.RequestItems {
		totalKeys += len(ka.Keys)
	}

	if totalKeys > 100 {
		writeDynamoDBError(w, "ValidationException", "Too many items requested for the BatchGetItem call", http.StatusBadRequest)

		return
	}

	responses, err := s.storage.BatchGetItem(r.Context(), req.RequestItems)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			writeDynamoDBError(w, tErr.Code, tErr.Message, http.StatusBadRequest)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	for tableName, keysAndAttributes := range req.RequestItems {
		if items, ok := responses[tableName]; ok {
			responses[tableName] = projectItemsForExpression(
				items,
				keysAndAttributes.ProjectionExpression,
				keysAndAttributes.ExpressionAttributeNames,
			)
		}
	}

	writeJSONResponse(w, BatchGetItemResponse{Responses: responses})
}

// DispatchAction routes the request to the appropriate handler based on X-Amz-Target header.
// This method implements the JSONProtocolService interface.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "DynamoDB_20120810.")

	handler, ok := s.actionHandlers()[action]
	if !ok {
		writeDynamoDBError(w, "UnknownOperationException", "The action "+action+" is not valid", http.StatusBadRequest)

		return
	}

	handler(w, r)
}

// convertAttributeUpdates converts legacy AttributeUpdates to UpdateExpression.
func convertAttributeUpdates(req *UpdateItemRequest) {
	if req.ExpressionAttributeNames == nil {
		req.ExpressionAttributeNames = make(map[string]string)
	}

	if req.ExpressionAttributeValues == nil {
		req.ExpressionAttributeValues = make(map[string]AttributeValue)
	}

	var setParts, removeParts, addParts []string

	idx := 0

	for attr := range req.AttributeUpdates {
		update := req.AttributeUpdates[attr]
		nameKey := fmt.Sprintf("#au%d", idx)
		valueKey := fmt.Sprintf(":au%d", idx)
		req.ExpressionAttributeNames[nameKey] = attr

		switch strings.ToUpper(update.Action) {
		case "PUT", "":
			req.ExpressionAttributeValues[valueKey] = update.Value

			setParts = append(setParts, nameKey+" = "+valueKey)
		case "DELETE":
			removeParts = append(removeParts, nameKey)
		case "ADD":
			req.ExpressionAttributeValues[valueKey] = update.Value

			addParts = append(addParts, nameKey+" "+valueKey)
		}

		idx++
	}

	var parts []string

	if len(setParts) > 0 {
		parts = append(parts, "SET "+strings.Join(setParts, ", "))
	}

	if len(removeParts) > 0 {
		parts = append(parts, "REMOVE "+strings.Join(removeParts, ", "))
	}

	if len(addParts) > 0 {
		parts = append(parts, "ADD "+strings.Join(addParts, ", "))
	}

	req.UpdateExpression = strings.Join(parts, " ")
}

// UpdateTable is a no-op that returns the current table description.
//
// terraform-provider-aws calls UpdateTable during terraform destroy to
// remove GSIs before deleting the table. Without this handler, kumo returns
// UnknownOperationException and destroy fails.
func (s *Service) UpdateTable(w http.ResponseWriter, r *http.Request) {
	var req UpdateTableRequest
	if err := readJSONRequest(r, &req); err != nil || req.TableName == "" {
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

// ListTagsOfResource returns the tags for a DynamoDB resource.
func (s *Service) ListTagsOfResource(w http.ResponseWriter, r *http.Request) {
	var req ListTagsOfResourceRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	tags, err := s.storage.ListTagsOfResource(r.Context(), req.ResourceArn)
	if err != nil {
		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, ListTagsOfResourceResponse{Tags: tags})
}

// TagResource adds tags to a DynamoDB resource.
func (s *Service) TagResource(w http.ResponseWriter, r *http.Request) {
	var req TagResourceRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.TagResource(r.Context(), req.ResourceArn, req.Tags); err != nil {
		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// UntagResource removes tags from a DynamoDB resource.
func (s *Service) UntagResource(w http.ResponseWriter, r *http.Request) {
	var req UntagResourceRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeDynamoDBError(w, "SerializationException", "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.UntagResource(r.Context(), req.ResourceArn, req.TagKeys); err != nil {
		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// DescribeContinuousBackups reports continuous backups as DISABLED for any
// existing table, returning TableNotFoundException for missing tables to
// match AWS semantics that terraform refresh paths depend on.
func (s *Service) DescribeContinuousBackups(w http.ResponseWriter, r *http.Request) {
	var req DescribeContinuousBackupsRequest
	if err := readJSONRequest(r, &req); err != nil || req.TableName == "" {
		writeDynamoDBError(w, "ValidationException", "TableName is required", http.StatusBadRequest)

		return
	}

	desc, err := s.storage.DescribeContinuousBackups(r.Context(), req.TableName)
	if err != nil {
		var tErr *TableError
		if errors.As(err, &tErr) {
			writeDynamoDBError(w, tErr.Code, tErr.Message, http.StatusBadRequest)

			return
		}

		writeDynamoDBError(w, "InternalServerError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, DescribeContinuousBackupsResponse{
		ContinuousBackupsDescription: *desc,
	})
}
