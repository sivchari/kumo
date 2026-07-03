package ssm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/storage"
)

// Default values.
const defaultRegion = "us-east-1"

// Storage defines the SSM Parameter Store storage interface.
type Storage interface {
	PutParameter(ctx context.Context, req *PutParameterRequest) (*Parameter, error)
	GetParameter(ctx context.Context, name string) (*Parameter, error)
	GetParameters(ctx context.Context, names []string) ([]*Parameter, []string, error)
	GetParametersByPath(ctx context.Context, path string, recursive bool, maxResults int, nextToken string) ([]*Parameter, string, error)
	DeleteParameter(ctx context.Context, name string) error
	DeleteParameters(ctx context.Context, names []string) ([]string, []string, error)
	DescribeParameters(ctx context.Context, filters []ParameterFilter, maxResults int, nextToken string) ([]*Parameter, string, error)
	ListTagsForResource(ctx context.Context, resourceType, resourceID string) ([]Tag, error)
	AddTagsToResource(ctx context.Context, resourceType, resourceID string, tags []Tag) error
	RemoveTagsFromResource(ctx context.Context, resourceType, resourceID string, tagKeys []string) error
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
	mu         sync.RWMutex          `json:"-"`
	Parameters map[string]*Parameter `json:"parameters"`
	Tags       map[string][]Tag      `json:"tags"`
	region     string
	accountID  string
	dataDir    string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Parameters: make(map[string]*Parameter),
		Tags:       make(map[string][]Tag),
		region:     region,
		accountID:  "000000000000",
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "ssm", s)
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

	if s.Parameters == nil {
		s.Parameters = make(map[string]*Parameter)
	}

	if s.Tags == nil {
		s.Tags = make(map[string][]Tag)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "ssm", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "ssm", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// PutParameter creates or updates a parameter.
func (s *MemoryStorage) PutParameter(_ context.Context, req *PutParameterRequest) (*Parameter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.Parameters[req.Name]

	// Check if parameter exists and overwrite is not set
	if exists && !req.Overwrite {
		return nil, &ParameterError{
			Type:    ErrParameterAlreadyExists,
			Message: "The parameter already exists. To overwrite this value, set the overwrite option in the request to true.",
		}
	}

	// Set defaults
	paramType := req.Type
	if paramType == "" {
		if exists {
			paramType = existing.Type
		} else {
			paramType = ParameterTypeString
		}
	}

	tier := req.Tier
	if tier == "" {
		tier = ParameterTierStandard
	}

	dataType := req.DataType
	if dataType == "" {
		dataType = "text"
	}

	version := int64(1)
	if exists {
		version = existing.Version + 1
	}

	param := &Parameter{
		Name:             req.Name,
		Type:             paramType,
		Value:            req.Value,
		Version:          version,
		LastModifiedDate: time.Now().UTC(),
		ARN:              fmt.Sprintf("arn:aws:ssm:%s:%s:parameter/%s", s.region, s.accountID, strings.TrimPrefix(req.Name, "/")),
		DataType:         dataType,
		Tier:             tier,
		Description:      req.Description,
	}

	s.Parameters[req.Name] = param

	s.saveLocked()

	return param, nil
}

// GetParameter retrieves a parameter by name.
func (s *MemoryStorage) GetParameter(_ context.Context, name string) (*Parameter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	param, exists := s.Parameters[name]
	if !exists {
		return nil, &ParameterError{
			Type:    ErrParameterNotFound,
			Message: fmt.Sprintf("Parameter %s not found.", name),
		}
	}

	return param, nil
}

// GetParameters retrieves multiple parameters by name.
func (s *MemoryStorage) GetParameters(_ context.Context, names []string) ([]*Parameter, []string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var params []*Parameter

	var invalidParams []string

	for _, name := range names {
		param, exists := s.Parameters[name]
		if exists {
			params = append(params, param)
		} else {
			invalidParams = append(invalidParams, name)
		}
	}

	return params, invalidParams, nil
}

// GetParametersByPath retrieves parameters by path prefix.
func (s *MemoryStorage) GetParametersByPath(_ context.Context, path string, recursive bool, maxResults int, nextToken string) ([]*Parameter, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults == 0 {
		maxResults = 10
	}

	// Normalize path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

	// Collect matching parameters
	var matches []*Parameter

	for name, param := range s.Parameters {
		if matchesPath(name, path, recursive) {
			matches = append(matches, param)
		}
	}

	// Sort by name for consistent pagination
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Name < matches[j].Name
	})

	// Handle pagination
	start := 0

	if nextToken != "" {
		for i, p := range matches {
			if p.Name == nextToken {
				start = i

				break
			}
		}
	}

	end := start + maxResults
	if end > len(matches) {
		end = len(matches)
	}

	result := matches[start:end]
	newNextToken := ""

	if end < len(matches) {
		newNextToken = matches[end].Name
	}

	return result, newNextToken, nil
}

// matchesPath checks if a parameter name matches the given path.
func matchesPath(name, path string, recursive bool) bool {
	// Normalize parameter name to have a leading slash for consistent matching.
	// AWS SSM treats parameters without a leading "/" as root-level parameters.
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}

	if !strings.HasPrefix(name, path) {
		return false
	}

	if recursive {
		return true
	}

	// For non-recursive, check that there are no more slashes after the path prefix
	remainder := strings.TrimPrefix(name, path)

	return !strings.Contains(remainder, "/")
}

// DeleteParameter deletes a parameter by name.
func (s *MemoryStorage) DeleteParameter(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Parameters[name]; !exists {
		return &ParameterError{
			Type:    ErrParameterNotFound,
			Message: fmt.Sprintf("Parameter %s not found.", name),
		}
	}

	delete(s.Parameters, name)

	s.saveLocked()

	return nil
}

// DeleteParameters deletes multiple parameters.
func (s *MemoryStorage) DeleteParameters(_ context.Context, names []string) ([]string, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var deleted []string

	var invalid []string

	for _, name := range names {
		if _, exists := s.Parameters[name]; exists {
			delete(s.Parameters, name)

			deleted = append(deleted, name)
		} else {
			invalid = append(invalid, name)
		}
	}

	s.saveLocked()

	return deleted, invalid, nil
}

// DescribeParameters lists all parameters with metadata.
func (s *MemoryStorage) DescribeParameters(_ context.Context, filters []ParameterFilter, maxResults int, nextToken string) ([]*Parameter, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults == 0 {
		maxResults = 50
	}

	// Collect all parameters, applying filters.
	params := make([]*Parameter, 0, len(s.Parameters))

	for _, p := range s.Parameters {
		if matchParameterFilters(p, filters) {
			params = append(params, p)
		}
	}

	// Sort by name for consistent pagination
	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})

	// Handle pagination
	start := 0

	if nextToken != "" {
		for i, p := range params {
			if p.Name == nextToken {
				start = i

				break
			}
		}
	}

	end := start + maxResults
	if end > len(params) {
		end = len(params)
	}

	result := params[start:end]
	newNextToken := ""

	if end < len(params) {
		newNextToken = params[end].Name
	}

	return result, newNextToken, nil
}

// matchParameterFilters checks if a parameter matches all filters.
// Supports Key=Name with Option=Equals (default), BeginsWith, and Contains.
func matchParameterFilters(p *Parameter, filters []ParameterFilter) bool {
	for _, f := range filters {
		if f.Key != "Name" || len(f.Values) == 0 {
			continue
		}

		option := f.Option
		if option == "" {
			option = "Equals"
		}

		if !matchNameFilter(p.Name, f.Values, option) {
			return false
		}
	}

	return true
}

func matchNameFilter(name string, values []string, option string) bool {
	for _, v := range values {
		switch option {
		case "Equals":
			if name == v {
				return true
			}
		case "BeginsWith":
			if strings.HasPrefix(name, v) {
				return true
			}
		case "Contains":
			if strings.Contains(name, v) {
				return true
			}
		}
	}

	return false
}

// tagKey builds the map key for the Tags store.
func tagKey(resourceType, resourceID string) string {
	return resourceType + ":" + resourceID
}

// ListTagsForResource returns the tags for a resource.
func (s *MemoryStorage) ListTagsForResource(_ context.Context, resourceType, resourceID string) ([]Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tags := s.Tags[tagKey(resourceType, resourceID)]
	if tags == nil {
		return []Tag{}, nil
	}

	out := make([]Tag, len(tags))
	copy(out, tags)

	return out, nil
}

// AddTagsToResource adds or overwrites tags on a resource.
func (s *MemoryStorage) AddTagsToResource(_ context.Context, resourceType, resourceID string, tags []Tag) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tagKey(resourceType, resourceID)
	existing := s.Tags[key]

	tagMap := make(map[string]string, len(existing)+len(tags))

	for _, t := range existing {
		tagMap[t.Key] = t.Value
	}

	for _, t := range tags {
		tagMap[t.Key] = t.Value
	}

	merged := make([]Tag, 0, len(tagMap))

	for k, v := range tagMap {
		merged = append(merged, Tag{Key: k, Value: v})
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Key < merged[j].Key
	})

	s.Tags[key] = merged

	s.saveLocked()

	return nil
}

// RemoveTagsFromResource removes tags by key from a resource.
func (s *MemoryStorage) RemoveTagsFromResource(_ context.Context, resourceType, resourceID string, tagKeys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tagKey(resourceType, resourceID)
	existing := s.Tags[key]

	keySet := make(map[string]struct{}, len(tagKeys))

	for _, k := range tagKeys {
		keySet[k] = struct{}{}
	}

	filtered := make([]Tag, 0, len(existing))

	for _, t := range existing {
		if _, remove := keySet[t.Key]; !remove {
			filtered = append(filtered, t)
		}
	}

	s.Tags[key] = filtered

	s.saveLocked()

	return nil
}
