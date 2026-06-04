package s3tables

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultAccountID = "000000000000"
	defaultRegion    = "us-east-1"
	defaultMaxItems  = 100
)

// Storage defines the S3 Tables storage interface.
type Storage interface {
	// TableBucket operations
	CreateTableBucket(ctx context.Context, name string) (*TableBucket, error)
	DeleteTableBucket(ctx context.Context, arn string) error
	GetTableBucket(ctx context.Context, arn string) (*TableBucket, error)
	ListTableBuckets(ctx context.Context, prefix, continuationToken string, maxBuckets int) ([]TableBucketSummary, string, error)

	// Namespace operations
	CreateNamespace(ctx context.Context, tableBucketArn string, namespace []string) (*Namespace, error)
	DeleteNamespace(ctx context.Context, tableBucketArn, namespace string) error
	GetNamespace(ctx context.Context, tableBucketArn, namespace string) (*Namespace, error)
	ListNamespaces(ctx context.Context, tableBucketArn, prefix string, maxNamespaces int) ([]NamespaceSummary, error)

	// Table operations
	CreateTable(ctx context.Context, tableBucketArn, namespace, name, format string) (*Table, error)
	DeleteTable(ctx context.Context, tableBucketArn, namespace, name string) error
	GetTable(ctx context.Context, tableBucketArn, namespace, name string) (*Table, error)
	ListTables(ctx context.Context, tableBucketArn, namespace, prefix string, maxTables int) ([]TableSummary, error)
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
	mu           sync.RWMutex                     `json:"-"`
	TableBuckets map[string]*TableBucket          `json:"tableBuckets"` // ARN -> TableBucket
	Namespaces   map[string]map[string]*Namespace `json:"namespaces"`   // TableBucketARN -> namespace -> Namespace
	Tables       map[string]map[string]*Table     `json:"tables"`       // TableBucketARN/namespace -> tableName -> Table
	dataDir      string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		TableBuckets: make(map[string]*TableBucket),
		Namespaces:   make(map[string]map[string]*Namespace),
		Tables:       make(map[string]map[string]*Table),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "s3tables", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (s *MemoryStorage) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(s)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (s *MemoryStorage) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(s)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if s.TableBuckets == nil {
		s.TableBuckets = make(map[string]*TableBucket)
	}

	if s.Namespaces == nil {
		s.Namespaces = make(map[string]map[string]*Namespace)
	}

	if s.Tables == nil {
		s.Tables = make(map[string]map[string]*Table)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "s3tables", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "s3tables", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateTableBucket creates a new table bucket.
func (s *MemoryStorage) CreateTableBucket(_ context.Context, name string) (*TableBucket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if bucket with same name already exists
	for _, bucket := range s.TableBuckets {
		if bucket.Name == name {
			return nil, &Error{
				Code:    errConflict,
				Message: fmt.Sprintf("Table bucket with name '%s' already exists", name),
			}
		}
	}

	bucketID := uuid.New().String()
	arn := fmt.Sprintf("arn:aws:s3tables:%s:%s:bucket/%s", defaultRegion, defaultAccountID, name)

	bucket := &TableBucket{
		Arn:       arn,
		ID:        bucketID,
		Name:      name,
		Type:      "customer",
		OwnerID:   defaultAccountID,
		CreatedAt: time.Now().UTC(),
	}

	s.TableBuckets[arn] = bucket
	s.Namespaces[arn] = make(map[string]*Namespace)

	s.saveLocked()

	return bucket, nil
}

// DeleteTableBucket deletes a table bucket.
func (s *MemoryStorage) DeleteTableBucket(_ context.Context, arn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.TableBuckets[arn]; !exists {
		return &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Table bucket '%s' not found", arn),
		}
	}

	// Check if bucket has namespaces
	if namespaces, exists := s.Namespaces[arn]; exists && len(namespaces) > 0 {
		return &Error{
			Code:    errConflict,
			Message: "Table bucket contains namespaces and cannot be deleted",
		}
	}

	delete(s.TableBuckets, arn)
	delete(s.Namespaces, arn)

	s.saveLocked()

	return nil
}

// GetTableBucket retrieves a table bucket.
func (s *MemoryStorage) GetTableBucket(_ context.Context, arn string) (*TableBucket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bucket, exists := s.TableBuckets[arn]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Table bucket '%s' not found", arn),
		}
	}

	return bucket, nil
}

// ListTableBuckets lists all table buckets with pagination support.
func (s *MemoryStorage) ListTableBuckets(_ context.Context, prefix, continuationToken string, maxBuckets int) ([]TableBucketSummary, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxBuckets <= 0 {
		maxBuckets = defaultMaxItems
	}

	// Collect all matching buckets sorted by name for consistent pagination
	allBuckets := make([]TableBucketSummary, 0)

	for _, bucket := range s.TableBuckets {
		if prefix != "" && !strings.HasPrefix(bucket.Name, prefix) {
			continue
		}

		allBuckets = append(allBuckets, TableBucketSummary{
			Arn:       bucket.Arn,
			ID:        bucket.ID,
			Name:      bucket.Name,
			Type:      bucket.Type,
			OwnerID:   bucket.OwnerID,
			CreatedAt: bucket.CreatedAt,
		})
	}

	// Sort by name for consistent ordering
	sortTableBucketSummaries(allBuckets)

	// Apply continuation token (skip buckets until we find the marker)
	startIdx := 0

	if continuationToken != "" {
		for i, bucket := range allBuckets {
			if bucket.Name == continuationToken {
				startIdx = i + 1

				break
			}
		}
	}

	// Apply pagination
	if startIdx >= len(allBuckets) {
		return []TableBucketSummary{}, "", nil
	}

	endIdx := startIdx + maxBuckets
	if endIdx > len(allBuckets) {
		endIdx = len(allBuckets)
	}

	result := allBuckets[startIdx:endIdx]

	// Set next continuation token if there are more results
	var nextToken string

	if endIdx < len(allBuckets) {
		nextToken = result[len(result)-1].Name
	}

	return result, nextToken, nil
}

// CreateNamespace creates a new namespace.
func (s *MemoryStorage) CreateNamespace(_ context.Context, tableBucketArn string, namespace []string) (*Namespace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.TableBuckets[tableBucketArn]; !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Table bucket '%s' not found", tableBucketArn),
		}
	}

	namespaceKey := strings.Join(namespace, ".")

	if s.Namespaces[tableBucketArn] == nil {
		s.Namespaces[tableBucketArn] = make(map[string]*Namespace)
	}

	if _, exists := s.Namespaces[tableBucketArn][namespaceKey]; exists {
		return nil, &Error{
			Code:    errConflict,
			Message: fmt.Sprintf("Namespace '%s' already exists", namespaceKey),
		}
	}

	ns := &Namespace{
		Namespace:      namespace,
		TableBucketArn: tableBucketArn,
		OwnerID:        defaultAccountID,
		CreatedAt:      time.Now().UTC(),
		CreatedBy:      defaultAccountID,
	}

	s.Namespaces[tableBucketArn][namespaceKey] = ns

	s.saveLocked()

	return ns, nil
}

// DeleteNamespace deletes a namespace.
func (s *MemoryStorage) DeleteNamespace(_ context.Context, tableBucketArn, namespace string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.TableBuckets[tableBucketArn]; !exists {
		return &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Table bucket '%s' not found", tableBucketArn),
		}
	}

	if _, exists := s.Namespaces[tableBucketArn][namespace]; !exists {
		return &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Namespace '%s' not found", namespace),
		}
	}

	// Check if namespace has tables
	tableKey := tableBucketArn + "/" + namespace
	if tables, exists := s.Tables[tableKey]; exists && len(tables) > 0 {
		return &Error{
			Code:    errConflict,
			Message: "Namespace contains tables and cannot be deleted",
		}
	}

	delete(s.Namespaces[tableBucketArn], namespace)

	s.saveLocked()

	return nil
}

// GetNamespace retrieves a namespace.
func (s *MemoryStorage) GetNamespace(_ context.Context, tableBucketArn, namespace string) (*Namespace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.TableBuckets[tableBucketArn]; !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Table bucket '%s' not found", tableBucketArn),
		}
	}

	ns, exists := s.Namespaces[tableBucketArn][namespace]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Namespace '%s' not found", namespace),
		}
	}

	return ns, nil
}

// ListNamespaces lists all namespaces in a table bucket.
func (s *MemoryStorage) ListNamespaces(_ context.Context, tableBucketArn, prefix string, maxNamespaces int) ([]NamespaceSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.TableBuckets[tableBucketArn]; !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Table bucket '%s' not found", tableBucketArn),
		}
	}

	if maxNamespaces <= 0 {
		maxNamespaces = defaultMaxItems
	}

	namespaces := make([]NamespaceSummary, 0)

	for _, ns := range s.Namespaces[tableBucketArn] {
		nsName := strings.Join(ns.Namespace, ".")
		if prefix != "" && !strings.HasPrefix(nsName, prefix) {
			continue
		}

		namespaces = append(namespaces, NamespaceSummary{
			Namespace: ns.Namespace,
			CreatedAt: ns.CreatedAt,
			CreatedBy: ns.CreatedBy,
			OwnerID:   ns.OwnerID,
		})

		if len(namespaces) >= maxNamespaces {
			break
		}
	}

	return namespaces, nil
}

// CreateTable creates a new table.
func (s *MemoryStorage) CreateTable(_ context.Context, tableBucketArn, namespace, name, format string) (*Table, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.TableBuckets[tableBucketArn]; !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Table bucket '%s' not found", tableBucketArn),
		}
	}

	if _, exists := s.Namespaces[tableBucketArn][namespace]; !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Namespace '%s' not found", namespace),
		}
	}

	tableKey := tableBucketArn + "/" + namespace
	if s.Tables[tableKey] == nil {
		s.Tables[tableKey] = make(map[string]*Table)
	}

	if _, exists := s.Tables[tableKey][name]; exists {
		return nil, &Error{
			Code:    errConflict,
			Message: fmt.Sprintf("Table '%s' already exists in namespace '%s'", name, namespace),
		}
	}

	// Extract bucket name from ARN for table ARN
	bucketName := extractBucketNameFromArn(tableBucketArn)
	tableArn := fmt.Sprintf("arn:aws:s3tables:%s:%s:bucket/%s/table/%s/%s",
		defaultRegion, defaultAccountID, bucketName, namespace, name)

	now := time.Now().UTC()
	versionToken := uuid.New().String()

	table := &Table{
		Arn:            tableArn,
		Name:           name,
		Namespace:      namespace,
		TableBucketArn: tableBucketArn,
		Type:           "customer",
		Format:         format,
		VersionToken:   versionToken,
		CreatedAt:      now,
		CreatedBy:      defaultAccountID,
		ModifiedAt:     now,
		ModifiedBy:     defaultAccountID,
		OwnerID:        defaultAccountID,
	}

	s.Tables[tableKey][name] = table

	s.saveLocked()

	return table, nil
}

// DeleteTable deletes a table.
func (s *MemoryStorage) DeleteTable(_ context.Context, tableBucketArn, namespace, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.TableBuckets[tableBucketArn]; !exists {
		return &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Table bucket '%s' not found", tableBucketArn),
		}
	}

	tableKey := tableBucketArn + "/" + namespace
	if _, exists := s.Tables[tableKey][name]; !exists {
		return &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Table '%s' not found in namespace '%s'", name, namespace),
		}
	}

	delete(s.Tables[tableKey], name)

	s.saveLocked()

	return nil
}

// GetTable retrieves a table.
func (s *MemoryStorage) GetTable(_ context.Context, tableBucketArn, namespace, name string) (*Table, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.TableBuckets[tableBucketArn]; !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Table bucket '%s' not found", tableBucketArn),
		}
	}

	tableKey := tableBucketArn + "/" + namespace
	table, exists := s.Tables[tableKey][name]

	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Table '%s' not found in namespace '%s'", name, namespace),
		}
	}

	return table, nil
}

// ListTables lists all tables in a namespace.
func (s *MemoryStorage) ListTables(_ context.Context, tableBucketArn, namespace, prefix string, maxTables int) ([]TableSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.TableBuckets[tableBucketArn]; !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Table bucket '%s' not found", tableBucketArn),
		}
	}

	if maxTables <= 0 {
		maxTables = defaultMaxItems
	}

	if namespace != "" {
		return s.listTablesInNamespace(tableBucketArn, namespace, prefix, maxTables), nil
	}

	return s.listTablesAcrossNamespaces(tableBucketArn, prefix, maxTables), nil
}

// listTablesInNamespace lists tables in a specific namespace.
func (s *MemoryStorage) listTablesInNamespace(tableBucketArn, namespace, prefix string, maxTables int) []TableSummary {
	tables := make([]TableSummary, 0)
	tableKey := tableBucketArn + "/" + namespace

	for _, table := range s.Tables[tableKey] {
		if prefix != "" && !strings.HasPrefix(table.Name, prefix) {
			continue
		}

		tables = append(tables, tableToSummary(table))

		if len(tables) >= maxTables {
			break
		}
	}

	return tables
}

// listTablesAcrossNamespaces lists tables across all namespaces.
func (s *MemoryStorage) listTablesAcrossNamespaces(tableBucketArn, prefix string, maxTables int) []TableSummary {
	tables := make([]TableSummary, 0)

	for key, tablemap := range s.Tables {
		if !strings.HasPrefix(key, tableBucketArn+"/") {
			continue
		}

		for _, table := range tablemap {
			if prefix != "" && !strings.HasPrefix(table.Name, prefix) {
				continue
			}

			tables = append(tables, tableToSummary(table))

			if len(tables) >= maxTables {
				return tables
			}
		}
	}

	return tables
}

// tableToSummary converts a Table to TableSummary.
func tableToSummary(table *Table) TableSummary {
	return TableSummary{
		Arn:        table.Arn,
		Name:       table.Name,
		Namespace:  []string{table.Namespace},
		Type:       table.Type,
		CreatedAt:  table.CreatedAt,
		ModifiedAt: table.ModifiedAt,
	}
}

// extractBucketNameFromArn extracts the bucket name from a table bucket ARN.
func extractBucketNameFromArn(arn string) string {
	// ARN format: arn:aws:s3tables:region:account:bucket/bucket-name
	parts := strings.Split(arn, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}

	return ""
}

// sortTableBucketSummaries sorts table bucket summaries by name in ascending order.
func sortTableBucketSummaries(buckets []TableBucketSummary) {
	for i := range len(buckets) {
		for j := i + 1; j < len(buckets); j++ {
			if buckets[i].Name > buckets[j].Name {
				buckets[i], buckets[j] = buckets[j], buckets[i]
			}
		}
	}
}
