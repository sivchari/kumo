package dynamodb

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
	"github.com/sivchari/kumo/internal/streams"
)

const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "000000000000"
)

// Storage defines the interface for DynamoDB storage operations.
type Storage interface {
	CreateTable(ctx context.Context, req *CreateTableRequest) (*Table, error)
	DeleteTable(ctx context.Context, tableName string) (*Table, error)
	ListTables(ctx context.Context, exclusiveStartTableName string, limit int) ([]string, string, error)
	DescribeTable(ctx context.Context, tableName string) (*Table, error)
	PutItem(ctx context.Context, tableName string, item Item, returnOld bool, cond ConditionInput) (Item, error)
	GetItem(ctx context.Context, tableName string, key Item) (Item, error)
	DeleteItem(ctx context.Context, tableName string, key Item, returnOld bool, cond ConditionInput) (Item, error)
	UpdateItem(ctx context.Context, tableName string, key Item, updateExpr string, exprNames map[string]string, exprValues map[string]AttributeValue, returnValues string, cond ConditionInput) (Item, error)
	Query(ctx context.Context, tableName, indexName string, keyCondExpr string, filterExpr string, exprNames map[string]string, exprValues map[string]AttributeValue, limit int, exclusiveStartKey Item, scanForward bool) ([]Item, Item, int, error)
	Scan(ctx context.Context, tableName string, filterExpr string, exprNames map[string]string, exprValues map[string]AttributeValue, limit int, exclusiveStartKey Item, segment, totalSegments *int) ([]Item, Item, int, error)
	TransactWriteItems(ctx context.Context, items []TransactWriteItem) ([]CancellationReason, error)
	TransactGetItems(ctx context.Context, items []TransactGetItem) ([]Item, error)
	BatchWriteItem(ctx context.Context, requestItems map[string][]WriteRequest) (map[string][]WriteRequest, error)
	BatchGetItem(ctx context.Context, requestItems map[string]KeysAndAttributes) (map[string][]Item, error)
	UpdateTimeToLive(ctx context.Context, tableName, attributeName string, enabled bool) error
	DescribeTimeToLive(ctx context.Context, tableName string) (string, bool, error)
	ListTagsOfResource(ctx context.Context, resourceArn string) ([]Tag, error)
	TagResource(ctx context.Context, resourceArn string, tags []Tag) error
	UntagResource(ctx context.Context, resourceArn string, tagKeys []string) error
	DescribeContinuousBackups(ctx context.Context, tableName string) (*ContinuousBackupsDescription, error)
}

// Option is a configuration option for MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables persistent storage in the specified directory.
func WithDataDir(dir string) Option {
	return func(s *MemoryStorage) {
		s.dataDir = dir
	}
}

// Compile-time interface checks.
var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MemoryStorage implements Storage with in-memory data.
type MemoryStorage struct {
	mu          sync.RWMutex          `json:"-"`
	Tables      map[string]*tableData `json:"tables"`
	Tags        map[string][]Tag      `json:"tags,omitempty"`
	baseURL     string
	region      string
	dataDir     string
	stopTTL     chan struct{}
	streamStore *streams.Store
}

type tableData struct {
	Table *Table          `json:"table"`
	Items map[string]Item `json:"items"`
}

// NewMemoryStorage creates a new in-memory DynamoDB storage.
func NewMemoryStorage(baseURL string, opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Tables:      make(map[string]*tableData),
		Tags:        make(map[string][]Tag),
		baseURL:     baseURL,
		region:      region,
		stopTTL:     make(chan struct{}),
		streamStore: streams.Global,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "dynamodb", s)
	}

	go s.ttlReaper()

	return s
}

// ttlReaper periodically scans tables with TTL enabled and deletes expired items.
func (m *MemoryStorage) ttlReaper() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopTTL:
			return
		case <-ticker.C:
			m.deleteExpiredItems()
		}
	}
}

// deleteExpiredItems removes items whose TTL attribute is before the current time.
func (m *MemoryStorage) deleteExpiredItems() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().Unix()

	for _, td := range m.Tables {
		if !td.Table.TTLEnabled || td.Table.TTLAttributeName == "" {
			continue
		}

		ttlAttr := td.Table.TTLAttributeName

		var keysToDelete []string

		for key, item := range td.Items {
			av, exists := item[ttlAttr]
			if !exists || av.N == nil {
				continue
			}

			ttlVal, err := strconv.ParseInt(*av.N, 10, 64)
			if err != nil {
				continue
			}

			if ttlVal <= now {
				keysToDelete = append(keysToDelete, key)
			}
		}

		for _, key := range keysToDelete {
			delete(td.Items, key)
		}
	}

	m.saveLocked()
}

// MarshalJSON serializes the storage state to JSON.
func (m *MemoryStorage) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(m)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (m *MemoryStorage) UnmarshalJSON(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(m)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if m.Tables == nil {
		m.Tables = make(map[string]*tableData)
	}

	if m.Tags == nil {
		m.Tags = make(map[string][]Tag)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "dynamodb", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "dynamodb", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateTable creates a new table.
//
//nolint:funlen // Table creation with stream setup.
func (m *MemoryStorage) CreateTable(_ context.Context, req *CreateTableRequest) (*Table, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Tables[req.TableName]; exists {
		return nil, &TableError{
			Code:    "ResourceInUseException",
			Message: fmt.Sprintf("Table already exists: %s", req.TableName),
		}
	}

	billingMode := req.BillingMode
	if billingMode == "" {
		billingMode = "PROVISIONED"
	}

	table := &Table{
		Name:                   req.TableName,
		KeySchema:              req.KeySchema,
		AttributeDefinitions:   req.AttributeDefinitions,
		ProvisionedThroughput:  req.ProvisionedThroughput,
		GlobalSecondaryIndexes: req.GlobalSecondaryIndexes,
		LocalSecondaryIndexes:  req.LocalSecondaryIndexes,
		CreationDateTime:       time.Now(),
		TableStatus:            "ACTIVE",
		ItemCount:              0,
		TableSizeBytes:         0,
		TableARN:               fmt.Sprintf("arn:aws:dynamodb:%s:%s:table/%s", m.region, defaultAccountID, req.TableName),
		BillingMode:            billingMode,
		DeletionProtection:     req.DeletionProtectionEnabled,
	}

	if req.StreamSpecification != nil && req.StreamSpecification.StreamEnabled {
		table.StreamEnabled = true
		table.StreamViewType = req.StreamSpecification.StreamViewType
		table.LatestStreamArn = fmt.Sprintf("%s/stream/%s", table.TableARN, time.Now().Format("2006-01-02T15:04:05.000"))

		// Register the stream in the shared event store so DynamoDB Streams can read it.
		keySchema := make([]streams.KeySchemaElement, len(req.KeySchema))
		for i, ks := range req.KeySchema {
			keySchema[i] = streams.KeySchemaElement{
				AttributeName: ks.AttributeName,
				KeyType:       ks.KeyType,
			}
		}

		m.streamStore.RegisterStream(&streams.StreamInfo{
			StreamARN:      table.LatestStreamArn,
			TableName:      table.Name,
			StreamViewType: table.StreamViewType,
			StreamLabel:    time.Now().Format("2006-01-02T15:04:05.000"),
			StreamStatus:   "ENABLED",
			KeySchema:      keySchema,
			CreationTime:   time.Now(),
		})
	}

	m.Tables[req.TableName] = &tableData{
		Table: table,
		Items: make(map[string]Item),
	}

	m.saveLocked()

	return table, nil
}

// DeleteTable deletes a table.
func (m *MemoryStorage) DeleteTable(_ context.Context, tableName string) (*Table, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	td, exists := m.Tables[tableName]
	if !exists {
		return nil, &TableError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("Requested resource not found: Table: %s not found", tableName),
		}
	}

	table := td.Table
	table.TableStatus = "DELETING"

	delete(m.Tables, tableName)

	m.saveLocked()

	return table, nil
}

// ListTables lists all tables.
func (m *MemoryStorage) ListTables(_ context.Context, exclusiveStartTableName string, limit int) ([]string, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	names := make([]string, 0, len(m.Tables))
	for name := range m.Tables {
		names = append(names, name)
	}

	sort.Strings(names)

	// Apply exclusive start.
	startIdx := 0

	if exclusiveStartTableName != "" {
		for i, name := range names {
			if name > exclusiveStartTableName {
				startIdx = i

				break
			}
		}
	}

	// Apply limit.
	endIdx := startIdx + limit
	if endIdx > len(names) {
		endIdx = len(names)
	}

	result := names[startIdx:endIdx]

	var lastEvaluated string
	if endIdx < len(names) {
		lastEvaluated = result[len(result)-1]
	}

	return result, lastEvaluated, nil
}

// DescribeTable describes a table.
func (m *MemoryStorage) DescribeTable(_ context.Context, tableName string) (*Table, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	td, exists := m.Tables[tableName]
	if !exists {
		return nil, &TableError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("Requested resource not found: Table: %s not found", tableName),
		}
	}

	// Update item count.
	td.Table.ItemCount = int64(len(td.Items))

	return td.Table, nil
}

// PutItem puts an item into a table.
func (m *MemoryStorage) PutItem(_ context.Context, tableName string, item Item, returnOld bool, cond ConditionInput) (Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	td, exists := m.Tables[tableName]
	if !exists {
		return nil, &TableError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("Requested resource not found: Table: %s not found", tableName),
		}
	}

	if err := validateItemKey(td.Table, item); err != nil {
		return nil, err
	}

	key := m.serializeKey(td.Table, item)

	// Evaluate condition against existing item (nil if not exists).
	var existingItem Item
	if existing, ok := td.Items[key]; ok {
		existingItem = existing
	}

	if ok, err := evaluateCondition(existingItem, cond); err != nil {
		return nil, &TableError{
			Code:    "ValidationException",
			Message: fmt.Sprintf("Invalid ConditionExpression: %s", err),
		}
	} else if !ok {
		return nil, &TableError{
			Code:    ErrCodeConditionalCheckFailed,
			Message: "The conditional request failed",
		}
	}

	var oldItem Item

	if returnOld && existingItem != nil {
		oldItem = m.copyItem(existingItem)
	}

	td.Items[key] = m.copyItem(item)

	// Emit stream event if streams are enabled for this table.
	if td.Table.StreamEnabled && td.Table.LatestStreamArn != "" {
		eventName := streams.OperationTypeInsert
		if existingItem != nil {
			eventName = streams.OperationTypeModify
		}

		m.emitStreamEvent(td.Table, eventName, m.extractKey(td.Table, item), existingItem, item)
	}

	m.saveLocked()

	return oldItem, nil
}

// GetItem gets an item from a table.
func (m *MemoryStorage) GetItem(_ context.Context, tableName string, key Item) (Item, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	td, exists := m.Tables[tableName]
	if !exists {
		return nil, &TableError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("Requested resource not found: Table: %s not found", tableName),
		}
	}

	if err := validateKey(td.Table, key); err != nil {
		return nil, err
	}

	keyStr := m.serializeKey(td.Table, key)
	if item, ok := td.Items[keyStr]; ok {
		return m.copyItem(item), nil
	}

	//nolint:nilnil // DynamoDB returns nil item when key not found (valid behavior).
	return nil, nil
}

// DeleteItem deletes an item from a table.
func (m *MemoryStorage) DeleteItem(_ context.Context, tableName string, key Item, returnOld bool, cond ConditionInput) (Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	td, exists := m.Tables[tableName]
	if !exists {
		return nil, &TableError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("Requested resource not found: Table: %s not found", tableName),
		}
	}

	if err := validateKey(td.Table, key); err != nil {
		return nil, err
	}

	keyStr := m.serializeKey(td.Table, key)

	// Evaluate condition against existing item.
	var existingItem Item
	if existing, ok := td.Items[keyStr]; ok {
		existingItem = existing
	}

	if ok, err := evaluateCondition(existingItem, cond); err != nil {
		return nil, &TableError{
			Code:    "ValidationException",
			Message: fmt.Sprintf("Invalid ConditionExpression: %s", err),
		}
	} else if !ok {
		return nil, &TableError{
			Code:    ErrCodeConditionalCheckFailed,
			Message: "The conditional request failed",
		}
	}

	var oldItem Item

	if existingItem != nil {
		if returnOld {
			oldItem = m.copyItem(existingItem)
		}

		delete(td.Items, keyStr)

		// Emit stream event if streams are enabled for this table.
		if td.Table.StreamEnabled && td.Table.LatestStreamArn != "" {
			m.emitStreamEvent(td.Table, streams.OperationTypeRemove, key, existingItem, nil)
		}
	}

	m.saveLocked()

	return oldItem, nil
}

// UpdateItem updates an item in a table.
//
//nolint:funlen,cyclop // UpdateItem keeps validation, condition evaluation, key-attribute enforcement, mutation, and return value handling together.
func (m *MemoryStorage) UpdateItem(_ context.Context, tableName string, key Item, updateExpr string, exprNames map[string]string, exprValues map[string]AttributeValue, returnValues string, cond ConditionInput) (Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	td, exists := m.Tables[tableName]
	if !exists {
		return nil, &TableError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("Requested resource not found: Table: %s not found", tableName),
		}
	}

	if err := validateKey(td.Table, key); err != nil {
		return nil, err
	}

	keyStr := m.serializeKey(td.Table, key)
	item, itemExists := td.Items[keyStr]

	// Evaluate condition against existing item.
	var condItem Item
	if itemExists {
		condItem = item
	}

	if ok, err := evaluateCondition(condItem, cond); err != nil {
		return nil, &TableError{
			Code:    "ValidationException",
			Message: fmt.Sprintf("Invalid ConditionExpression: %s", err),
		}
	} else if !ok {
		return nil, &TableError{
			Code:    ErrCodeConditionalCheckFailed,
			Message: "The conditional request failed",
		}
	}

	var oldItem Item
	if itemExists {
		oldItem = m.copyItem(item)
	} else {
		// Create new item with key attributes.
		item = m.copyItem(key)
	}

	// Parse and apply update expression.
	if updateExpr != "" {
		item = m.applyUpdateExpression(item, updateExpr, exprNames, exprValues)
	}

	td.Items[keyStr] = item

	// Emit stream event if streams are enabled for this table.
	if td.Table.StreamEnabled && td.Table.LatestStreamArn != "" {
		eventName := streams.OperationTypeInsert
		if itemExists {
			eventName = streams.OperationTypeModify
		}

		m.emitStreamEvent(td.Table, eventName, key, oldItem, item)
	}

	m.saveLocked()

	// Return based on returnValues.
	switch returnValues {
	case ReturnValuesAllOld:
		return oldItem, nil
	case ReturnValuesAllNew:
		return m.copyItem(item), nil
	case ReturnValuesUpdatedOld, ReturnValuesUpdatedNew:
		// Simplified: return all attributes.
		if returnValues == ReturnValuesUpdatedOld {
			return oldItem, nil
		}

		return m.copyItem(item), nil
	default:
		//nolint:nilnil // DynamoDB returns nil when ReturnValues is NONE (valid behavior).
		return nil, nil
	}
}

// resolveKeySchema returns the key schema for the given index name.
// If indexName is empty, the table's key schema is returned.
// It searches both GSIs and LSIs.
func resolveKeySchema(table *Table, indexName string) ([]KeySchemaElement, error) {
	if indexName == "" {
		return table.KeySchema, nil
	}

	for _, gsi := range table.GlobalSecondaryIndexes {
		if gsi.IndexName == indexName {
			return gsi.KeySchema, nil
		}
	}

	for _, lsi := range table.LocalSecondaryIndexes {
		if lsi.IndexName == indexName {
			return lsi.KeySchema, nil
		}
	}

	return nil, &TableError{
		Code:    "ValidationException",
		Message: fmt.Sprintf("The table does not have the specified index: %s", indexName),
	}
}

// Query queries items from a table.
//
//nolint:cyclop,funlen,gocognit // Query has inherent complexity from DynamoDB protocol requirements.
func (m *MemoryStorage) Query(_ context.Context, tableName, indexName, keyCondExpr, filterExpr string, exprNames map[string]string, exprValues map[string]AttributeValue, limit int, exclusiveStartKey Item, scanForward bool) ([]Item, Item, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	td, exists := m.Tables[tableName]
	if !exists {
		return nil, nil, 0, &TableError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("Requested resource not found: Table: %s not found", tableName),
		}
	}

	// Determine key schema to use (table, GSI, or LSI).
	keySchema, err := resolveKeySchema(td.Table, indexName)
	if err != nil {
		return nil, nil, 0, err
	}

	// Get partition key attribute name from the resolved key schema.
	var partitionKeyName string

	for _, ks := range keySchema {
		if ks.KeyType == "HASH" {
			partitionKeyName = ks.AttributeName

			break
		}
	}

	// Parse key condition to extract partition key value.
	partitionKeyValue := m.extractPartitionKeyValue(keyCondExpr, partitionKeyName, exprNames, exprValues)

	// Resolve expression attribute names in key condition.
	resolvedKeyCondExpr := keyCondExpr
	for placeholder, name := range exprNames {
		resolvedKeyCondExpr = strings.ReplaceAll(resolvedKeyCondExpr, placeholder, name)
	}

	// Validate expressions up front so unparseable ones are rejected even when no
	// item is evaluated (empty table or no key match).
	if resolvedKeyCondExpr != "" {
		if _, err := evaluateCondition(Item{}, ConditionInput{Expression: resolvedKeyCondExpr, ExprValues: exprValues}); err != nil {
			return nil, nil, 0, invalidKeyConditionExpression(err)
		}
	}

	if err := m.validateFilterExpression(filterExpr, exprNames, exprValues); err != nil {
		return nil, nil, 0, err
	}

	// Collect matching items.
	var results []Item

	scannedCount := 0

	for _, item := range td.Items {
		scannedCount++

		// Check partition key match.
		if partitionKeyValue != nil {
			if itemVal, ok := item[partitionKeyName]; ok {
				if !m.attributeValuesEqual(itemVal, *partitionKeyValue) {
					continue
				}
			} else {
				continue
			}
		}

		// Evaluate full key condition expression (includes RANGE key conditions like >=, BETWEEN, begins_with).
		if resolvedKeyCondExpr != "" {
			keyCond := ConditionInput{
				Expression: resolvedKeyCondExpr,
				ExprValues: exprValues,
			}

			ok, err := evaluateCondition(item, keyCond)
			if err != nil {
				return nil, nil, 0, invalidKeyConditionExpression(err)
			}

			if !ok {
				continue
			}
		}

		// Apply filter expression.
		match, err := m.filterItem(item, filterExpr, exprNames, exprValues)
		if err != nil {
			return nil, nil, 0, err
		}

		if !match {
			continue
		}

		results = append(results, m.copyItem(item))
	}

	// Sort results by sort key (DynamoDB Query always returns sorted by sort key).
	var sortKeyName string

	for _, ks := range keySchema {
		if ks.KeyType == "RANGE" {
			sortKeyName = ks.AttributeName

			break
		}
	}

	if sortKeyName != "" {
		sort.Slice(results, func(i, j int) bool {
			vi := results[i][sortKeyName]
			vj := results[j][sortKeyName]

			return m.compareForSort(&vi, &vj)
		})
	}

	if !scanForward {
		for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
			results[i], results[j] = results[j], results[i]
		}
	}

	// Apply pagination.
	startIdx := 0

	if exclusiveStartKey != nil {
		startKeyStr := m.serializeKey(td.Table, exclusiveStartKey)

		for i, item := range results {
			itemKeyStr := m.serializeKey(td.Table, item)
			if itemKeyStr == startKeyStr {
				startIdx = i + 1

				break
			}
		}
	}

	if startIdx >= len(results) {
		return []Item{}, nil, scannedCount, nil
	}

	results = results[startIdx:]

	var lastEvaluatedKey Item

	if limit > 0 && len(results) > limit {
		results = results[:limit]
		lastEvaluatedKey = m.extractKey(td.Table, results[len(results)-1])
	}

	return results, lastEvaluatedKey, scannedCount, nil
}

// Scan scans items from a table.
//
//nolint:funlen // Scan requires pagination logic that exceeds line limit.
func (m *MemoryStorage) Scan(_ context.Context, tableName, filterExpr string, exprNames map[string]string, exprValues map[string]AttributeValue, limit int, exclusiveStartKey Item, segment, totalSegments *int) ([]Item, Item, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	td, exists := m.Tables[tableName]
	if !exists {
		return nil, nil, 0, &TableError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("Requested resource not found: Table: %s not found", tableName),
		}
	}

	if err := validateScanSegment(segment, totalSegments); err != nil {
		return nil, nil, 0, err
	}

	// Validate the filter up front so an unparseable FilterExpression is rejected
	// even when the table is empty, instead of returning an empty result.
	if err := m.validateFilterExpression(filterExpr, exprNames, exprValues); err != nil {
		return nil, nil, 0, err
	}

	// Collect all items matching the segment and filter.
	results, scannedCount, err := m.scanCollectMatches(td, filterExpr, exprNames, exprValues, segment, totalSegments)
	if err != nil {
		return nil, nil, 0, err
	}

	// Sort by key for consistent pagination. Pre-compute keys once so the
	// sort comparator and the pagination lookup don't re-serialize each item
	// on every comparison (was N log N serializeKey calls; now N).
	type keyedItem struct {
		key  string
		item Item
	}

	pairs := make([]keyedItem, len(results))

	for i, it := range results {
		pairs[i] = keyedItem{key: m.serializeKey(td.Table, it), item: it}
	}

	sort.Slice(pairs, func(i, j int) bool { return pairs[i].key < pairs[j].key })

	// Apply pagination.
	startIdx := 0

	if exclusiveStartKey != nil {
		startKeyStr := m.serializeKey(td.Table, exclusiveStartKey)
		for i, p := range pairs {
			if p.key == startKeyStr {
				startIdx = i + 1

				break
			}
		}
	}

	results = results[:0:len(pairs)]
	for _, p := range pairs {
		results = append(results, p.item)
	}

	if startIdx >= len(results) {
		return []Item{}, nil, scannedCount, nil
	}

	results = results[startIdx:]

	var lastEvaluatedKey Item

	if limit > 0 && len(results) > limit {
		results = results[:limit]
		lastEvaluatedKey = m.extractKey(td.Table, results[len(results)-1])
	}

	return results, lastEvaluatedKey, scannedCount, nil
}

// scanCollectMatches walks every item, applies parallel-scan segment filtering
// and the filter expression, and returns the matched items plus the scanned
// count. A filter parse error is returned as a ValidationException.
func (m *MemoryStorage) scanCollectMatches(td *tableData, filterExpr string, exprNames map[string]string, exprValues map[string]AttributeValue, segment, totalSegments *int) ([]Item, int, error) {
	var results []Item

	scannedCount := 0

	for _, item := range td.Items {
		key := m.serializeKey(td.Table, item)
		if !scanSegmentMatches(key, segment, totalSegments) {
			continue
		}

		scannedCount++

		match, err := m.filterItem(item, filterExpr, exprNames, exprValues)
		if err != nil {
			return nil, 0, err
		}

		if !match {
			continue
		}

		results = append(results, m.copyItem(item))
	}

	return results, scannedCount, nil
}

func validateScanSegment(segment, totalSegments *int) error {
	if segment == nil && totalSegments == nil {
		return nil
	}

	if segment == nil || totalSegments == nil {
		return &TableError{
			Code:    "ValidationException",
			Message: "Segment and TotalSegments must be specified together",
		}
	}

	if *totalSegments <= 0 {
		return &TableError{
			Code:    "ValidationException",
			Message: "TotalSegments must be greater than zero",
		}
	}

	if *segment < 0 || *segment >= *totalSegments {
		return &TableError{
			Code:    "ValidationException",
			Message: "Segment must be greater than or equal to zero and less than TotalSegments",
		}
	}

	return nil
}

func scanSegmentMatches(key string, segment, totalSegments *int) bool {
	if segment == nil || totalSegments == nil {
		return true
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(key))

	//nolint:gosec // totalSegments is validated to be positive before this helper is called.
	return int(h.Sum64()%uint64(*totalSegments)) == *segment
}

// serializeKey creates a string key from the primary key attributes.
func (m *MemoryStorage) serializeKey(table *Table, item Item) string {
	var parts []string

	for _, ks := range table.KeySchema {
		if val, ok := item[ks.AttributeName]; ok {
			parts = append(parts, m.serializeAttributeValue(val))
		}
	}

	return strings.Join(parts, "|")
}

// serializeAttributeValue serializes an attribute value to a string.
//
//nolint:gocritic // hugeParam: AttributeValue must be passed by value to avoid mutation.
func (m *MemoryStorage) serializeAttributeValue(av AttributeValue) string {
	if av.S != nil {
		return "S:" + *av.S
	}

	if av.N != nil {
		return "N:" + *av.N
	}

	if av.B != nil {
		return "B:" + string(av.B)
	}

	return "NULL:" + uuid.New().String()
}

func validateItemKey(table *Table, item Item) error {
	return validateKeyAttributes(table, item, false)
}

func validateKey(table *Table, key Item) error {
	return validateKeyAttributes(table, key, true)
}

func validateKeyAttributes(table *Table, item Item, keyOnly bool) error {
	if item == nil {
		return newKeySchemaValidationException()
	}

	if keyOnly && len(item) != len(table.KeySchema) {
		return newKeySchemaValidationException()
	}

	attrTypes := keyAttributeTypes(table)

	for _, ks := range table.KeySchema {
		av, ok := item[ks.AttributeName]
		if !ok {
			return newKeySchemaValidationException()
		}

		if !matchesKeyAttributeType(av, attrTypes[ks.AttributeName]) {
			return newKeySchemaValidationException()
		}
	}

	return nil
}

func keyAttributeTypes(table *Table) map[string]string {
	types := make(map[string]string, len(table.AttributeDefinitions))
	for _, def := range table.AttributeDefinitions {
		types[def.AttributeName] = def.AttributeType
	}

	return types
}

//nolint:gocritic // AttributeValue is passed by value consistently in storage helpers.
func matchesKeyAttributeType(av AttributeValue, attrType string) bool {
	switch attrType {
	case "S":
		return av.S != nil
	case "N":
		return av.N != nil
	case "B":
		return av.B != nil
	default:
		return false
	}
}

func newKeySchemaValidationException() *TableError {
	return newValidationException("The provided key element does not match the schema")
}

func newValidationException(message string) *TableError {
	return &TableError{Code: "ValidationException", Message: message}
}

// copyItem creates a deep copy of an item.
//
//nolint:gocritic // rangeValCopy: intentional copy for deep clone operation.
func (m *MemoryStorage) copyItem(item Item) Item {
	if item == nil {
		return nil
	}

	result := make(Item)

	for k, v := range item {
		result[k] = m.copyAttributeValue(v)
	}

	return result
}

// copyAttributeValue creates a deep copy of an attribute value.
//
//nolint:funlen,gocritic // Deep copy of all AttributeValue fields requires many statements.
func (m *MemoryStorage) copyAttributeValue(av AttributeValue) AttributeValue {
	result := AttributeValue{}

	if av.S != nil {
		s := *av.S
		result.S = &s
	}

	if av.N != nil {
		n := *av.N
		result.N = &n
	}

	if av.B != nil {
		b := make([]byte, len(av.B))
		copy(b, av.B)
		result.B = b
	}

	if av.SS != nil {
		ss := make([]string, len(av.SS))
		copy(ss, av.SS)
		result.SS = ss
	}

	if av.NS != nil {
		ns := make([]string, len(av.NS))
		copy(ns, av.NS)
		result.NS = ns
	}

	if av.BS != nil {
		bs := make([][]byte, len(av.BS))
		for i, b := range av.BS {
			bs[i] = make([]byte, len(b))
			copy(bs[i], b)
		}

		result.BS = bs
	}

	if av.M != nil {
		mapCopy := make(map[string]*AttributeValue)

		for k, v := range av.M {
			copied := m.copyAttributeValue(*v)
			mapCopy[k] = &copied
		}

		result.M = mapCopy
	}

	if av.L != nil {
		listCopy := make([]*AttributeValue, len(av.L))

		for i, v := range av.L {
			copied := m.copyAttributeValue(*v)
			listCopy[i] = &copied
		}

		result.L = listCopy
	}

	if av.NULL != nil {
		n := *av.NULL
		result.NULL = &n
	}

	if av.BOOL != nil {
		b := *av.BOOL
		result.BOOL = &b
	}

	return result
}

// extractKey extracts the primary key from an item.
func (m *MemoryStorage) extractKey(table *Table, item Item) Item {
	key := make(Item)

	for _, ks := range table.KeySchema {
		if val, ok := item[ks.AttributeName]; ok {
			key[ks.AttributeName] = val
		}
	}

	return key
}

// extractPartitionKeyValue extracts the partition key value from a key condition expression.
func (m *MemoryStorage) extractPartitionKeyValue(keyCondExpr, partitionKeyName string, exprNames map[string]string, exprValues map[string]AttributeValue) *AttributeValue {
	if keyCondExpr == "" {
		return nil
	}

	// Simple parsing: look for "attrName = :value" pattern.
	// Replace expression attribute names.
	expr := keyCondExpr
	for placeholder, name := range exprNames {
		expr = strings.ReplaceAll(expr, placeholder, name)
	}

	// Look for partition key equality.
	parts := strings.Split(expr, " AND ")

	for _, part := range parts {
		part = strings.TrimSpace(part)

		//nolint:nestif // Parsing key condition expression requires nested validation.
		if strings.Contains(part, "=") {
			eqParts := strings.SplitN(part, "=", 2)
			if len(eqParts) == 2 {
				attrName := strings.TrimSpace(eqParts[0])
				valuePlaceholder := strings.TrimSpace(eqParts[1])

				if attrName == partitionKeyName {
					if val, ok := exprValues[valuePlaceholder]; ok {
						return &val
					}
				}
			}
		}
	}

	return nil
}

// filterItem reports whether an item passes a filter expression, where an
// empty expression passes everything.
func (m *MemoryStorage) filterItem(item Item, filterExpr string, exprNames map[string]string, exprValues map[string]AttributeValue) (bool, error) {
	if filterExpr == "" {
		return true, nil
	}

	return m.evaluateFilterExpression(item, filterExpr, exprNames, exprValues)
}

// validateFilterExpression rejects an unparseable FilterExpression up front, so
// the error surfaces even when no item is evaluated (empty table or zero key
// matches) rather than only when an item happens to be scanned.
func (m *MemoryStorage) validateFilterExpression(filterExpr string, exprNames map[string]string, exprValues map[string]AttributeValue) error {
	if filterExpr == "" {
		return nil
	}

	// evaluateFilterExpression returns a ValidationException TableError on a
	// parse error; the match result against the synthetic item is discarded.
	_, err := m.evaluateFilterExpression(Item{}, filterExpr, exprNames, exprValues)

	return err
}

// invalidKeyConditionExpression builds the ValidationException returned for an
// unparseable KeyConditionExpression.
func invalidKeyConditionExpression(err error) *TableError {
	return &TableError{
		Code:    "ValidationException",
		Message: fmt.Sprintf("Invalid KeyConditionExpression: %s", err),
	}
}

// evaluateFilterExpression evaluates a filter expression against an item. An
// expression the parser cannot handle is a ValidationException, matching
// DynamoDB; it must never silently match (or filter out) items.
func (m *MemoryStorage) evaluateFilterExpression(item Item, filterExpr string, exprNames map[string]string, exprValues map[string]AttributeValue) (bool, error) {
	result, err := evaluateCondition(item, ConditionInput{
		Expression: filterExpr,
		ExprNames:  exprNames,
		ExprValues: exprValues,
	})
	if err != nil {
		return false, &TableError{
			Code:    "ValidationException",
			Message: fmt.Sprintf("Invalid FilterExpression: %s", err),
		}
	}

	return result, nil
}

// attributeValuesEqual compares two attribute values for equality.
//
//nolint:gocritic // hugeParam: AttributeValue passed by value for comparison.
func (m *MemoryStorage) attributeValuesEqual(a, b AttributeValue) bool {
	if a.S != nil && b.S != nil {
		return *a.S == *b.S
	}

	if a.N != nil && b.N != nil {
		return *a.N == *b.N
	}

	if a.BOOL != nil && b.BOOL != nil {
		return *a.BOOL == *b.BOOL
	}

	return false
}

// applyUpdateExpression applies an update expression to an item.
// Supports SET, ADD, DELETE, and REMOVE clauses.
func (m *MemoryStorage) applyUpdateExpression(item Item, updateExpr string, exprNames map[string]string, exprValues map[string]AttributeValue) Item {
	// Replace expression attribute names.
	expr := updateExpr
	for placeholder, name := range exprNames {
		expr = strings.ReplaceAll(expr, placeholder, name)
	}

	// Split expression into clauses (SET, ADD, DELETE, REMOVE).
	clauses := parseUpdateClauses(expr)

	for _, clause := range clauses {
		switch clause.action {
		case "SET":
			item = applySetClause(item, clause.body, exprValues)
		case "ADD":
			item = applyAddClause(item, clause.body, exprValues)
		case "DELETE":
			item = applyDeleteClause(item, clause.body, exprValues)
		case "REMOVE":
			item = applyRemoveClause(item, clause.body)
		}
	}

	return item
}

type updateClause struct {
	action string // SET, ADD, DELETE, REMOVE
	body   string
}

// parseUpdateClauses splits an update expression into individual clauses.
func parseUpdateClauses(expr string) []updateClause {
	keywords := []string{"SET", "ADD", "DELETE", "REMOVE"}
	upper := asciiUpper(expr)

	type pos struct {
		idx    int
		action string
	}

	var positions []pos

	for _, kw := range keywords {
		idx := 0

		for {
			found := strings.Index(upper[idx:], kw)
			if found == -1 {
				break
			}

			absIdx := idx + found

			// Ensure it's a keyword boundary (start of string or preceded by space).
			if absIdx == 0 || upper[absIdx-1] == ' ' {
				// Ensure it's followed by a space or end of string.
				endIdx := absIdx + len(kw)
				if endIdx >= len(upper) || upper[endIdx] == ' ' {
					positions = append(positions, pos{idx: absIdx, action: kw})
				}
			}

			idx = absIdx + len(kw)
		}
	}

	// Sort by position.
	sort.Slice(positions, func(i, j int) bool {
		return positions[i].idx < positions[j].idx
	})

	clauses := make([]updateClause, 0, len(positions))

	for i, p := range positions {
		start := p.idx + len(p.action)

		end := len(expr)
		if i+1 < len(positions) {
			end = positions[i+1].idx
		}

		body := strings.TrimSpace(expr[start:end])
		clauses = append(clauses, updateClause{action: p.action, body: body})
	}

	return clauses
}

func asciiUpper(s string) string {
	out := []byte(s)
	for i, b := range out {
		if b >= 'a' && b <= 'z' {
			out[i] = b - ('a' - 'A')
		}
	}

	return string(out)
}

// applySetClause handles SET attr = :val, SET attr = if_not_exists(attr, :val).
func applySetClause(item Item, clause string, exprValues map[string]AttributeValue) Item {
	assignments := splitAssignments(clause)
	for _, assignment := range assignments {
		parts := strings.SplitN(strings.TrimSpace(assignment), "=", 2)
		if len(parts) != 2 {
			continue
		}

		attrName := strings.TrimSpace(parts[0])
		valueExpr := strings.TrimSpace(parts[1])

		// Handle if_not_exists(attr, :val) — but only if not combined with arithmetic.
		if strings.HasPrefix(valueExpr, "if_not_exists(") && !containsArithmeticOp(valueExpr) {
			applyIfNotExists(item, attrName, valueExpr, exprValues)

			continue
		}

		// Handle arithmetic: path + :val, path - :val, if_not_exists(...) + :val
		if val, ok := evaluateSetArithmetic(item, valueExpr, exprValues); ok {
			item[attrName] = val

			continue
		}

		if val, ok := exprValues[valueExpr]; ok {
			item[attrName] = val
		}
	}

	return item
}

// splitAssignments splits a SET clause into individual assignments, respecting parentheses.
func splitAssignments(clause string) []string {
	var result []string

	depth := 0
	start := 0

	for i, ch := range clause {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				result = append(result, clause[start:i])
				start = i + 1
			}
		}
	}

	result = append(result, clause[start:])

	return result
}

// containsArithmeticOp checks if expression contains + or - operators outside of parentheses.
func containsArithmeticOp(expr string) bool {
	depth := 0

	for i, ch := range expr {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		case '+', '-':
			if depth == 0 && i > 0 && expr[i-1] == ' ' {
				return true
			}
		}
	}

	return false
}

// evaluateSetArithmetic handles "path + :val" and "path - :val" expressions.
func evaluateSetArithmetic(item Item, expr string, exprValues map[string]AttributeValue) (AttributeValue, bool) {
	for _, op := range []string{" + ", " - "} {
		idx := strings.Index(expr, op)
		if idx == -1 {
			continue
		}

		leftToken := strings.TrimSpace(expr[:idx])
		rightToken := strings.TrimSpace(expr[idx+len(op):])

		left := resolveSetOperand(item, leftToken, exprValues)
		right := resolveSetOperand(item, rightToken, exprValues)

		if left.N == nil || right.N == nil {
			return AttributeValue{}, false
		}

		leftNum, err1 := strconv.ParseFloat(*left.N, 64)
		rightNum, err2 := strconv.ParseFloat(*right.N, 64)

		if err1 != nil || err2 != nil {
			return AttributeValue{}, false
		}

		var result float64
		if op == " + " {
			result = leftNum + rightNum
		} else {
			result = leftNum - rightNum
		}

		resultStr := strconv.FormatFloat(result, 'f', -1, 64)

		return AttributeValue{N: &resultStr}, true
	}

	return AttributeValue{}, false
}

// resolveSetOperand resolves a token to an AttributeValue for SET expressions.
// Supports: :placeholder, path, and if_not_exists(path, :default).
func resolveSetOperand(item Item, token string, exprValues map[string]AttributeValue) AttributeValue {
	// Handle if_not_exists(path, :default)
	if strings.HasPrefix(token, "if_not_exists(") {
		return resolveIfNotExists(item, token, exprValues)
	}

	if strings.HasPrefix(token, ":") {
		if val, ok := exprValues[token]; ok {
			return val
		}

		return AttributeValue{}
	}

	if val, ok := item[token]; ok {
		return val
	}

	return AttributeValue{}
}

// resolveIfNotExists evaluates if_not_exists(path, :default) and returns the resolved value.
func resolveIfNotExists(item Item, token string, exprValues map[string]AttributeValue) AttributeValue {
	inner := strings.TrimPrefix(token, "if_not_exists(")
	inner = strings.TrimSuffix(inner, ")")

	parts := strings.SplitN(inner, ",", 2)
	if len(parts) != 2 {
		return AttributeValue{}
	}

	path := strings.TrimSpace(parts[0])
	defaultPlaceholder := strings.TrimSpace(parts[1])

	if val, ok := item[path]; ok {
		return val
	}

	if val, ok := exprValues[defaultPlaceholder]; ok {
		return val
	}

	return AttributeValue{}
}

// applyIfNotExists handles the if_not_exists(attr, :val) function within a SET clause.
func applyIfNotExists(item Item, attrName, valueExpr string, exprValues map[string]AttributeValue) {
	inner := strings.TrimPrefix(valueExpr, "if_not_exists(")
	inner = strings.TrimSuffix(inner, ")")

	ifParts := strings.SplitN(inner, ",", 2)
	if len(ifParts) != 2 {
		return
	}

	checkAttr := strings.TrimSpace(ifParts[0])
	if _, exists := item[checkAttr]; exists {
		return
	}

	defaultPlaceholder := strings.TrimSpace(ifParts[1])

	if val, ok := exprValues[defaultPlaceholder]; ok {
		item[attrName] = val
	}
}

// applyAddClause handles ADD attr :val.
// For numbers: atomically increments the value.
// For sets (SS, NS, BS): adds elements to the set.
func applyAddClause(item Item, clause string, exprValues map[string]AttributeValue) Item {
	actions := strings.Split(clause, ",")
	for _, action := range actions {
		parts := strings.Fields(strings.TrimSpace(action))
		if len(parts) != 2 {
			continue
		}

		attrName := parts[0]
		valuePlaceholder := parts[1]

		val, ok := exprValues[valuePlaceholder]
		if !ok {
			continue
		}

		existing, exists := item[attrName]
		item[attrName] = addAttributeValue(&existing, exists, &val)
	}

	return item
}

// addAttributeValue merges a new value into an existing attribute for the ADD operation.
func addAttributeValue(existing *AttributeValue, exists bool, val *AttributeValue) AttributeValue {
	switch {
	case val.N != nil:
		if !exists || existing.N == nil {
			return *val
		}

		result := addNumbers(*existing.N, *val.N)

		return AttributeValue{N: &result}

	case len(val.SS) > 0:
		if !exists || len(existing.SS) == 0 {
			return *val
		}

		return AttributeValue{SS: mergeStringSet(existing.SS, val.SS)}

	case len(val.NS) > 0:
		if !exists || len(existing.NS) == 0 {
			return *val
		}

		return AttributeValue{NS: mergeStringSet(existing.NS, val.NS)}

	case len(val.BS) > 0:
		if !exists || len(existing.BS) == 0 {
			return *val
		}

		return AttributeValue{BS: append(existing.BS, val.BS...)}

	default:
		return *val
	}
}

// applyDeleteClause handles DELETE attr :val.
// Removes elements from a set (SS, NS, BS).
func applyDeleteClause(item Item, clause string, exprValues map[string]AttributeValue) Item {
	actions := strings.Split(clause, ",")
	for _, action := range actions {
		parts := strings.Fields(strings.TrimSpace(action))
		if len(parts) != 2 {
			continue
		}

		attrName := parts[0]
		valuePlaceholder := parts[1]

		val, ok := exprValues[valuePlaceholder]
		if !ok {
			continue
		}

		existing, exists := item[attrName]
		if !exists {
			continue
		}

		switch {
		// DELETE from StringSet
		case len(val.SS) > 0 && len(existing.SS) > 0:
			remaining := subtractStringSet(existing.SS, val.SS)
			if len(remaining) == 0 {
				delete(item, attrName)
			} else {
				item[attrName] = AttributeValue{SS: remaining}
			}

		// DELETE from NumberSet
		case len(val.NS) > 0 && len(existing.NS) > 0:
			remaining := subtractStringSet(existing.NS, val.NS)
			if len(remaining) == 0 {
				delete(item, attrName)
			} else {
				item[attrName] = AttributeValue{NS: remaining}
			}
		}
	}

	return item
}

// applyRemoveClause handles REMOVE attr1, attr2, ...
func applyRemoveClause(item Item, clause string) Item {
	attrs := strings.Split(clause, ",")
	for _, attr := range attrs {
		delete(item, strings.TrimSpace(attr))
	}

	return item
}

// addNumbers adds two DynamoDB number strings.
func addNumbers(a, b string) string {
	fa, err1 := strconv.ParseFloat(a, 64)
	fb, err2 := strconv.ParseFloat(b, 64)

	if err1 != nil || err2 != nil {
		return a
	}

	result := fa + fb

	// Return integer format if result is a whole number.
	if result == float64(int64(result)) {
		return strconv.FormatInt(int64(result), 10)
	}

	return strconv.FormatFloat(result, 'f', -1, 64)
}

// mergeStringSet merges two string slices, removing duplicates.
func mergeStringSet(existing, additions []string) []string {
	set := make(map[string]struct{}, len(existing))
	for _, v := range existing {
		set[v] = struct{}{}
	}

	for _, v := range additions {
		set[v] = struct{}{}
	}

	result := make([]string, 0, len(set))
	for v := range set {
		result = append(result, v)
	}

	sort.Strings(result)

	return result
}

// subtractStringSet removes elements in removals from existing.
func subtractStringSet(existing, removals []string) []string {
	remove := make(map[string]struct{}, len(removals))
	for _, v := range removals {
		remove[v] = struct{}{}
	}

	var result []string

	for _, v := range existing {
		if _, ok := remove[v]; !ok {
			result = append(result, v)
		}
	}

	return result
}

// TransactWriteItems executes a transactional write with all-or-nothing semantics.
func (m *MemoryStorage) TransactWriteItems(_ context.Context, items []TransactWriteItem) ([]CancellationReason, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	reasons := make([]CancellationReason, len(items))
	hasFailure := false

	// Phase 1: Validate all conditions without modifying state.
	for i, twi := range items {
		reason, err := m.validateTransactWriteItem(twi)
		if err != nil {
			return nil, err
		}

		if reason != nil {
			reasons[i] = *reason
			hasFailure = true
		}
	}

	if hasFailure {
		return reasons, &TableError{
			Code:    "TransactionCanceledException",
			Message: "Transaction cancelled, please refer cancellation reasons for specific reasons [CancellationReason]",
		}
	}

	// Phase 2: Apply all mutations atomically.
	for _, twi := range items {
		m.applyTransactWriteItem(twi)
	}

	m.saveLocked()

	return nil, nil // Success: nil CancellationReasons means no failures.
}

func countTransactWriteActions(twi TransactWriteItem) int {
	count := 0
	if twi.ConditionCheck != nil {
		count++
	}

	if twi.Delete != nil {
		count++
	}

	if twi.Put != nil {
		count++
	}

	if twi.Update != nil {
		count++
	}

	return count
}

// validateTransactWriteItem validates a single write item's condition without applying changes.
func (m *MemoryStorage) validateTransactWriteItem(twi TransactWriteItem) (*CancellationReason, error) {
	if countTransactWriteActions(twi) != 1 {
		return nil, newValidationException("TransactItems member must contain exactly one action")
	}

	switch {
	case twi.Put != nil:
		td, exists := m.Tables[twi.Put.TableName]
		if !exists {
			return nil, &TableError{Code: "ResourceNotFoundException", Message: fmt.Sprintf("Table: %s not found", twi.Put.TableName)}
		}

		if err := validateItemKey(td.Table, twi.Put.Item); err != nil {
			return nil, err
		}

		return m.checkTransactCondition(twi.Put.TableName, twi.Put.Item, ConditionInput{
			Expression: twi.Put.ConditionExpression, ExprNames: twi.Put.ExpressionAttributeNames, ExprValues: twi.Put.ExpressionAttributeValues,
		})
	case twi.Delete != nil:
		td, exists := m.Tables[twi.Delete.TableName]
		if !exists {
			return nil, &TableError{Code: "ResourceNotFoundException", Message: fmt.Sprintf("Table: %s not found", twi.Delete.TableName)}
		}

		if err := validateKey(td.Table, twi.Delete.Key); err != nil {
			return nil, err
		}

		return m.checkTransactCondition(twi.Delete.TableName, twi.Delete.Key, ConditionInput{
			Expression: twi.Delete.ConditionExpression, ExprNames: twi.Delete.ExpressionAttributeNames, ExprValues: twi.Delete.ExpressionAttributeValues,
		})
	case twi.Update != nil:
		td, exists := m.Tables[twi.Update.TableName]
		if !exists {
			return nil, &TableError{Code: "ResourceNotFoundException", Message: fmt.Sprintf("Table: %s not found", twi.Update.TableName)}
		}

		if err := validateKey(td.Table, twi.Update.Key); err != nil {
			return nil, err
		}

		return m.checkTransactCondition(twi.Update.TableName, twi.Update.Key, ConditionInput{
			Expression: twi.Update.ConditionExpression, ExprNames: twi.Update.ExpressionAttributeNames, ExprValues: twi.Update.ExpressionAttributeValues,
		})
	case twi.ConditionCheck != nil:
		td, exists := m.Tables[twi.ConditionCheck.TableName]
		if !exists {
			return nil, &TableError{Code: "ResourceNotFoundException", Message: fmt.Sprintf("Table: %s not found", twi.ConditionCheck.TableName)}
		}

		if err := validateKey(td.Table, twi.ConditionCheck.Key); err != nil {
			return nil, err
		}

		return m.checkTransactCondition(twi.ConditionCheck.TableName, twi.ConditionCheck.Key, ConditionInput{
			Expression: twi.ConditionCheck.ConditionExpression, ExprNames: twi.ConditionCheck.ExpressionAttributeNames, ExprValues: twi.ConditionCheck.ExpressionAttributeValues,
		})
	}

	return nil, newValidationException("TransactItems member must contain exactly one action")
}

// checkTransactCondition checks a condition against the existing item in a table.
// Must be called under lock.
func (m *MemoryStorage) checkTransactCondition(tableName string, keyOrItem Item, cond ConditionInput) (*CancellationReason, error) {
	td, exists := m.Tables[tableName]
	if !exists {
		return nil, &TableError{Code: "ResourceNotFoundException", Message: fmt.Sprintf("Table: %s not found", tableName)}
	}

	key := m.serializeKey(td.Table, keyOrItem)

	var existing Item
	if e, ok := td.Items[key]; ok {
		existing = e
	}

	ok, err := evaluateCondition(existing, cond)
	if err != nil {
		return &CancellationReason{Code: "ValidationError", Message: err.Error()}, nil //nolint:nilerr // Condition error is returned as cancellation reason, not as error.
	}

	if !ok {
		return &CancellationReason{Code: "ConditionalCheckFailed"}, nil
	}

	return nil, nil //nolint:nilnil // Condition passed, no cancellation reason.
}

// applyTransactWriteItem applies a single write item mutation. Must be called under lock.
func (m *MemoryStorage) applyTransactWriteItem(twi TransactWriteItem) {
	switch {
	case twi.Put != nil:
		td := m.Tables[twi.Put.TableName]
		key := m.serializeKey(td.Table, twi.Put.Item)
		td.Items[key] = m.copyItem(twi.Put.Item)

	case twi.Delete != nil:
		td := m.Tables[twi.Delete.TableName]
		key := m.serializeKey(td.Table, twi.Delete.Key)
		delete(td.Items, key)

	case twi.Update != nil:
		td := m.Tables[twi.Update.TableName]
		key := m.serializeKey(td.Table, twi.Update.Key)

		item, ok := td.Items[key]
		if !ok {
			item = m.copyItem(twi.Update.Key)
		}

		if twi.Update.UpdateExpression != "" {
			item = m.applyUpdateExpression(item, twi.Update.UpdateExpression, twi.Update.ExpressionAttributeNames, twi.Update.ExpressionAttributeValues)
		}

		td.Items[key] = item
	case twi.ConditionCheck != nil:
	}
}

// TransactGetItems retrieves multiple items transactionally.
func (m *MemoryStorage) TransactGetItems(_ context.Context, items []TransactGetItem) ([]Item, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]Item, len(items))

	for i, tgi := range items {
		if tgi.Get == nil {
			return nil, newValidationException("TransactGetItems member must contain Get")
		}

		td, exists := m.Tables[tgi.Get.TableName]
		if !exists {
			return nil, &TableError{
				Code:    "ResourceNotFoundException",
				Message: fmt.Sprintf("Requested resource not found: Table: %s not found", tgi.Get.TableName),
			}
		}

		if err := validateKey(td.Table, tgi.Get.Key); err != nil {
			return nil, err
		}

		key := m.serializeKey(td.Table, tgi.Get.Key)
		if item, ok := td.Items[key]; ok {
			results[i] = m.copyItem(item)
		}
	}

	return results, nil
}

// BatchWriteItem writes/deletes multiple items across tables.
func (m *MemoryStorage) BatchWriteItem(_ context.Context, requestItems map[string][]WriteRequest) (map[string][]WriteRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for tableName, requests := range requestItems {
		td, exists := m.Tables[tableName]
		if !exists {
			return nil, &TableError{
				Code:    "ResourceNotFoundException",
				Message: fmt.Sprintf("Requested resource not found: Table: %s not found", tableName),
			}
		}

		for _, req := range requests {
			if countBatchWriteActions(req) != 1 {
				return nil, newValidationException("WriteRequest must contain exactly one action")
			}

			switch {
			case req.PutRequest != nil:
				if err := validateItemKey(td.Table, req.PutRequest.Item); err != nil {
					return nil, err
				}

				key := m.serializeKey(td.Table, req.PutRequest.Item)
				td.Items[key] = m.copyItem(req.PutRequest.Item)
			case req.DeleteRequest != nil:
				if err := validateKey(td.Table, req.DeleteRequest.Key); err != nil {
					return nil, err
				}

				key := m.serializeKey(td.Table, req.DeleteRequest.Key)
				delete(td.Items, key)
			}
		}
	}

	m.saveLocked()

	// kumo processes all items; never returns UnprocessedItems.
	return nil, nil //nolint:nilnil // Intentional: nil UnprocessedItems means all items were processed.
}

func countBatchWriteActions(req WriteRequest) int {
	count := 0
	if req.PutRequest != nil {
		count++
	}

	if req.DeleteRequest != nil {
		count++
	}

	return count
}

// BatchGetItem retrieves multiple items across tables.
func (m *MemoryStorage) BatchGetItem(_ context.Context, requestItems map[string]KeysAndAttributes) (map[string][]Item, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	responses := make(map[string][]Item)

	for tableName, ka := range requestItems {
		td, exists := m.Tables[tableName]
		if !exists {
			return nil, &TableError{
				Code:    "ResourceNotFoundException",
				Message: fmt.Sprintf("Requested resource not found: Table: %s not found", tableName),
			}
		}

		var items []Item

		for _, key := range ka.Keys {
			if err := validateKey(td.Table, key); err != nil {
				return nil, err
			}

			keyStr := m.serializeKey(td.Table, key)
			if item, ok := td.Items[keyStr]; ok {
				items = append(items, m.copyItem(item))
			}
		}

		if len(items) > 0 {
			responses[tableName] = items
		}
	}

	return responses, nil
}

// UpdateTimeToLive updates the TTL configuration for a table.
func (m *MemoryStorage) UpdateTimeToLive(_ context.Context, tableName, attributeName string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	td, exists := m.Tables[tableName]
	if !exists {
		return &TableError{Code: "ResourceNotFoundException", Message: "Requested resource not found"}
	}

	td.Table.TTLAttributeName = attributeName
	td.Table.TTLEnabled = enabled

	m.saveLocked()

	return nil
}

// DescribeTimeToLive returns the TTL configuration for a table.
func (m *MemoryStorage) DescribeTimeToLive(_ context.Context, tableName string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	td, exists := m.Tables[tableName]
	if !exists {
		return "", false, &TableError{Code: "ResourceNotFoundException", Message: "Requested resource not found"}
	}

	return td.Table.TTLAttributeName, td.Table.TTLEnabled, nil
}

// compareForSort compares two AttributeValues for sorting (S or N type).
func (m *MemoryStorage) compareForSort(a, b *AttributeValue) bool {
	if a.S != nil && b.S != nil {
		return *a.S < *b.S
	}

	if a.N != nil && b.N != nil {
		var an, bn float64

		_, _ = fmt.Sscanf(*a.N, "%f", &an)
		_, _ = fmt.Sscanf(*b.N, "%f", &bn)

		return an < bn
	}

	return false
}

// emitStreamEvent publishes a stream record to the shared event store.
// Must be called under m.mu lock since it reads table metadata.
func (m *MemoryStorage) emitStreamEvent(table *Table, eventName streams.OperationType, keyItem, oldItem, newItem Item) {
	record := &streams.StreamRecord{
		EventID:        uuid.New().String(),
		EventName:      eventName,
		AwsRegion:      m.region,
		StreamViewType: table.StreamViewType,
		TableName:      table.Name,
		StreamARN:      table.LatestStreamArn,
		Keys:           convertItemToStreamAttrs(keyItem),
		SizeBytes:      100, // Approximate size.
	}

	// Include old/new images based on stream view type.
	switch table.StreamViewType {
	case "NEW_AND_OLD_IMAGES":
		record.OldImage = convertItemToStreamAttrs(oldItem)
		record.NewImage = convertItemToStreamAttrs(newItem)
	case "NEW_IMAGE":
		record.NewImage = convertItemToStreamAttrs(newItem)
	case "OLD_IMAGE":
		record.OldImage = convertItemToStreamAttrs(oldItem)
	case "KEYS_ONLY":
		// Only keys are included; already set above.
	} //nolint:wsl // Intentional empty case with comment.

	m.streamStore.PutRecord(record)
}

// convertItemToStreamAttrs converts DynamoDB Item to streams.AttributeValue map.
//
//nolint:gocritic // rangeValCopy: copy needed for value conversion.
func convertItemToStreamAttrs(item Item) map[string]streams.AttributeValue {
	if item == nil {
		return nil
	}

	result := make(map[string]streams.AttributeValue, len(item))

	for k, v := range item {
		result[k] = convertAttrToStreamAttr(v)
	}

	return result
}

// convertAttrToStreamAttr converts a single DynamoDB AttributeValue to streams.AttributeValue.
//
//nolint:funlen,gocritic // hugeParam + exhaustive type switch for all DynamoDB attribute types.
func convertAttrToStreamAttr(av AttributeValue) streams.AttributeValue {
	result := streams.AttributeValue{}

	if av.S != nil {
		s := *av.S
		result.S = &s
	}

	if av.N != nil {
		n := *av.N
		result.N = &n
	}

	if av.B != nil {
		b := make([]byte, len(av.B))
		copy(b, av.B)
		result.B = b
	}

	if av.SS != nil {
		ss := make([]string, len(av.SS))
		copy(ss, av.SS)
		result.SS = ss
	}

	if av.NS != nil {
		ns := make([]string, len(av.NS))
		copy(ns, av.NS)
		result.NS = ns
	}

	if av.BS != nil {
		bs := make([][]byte, len(av.BS))
		for i, b := range av.BS {
			bs[i] = make([]byte, len(b))
			copy(bs[i], b)
		}

		result.BS = bs
	}

	if av.M != nil {
		m := make(map[string]*streams.AttributeValue, len(av.M))

		for k, v := range av.M {
			converted := convertAttrToStreamAttr(*v)
			m[k] = &converted
		}

		result.M = m
	}

	if av.L != nil {
		l := make([]*streams.AttributeValue, len(av.L))

		for i, v := range av.L {
			converted := convertAttrToStreamAttr(*v)
			l[i] = &converted
		}

		result.L = l
	}

	if av.NULL != nil {
		n := *av.NULL
		result.NULL = &n
	}

	if av.BOOL != nil {
		b := *av.BOOL
		result.BOOL = &b
	}

	return result
}

const continuousBackupsDisabled = "DISABLED"

// ListTagsOfResource returns the tags for a given resource ARN.
func (m *MemoryStorage) ListTagsOfResource(_ context.Context, resourceArn string) ([]Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tags, ok := m.Tags[resourceArn]
	if !ok {
		return []Tag{}, nil
	}

	// Return a copy to avoid external mutation.
	result := make([]Tag, len(tags))
	copy(result, tags)

	return result, nil
}

// TagResource adds or overwrites tags on a resource ARN.
func (m *MemoryStorage) TagResource(_ context.Context, resourceArn string, tags []Tag) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing := m.Tags[resourceArn]

	// Build a map of existing tags for efficient lookup.
	tagMap := make(map[string]int, len(existing))
	for i, tag := range existing {
		tagMap[tag.Key] = i
	}

	for _, newTag := range tags {
		if idx, ok := tagMap[newTag.Key]; ok {
			existing[idx].Value = newTag.Value
		} else {
			existing = append(existing, newTag)
			tagMap[newTag.Key] = len(existing) - 1
		}
	}

	m.Tags[resourceArn] = existing

	m.saveLocked()

	return nil
}

// UntagResource removes tags by key from a resource ARN.
func (m *MemoryStorage) UntagResource(_ context.Context, resourceArn string, tagKeys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing := m.Tags[resourceArn]
	if len(existing) == 0 {
		return nil
	}

	removeSet := make(map[string]struct{}, len(tagKeys))
	for _, key := range tagKeys {
		removeSet[key] = struct{}{}
	}

	var remaining []Tag

	for _, tag := range existing {
		if _, ok := removeSet[tag.Key]; !ok {
			remaining = append(remaining, tag)
		}
	}

	if len(remaining) == 0 {
		delete(m.Tags, resourceArn)
	} else {
		m.Tags[resourceArn] = remaining
	}

	m.saveLocked()

	return nil
}

// DescribeContinuousBackups returns the continuous backups status for a table.
func (m *MemoryStorage) DescribeContinuousBackups(_ context.Context, tableName string) (*ContinuousBackupsDescription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.Tables[tableName]; !exists {
		return nil, &TableError{
			Code:    "TableNotFoundException",
			Message: "Table not found: " + tableName,
		}
	}

	return &ContinuousBackupsDescription{
		ContinuousBackupsStatus: continuousBackupsDisabled,
		PointInTimeRecoveryDescription: PointInTimeRecoveryDescription{
			PointInTimeRecoveryStatus: continuousBackupsDisabled,
		},
	}, nil
}
