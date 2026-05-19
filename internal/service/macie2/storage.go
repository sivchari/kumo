package macie2

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errResourceNotFoundException     = "ResourceNotFoundException"
	errValidationException           = "ValidationException"
	errConflictException             = "ConflictException"
	errInternalServerException       = "InternalServerException"
	errServiceQuotaExceededException = "ServiceQuotaExceededException"
)

// Default values.
const (
	defaultRegion                     = "us-east-1"
	defaultAccountID                  = "123456789012"
	defaultFindingPublishingFrequency = "FIFTEEN_MINUTES"
	defaultStatus                     = "ENABLED"
	defaultJobStatus                  = "RUNNING"
	defaultServiceRole                = "arn:aws:iam::123456789012:role/aws-service-role/macie.amazonaws.com/AWSServiceRoleForAmazonMacie"
	defaultMaxResults                 = int32(100)
)

// Storage defines the Macie2 service storage interface.
type Storage interface {
	// Macie session operations
	EnableMacie(ctx context.Context, req *EnableMacieRequest) (*EnableMacieResponse, error)
	GetMacieSession(ctx context.Context) (*GetMacieSessionResponse, error)
	UpdateMacieSession(ctx context.Context, req *UpdateMacieSessionRequest) (*UpdateMacieSessionResponse, error)
	DisableMacie(ctx context.Context) (*DisableMacieResponse, error)

	// Allow list operations
	CreateAllowList(ctx context.Context, req *CreateAllowListRequest) (*CreateAllowListResponse, error)
	GetAllowList(ctx context.Context, id string) (*GetAllowListResponse, error)
	UpdateAllowList(ctx context.Context, id string, req *UpdateAllowListRequest) (*UpdateAllowListResponse, error)
	DeleteAllowList(ctx context.Context, id string) error
	ListAllowLists(ctx context.Context) (*ListAllowListsResponse, error)

	// Classification job operations
	CreateClassificationJob(ctx context.Context, req *CreateClassificationJobRequest) (*CreateClassificationJobResponse, error)
	DescribeClassificationJob(ctx context.Context, jobID string) (*DescribeClassificationJobResponse, error)
	ListClassificationJobs(ctx context.Context, maxResults *int32, nextToken string) (*ListClassificationJobsResponse, error)
	UpdateClassificationJob(ctx context.Context, jobID string, req *UpdateClassificationJobRequest) (*UpdateClassificationJobResponse, error)

	// Custom data identifier operations
	CreateCustomDataIdentifier(ctx context.Context, req *CreateCustomDataIdentifierRequest) (*CreateCustomDataIdentifierResponse, error)
	GetCustomDataIdentifier(ctx context.Context, id string) (*GetCustomDataIdentifierResponse, error)
	DeleteCustomDataIdentifier(ctx context.Context, id string) error
	ListCustomDataIdentifiers(ctx context.Context, maxResults *int32, nextToken string) (*ListCustomDataIdentifiersResponse, error)

	// Findings filter operations
	CreateFindingsFilter(ctx context.Context, req *CreateFindingsFilterRequest) (*CreateFindingsFilterResponse, error)
	GetFindingsFilter(ctx context.Context, id string) (*GetFindingsFilterResponse, error)
	UpdateFindingsFilter(ctx context.Context, id string, req *UpdateFindingsFilterRequest) (*UpdateFindingsFilterResponse, error)
	DeleteFindingsFilter(ctx context.Context, id string) error
	ListFindingsFilters(ctx context.Context) (*ListFindingsFiltersResponse, error)

	// Findings operations
	GetFindings(ctx context.Context, req *GetFindingsRequest) (*GetFindingsResponse, error)
	ListFindings(ctx context.Context, req *ListFindingsRequest) (*ListFindingsResponse, error)
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
	mu                    sync.RWMutex                     `json:"-"`
	Session               *MacieSession                    `json:"session"`
	AllowLists            map[string]*AllowList            `json:"allowLists"`
	ClassificationJobs    map[string]*ClassificationJob    `json:"classificationJobs"`
	CustomDataIdentifiers map[string]*CustomDataIdentifier `json:"customDataIdentifiers"`
	FindingsFilters       map[string]*FindingsFilter       `json:"findingsFilters"`
	Findings              map[string]*Finding              `json:"findings"`
	region                string
	accountID             string
	dataDir               string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		AllowLists:            make(map[string]*AllowList),
		ClassificationJobs:    make(map[string]*ClassificationJob),
		CustomDataIdentifiers: make(map[string]*CustomDataIdentifier),
		FindingsFilters:       make(map[string]*FindingsFilter),
		Findings:              make(map[string]*Finding),
		region:                region,
		accountID:             defaultAccountID,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "macie2", s)
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

	if m.AllowLists == nil {
		m.AllowLists = make(map[string]*AllowList)
	}

	if m.ClassificationJobs == nil {
		m.ClassificationJobs = make(map[string]*ClassificationJob)
	}

	if m.CustomDataIdentifiers == nil {
		m.CustomDataIdentifiers = make(map[string]*CustomDataIdentifier)
	}

	if m.FindingsFilters == nil {
		m.FindingsFilters = make(map[string]*FindingsFilter)
	}

	if m.Findings == nil {
		m.Findings = make(map[string]*Finding)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	type alias MemoryStorage

	data, err := json.Marshal(&struct{ *alias }{alias: (*alias)(m)})
	if err != nil {
		return
	}

	_ = storage.SaveBytes(m.dataDir, "macie2", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "macie2", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// --- Macie session operations ---

// EnableMacie enables Amazon Macie for the account.
func (m *MemoryStorage) EnableMacie(_ context.Context, req *EnableMacieRequest) (*EnableMacieResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Session != nil && m.Session.Status == defaultStatus {
		return nil, &Error{Code: errConflictException, Message: "Macie is already enabled"}
	}

	now := time.Now()
	freq := defaultString(req.FindingPublishingFrequency, defaultFindingPublishingFrequency)
	status := defaultString(req.Status, defaultStatus)

	m.Session = &MacieSession{
		FindingPublishingFrequency: freq,
		ServiceRole:                defaultServiceRole,
		Status:                     status,
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}

	m.saveLocked()

	return &EnableMacieResponse{}, nil
}

// GetMacieSession retrieves the Macie session.
func (m *MemoryStorage) GetMacieSession(_ context.Context) (*GetMacieSessionResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.Session == nil {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Macie is not enabled"}
	}

	return &GetMacieSessionResponse{
		CreatedAt:                  m.Session.CreatedAt,
		FindingPublishingFrequency: m.Session.FindingPublishingFrequency,
		ServiceRole:                m.Session.ServiceRole,
		Status:                     m.Session.Status,
		UpdatedAt:                  m.Session.UpdatedAt,
	}, nil
}

// UpdateMacieSession updates the Macie session.
func (m *MemoryStorage) UpdateMacieSession(_ context.Context, req *UpdateMacieSessionRequest) (*UpdateMacieSessionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Session == nil {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Macie is not enabled"}
	}

	if req.FindingPublishingFrequency != "" {
		m.Session.FindingPublishingFrequency = req.FindingPublishingFrequency
	}

	if req.Status != "" {
		m.Session.Status = req.Status
	}

	m.Session.UpdatedAt = time.Now()

	m.saveLocked()

	return &UpdateMacieSessionResponse{}, nil
}

// DisableMacie disables Amazon Macie for the account.
func (m *MemoryStorage) DisableMacie(_ context.Context) (*DisableMacieResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Session == nil {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Macie is not enabled"}
	}

	m.Session = nil

	m.saveLocked()

	return &DisableMacieResponse{}, nil
}

// --- Allow list operations ---

// CreateAllowList creates a new allow list.
func (m *MemoryStorage) CreateAllowList(_ context.Context, req *CreateAllowListRequest) (*CreateAllowListResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	now := time.Now()
	arn := fmt.Sprintf("arn:aws:macie2:%s:%s:allow-list/%s", m.region, m.accountID, id)

	criteria := AllowListCriteria{
		Regex: req.Criteria.Regex,
	}
	if req.Criteria.S3WordsList != nil {
		criteria.S3WordsList = &S3WordsList{
			BucketName: req.Criteria.S3WordsList.BucketName,
			ObjectKey:  req.Criteria.S3WordsList.ObjectKey,
		}
	}

	m.AllowLists[id] = &AllowList{
		ID:          id,
		Name:        req.Name,
		ARN:         arn,
		Description: req.Description,
		Criteria:    criteria,
		Tags:        maps.Clone(req.Tags),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.saveLocked()

	return &CreateAllowListResponse{
		ID:  id,
		ARN: arn,
	}, nil
}

// GetAllowList retrieves an allow list by ID.
func (m *MemoryStorage) GetAllowList(_ context.Context, id string) (*GetAllowListResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	al, exists := m.AllowLists[id]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Allow list not found: " + id}
	}

	criteriaOut := GetAllowListCriteriaOutput{
		Regex: al.Criteria.Regex,
	}
	if al.Criteria.S3WordsList != nil {
		criteriaOut.S3WordsList = &S3WordsListOutput{
			BucketName: al.Criteria.S3WordsList.BucketName,
			ObjectKey:  al.Criteria.S3WordsList.ObjectKey,
		}
	}

	return &GetAllowListResponse{
		ID:          al.ID,
		Name:        al.Name,
		ARN:         al.ARN,
		Description: al.Description,
		Criteria:    criteriaOut,
		Tags:        al.Tags,
		CreatedAt:   al.CreatedAt,
		UpdatedAt:   al.UpdatedAt,
	}, nil
}

// UpdateAllowList updates an allow list.
func (m *MemoryStorage) UpdateAllowList(_ context.Context, id string, req *UpdateAllowListRequest) (*UpdateAllowListResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	al, exists := m.AllowLists[id]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Allow list not found: " + id}
	}

	if req.Name != "" {
		al.Name = req.Name
	}

	if req.Description != "" {
		al.Description = req.Description
	}

	criteria := AllowListCriteria{
		Regex: req.Criteria.Regex,
	}
	if req.Criteria.S3WordsList != nil {
		criteria.S3WordsList = &S3WordsList{
			BucketName: req.Criteria.S3WordsList.BucketName,
			ObjectKey:  req.Criteria.S3WordsList.ObjectKey,
		}
	}

	al.Criteria = criteria
	al.UpdatedAt = time.Now()

	m.saveLocked()

	return &UpdateAllowListResponse{
		ID:  al.ID,
		ARN: al.ARN,
	}, nil
}

// DeleteAllowList deletes an allow list.
func (m *MemoryStorage) DeleteAllowList(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.AllowLists[id]; !exists {
		return &Error{Code: errResourceNotFoundException, Message: "Allow list not found: " + id}
	}

	delete(m.AllowLists, id)

	m.saveLocked()

	return nil
}

// ListAllowLists lists all allow lists.
func (m *MemoryStorage) ListAllowLists(_ context.Context) (*ListAllowListsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]AllowListSummary, 0, len(m.AllowLists))

	for _, al := range m.AllowLists {
		entries = append(entries, AllowListSummary{
			ID:          al.ID,
			Name:        al.Name,
			ARN:         al.ARN,
			Description: al.Description,
			CreatedAt:   al.CreatedAt,
			UpdatedAt:   al.UpdatedAt,
		})
	}

	return &ListAllowListsResponse{AllowLists: entries}, nil
}

// --- Classification job operations ---

// CreateClassificationJob creates a new classification job.
func (m *MemoryStorage) CreateClassificationJob(_ context.Context, req *CreateClassificationJobRequest) (*CreateClassificationJobResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	jobID := uuid.New().String()
	now := time.Now()
	arn := fmt.Sprintf("arn:aws:macie2:%s:%s:classification-job/%s", m.region, m.accountID, jobID)

	bucketDefs := make([]BucketDefinition, 0, len(req.S3JobDefinition.BucketDefinitions))
	for _, bd := range req.S3JobDefinition.BucketDefinitions {
		bucketDefs = append(bucketDefs, BucketDefinition{
			AccountID: bd.AccountID,
			Buckets:   slices.Clone(bd.Buckets),
		})
	}

	m.ClassificationJobs[jobID] = &ClassificationJob{
		JobID:       jobID,
		Name:        req.Name,
		Description: req.Description,
		JobType:     req.JobType,
		JobStatus:   defaultJobStatus,
		S3JobDefinition: S3JobDefinition{
			BucketDefinitions: bucketDefs,
		},
		Tags:      maps.Clone(req.Tags),
		CreatedAt: now,
	}

	m.saveLocked()

	return &CreateClassificationJobResponse{
		JobID:  jobID,
		JobArn: arn,
	}, nil
}

// DescribeClassificationJob describes a classification job.
func (m *MemoryStorage) DescribeClassificationJob(_ context.Context, jobID string) (*DescribeClassificationJobResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.ClassificationJobs[jobID]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Classification job not found: " + jobID}
	}

	bucketDefs := make([]BucketDefinitionOutput, 0, len(job.S3JobDefinition.BucketDefinitions))
	for _, bd := range job.S3JobDefinition.BucketDefinitions {
		bucketDefs = append(bucketDefs, BucketDefinitionOutput(bd))
	}

	return &DescribeClassificationJobResponse{
		JobID:       job.JobID,
		Name:        job.Name,
		Description: job.Description,
		JobType:     job.JobType,
		JobStatus:   job.JobStatus,
		S3JobDefinition: S3JobDefinitionOutput{
			BucketDefinitions: bucketDefs,
		},
		Tags:      job.Tags,
		CreatedAt: job.CreatedAt,
	}, nil
}

// ListClassificationJobs lists classification jobs.
func (m *MemoryStorage) ListClassificationJobs(_ context.Context, maxResults *int32, _ string) (*ListClassificationJobsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := defaultMaxResults
	if maxResults != nil && *maxResults > 0 {
		limit = *maxResults
	}

	items := make([]ClassificationJobSummary, 0, len(m.ClassificationJobs))

	for _, job := range m.ClassificationJobs {
		items = append(items, ClassificationJobSummary{
			JobID:     job.JobID,
			Name:      job.Name,
			JobType:   job.JobType,
			JobStatus: job.JobStatus,
			CreatedAt: job.CreatedAt,
		})

		//nolint:gosec // len(items) is bounded by the number of classification jobs.
		if int32(len(items)) >= limit {
			break
		}
	}

	return &ListClassificationJobsResponse{Items: items}, nil
}

// UpdateClassificationJob updates a classification job.
func (m *MemoryStorage) UpdateClassificationJob(_ context.Context, jobID string, req *UpdateClassificationJobRequest) (*UpdateClassificationJobResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.ClassificationJobs[jobID]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Classification job not found: " + jobID}
	}

	if req.JobStatus != "" {
		job.JobStatus = req.JobStatus
	}

	m.saveLocked()

	return &UpdateClassificationJobResponse{}, nil
}

// --- Custom data identifier operations ---

// CreateCustomDataIdentifier creates a new custom data identifier.
func (m *MemoryStorage) CreateCustomDataIdentifier(_ context.Context, req *CreateCustomDataIdentifierRequest) (*CreateCustomDataIdentifierResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	now := time.Now()
	arn := fmt.Sprintf("arn:aws:macie2:%s:%s:custom-data-identifier/%s", m.region, m.accountID, id)

	m.CustomDataIdentifiers[id] = &CustomDataIdentifier{
		ID:          id,
		Name:        req.Name,
		ARN:         arn,
		Description: req.Description,
		Regex:       req.Regex,
		Keywords:    slices.Clone(req.Keywords),
		Tags:        maps.Clone(req.Tags),
		CreatedAt:   now,
	}

	m.saveLocked()

	return &CreateCustomDataIdentifierResponse{
		CustomDataIdentifierID: id,
	}, nil
}

// GetCustomDataIdentifier retrieves a custom data identifier by ID.
func (m *MemoryStorage) GetCustomDataIdentifier(_ context.Context, id string) (*GetCustomDataIdentifierResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cdi, exists := m.CustomDataIdentifiers[id]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Custom data identifier not found: " + id}
	}

	return &GetCustomDataIdentifierResponse{
		ID:          cdi.ID,
		Name:        cdi.Name,
		ARN:         cdi.ARN,
		Description: cdi.Description,
		Regex:       cdi.Regex,
		Keywords:    cdi.Keywords,
		Tags:        cdi.Tags,
		CreatedAt:   cdi.CreatedAt,
	}, nil
}

// DeleteCustomDataIdentifier deletes a custom data identifier.
func (m *MemoryStorage) DeleteCustomDataIdentifier(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.CustomDataIdentifiers[id]; !exists {
		return &Error{Code: errResourceNotFoundException, Message: "Custom data identifier not found: " + id}
	}

	delete(m.CustomDataIdentifiers, id)

	m.saveLocked()

	return nil
}

// ListCustomDataIdentifiers lists custom data identifiers.
func (m *MemoryStorage) ListCustomDataIdentifiers(_ context.Context, maxResults *int32, _ string) (*ListCustomDataIdentifiersResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := defaultMaxResults
	if maxResults != nil && *maxResults > 0 {
		limit = *maxResults
	}

	items := make([]CustomDataIdentifierSummary, 0, len(m.CustomDataIdentifiers))

	for _, cdi := range m.CustomDataIdentifiers {
		items = append(items, CustomDataIdentifierSummary{
			ID:          cdi.ID,
			Name:        cdi.Name,
			ARN:         cdi.ARN,
			Description: cdi.Description,
			CreatedAt:   cdi.CreatedAt,
		})

		//nolint:gosec // len(items) is bounded by the number of custom data identifiers.
		if int32(len(items)) >= limit {
			break
		}
	}

	return &ListCustomDataIdentifiersResponse{Items: items}, nil
}

// --- Findings filter operations ---

// CreateFindingsFilter creates a new findings filter.
func (m *MemoryStorage) CreateFindingsFilter(_ context.Context, req *CreateFindingsFilterRequest) (*CreateFindingsFilterResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	arn := fmt.Sprintf("arn:aws:macie2:%s:%s:findings-filter/%s", m.region, m.accountID, id)

	var position int32
	if req.Position != nil {
		position = *req.Position
	} else {
		//nolint:gosec // len(m.FindingsFilters) is bounded.
		position = int32(len(m.FindingsFilters)) + 1
	}

	criterion := make(map[string]CriterionValues, len(req.FindingCriteria.Criterion))
	for k, v := range req.FindingCriteria.Criterion {
		criterion[k] = CriterionValues{
			Eq:  slices.Clone(v.Eq),
			Neq: slices.Clone(v.Neq),
			Gt:  v.Gt,
			Gte: v.Gte,
			Lt:  v.Lt,
			Lte: v.Lte,
		}
	}

	m.FindingsFilters[id] = &FindingsFilter{
		ID:          id,
		Name:        req.Name,
		ARN:         arn,
		Description: req.Description,
		Action:      req.Action,
		FindingCriteria: FindingCriteria{
			Criterion: criterion,
		},
		Tags:     maps.Clone(req.Tags),
		Position: position,
	}

	m.saveLocked()

	return &CreateFindingsFilterResponse{
		ID:  id,
		ARN: arn,
	}, nil
}

// GetFindingsFilter retrieves a findings filter by ID.
func (m *MemoryStorage) GetFindingsFilter(_ context.Context, id string) (*GetFindingsFilterResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ff, exists := m.FindingsFilters[id]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Findings filter not found: " + id}
	}

	criterionOut := make(map[string]CriterionValuesOutput, len(ff.FindingCriteria.Criterion))
	for k, v := range ff.FindingCriteria.Criterion {
		criterionOut[k] = CriterionValuesOutput(v)
	}

	return &GetFindingsFilterResponse{
		ID:          ff.ID,
		Name:        ff.Name,
		ARN:         ff.ARN,
		Description: ff.Description,
		Action:      ff.Action,
		FindingCriteria: FindingCriteriaOutput{
			Criterion: criterionOut,
		},
		Tags:     ff.Tags,
		Position: ff.Position,
	}, nil
}

// UpdateFindingsFilter updates a findings filter.
func (m *MemoryStorage) UpdateFindingsFilter(_ context.Context, id string, req *UpdateFindingsFilterRequest) (*UpdateFindingsFilterResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ff, exists := m.FindingsFilters[id]
	if !exists {
		return nil, &Error{Code: errResourceNotFoundException, Message: "Findings filter not found: " + id}
	}

	if req.Name != "" {
		ff.Name = req.Name
	}

	if req.Description != "" {
		ff.Description = req.Description
	}

	if req.Action != "" {
		ff.Action = req.Action
	}

	if req.Position != nil {
		ff.Position = *req.Position
	}

	if req.FindingCriteria != nil {
		criterion := make(map[string]CriterionValues, len(req.FindingCriteria.Criterion))
		for k, v := range req.FindingCriteria.Criterion {
			criterion[k] = CriterionValues{
				Eq:  slices.Clone(v.Eq),
				Neq: slices.Clone(v.Neq),
				Gt:  v.Gt,
				Gte: v.Gte,
				Lt:  v.Lt,
				Lte: v.Lte,
			}
		}

		ff.FindingCriteria = FindingCriteria{Criterion: criterion}
	}

	m.saveLocked()

	return &UpdateFindingsFilterResponse{
		ID:  ff.ID,
		ARN: ff.ARN,
	}, nil
}

// DeleteFindingsFilter deletes a findings filter.
func (m *MemoryStorage) DeleteFindingsFilter(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.FindingsFilters[id]; !exists {
		return &Error{Code: errResourceNotFoundException, Message: "Findings filter not found: " + id}
	}

	delete(m.FindingsFilters, id)

	m.saveLocked()

	return nil
}

// ListFindingsFilters lists all findings filters.
func (m *MemoryStorage) ListFindingsFilters(_ context.Context) (*ListFindingsFiltersResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]FindingsFilterSummary, 0, len(m.FindingsFilters))

	for _, ff := range m.FindingsFilters {
		items = append(items, FindingsFilterSummary{
			ID:     ff.ID,
			Name:   ff.Name,
			ARN:    ff.ARN,
			Action: ff.Action,
			Tags:   ff.Tags,
		})
	}

	return &ListFindingsFiltersResponse{FindingsFilterListItems: items}, nil
}

// --- Findings operations ---

// GetFindings retrieves findings by IDs.
func (m *MemoryStorage) GetFindings(_ context.Context, req *GetFindingsRequest) (*GetFindingsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	findings := make([]FindingDetail, 0, len(req.FindingIDs))

	for _, id := range req.FindingIDs {
		f, exists := m.Findings[id]
		if !exists {
			continue
		}

		findings = append(findings, FindingDetail{
			ID:          f.ID,
			Type:        f.Type,
			Description: f.Description,
			Severity: FindingSeverityOutput{
				Score:       f.Severity.Score,
				Description: f.Severity.Description,
			},
			CreatedAt: f.CreatedAt,
			UpdatedAt: f.UpdatedAt,
		})
	}

	return &GetFindingsResponse{Findings: findings}, nil
}

// ListFindings lists finding IDs.
func (m *MemoryStorage) ListFindings(_ context.Context, req *ListFindingsRequest) (*ListFindingsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := defaultMaxResults
	if req.MaxResults != nil && *req.MaxResults > 0 {
		limit = *req.MaxResults
	}

	ids := make([]string, 0, len(m.Findings))

	for id := range m.Findings {
		ids = append(ids, id)

		//nolint:gosec // len(ids) is bounded by the number of findings.
		if int32(len(ids)) >= limit {
			break
		}
	}

	return &ListFindingsResponse{FindingIDs: ids}, nil
}

// Helper functions.

func defaultString(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}

	return value
}
