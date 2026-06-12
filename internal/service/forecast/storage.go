package forecast

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Default values.
const defaultRegion = "us-east-1"

// Storage defines the interface for Forecast storage operations.
type Storage interface {
	// Dataset operations
	CreateDataset(ctx context.Context, req *CreateDatasetInput) (string, error)
	DescribeDataset(ctx context.Context, datasetArn string) (*Dataset, error)
	ListDatasets(ctx context.Context, maxResults *int32, nextToken string) ([]*DatasetSummary, string, error)
	DeleteDataset(ctx context.Context, datasetArn string) error

	// DatasetGroup operations
	CreateDatasetGroup(ctx context.Context, req *CreateDatasetGroupInput) (string, error)
	DescribeDatasetGroup(ctx context.Context, datasetGroupArn string) (*DatasetGroup, error)
	ListDatasetGroups(ctx context.Context, maxResults *int32, nextToken string) ([]*DatasetGroupSummary, string, error)
	DeleteDatasetGroup(ctx context.Context, datasetGroupArn string) error
	UpdateDatasetGroup(ctx context.Context, datasetGroupArn string, datasetArns []string) error

	// Predictor operations
	CreatePredictor(ctx context.Context, req *CreatePredictorInput) (string, error)
	DescribePredictor(ctx context.Context, predictorArn string) (*Predictor, error)
	ListPredictors(ctx context.Context, maxResults *int32, nextToken string, filters []Filter) ([]*PredictorSummary, string, error)
	DeletePredictor(ctx context.Context, predictorArn string) error

	// Forecast operations
	CreateForecast(ctx context.Context, req *CreateForecastInput) (string, error)
	DescribeForecast(ctx context.Context, forecastArn string) (*Forecast, error)
	ListForecasts(ctx context.Context, maxResults *int32, nextToken string, filters []Filter) ([]*ForecastSummary, string, error)
	DeleteForecast(ctx context.Context, forecastArn string) error
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

// MemoryStorage implements Storage using in-memory storage.
type MemoryStorage struct {
	mu            sync.RWMutex             `json:"-"`
	Datasets      map[string]*Dataset      `json:"datasets"`
	DatasetGroups map[string]*DatasetGroup `json:"datasetGroups"`
	Predictors    map[string]*Predictor    `json:"predictors"`
	Forecasts     map[string]*Forecast     `json:"forecasts"`
	accountID     string
	region        string
	dataDir       string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Datasets:      make(map[string]*Dataset),
		DatasetGroups: make(map[string]*DatasetGroup),
		Predictors:    make(map[string]*Predictor),
		Forecasts:     make(map[string]*Forecast),
		accountID:     "123456789012",
		region:        region,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "forecast", s)
	}

	return s
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

	if m.Datasets == nil {
		m.Datasets = make(map[string]*Dataset)
	}

	if m.DatasetGroups == nil {
		m.DatasetGroups = make(map[string]*DatasetGroup)
	}

	if m.Predictors == nil {
		m.Predictors = make(map[string]*Predictor)
	}

	if m.Forecasts == nil {
		m.Forecasts = make(map[string]*Forecast)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "forecast", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "forecast", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// Dataset operations.

// CreateDataset creates a new dataset.
func (m *MemoryStorage) CreateDataset(_ context.Context, req *CreateDatasetInput) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate name.
	for _, ds := range m.Datasets {
		if ds.DatasetName == req.DatasetName {
			return "", &Error{
				Code:    errResourceAlreadyExistsException,
				Message: fmt.Sprintf("A dataset with name %s already exists", req.DatasetName),
			}
		}
	}

	// Validate dataset type.
	if !isValidDatasetType(req.DatasetType) {
		return "", &Error{
			Code:    errInvalidInputException,
			Message: fmt.Sprintf("Invalid dataset type: %s", req.DatasetType),
		}
	}

	// Validate domain.
	if !isValidDomain(req.Domain) {
		return "", &Error{
			Code:    errInvalidInputException,
			Message: fmt.Sprintf("Invalid domain: %s", req.Domain),
		}
	}

	datasetArn := fmt.Sprintf("arn:aws:forecast:%s:%s:dataset/%s", m.region, m.accountID, req.DatasetName)
	now := time.Now()

	dataset := &Dataset{
		DatasetArn:           datasetArn,
		DatasetName:          req.DatasetName,
		DatasetType:          req.DatasetType,
		Domain:               req.Domain,
		DataFrequency:        req.DataFrequency,
		Schema:               req.Schema,
		Status:               statusActive,
		CreationTime:         ToAWSTimestamp(now),
		LastModificationTime: ToAWSTimestamp(now),
		EncryptionConfig:     req.EncryptionConfig,
	}

	m.Datasets[datasetArn] = dataset

	m.saveLocked()

	return datasetArn, nil
}

// DescribeDataset returns a dataset.
func (m *MemoryStorage) DescribeDataset(_ context.Context, datasetArn string) (*Dataset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dataset, exists := m.Datasets[datasetArn]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Dataset %s not found", datasetArn),
		}
	}

	return dataset, nil
}

// ListDatasets returns all datasets.
func (m *MemoryStorage) ListDatasets(_ context.Context, maxResults *int32, _ string) ([]*DatasetSummary, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := int32(100)
	if maxResults != nil && *maxResults > 0 && *maxResults < limit {
		limit = *maxResults
	}

	summaries := make([]*DatasetSummary, 0, len(m.Datasets))

	for _, ds := range m.Datasets {
		if int32(len(summaries)) >= limit { //nolint:gosec // G115: len(summaries) is bounded by limit which is int32
			break
		}

		summaries = append(summaries, &DatasetSummary{
			DatasetArn:           ds.DatasetArn,
			DatasetName:          ds.DatasetName,
			DatasetType:          ds.DatasetType,
			Domain:               ds.Domain,
			CreationTime:         ds.CreationTime,
			LastModificationTime: ds.LastModificationTime,
		})
	}

	return summaries, "", nil
}

// DeleteDataset deletes a dataset.
func (m *MemoryStorage) DeleteDataset(_ context.Context, datasetArn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Datasets[datasetArn]; !exists {
		return &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Dataset %s not found", datasetArn),
		}
	}

	// Check if dataset is in use by any dataset group.
	for _, dg := range m.DatasetGroups {
		if slices.Contains(dg.DatasetArns, datasetArn) {
			return &Error{
				Code:    errResourceInUseException,
				Message: fmt.Sprintf("Dataset %s is in use by dataset group %s", datasetArn, dg.DatasetGroupArn),
			}
		}
	}

	delete(m.Datasets, datasetArn)

	m.saveLocked()

	return nil
}

// DatasetGroup operations.

// CreateDatasetGroup creates a new dataset group.
func (m *MemoryStorage) CreateDatasetGroup(_ context.Context, req *CreateDatasetGroupInput) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate name.
	for _, dg := range m.DatasetGroups {
		if dg.DatasetGroupName == req.DatasetGroupName {
			return "", &Error{
				Code:    errResourceAlreadyExistsException,
				Message: fmt.Sprintf("A dataset group with name %s already exists", req.DatasetGroupName),
			}
		}
	}

	// Validate domain.
	if !isValidDomain(req.Domain) {
		return "", &Error{
			Code:    errInvalidInputException,
			Message: fmt.Sprintf("Invalid domain: %s", req.Domain),
		}
	}

	// Validate dataset ARNs exist and have matching domain.
	for _, dsArn := range req.DatasetArns {
		ds, exists := m.Datasets[dsArn]
		if !exists {
			return "", &Error{
				Code:    errResourceNotFoundException,
				Message: fmt.Sprintf("Dataset %s not found", dsArn),
			}
		}

		if ds.Domain != req.Domain {
			return "", &Error{
				Code:    errInvalidInputException,
				Message: fmt.Sprintf("Dataset %s has domain %s, but dataset group has domain %s", dsArn, ds.Domain, req.Domain),
			}
		}
	}

	datasetGroupArn := fmt.Sprintf("arn:aws:forecast:%s:%s:dataset-group/%s", m.region, m.accountID, req.DatasetGroupName)
	now := time.Now()

	datasetGroup := &DatasetGroup{
		DatasetGroupArn:      datasetGroupArn,
		DatasetGroupName:     req.DatasetGroupName,
		Domain:               req.Domain,
		DatasetArns:          req.DatasetArns,
		Status:               statusActive,
		CreationTime:         ToAWSTimestamp(now),
		LastModificationTime: ToAWSTimestamp(now),
	}

	m.DatasetGroups[datasetGroupArn] = datasetGroup

	m.saveLocked()

	return datasetGroupArn, nil
}

// DescribeDatasetGroup returns a dataset group.
func (m *MemoryStorage) DescribeDatasetGroup(_ context.Context, datasetGroupArn string) (*DatasetGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	datasetGroup, exists := m.DatasetGroups[datasetGroupArn]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Dataset group %s not found", datasetGroupArn),
		}
	}

	return datasetGroup, nil
}

// ListDatasetGroups returns all dataset groups.
func (m *MemoryStorage) ListDatasetGroups(_ context.Context, maxResults *int32, _ string) ([]*DatasetGroupSummary, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := int32(100)
	if maxResults != nil && *maxResults > 0 && *maxResults < limit {
		limit = *maxResults
	}

	summaries := make([]*DatasetGroupSummary, 0, len(m.DatasetGroups))

	for _, dg := range m.DatasetGroups {
		if int32(len(summaries)) >= limit { //nolint:gosec // G115: len(summaries) is bounded by limit which is int32
			break
		}

		summaries = append(summaries, &DatasetGroupSummary{
			DatasetGroupArn:      dg.DatasetGroupArn,
			DatasetGroupName:     dg.DatasetGroupName,
			CreationTime:         dg.CreationTime,
			LastModificationTime: dg.LastModificationTime,
		})
	}

	return summaries, "", nil
}

// DeleteDatasetGroup deletes a dataset group.
func (m *MemoryStorage) DeleteDatasetGroup(_ context.Context, datasetGroupArn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.DatasetGroups[datasetGroupArn]; !exists {
		return &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Dataset group %s not found", datasetGroupArn),
		}
	}

	// Check if dataset group is in use by any predictor.
	for _, p := range m.Predictors {
		if p.InputDataConfig != nil && p.InputDataConfig.DatasetGroupArn == datasetGroupArn {
			return &Error{
				Code:    errResourceInUseException,
				Message: fmt.Sprintf("Dataset group %s is in use by predictor %s", datasetGroupArn, p.PredictorArn),
			}
		}
	}

	delete(m.DatasetGroups, datasetGroupArn)

	m.saveLocked()

	return nil
}

// UpdateDatasetGroup updates a dataset group.
func (m *MemoryStorage) UpdateDatasetGroup(_ context.Context, datasetGroupArn string, datasetArns []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dg, exists := m.DatasetGroups[datasetGroupArn]
	if !exists {
		return &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Dataset group %s not found", datasetGroupArn),
		}
	}

	// Validate dataset ARNs exist and have matching domain.
	for _, dsArn := range datasetArns {
		ds, dsExists := m.Datasets[dsArn]
		if !dsExists {
			return &Error{
				Code:    errResourceNotFoundException,
				Message: fmt.Sprintf("Dataset %s not found", dsArn),
			}
		}

		if ds.Domain != dg.Domain {
			return &Error{
				Code:    errInvalidInputException,
				Message: fmt.Sprintf("Dataset %s has domain %s, but dataset group has domain %s", dsArn, ds.Domain, dg.Domain),
			}
		}
	}

	dg.DatasetArns = datasetArns
	dg.LastModificationTime = ToAWSTimestamp(time.Now())

	m.saveLocked()

	return nil
}

// Predictor operations.

// CreatePredictor creates a new predictor.
func (m *MemoryStorage) CreatePredictor(_ context.Context, req *CreatePredictorInput) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	dg, verr := m.validateCreatePredictor(req)
	if verr != nil {
		return "", verr
	}

	predictorID := uuid.New().String()[:8]
	predictorArn := fmt.Sprintf("arn:aws:forecast:%s:%s:predictor/%s_%s", m.region, m.accountID, req.PredictorName, predictorID)
	now := time.Now()

	forecastTypes := req.ForecastTypes
	if len(forecastTypes) == 0 {
		forecastTypes = []string{"0.1", "0.5", "0.9"}
	}

	predictor := &Predictor{
		PredictorArn:    predictorArn,
		PredictorName:   req.PredictorName,
		AlgorithmArn:    req.AlgorithmArn,
		ForecastHorizon: req.ForecastHorizon,
		ForecastTypes:   forecastTypes,
		InputDataConfig: &InputDataConfig{
			DatasetGroupArn: dg.DatasetGroupArn,
		},
		FeaturizationConfig:  req.FeaturizationConfig,
		Status:               statusActive,
		CreationTime:         ToAWSTimestamp(now),
		LastModificationTime: ToAWSTimestamp(now),
	}

	m.Predictors[predictorArn] = predictor

	m.saveLocked()

	return predictorArn, nil
}

// validateCreatePredictor validates a CreatePredictor request and returns the
// resolved dataset group. It must be called with m.mu held.
func (m *MemoryStorage) validateCreatePredictor(req *CreatePredictorInput) (*DatasetGroup, *Error) {
	// Check for duplicate name.
	for _, p := range m.Predictors {
		if p.PredictorName == req.PredictorName {
			return nil, &Error{
				Code:    errResourceAlreadyExistsException,
				Message: fmt.Sprintf("A predictor with name %s already exists", req.PredictorName),
			}
		}
	}

	if req.InputDataConfig == nil || req.InputDataConfig.DatasetGroupArn == "" {
		return nil, &Error{
			Code:    errInvalidInputException,
			Message: "InputDataConfig.DatasetGroupArn is required",
		}
	}

	dg, exists := m.DatasetGroups[req.InputDataConfig.DatasetGroupArn]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Dataset group %s not found", req.InputDataConfig.DatasetGroupArn),
		}
	}

	if req.ForecastHorizon < 1 || req.ForecastHorizon > 500 {
		return nil, &Error{
			Code:    errInvalidInputException,
			Message: "ForecastHorizon must be between 1 and 500",
		}
	}

	return dg, nil
}

// DescribePredictor returns a predictor.
func (m *MemoryStorage) DescribePredictor(_ context.Context, predictorArn string) (*Predictor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	predictor, exists := m.Predictors[predictorArn]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Predictor %s not found", predictorArn),
		}
	}

	return predictor, nil
}

// ListPredictors returns all predictors.
func (m *MemoryStorage) ListPredictors(_ context.Context, maxResults *int32, _ string, filters []Filter) ([]*PredictorSummary, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := int32(100)
	if maxResults != nil && *maxResults > 0 && *maxResults < limit {
		limit = *maxResults
	}

	summaries := make([]*PredictorSummary, 0, len(m.Predictors))

	for _, p := range m.Predictors {
		if int32(len(summaries)) >= limit { //nolint:gosec // G115: len(summaries) is bounded by limit which is int32
			break
		}

		// Apply filters.
		if !matchesFilters(p, filters) {
			continue
		}

		datasetGroupArn := ""
		if p.InputDataConfig != nil {
			datasetGroupArn = p.InputDataConfig.DatasetGroupArn
		}

		summaries = append(summaries, &PredictorSummary{
			PredictorArn:         p.PredictorArn,
			PredictorName:        p.PredictorName,
			DatasetGroupArn:      datasetGroupArn,
			Status:               p.Status,
			CreationTime:         p.CreationTime,
			LastModificationTime: p.LastModificationTime,
			Message:              p.Message,
		})
	}

	return summaries, "", nil
}

// DeletePredictor deletes a predictor.
func (m *MemoryStorage) DeletePredictor(_ context.Context, predictorArn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Predictors[predictorArn]; !exists {
		return &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Predictor %s not found", predictorArn),
		}
	}

	// Check if predictor is in use by any forecast.
	for _, f := range m.Forecasts {
		if f.PredictorArn == predictorArn {
			return &Error{
				Code:    errResourceInUseException,
				Message: fmt.Sprintf("Predictor %s is in use by forecast %s", predictorArn, f.ForecastArn),
			}
		}
	}

	delete(m.Predictors, predictorArn)

	m.saveLocked()

	return nil
}

// Forecast operations.

// CreateForecast creates a new forecast.
func (m *MemoryStorage) CreateForecast(_ context.Context, req *CreateForecastInput) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate name.
	for _, f := range m.Forecasts {
		if f.ForecastName == req.ForecastName {
			return "", &Error{
				Code:    errResourceAlreadyExistsException,
				Message: fmt.Sprintf("A forecast with name %s already exists", req.ForecastName),
			}
		}
	}

	// Validate predictor exists.
	predictor, exists := m.Predictors[req.PredictorArn]
	if !exists {
		return "", &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Predictor %s not found", req.PredictorArn),
		}
	}

	forecastID := uuid.New().String()[:8]
	forecastArn := fmt.Sprintf("arn:aws:forecast:%s:%s:forecast/%s_%s", m.region, m.accountID, req.ForecastName, forecastID)
	now := time.Now()

	forecastTypes := req.ForecastTypes
	if len(forecastTypes) == 0 {
		forecastTypes = predictor.ForecastTypes
	}

	datasetGroupArn := ""
	if predictor.InputDataConfig != nil {
		datasetGroupArn = predictor.InputDataConfig.DatasetGroupArn
	}

	forecast := &Forecast{
		ForecastArn:          forecastArn,
		ForecastName:         req.ForecastName,
		PredictorArn:         req.PredictorArn,
		DatasetGroupArn:      datasetGroupArn,
		ForecastTypes:        forecastTypes,
		Status:               statusActive,
		CreationTime:         ToAWSTimestamp(now),
		LastModificationTime: ToAWSTimestamp(now),
	}

	m.Forecasts[forecastArn] = forecast

	m.saveLocked()

	return forecastArn, nil
}

// DescribeForecast returns a forecast.
func (m *MemoryStorage) DescribeForecast(_ context.Context, forecastArn string) (*Forecast, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	forecast, exists := m.Forecasts[forecastArn]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Forecast %s not found", forecastArn),
		}
	}

	return forecast, nil
}

// ListForecasts returns all forecasts.
func (m *MemoryStorage) ListForecasts(_ context.Context, maxResults *int32, _ string, filters []Filter) ([]*ForecastSummary, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := int32(100)
	if maxResults != nil && *maxResults > 0 && *maxResults < limit {
		limit = *maxResults
	}

	summaries := make([]*ForecastSummary, 0, len(m.Forecasts))

	for _, f := range m.Forecasts {
		if int32(len(summaries)) >= limit { //nolint:gosec // G115: len(summaries) is bounded by limit which is int32
			break
		}

		// Apply filters.
		if !matchesForecastFilters(f, filters) {
			continue
		}

		summaries = append(summaries, &ForecastSummary{
			ForecastArn:          f.ForecastArn,
			ForecastName:         f.ForecastName,
			PredictorArn:         f.PredictorArn,
			DatasetGroupArn:      f.DatasetGroupArn,
			Status:               f.Status,
			CreationTime:         f.CreationTime,
			LastModificationTime: f.LastModificationTime,
			Message:              f.Message,
		})
	}

	return summaries, "", nil
}

// DeleteForecast deletes a forecast.
func (m *MemoryStorage) DeleteForecast(_ context.Context, forecastArn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Forecasts[forecastArn]; !exists {
		return &Error{
			Code:    errResourceNotFoundException,
			Message: fmt.Sprintf("Forecast %s not found", forecastArn),
		}
	}

	delete(m.Forecasts, forecastArn)

	m.saveLocked()

	return nil
}

// Helper functions.

func isValidDatasetType(datasetType string) bool {
	switch datasetType {
	case datasetTypeTargetTimeSeries, datasetTypeRelatedTimeSeries, datasetTypeItemMetadata:
		return true
	default:
		return false
	}
}

func isValidDomain(domain string) bool {
	switch domain {
	case domainRetail, domainCustom, domainInventoryPlanning, domainEC2Capacity, domainWorkForce, domainWebTraffic, domainMetrics:
		return true
	default:
		return false
	}
}

func matchesFilters(p *Predictor, filters []Filter) bool {
	for _, f := range filters {
		switch f.Key {
		case "DatasetGroupArn":
			if p.InputDataConfig == nil || p.InputDataConfig.DatasetGroupArn != f.Value {
				return false
			}
		case "Status":
			if p.Status != f.Value {
				return false
			}
		}
	}

	return true
}

func matchesForecastFilters(f *Forecast, filters []Filter) bool {
	for _, filter := range filters {
		switch filter.Key {
		case "PredictorArn":
			if f.PredictorArn != filter.Value {
				return false
			}
		case "DatasetGroupArn":
			if f.DatasetGroupArn != filter.Value {
				return false
			}
		case "Status":
			if f.Status != filter.Value {
				return false
			}
		}
	}

	return true
}
