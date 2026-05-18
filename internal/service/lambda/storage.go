package lambda

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/storage"
)

// Storage defines the Lambda storage interface.
type Storage interface {
	CreateFunction(ctx context.Context, req *CreateFunctionRequest) (*Function, error)
	GetFunction(ctx context.Context, name string) (*Function, error)
	GetFunctionByARN(ctx context.Context, arn string) (*Function, error)
	DeleteFunction(ctx context.Context, name string) error
	ListFunctions(ctx context.Context, marker string, maxItems int) ([]*Function, string, error)
	UpdateFunctionCode(ctx context.Context, name string, req *UpdateFunctionCodeRequest) (*Function, error)
	UpdateFunctionConfiguration(ctx context.Context, name string, req *UpdateFunctionConfigurationRequest) (*Function, error)

	// Tag operations
	ListTags(ctx context.Context, arn string) (map[string]string, error)
	TagResource(ctx context.Context, arn string, tags map[string]string) error
	UntagResource(ctx context.Context, arn string, tagKeys []string) error

	// Permission operations
	AddPermission(ctx context.Context, functionName string, stmt *PolicyStatement) error
	RemovePermission(ctx context.Context, functionName, statementID string) error
	GetPolicy(ctx context.Context, functionName string) (*ResourcePolicy, error)

	// Read-only accessors
	ListVersionsByFunction(ctx context.Context, functionName string) (*Function, error)
	ListAliases(ctx context.Context, functionName string) error
	ListFunctionEventInvokeConfigs(ctx context.Context, functionName string) error
	GetFunctionCodeSigningConfig(ctx context.Context, functionName string) (string, error)

	// EventSourceMapping operations
	CreateEventSourceMapping(ctx context.Context, req *CreateEventSourceMappingRequest) (*EventSourceMapping, error)
	GetEventSourceMapping(ctx context.Context, uuid string) (*EventSourceMapping, error)
	DeleteEventSourceMapping(ctx context.Context, uuid string) error
	ListEventSourceMappings(ctx context.Context, functionName, eventSourceArn, marker string, maxItems int) ([]*EventSourceMapping, string, error)
	UpdateEventSourceMapping(ctx context.Context, uuid string, req *UpdateEventSourceMappingRequest) (*EventSourceMapping, error)
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
	mu                  sync.RWMutex                   `json:"-"`
	Functions           map[string]*Function           `json:"functions"`
	EventSourceMappings map[string]*EventSourceMapping `json:"eventSourceMappings"`
	baseURL             string
	region              string
	accountID           string
	dataDir             string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(baseURL string, opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Functions:           make(map[string]*Function),
		EventSourceMappings: make(map[string]*EventSourceMapping),
		baseURL:             baseURL,
		region:              "us-east-1",
		accountID:           "000000000000",
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "lambda", s)
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

	if s.Functions == nil {
		s.Functions = make(map[string]*Function)
	}

	if s.EventSourceMappings == nil {
		s.EventSourceMappings = make(map[string]*EventSourceMapping)
	}

	return nil
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "lambda", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateFunction creates a new Lambda function.
func (s *MemoryStorage) CreateFunction(_ context.Context, req *CreateFunctionRequest) (*Function, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Functions[req.FunctionName]; exists {
		return nil, &FunctionError{
			Type:    ErrResourceConflict,
			Message: fmt.Sprintf("Function already exist: %s", req.FunctionName),
		}
	}

	fn := s.buildFunction(req)
	s.Functions[req.FunctionName] = fn

	return fn, nil
}

// buildFunction creates a Function from a CreateFunctionRequest with defaults applied.
func (s *MemoryStorage) buildFunction(req *CreateFunctionRequest) *Function {
	codeHash := sha256.Sum256(req.Code.ZipFile)
	codeSha256 := base64.StdEncoding.EncodeToString(codeHash[:])

	timeout := req.Timeout
	if timeout == 0 {
		timeout = 3
	}

	memorySize := req.MemorySize
	if memorySize == 0 {
		memorySize = 128
	}

	packageType := req.PackageType
	if packageType == "" {
		packageType = "Zip"
	}

	architectures := req.Architectures
	if len(architectures) == 0 {
		architectures = []string{"x86_64"}
	}

	return &Function{
		FunctionName:   req.FunctionName,
		FunctionArn:    fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s", s.region, s.accountID, req.FunctionName),
		Runtime:        req.Runtime,
		Role:           req.Role,
		Handler:        req.Handler,
		Description:    req.Description,
		Timeout:        timeout,
		MemorySize:     memorySize,
		CodeSize:       int64(len(req.Code.ZipFile)),
		CodeSha256:     codeSha256,
		Version:        "$LATEST",
		LastModified:   time.Now().UTC(),
		State:          "Active",
		PackageType:    packageType,
		Architectures:  architectures,
		Environment:    req.Environment,
		Tags:           req.Tags,
		InvokeEndpoint: req.InvokeEndpoint,
		Code: &FunctionCode{
			ZipFile:         req.Code.ZipFile,
			S3Bucket:        req.Code.S3Bucket,
			S3Key:           req.Code.S3Key,
			S3ObjectVersion: req.Code.S3ObjectVersion,
			ImageURI:        req.Code.ImageURI,
		},
	}
}

// GetFunction retrieves a Lambda function by name.
func (s *MemoryStorage) GetFunction(_ context.Context, name string) (*Function, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fn, exists := s.Functions[name]
	if !exists {
		return nil, &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Function not found: %s", name),
		}
	}

	return fn, nil
}

// DeleteFunction deletes a Lambda function.
func (s *MemoryStorage) DeleteFunction(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Functions[name]; !exists {
		return &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Function not found: %s", name),
		}
	}

	delete(s.Functions, name)

	return nil
}

// ListFunctions lists all Lambda functions.
func (s *MemoryStorage) ListFunctions(_ context.Context, marker string, maxItems int) ([]*Function, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxItems == 0 {
		maxItems = 50
	}

	functions := make([]*Function, 0, len(s.Functions))
	for _, fn := range s.Functions {
		functions = append(functions, fn)
	}

	// Simple pagination (not production-ready).
	start := 0

	if marker != "" {
		for i, fn := range functions {
			if fn.FunctionName == marker {
				start = i + 1

				break
			}
		}
	}

	end := start + maxItems
	if end > len(functions) {
		end = len(functions)
	}

	result := functions[start:end]
	nextMarker := ""

	if end < len(functions) {
		nextMarker = functions[end-1].FunctionName
	}

	return result, nextMarker, nil
}

// UpdateFunctionCode updates the code of a Lambda function.
func (s *MemoryStorage) UpdateFunctionCode(_ context.Context, name string, req *UpdateFunctionCodeRequest) (*Function, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fn, exists := s.Functions[name]
	if !exists {
		return nil, &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Function not found: %s", name),
		}
	}

	// Update code.
	if len(req.ZipFile) > 0 {
		fn.Code.ZipFile = req.ZipFile
		codeHash := sha256.Sum256(req.ZipFile)
		fn.CodeSha256 = base64.StdEncoding.EncodeToString(codeHash[:])
		fn.CodeSize = int64(len(req.ZipFile))
	}

	if req.S3Bucket != "" {
		fn.Code.S3Bucket = req.S3Bucket
	}

	if req.S3Key != "" {
		fn.Code.S3Key = req.S3Key
	}

	if req.S3ObjectVersion != "" {
		fn.Code.S3ObjectVersion = req.S3ObjectVersion
	}

	if req.ImageURI != "" {
		fn.Code.ImageURI = req.ImageURI
	}

	if len(req.Architectures) > 0 {
		fn.Architectures = req.Architectures
	}

	fn.LastModified = time.Now().UTC()

	return fn, nil
}

// UpdateFunctionConfiguration updates the configuration of a Lambda function.
func (s *MemoryStorage) UpdateFunctionConfiguration(_ context.Context, name string, req *UpdateFunctionConfigurationRequest) (*Function, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fn, exists := s.Functions[name]
	if !exists {
		return nil, &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Function not found: %s", name),
		}
	}

	if req.Description != "" {
		fn.Description = req.Description
	}

	if req.Handler != "" {
		fn.Handler = req.Handler
	}

	if req.MemorySize > 0 {
		fn.MemorySize = req.MemorySize
	}

	if req.Role != "" {
		fn.Role = req.Role
	}

	if req.Runtime != "" {
		fn.Runtime = req.Runtime
	}

	if req.Timeout > 0 {
		fn.Timeout = req.Timeout
	}

	if req.Environment != nil {
		fn.Environment = req.Environment
	}

	if req.InvokeEndpoint != "" {
		fn.InvokeEndpoint = req.InvokeEndpoint
	}

	fn.LastModified = time.Now().UTC()

	return fn, nil
}

// GetFunctionByARN retrieves a Lambda function by its ARN.
func (s *MemoryStorage) GetFunctionByARN(_ context.Context, arn string) (*Function, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, fn := range s.Functions {
		if fn.FunctionArn == arn {
			return fn, nil
		}
	}

	return nil, &FunctionError{
		Type:    ErrResourceNotFound,
		Message: fmt.Sprintf("Function not found: %s", arn),
	}
}

// AddPermission adds a statement to a function's resource policy.
func (s *MemoryStorage) AddPermission(_ context.Context, functionName string, stmt *PolicyStatement) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fn, exists := s.Functions[functionName]
	if !exists {
		return &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Function not found: %s", functionName),
		}
	}

	if fn.Policy == nil {
		fn.Policy = &ResourcePolicy{
			Version: "2012-10-17",
			ID:      policyID(functionName),
		}
	}

	// Check for duplicate statement ID.
	for _, existing := range fn.Policy.Statements {
		if existing.Sid == stmt.Sid {
			return &FunctionError{
				Type:    ErrResourceConflict,
				Message: fmt.Sprintf("The statement id (%s) provided already exists. Please provide a new statement id, or remove the existing statement.", stmt.Sid),
			}
		}
	}

	fn.Policy.Statements = append(fn.Policy.Statements, stmt)

	return nil
}

// RemovePermission removes a statement from a function's resource policy.
func (s *MemoryStorage) RemovePermission(_ context.Context, functionName, statementID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fn, exists := s.Functions[functionName]
	if !exists {
		return &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Function not found: %s", functionName),
		}
	}

	if fn.Policy == nil {
		return &FunctionError{
			Type:    ErrResourceNotFound,
			Message: "No policy is associated with the given resource.",
		}
	}

	for i, stmt := range fn.Policy.Statements {
		if stmt.Sid == statementID {
			fn.Policy.Statements = append(fn.Policy.Statements[:i], fn.Policy.Statements[i+1:]...)

			return nil
		}
	}

	return &FunctionError{
		Type:    ErrResourceNotFound,
		Message: fmt.Sprintf("Statement %s is not found in resource policy.", statementID),
	}
}

// GetPolicy returns the resource policy for a function.
func (s *MemoryStorage) GetPolicy(_ context.Context, functionName string) (*ResourcePolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fn, exists := s.Functions[functionName]
	if !exists {
		return nil, &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Function not found: %s", functionName),
		}
	}

	return fn.Policy, nil
}

// ListTags returns the tags for a function identified by its ARN.
func (s *MemoryStorage) ListTags(_ context.Context, arn string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, fn := range s.Functions {
		if fn.FunctionArn == arn {
			tags := fn.Tags
			if tags == nil {
				tags = make(map[string]string)
			}

			return tags, nil
		}
	}

	return nil, &FunctionError{
		Type:    ErrResourceNotFound,
		Message: fmt.Sprintf("Function not found: %s", arn),
	}
}

// TagResource adds or overwrites tags on a function identified by its ARN.
func (s *MemoryStorage) TagResource(_ context.Context, arn string, tags map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, fn := range s.Functions {
		if fn.FunctionArn == arn {
			if fn.Tags == nil {
				fn.Tags = make(map[string]string)
			}

			for k, v := range tags {
				fn.Tags[k] = v
			}

			return nil
		}
	}

	return &FunctionError{
		Type:    ErrResourceNotFound,
		Message: fmt.Sprintf("Function not found: %s", arn),
	}
}

// UntagResource removes tags from a function identified by its ARN.
func (s *MemoryStorage) UntagResource(_ context.Context, arn string, tagKeys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, fn := range s.Functions {
		if fn.FunctionArn == arn {
			if fn.Tags == nil {
				return nil
			}

			for _, key := range tagKeys {
				delete(fn.Tags, key)
			}

			return nil
		}
	}

	return &FunctionError{
		Type:    ErrResourceNotFound,
		Message: fmt.Sprintf("Function not found: %s", arn),
	}
}

// ListVersionsByFunction returns the function for version listing.
func (s *MemoryStorage) ListVersionsByFunction(_ context.Context, functionName string) (*Function, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fn, exists := s.Functions[functionName]
	if !exists {
		return nil, &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Function not found: %s", functionName),
		}
	}

	return fn, nil
}

// ListAliases validates the function exists for alias listing.
func (s *MemoryStorage) ListAliases(_ context.Context, functionName string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.Functions[functionName]; !exists {
		return &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Function not found: %s", functionName),
		}
	}

	return nil
}

// ListFunctionEventInvokeConfigs validates the function exists for event invoke config listing.
func (s *MemoryStorage) ListFunctionEventInvokeConfigs(_ context.Context, functionName string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.Functions[functionName]; !exists {
		return &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Function not found: %s", functionName),
		}
	}

	return nil
}

// GetFunctionCodeSigningConfig returns the code signing config ARN for a function.
func (s *MemoryStorage) GetFunctionCodeSigningConfig(_ context.Context, functionName string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.Functions[functionName]; !exists {
		return "", &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Function not found: %s", functionName),
		}
	}

	// Code signing is not modeled; return the function name only.
	return functionName, nil
}

// CreateEventSourceMapping creates a new event source mapping.
func (s *MemoryStorage) CreateEventSourceMapping(_ context.Context, req *CreateEventSourceMappingRequest) (*EventSourceMapping, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate function exists
	fn, exists := s.Functions[req.FunctionName]
	if !exists {
		return nil, &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Function not found: %s", req.FunctionName),
		}
	}

	// Generate UUID
	mappingUUID := generateUUID()

	// Set defaults
	batchSize := req.BatchSize
	if batchSize == 0 {
		batchSize = 10
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	state := "Enabled"
	if !enabled {
		state = "Disabled"
	}

	mapping := &EventSourceMapping{
		UUID:                           mappingUUID,
		FunctionArn:                    fn.FunctionArn,
		EventSourceArn:                 req.EventSourceArn,
		State:                          state,
		BatchSize:                      batchSize,
		MaximumBatchingWindowInSeconds: req.MaximumBatchingWindowInSeconds,
		Enabled:                        &enabled,
		LastModified:                   toUnixTimestamp(time.Now().UTC()),
		LastProcessingResult:           "No records processed",
	}

	s.EventSourceMappings[mappingUUID] = mapping

	return mapping, nil
}

// GetEventSourceMapping retrieves an event source mapping by UUID.
func (s *MemoryStorage) GetEventSourceMapping(_ context.Context, uuid string) (*EventSourceMapping, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mapping, exists := s.EventSourceMappings[uuid]
	if !exists {
		return nil, &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Event source mapping not found: %s", uuid),
		}
	}

	return mapping, nil
}

// DeleteEventSourceMapping deletes an event source mapping.
func (s *MemoryStorage) DeleteEventSourceMapping(_ context.Context, uuid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mapping, exists := s.EventSourceMappings[uuid]
	if !exists {
		return &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Event source mapping not found: %s", uuid),
		}
	}

	// Mark as deleting state before removing
	mapping.State = "Deleting"

	delete(s.EventSourceMappings, uuid)

	return nil
}

// ListEventSourceMappings lists event source mappings with optional filters.
func (s *MemoryStorage) ListEventSourceMappings(_ context.Context, functionName, eventSourceArn, marker string, maxItems int) ([]*EventSourceMapping, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxItems == 0 {
		maxItems = 100
	}

	// Collect matching mappings
	var mappings []*EventSourceMapping

	for _, m := range s.EventSourceMappings {
		// Filter by function name if specified
		if functionName != "" && !matchesFunctionName(m.FunctionArn, functionName) {
			continue
		}

		// Filter by event source ARN if specified
		if eventSourceArn != "" && m.EventSourceArn != eventSourceArn {
			continue
		}

		mappings = append(mappings, m)
	}

	// Simple pagination
	start := 0

	if marker != "" {
		for i, m := range mappings {
			if m.UUID == marker {
				start = i + 1

				break
			}
		}
	}

	end := start + maxItems
	if end > len(mappings) {
		end = len(mappings)
	}

	result := mappings[start:end]
	nextMarker := ""

	if end < len(mappings) {
		nextMarker = mappings[end-1].UUID
	}

	return result, nextMarker, nil
}

// UpdateEventSourceMapping updates an event source mapping.
func (s *MemoryStorage) UpdateEventSourceMapping(_ context.Context, uuid string, req *UpdateEventSourceMappingRequest) (*EventSourceMapping, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	mapping, exists := s.EventSourceMappings[uuid]
	if !exists {
		return nil, &FunctionError{
			Type:    ErrResourceNotFound,
			Message: fmt.Sprintf("Event source mapping not found: %s", uuid),
		}
	}

	// Update function ARN if function name is specified
	if req.FunctionName != "" {
		fn, fnExists := s.Functions[req.FunctionName]
		if !fnExists {
			return nil, &FunctionError{
				Type:    ErrResourceNotFound,
				Message: fmt.Sprintf("Function not found: %s", req.FunctionName),
			}
		}

		mapping.FunctionArn = fn.FunctionArn
	}

	if req.BatchSize > 0 {
		mapping.BatchSize = req.BatchSize
	}

	if req.MaximumBatchingWindowInSeconds > 0 {
		mapping.MaximumBatchingWindowInSeconds = req.MaximumBatchingWindowInSeconds
	}

	if req.Enabled != nil {
		mapping.Enabled = req.Enabled

		if *req.Enabled {
			mapping.State = "Enabled"
		} else {
			mapping.State = "Disabled"
		}
	}

	mapping.LastModified = toUnixTimestamp(time.Now().UTC())

	return mapping, nil
}

// generateUUID generates a UUID for event source mapping.
func generateUUID() string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		time.Now().UnixNano()&0xFFFFFFFF,
		time.Now().UnixNano()>>32&0xFFFF,
		0x4000|time.Now().UnixNano()>>48&0x0FFF,
		0x8000|time.Now().UnixNano()>>60&0x3FFF,
		time.Now().UnixNano()&0xFFFFFFFFFFFF)
}

// toUnixTimestamp converts time.Time to Unix timestamp as float64.
func toUnixTimestamp(t time.Time) float64 {
	return float64(t.Unix()) + float64(t.Nanosecond())/1e9
}

// matchesFunctionName checks if the function ARN matches the function name.
func matchesFunctionName(functionArn, functionName string) bool {
	// Function name can be the full ARN or just the function name
	if functionArn == functionName {
		return true
	}

	// Extract function name from ARN
	// ARN format: arn:aws:lambda:region:account:function:name
	parts := splitARN(functionArn)
	if len(parts) >= 7 && parts[6] == functionName {
		return true
	}

	return false
}

// splitARN splits an ARN into its components.
func splitARN(arn string) []string {
	return splitString(arn, ':')
}

// splitString splits a string by separator.
func splitString(s string, sep byte) []string {
	var result []string

	start := 0

	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}

	result = append(result, s[start:])

	return result
}
