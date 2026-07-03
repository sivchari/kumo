package ce

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "123456789012"
)

// Storage defines the interface for Cost Explorer storage.
type Storage interface {
	// GetCostAndUsage retrieves cost and usage data.
	GetCostAndUsage(ctx context.Context, req *GetCostAndUsageRequest) (*GetCostAndUsageResponse, error)

	// GetDimensionValues retrieves dimension values.
	GetDimensionValues(ctx context.Context, req *GetDimensionValuesRequest) (*GetDimensionValuesResponse, error)

	// GetTags retrieves tag values.
	GetTags(ctx context.Context, req *GetTagsRequest) (*GetTagsResponse, error)

	// GetCostForecast retrieves cost forecast.
	GetCostForecast(ctx context.Context, req *GetCostForecastRequest) (*GetCostForecastResponse, error)

	// CreateCostCategoryDefinition creates a cost category definition.
	CreateCostCategoryDefinition(ctx context.Context, req *CreateCostCategoryDefinitionRequest) (*CostCategoryDefinition, error)

	// DescribeCostCategoryDefinition describes a cost category definition.
	DescribeCostCategoryDefinition(ctx context.Context, arn string) (*CostCategoryDefinition, error)

	// DeleteCostCategoryDefinition deletes a cost category definition.
	DeleteCostCategoryDefinition(ctx context.Context, arn string) error

	// ListCostCategoryDefinitions lists cost category definitions.
	ListCostCategoryDefinitions(ctx context.Context, req *ListCostCategoryDefinitionsRequest) (*ListCostCategoryDefinitionsResponse, error)
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

// MemoryStorage implements in-memory storage for Cost Explorer.
type MemoryStorage struct {
	mu             sync.RWMutex                       `json:"-"`
	CostCategories map[string]*CostCategoryDefinition `json:"costCategories"`
	dataDir        string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		CostCategories: make(map[string]*CostCategoryDefinition),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "ce", s)
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

	if s.CostCategories == nil {
		s.CostCategories = make(map[string]*CostCategoryDefinition)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "ce", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "ce", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// GetCostAndUsage retrieves cost and usage data.
func (s *MemoryStorage) GetCostAndUsage(_ context.Context, req *GetCostAndUsageRequest) (*GetCostAndUsageResponse, error) {
	if req.TimePeriod.Start == "" || req.TimePeriod.End == "" {
		return nil, &ServiceError{
			Code:    errValidation,
			Message: "TimePeriod.Start and TimePeriod.End are required",
		}
	}

	if req.Granularity == "" {
		return nil, &ServiceError{
			Code:    errValidation,
			Message: "Granularity is required",
		}
	}

	results := generateMockCostData(req)

	resp := &GetCostAndUsageResponse{
		ResultsByTime:    results,
		GroupDefinitions: req.GroupBy,
	}

	return resp, nil
}

// GetDimensionValues retrieves dimension values.
func (s *MemoryStorage) GetDimensionValues(_ context.Context, req *GetDimensionValuesRequest) (*GetDimensionValuesResponse, error) {
	if req.Dimension == "" {
		return nil, &ServiceError{
			Code:    errValidation,
			Message: "Dimension is required",
		}
	}

	if req.TimePeriod.Start == "" || req.TimePeriod.End == "" {
		return nil, &ServiceError{
			Code:    errValidation,
			Message: "TimePeriod.Start and TimePeriod.End are required",
		}
	}

	values := getMockDimensionValues(req.Dimension, req.SearchString)

	return &GetDimensionValuesResponse{
		DimensionValues: values,
		ReturnSize:      len(values),
		TotalSize:       len(values),
	}, nil
}

// GetTags retrieves tag values.
func (s *MemoryStorage) GetTags(_ context.Context, req *GetTagsRequest) (*GetTagsResponse, error) {
	if req.TimePeriod.Start == "" || req.TimePeriod.End == "" {
		return nil, &ServiceError{
			Code:    errValidation,
			Message: "TimePeriod.Start and TimePeriod.End are required",
		}
	}

	tags := getMockTags(req.TagKey, req.SearchString)

	return &GetTagsResponse{
		Tags:       tags,
		ReturnSize: len(tags),
		TotalSize:  len(tags),
	}, nil
}

// GetCostForecast retrieves cost forecast.
func (s *MemoryStorage) GetCostForecast(_ context.Context, req *GetCostForecastRequest) (*GetCostForecastResponse, error) {
	if req.TimePeriod.Start == "" || req.TimePeriod.End == "" {
		return nil, &ServiceError{
			Code:    errValidation,
			Message: "TimePeriod.Start and TimePeriod.End are required",
		}
	}

	if req.Metric == "" {
		return nil, &ServiceError{
			Code:    errValidation,
			Message: "Metric is required",
		}
	}

	if req.Granularity == "" {
		return nil, &ServiceError{
			Code:    errValidation,
			Message: "Granularity is required",
		}
	}

	forecasts := generateMockForecast(req)

	return &GetCostForecastResponse{
		Total: MetricValue{
			Amount: "1500.00",
			Unit:   "USD",
		},
		ForecastResultsT: forecasts,
	}, nil
}

// validateCreateCostCategoryRequest validates the request fields for creating a cost category.
func validateCreateCostCategoryRequest(req *CreateCostCategoryDefinitionRequest) error {
	if req.Name == "" {
		return &ServiceError{Code: errValidation, Message: "Name is required"}
	}

	if req.RuleVersion == "" {
		return &ServiceError{Code: errValidation, Message: "RuleVersion is required"}
	}

	if len(req.Rules) == 0 {
		return &ServiceError{Code: errValidation, Message: "Rules are required"}
	}

	return nil
}

// CreateCostCategoryDefinition creates a cost category definition.
func (s *MemoryStorage) CreateCostCategoryDefinition(_ context.Context, req *CreateCostCategoryDefinitionRequest) (*CostCategoryDefinition, error) {
	if err := validateCreateCostCategoryRequest(req); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate name
	for _, cc := range s.CostCategories {
		if cc.Name == req.Name {
			return nil, &ServiceError{
				Code:    errValidation,
				Message: fmt.Sprintf("Cost category with name %s already exists", req.Name),
			}
		}
	}

	arn := fmt.Sprintf("arn:aws:ce::%s:costcategory/%s", defaultAccountID, uuid.New().String())
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	effectiveStart := req.EffectiveStart
	if effectiveStart == "" {
		effectiveStart = now
	}

	cc := &CostCategoryDefinition{
		CostCategoryArn:  arn,
		Name:             req.Name,
		RuleVersion:      req.RuleVersion,
		Rules:            req.Rules,
		DefaultValue:     req.DefaultValue,
		SplitChargeRules: req.SplitChargeRules,
		EffectiveStart:   effectiveStart,
		ProcessingStatus: []CostCategoryProcessingStatus{
			{
				Component: "COST_EXPLORER",
				Status:    "APPLIED",
			},
		},
	}

	s.CostCategories[arn] = cc

	s.saveLocked()

	return cc, nil
}

// DescribeCostCategoryDefinition describes a cost category definition.
func (s *MemoryStorage) DescribeCostCategoryDefinition(_ context.Context, arn string) (*CostCategoryDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cc, ok := s.CostCategories[arn]
	if !ok {
		return nil, &ServiceError{
			Code:    errNotFound,
			Message: fmt.Sprintf("Cost category with ARN %s not found", arn),
		}
	}

	return cc, nil
}

// DeleteCostCategoryDefinition deletes a cost category definition.
func (s *MemoryStorage) DeleteCostCategoryDefinition(_ context.Context, arn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.CostCategories[arn]; !ok {
		return &ServiceError{
			Code:    errNotFound,
			Message: fmt.Sprintf("Cost category with ARN %s not found", arn),
		}
	}

	delete(s.CostCategories, arn)

	s.saveLocked()

	return nil
}

// ListCostCategoryDefinitions lists cost category definitions.
func (s *MemoryStorage) ListCostCategoryDefinitions(_ context.Context, _ *ListCostCategoryDefinitionsRequest) (*ListCostCategoryDefinitionsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	refs := make([]CostCategoryReference, 0, len(s.CostCategories))

	for _, cc := range s.CostCategories {
		refs = append(refs, CostCategoryReference{
			CostCategoryArn: cc.CostCategoryArn,
			Name:            cc.Name,
			EffectiveStart:  cc.EffectiveStart,
			EffectiveEnd:    cc.EffectiveEnd,
			NumberOfRules:   len(cc.Rules),
			DefaultValue:    cc.DefaultValue,
		})
	}

	return &ListCostCategoryDefinitionsResponse{
		CostCategoryReferences: refs,
	}, nil
}

// generateMockCostData generates mock cost data for testing.
func generateMockCostData(req *GetCostAndUsageRequest) []ResultByTime {
	services := []string{
		"Amazon Simple Storage Service",
		"Amazon Elastic Compute Cloud - Compute",
		"Amazon DynamoDB",
		"AWS Lambda",
		"Amazon CloudWatch",
	}

	metrics := req.Metrics
	if len(metrics) == 0 {
		metrics = []string{"UnblendedCost"}
	}

	result := ResultByTime{
		TimePeriod: req.TimePeriod,
		Estimated:  false,
	}

	if len(req.GroupBy) > 0 {
		groups := make([]Group, 0, len(services))

		for i, svc := range services {
			metricValues := make(map[string]MetricValue)

			for _, metric := range metrics {
				amount := float64(10+i*5) + float64(i)*0.5
				metricValues[metric] = MetricValue{
					Amount: strconv.FormatFloat(amount, 'f', 2, 64),
					Unit:   "USD",
				}
			}

			groups = append(groups, Group{
				Keys:    []string{svc},
				Metrics: metricValues,
			})
		}

		result.Groups = groups
	} else {
		totalMetrics := make(map[string]MetricValue)

		for _, metric := range metrics {
			totalMetrics[metric] = MetricValue{
				Amount: "150.50",
				Unit:   "USD",
			}
		}

		result.Total = totalMetrics
	}

	return []ResultByTime{result}
}

// getMockDimensionValues returns mock dimension values.
func getMockDimensionValues(dimension, searchString string) []DimensionValueItem {
	dimensionData := map[string][]string{
		"SERVICE": {
			"Amazon Simple Storage Service",
			"Amazon Elastic Compute Cloud - Compute",
			"Amazon DynamoDB",
			"AWS Lambda",
			"Amazon CloudWatch",
			"Amazon RDS",
			"Amazon ElastiCache",
			"Amazon Kinesis",
		},
		"REGION": {
			"us-east-1",
			"us-west-2",
			"eu-west-1",
			"ap-northeast-1",
			"ap-southeast-1",
		},
		"INSTANCE_TYPE": {
			"t3.micro",
			"t3.small",
			"t3.medium",
			"t3.large",
			"m5.large",
			"m5.xlarge",
		},
		"LINKED_ACCOUNT": {
			defaultAccountID,
		},
		"USAGE_TYPE": {
			"DataTransfer-Out-Bytes",
			"DataTransfer-In-Bytes",
			"Requests-Tier1",
			"Requests-Tier2",
			"TimedStorage-ByteHrs",
		},
	}

	values, ok := dimensionData[dimension]
	if !ok {
		values = []string{}
	}

	result := make([]DimensionValueItem, 0, len(values))

	for _, v := range values {
		if searchString != "" && !containsIgnoreCase(v, searchString) {
			continue
		}

		result = append(result, DimensionValueItem{
			Value:      v,
			Attributes: map[string]string{},
		})
	}

	return result
}

// getMockTags returns mock tag values.
func getMockTags(tagKey, searchString string) []string {
	tags := []string{
		"Environment",
		"Project",
		"Team",
		"CostCenter",
		"Application",
	}

	if tagKey != "" {
		// Return tag values for specific key
		tagValues := map[string][]string{
			"Environment": {"production", "staging", "development"},
			"Project":     {"project-a", "project-b", "project-c"},
			"Team":        {"engineering", "data", "platform"},
			"CostCenter":  {"cc-001", "cc-002", "cc-003"},
			"Application": {"web-app", "api-service", "batch-job"},
		}

		if values, ok := tagValues[tagKey]; ok {
			return filterStrings(values, searchString)
		}

		return []string{}
	}

	return filterStrings(tags, searchString)
}

// generateMockForecast generates mock forecast data.
func generateMockForecast(req *GetCostForecastRequest) []ForecastResult {
	start, _ := time.Parse("2006-01-02", req.TimePeriod.Start)
	end, _ := time.Parse("2006-01-02", req.TimePeriod.End)

	results := make([]ForecastResult, 0)
	current := start

	for current.Before(end) {
		var next time.Time

		switch req.Granularity {
		case "DAILY":
			next = current.AddDate(0, 0, 1)
		case "MONTHLY":
			next = current.AddDate(0, 1, 0)
		default:
			next = current.AddDate(0, 0, 1)
		}

		if next.After(end) {
			next = end
		}

		meanValue := 50.0 + float64(current.Day())*2
		lowerBound := meanValue * 0.9
		upperBound := meanValue * 1.1

		results = append(results, ForecastResult{
			TimePeriod: DateInterval{
				Start: current.Format("2006-01-02"),
				End:   next.Format("2006-01-02"),
			},
			MeanValue:                    strconv.FormatFloat(meanValue, 'f', 2, 64),
			PredictionIntervalLowerBound: strconv.FormatFloat(lowerBound, 'f', 2, 64),
			PredictionIntervalUpperBound: strconv.FormatFloat(upperBound, 'f', 2, 64),
		})

		current = next
	}

	return results
}

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldASCII(s[i:i+len(substr)], substr) {
			return true
		}
	}

	return false
}

// equalFoldASCII checks if two strings are equal (case-insensitive ASCII).
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}

		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}

		if ca != cb {
			return false
		}
	}

	return true
}

// filterStrings filters strings by search string.
func filterStrings(values []string, searchString string) []string {
	if searchString == "" {
		return values
	}

	result := make([]string, 0)

	for _, v := range values {
		if containsIgnoreCase(v, searchString) {
			result = append(result, v)
		}
	}

	return result
}
