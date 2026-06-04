package location

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errResourceNotFoundException = "ResourceNotFoundException"
	errValidationException       = "ValidationException"
	errConflictException         = "ConflictException"
	errInternalServerException   = "InternalServerException"
)

// Default values.
const (
	defaultRegion            = "us-east-1"
	defaultAccountID         = "123456789012"
	defaultPricingPlan       = "RequestBasedUsage"
	defaultPositionFiltering = "TimeBased"
	defaultMapStyle          = "VectorHereExplore"
	defaultDataSource        = "Esri"
	defaultMaxResults        = int32(100)
)

// Storage defines the Location service storage interface.
type Storage interface {
	// Map operations
	CreateMap(ctx context.Context, req *CreateMapRequest) (*CreateMapResponse, error)
	DescribeMap(ctx context.Context, name string) (*DescribeMapResponse, error)
	UpdateMap(ctx context.Context, name string, req *UpdateMapRequest) (*UpdateMapResponse, error)
	DeleteMap(ctx context.Context, name string) error
	ListMaps(ctx context.Context, maxResults *int32, nextToken string) (*ListMapsResponse, error)

	// Place index operations
	CreatePlaceIndex(ctx context.Context, req *CreatePlaceIndexRequest) (*CreatePlaceIndexResponse, error)
	DescribePlaceIndex(ctx context.Context, name string) (*DescribePlaceIndexResponse, error)
	UpdatePlaceIndex(ctx context.Context, name string, req *UpdatePlaceIndexRequest) (*UpdatePlaceIndexResponse, error)
	DeletePlaceIndex(ctx context.Context, name string) error
	ListPlaceIndexes(ctx context.Context, maxResults *int32, nextToken string) (*ListPlaceIndexesResponse, error)

	// Route calculator operations
	CreateRouteCalculator(ctx context.Context, req *CreateRouteCalculatorRequest) (*CreateRouteCalculatorResponse, error)
	DescribeRouteCalculator(ctx context.Context, name string) (*DescribeRouteCalculatorResponse, error)
	UpdateRouteCalculator(ctx context.Context, name string, req *UpdateRouteCalculatorRequest) (*UpdateRouteCalculatorResponse, error)
	DeleteRouteCalculator(ctx context.Context, name string) error
	ListRouteCalculators(ctx context.Context, maxResults *int32, nextToken string) (*ListRouteCalculatorsResponse, error)

	// Geofence collection operations
	CreateGeofenceCollection(ctx context.Context, req *CreateGeofenceCollectionRequest) (*CreateGeofenceCollectionResponse, error)
	DescribeGeofenceCollection(ctx context.Context, name string) (*DescribeGeofenceCollectionResponse, error)
	UpdateGeofenceCollection(ctx context.Context, name string, req *UpdateGeofenceCollectionRequest) (*UpdateGeofenceCollectionResponse, error)
	DeleteGeofenceCollection(ctx context.Context, name string) error
	ListGeofenceCollections(ctx context.Context, maxResults *int32, nextToken string) (*ListGeofenceCollectionsResponse, error)

	// Tracker operations
	CreateTracker(ctx context.Context, req *CreateTrackerRequest) (*CreateTrackerResponse, error)
	DescribeTracker(ctx context.Context, name string) (*DescribeTrackerResponse, error)
	UpdateTracker(ctx context.Context, name string, req *UpdateTrackerRequest) (*UpdateTrackerResponse, error)
	DeleteTracker(ctx context.Context, name string) error
	ListTrackers(ctx context.Context, maxResults *int32, nextToken string) (*ListTrackersResponse, error)
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
	Maps                map[string]*MapResource        `json:"maps"`
	PlaceIndexes        map[string]*PlaceIndex         `json:"placeIndexes"`
	RouteCalculators    map[string]*RouteCalculator    `json:"routeCalculators"`
	GeofenceCollections map[string]*GeofenceCollection `json:"geofenceCollections"`
	Trackers            map[string]*Tracker            `json:"trackers"`
	region              string
	accountID           string
	dataDir             string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Maps:                make(map[string]*MapResource),
		PlaceIndexes:        make(map[string]*PlaceIndex),
		RouteCalculators:    make(map[string]*RouteCalculator),
		GeofenceCollections: make(map[string]*GeofenceCollection),
		Trackers:            make(map[string]*Tracker),
		region:              region,
		accountID:           defaultAccountID,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "location", s)
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

	if m.Maps == nil {
		m.Maps = make(map[string]*MapResource)
	}

	if m.PlaceIndexes == nil {
		m.PlaceIndexes = make(map[string]*PlaceIndex)
	}

	if m.RouteCalculators == nil {
		m.RouteCalculators = make(map[string]*RouteCalculator)
	}

	if m.GeofenceCollections == nil {
		m.GeofenceCollections = make(map[string]*GeofenceCollection)
	}

	if m.Trackers == nil {
		m.Trackers = make(map[string]*Tracker)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "location", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "location", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// --- Map operations ---

// CreateMap creates a new map resource.
func (m *MemoryStorage) CreateMap(_ context.Context, req *CreateMapRequest) (*CreateMapResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Maps[req.MapName]; exists {
		return nil, &Error{Code: errConflictException, Message: "Map already exists: " + req.MapName}
	}

	now := time.Now()
	style := defaultString(req.Configuration.Style, defaultMapStyle)
	arn := fmt.Sprintf("arn:aws:geo:%s:%s:map/%s", m.region, m.accountID, req.MapName)

	m.Maps[req.MapName] = &MapResource{
		Name:          req.MapName,
		ARN:           arn,
		Description:   req.Description,
		Configuration: MapConfiguration{Style: style},
		PricingPlan:   defaultString(req.PricingPlan, defaultPricingPlan),
		Tags:          maps.Clone(req.Tags),
		CreateTime:    now,
		UpdateTime:    now,
	}

	m.saveLocked()

	return &CreateMapResponse{
		MapName:    req.MapName,
		MapArn:     arn,
		CreateTime: now,
	}, nil
}

// DescribeMap describes a map resource.
func (m *MemoryStorage) DescribeMap(_ context.Context, name string) (*DescribeMapResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mr, exists := m.Maps[name]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Map not found: " + name}
	}

	return &DescribeMapResponse{
		MapName:       mr.Name,
		MapArn:        mr.ARN,
		Configuration: MapConfigurationOutput{Style: mr.Configuration.Style},
		Description:   mr.Description,
		PricingPlan:   mr.PricingPlan,
		Tags:          mr.Tags,
		CreateTime:    mr.CreateTime,
		UpdateTime:    mr.UpdateTime,
	}, nil
}

// UpdateMap updates a map resource.
func (m *MemoryStorage) UpdateMap(_ context.Context, name string, req *UpdateMapRequest) (*UpdateMapResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mr, exists := m.Maps[name]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Map not found: " + name}
	}

	if req.Description != "" {
		mr.Description = req.Description
	}

	if req.PricingPlan != "" {
		mr.PricingPlan = req.PricingPlan
	}

	mr.UpdateTime = time.Now()

	m.saveLocked()

	return &UpdateMapResponse{
		MapName:    mr.Name,
		MapArn:     mr.ARN,
		UpdateTime: mr.UpdateTime,
	}, nil
}

// DeleteMap deletes a map resource.
func (m *MemoryStorage) DeleteMap(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Maps[name]; !exists {
		return &Error{Code: errResourceNotFoundException, Message: "Map not found: " + name}
	}

	delete(m.Maps, name)

	m.saveLocked()

	return nil
}

// ListMaps lists map resources.
func (m *MemoryStorage) ListMaps(_ context.Context, maxResults *int32, _ string) (*ListMapsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := defaultMaxResults
	if maxResults != nil && *maxResults > 0 {
		limit = *maxResults
	}

	entries := make([]ListMapsEntry, 0, len(m.Maps))

	for _, mr := range m.Maps {
		entries = append(entries, ListMapsEntry{
			MapName:     mr.Name,
			Description: mr.Description,
			CreateTime:  mr.CreateTime,
			UpdateTime:  mr.UpdateTime,
		})

		//nolint:gosec // len(entries) is bounded by the number of maps.
		if int32(len(entries)) >= limit {
			break
		}
	}

	return &ListMapsResponse{Entries: entries}, nil
}

// --- Place index operations ---

// CreatePlaceIndex creates a new place index.
func (m *MemoryStorage) CreatePlaceIndex(_ context.Context, req *CreatePlaceIndexRequest) (*CreatePlaceIndexResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.PlaceIndexes[req.IndexName]; exists {
		return nil, &Error{Code: errConflictException, Message: "Place index already exists: " + req.IndexName}
	}

	now := time.Now()
	arn := fmt.Sprintf("arn:aws:geo:%s:%s:place-index/%s", m.region, m.accountID, req.IndexName)

	dsc := DataSourceConfiguration{}
	if req.DataSourceConfiguration != nil {
		dsc.IntendedUse = req.DataSourceConfiguration.IntendedUse
	}

	m.PlaceIndexes[req.IndexName] = &PlaceIndex{
		IndexName:               req.IndexName,
		ARN:                     arn,
		Description:             req.Description,
		DataSource:              defaultString(req.DataSource, defaultDataSource),
		DataSourceConfiguration: dsc,
		PricingPlan:             defaultString(req.PricingPlan, defaultPricingPlan),
		Tags:                    maps.Clone(req.Tags),
		CreateTime:              now,
		UpdateTime:              now,
	}

	m.saveLocked()

	return &CreatePlaceIndexResponse{
		IndexName:  req.IndexName,
		IndexArn:   arn,
		CreateTime: now,
	}, nil
}

// DescribePlaceIndex describes a place index.
func (m *MemoryStorage) DescribePlaceIndex(_ context.Context, name string) (*DescribePlaceIndexResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pi, exists := m.PlaceIndexes[name]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Place index not found: " + name}
	}

	return &DescribePlaceIndexResponse{
		IndexName:               pi.IndexName,
		IndexArn:                pi.ARN,
		DataSource:              pi.DataSource,
		DataSourceConfiguration: DataSourceConfigurationOutput{IntendedUse: pi.DataSourceConfiguration.IntendedUse},
		Description:             pi.Description,
		PricingPlan:             pi.PricingPlan,
		Tags:                    pi.Tags,
		CreateTime:              pi.CreateTime,
		UpdateTime:              pi.UpdateTime,
	}, nil
}

// UpdatePlaceIndex updates a place index.
func (m *MemoryStorage) UpdatePlaceIndex(_ context.Context, name string, req *UpdatePlaceIndexRequest) (*UpdatePlaceIndexResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pi, exists := m.PlaceIndexes[name]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Place index not found: " + name}
	}

	if req.Description != "" {
		pi.Description = req.Description
	}

	if req.PricingPlan != "" {
		pi.PricingPlan = req.PricingPlan
	}

	if req.DataSourceConfiguration != nil && req.DataSourceConfiguration.IntendedUse != "" {
		pi.DataSourceConfiguration.IntendedUse = req.DataSourceConfiguration.IntendedUse
	}

	pi.UpdateTime = time.Now()

	m.saveLocked()

	return &UpdatePlaceIndexResponse{
		IndexName:  pi.IndexName,
		IndexArn:   pi.ARN,
		UpdateTime: pi.UpdateTime,
	}, nil
}

// DeletePlaceIndex deletes a place index.
func (m *MemoryStorage) DeletePlaceIndex(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.PlaceIndexes[name]; !exists {
		return &Error{Code: errResourceNotFoundException, Message: "Place index not found: " + name}
	}

	delete(m.PlaceIndexes, name)

	m.saveLocked()

	return nil
}

// ListPlaceIndexes lists place indexes.
func (m *MemoryStorage) ListPlaceIndexes(_ context.Context, maxResults *int32, _ string) (*ListPlaceIndexesResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := defaultMaxResults
	if maxResults != nil && *maxResults > 0 {
		limit = *maxResults
	}

	entries := make([]ListPlaceIndexesEntry, 0, len(m.PlaceIndexes))

	for _, pi := range m.PlaceIndexes {
		entries = append(entries, ListPlaceIndexesEntry{
			IndexName:   pi.IndexName,
			Description: pi.Description,
			DataSource:  pi.DataSource,
			CreateTime:  pi.CreateTime,
			UpdateTime:  pi.UpdateTime,
		})

		//nolint:gosec // len(entries) is bounded by the number of place indexes.
		if int32(len(entries)) >= limit {
			break
		}
	}

	return &ListPlaceIndexesResponse{Entries: entries}, nil
}

// --- Route calculator operations ---

// CreateRouteCalculator creates a new route calculator.
func (m *MemoryStorage) CreateRouteCalculator(_ context.Context, req *CreateRouteCalculatorRequest) (*CreateRouteCalculatorResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.RouteCalculators[req.CalculatorName]; exists {
		return nil, &Error{Code: errConflictException, Message: "Route calculator already exists: " + req.CalculatorName}
	}

	now := time.Now()
	arn := fmt.Sprintf("arn:aws:geo:%s:%s:route-calculator/%s", m.region, m.accountID, req.CalculatorName)

	m.RouteCalculators[req.CalculatorName] = &RouteCalculator{
		CalculatorName: req.CalculatorName,
		ARN:            arn,
		Description:    req.Description,
		DataSource:     defaultString(req.DataSource, defaultDataSource),
		PricingPlan:    defaultString(req.PricingPlan, defaultPricingPlan),
		Tags:           maps.Clone(req.Tags),
		CreateTime:     now,
		UpdateTime:     now,
	}

	m.saveLocked()

	return &CreateRouteCalculatorResponse{
		CalculatorName: req.CalculatorName,
		CalculatorArn:  arn,
		CreateTime:     now,
	}, nil
}

// DescribeRouteCalculator describes a route calculator.
func (m *MemoryStorage) DescribeRouteCalculator(_ context.Context, name string) (*DescribeRouteCalculatorResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rc, exists := m.RouteCalculators[name]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Route calculator not found: " + name}
	}

	return &DescribeRouteCalculatorResponse{
		CalculatorName: rc.CalculatorName,
		CalculatorArn:  rc.ARN,
		DataSource:     rc.DataSource,
		Description:    rc.Description,
		PricingPlan:    rc.PricingPlan,
		Tags:           rc.Tags,
		CreateTime:     rc.CreateTime,
		UpdateTime:     rc.UpdateTime,
	}, nil
}

// UpdateRouteCalculator updates a route calculator.
func (m *MemoryStorage) UpdateRouteCalculator(_ context.Context, name string, req *UpdateRouteCalculatorRequest) (*UpdateRouteCalculatorResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rc, exists := m.RouteCalculators[name]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Route calculator not found: " + name}
	}

	if req.Description != "" {
		rc.Description = req.Description
	}

	if req.PricingPlan != "" {
		rc.PricingPlan = req.PricingPlan
	}

	rc.UpdateTime = time.Now()

	m.saveLocked()

	return &UpdateRouteCalculatorResponse{
		CalculatorName: rc.CalculatorName,
		CalculatorArn:  rc.ARN,
		UpdateTime:     rc.UpdateTime,
	}, nil
}

// DeleteRouteCalculator deletes a route calculator.
func (m *MemoryStorage) DeleteRouteCalculator(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.RouteCalculators[name]; !exists {
		return &Error{Code: errResourceNotFoundException, Message: "Route calculator not found: " + name}
	}

	delete(m.RouteCalculators, name)

	m.saveLocked()

	return nil
}

// ListRouteCalculators lists route calculators.
func (m *MemoryStorage) ListRouteCalculators(_ context.Context, maxResults *int32, _ string) (*ListRouteCalculatorsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := defaultMaxResults
	if maxResults != nil && *maxResults > 0 {
		limit = *maxResults
	}

	entries := make([]ListRouteCalculatorsEntry, 0, len(m.RouteCalculators))

	for _, rc := range m.RouteCalculators {
		entries = append(entries, ListRouteCalculatorsEntry{
			CalculatorName: rc.CalculatorName,
			Description:    rc.Description,
			DataSource:     rc.DataSource,
			CreateTime:     rc.CreateTime,
			UpdateTime:     rc.UpdateTime,
		})

		//nolint:gosec // len(entries) is bounded by the number of route calculators.
		if int32(len(entries)) >= limit {
			break
		}
	}

	return &ListRouteCalculatorsResponse{Entries: entries}, nil
}

// --- Geofence collection operations ---

// CreateGeofenceCollection creates a new geofence collection.
func (m *MemoryStorage) CreateGeofenceCollection(_ context.Context, req *CreateGeofenceCollectionRequest) (*CreateGeofenceCollectionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.GeofenceCollections[req.CollectionName]; exists {
		return nil, &Error{Code: errConflictException, Message: "Geofence collection already exists: " + req.CollectionName}
	}

	now := time.Now()
	arn := fmt.Sprintf("arn:aws:geo:%s:%s:geofence-collection/%s", m.region, m.accountID, req.CollectionName)

	m.GeofenceCollections[req.CollectionName] = &GeofenceCollection{
		CollectionName: req.CollectionName,
		ARN:            arn,
		Description:    req.Description,
		PricingPlan:    defaultString(req.PricingPlan, defaultPricingPlan),
		Tags:           maps.Clone(req.Tags),
		CreateTime:     now,
		UpdateTime:     now,
	}

	m.saveLocked()

	return &CreateGeofenceCollectionResponse{
		CollectionName: req.CollectionName,
		CollectionArn:  arn,
		CreateTime:     now,
	}, nil
}

// DescribeGeofenceCollection describes a geofence collection.
func (m *MemoryStorage) DescribeGeofenceCollection(_ context.Context, name string) (*DescribeGeofenceCollectionResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	gc, exists := m.GeofenceCollections[name]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Geofence collection not found: " + name}
	}

	return &DescribeGeofenceCollectionResponse{
		CollectionName: gc.CollectionName,
		CollectionArn:  gc.ARN,
		Description:    gc.Description,
		PricingPlan:    gc.PricingPlan,
		Tags:           gc.Tags,
		CreateTime:     gc.CreateTime,
		UpdateTime:     gc.UpdateTime,
	}, nil
}

// UpdateGeofenceCollection updates a geofence collection.
func (m *MemoryStorage) UpdateGeofenceCollection(_ context.Context, name string, req *UpdateGeofenceCollectionRequest) (*UpdateGeofenceCollectionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	gc, exists := m.GeofenceCollections[name]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Geofence collection not found: " + name}
	}

	if req.Description != "" {
		gc.Description = req.Description
	}

	if req.PricingPlan != "" {
		gc.PricingPlan = req.PricingPlan
	}

	gc.UpdateTime = time.Now()

	m.saveLocked()

	return &UpdateGeofenceCollectionResponse{
		CollectionName: gc.CollectionName,
		CollectionArn:  gc.ARN,
		UpdateTime:     gc.UpdateTime,
	}, nil
}

// DeleteGeofenceCollection deletes a geofence collection.
func (m *MemoryStorage) DeleteGeofenceCollection(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.GeofenceCollections[name]; !exists {
		return &Error{Code: errResourceNotFoundException, Message: "Geofence collection not found: " + name}
	}

	delete(m.GeofenceCollections, name)

	m.saveLocked()

	return nil
}

// ListGeofenceCollections lists geofence collections.
func (m *MemoryStorage) ListGeofenceCollections(_ context.Context, maxResults *int32, _ string) (*ListGeofenceCollectionsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := defaultMaxResults
	if maxResults != nil && *maxResults > 0 {
		limit = *maxResults
	}

	entries := make([]ListGeofenceCollectionsEntry, 0, len(m.GeofenceCollections))

	for _, gc := range m.GeofenceCollections {
		entries = append(entries, ListGeofenceCollectionsEntry{
			CollectionName: gc.CollectionName,
			Description:    gc.Description,
			CreateTime:     gc.CreateTime,
			UpdateTime:     gc.UpdateTime,
		})

		//nolint:gosec // len(entries) is bounded by the number of geofence collections.
		if int32(len(entries)) >= limit {
			break
		}
	}

	return &ListGeofenceCollectionsResponse{Entries: entries}, nil
}

// --- Tracker operations ---

// CreateTracker creates a new tracker.
func (m *MemoryStorage) CreateTracker(_ context.Context, req *CreateTrackerRequest) (*CreateTrackerResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Trackers[req.TrackerName]; exists {
		return nil, &Error{Code: errConflictException, Message: "Tracker already exists: " + req.TrackerName}
	}

	now := time.Now()
	arn := fmt.Sprintf("arn:aws:geo:%s:%s:tracker/%s", m.region, m.accountID, req.TrackerName)

	m.Trackers[req.TrackerName] = &Tracker{
		TrackerName:       req.TrackerName,
		ARN:               arn,
		Description:       req.Description,
		PricingPlan:       defaultString(req.PricingPlan, defaultPricingPlan),
		PositionFiltering: defaultString(req.PositionFiltering, defaultPositionFiltering),
		Tags:              maps.Clone(req.Tags),
		CreateTime:        now,
		UpdateTime:        now,
	}

	m.saveLocked()

	return &CreateTrackerResponse{
		TrackerName: req.TrackerName,
		TrackerArn:  arn,
		CreateTime:  now,
	}, nil
}

// DescribeTracker describes a tracker.
func (m *MemoryStorage) DescribeTracker(_ context.Context, name string) (*DescribeTrackerResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, exists := m.Trackers[name]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Tracker not found: " + name}
	}

	return &DescribeTrackerResponse{
		TrackerName:       t.TrackerName,
		TrackerArn:        t.ARN,
		Description:       t.Description,
		PricingPlan:       t.PricingPlan,
		PositionFiltering: t.PositionFiltering,
		Tags:              t.Tags,
		CreateTime:        t.CreateTime,
		UpdateTime:        t.UpdateTime,
	}, nil
}

// UpdateTracker updates a tracker.
func (m *MemoryStorage) UpdateTracker(_ context.Context, name string, req *UpdateTrackerRequest) (*UpdateTrackerResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, exists := m.Trackers[name]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Tracker not found: " + name}
	}

	if req.Description != "" {
		t.Description = req.Description
	}

	if req.PricingPlan != "" {
		t.PricingPlan = req.PricingPlan
	}

	if req.PositionFiltering != "" {
		t.PositionFiltering = req.PositionFiltering
	}

	t.UpdateTime = time.Now()

	m.saveLocked()

	return &UpdateTrackerResponse{
		TrackerName: t.TrackerName,
		TrackerArn:  t.ARN,
		UpdateTime:  t.UpdateTime,
	}, nil
}

// DeleteTracker deletes a tracker.
func (m *MemoryStorage) DeleteTracker(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Trackers[name]; !exists {
		return &Error{Code: errResourceNotFoundException, Message: "Tracker not found: " + name}
	}

	delete(m.Trackers, name)

	m.saveLocked()

	return nil
}

// ListTrackers lists trackers.
func (m *MemoryStorage) ListTrackers(_ context.Context, maxResults *int32, _ string) (*ListTrackersResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := defaultMaxResults
	if maxResults != nil && *maxResults > 0 {
		limit = *maxResults
	}

	entries := make([]ListTrackersEntry, 0, len(m.Trackers))

	for _, t := range m.Trackers {
		entries = append(entries, ListTrackersEntry{
			TrackerName: t.TrackerName,
			Description: t.Description,
			CreateTime:  t.CreateTime,
			UpdateTime:  t.UpdateTime,
		})

		//nolint:gosec // len(entries) is bounded by the number of trackers.
		if int32(len(entries)) >= limit {
			break
		}
	}

	return &ListTrackersResponse{Entries: entries}, nil
}

// Helper functions.

func defaultString(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}

	return value
}
