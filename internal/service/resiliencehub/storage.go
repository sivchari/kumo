package resiliencehub

import (
	"encoding/json"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	errResourceNotFound = "ResourceNotFoundException"
	errConflict         = "ConflictException"
)

// Storage defines the interface for Resilience Hub data storage.
type Storage interface {
	// App operations
	CreateApp(req *CreateAppRequest) (*App, error)
	DescribeApp(appARN string) (*App, error)
	UpdateApp(req *UpdateAppRequest) (*App, error)
	DeleteApp(appARN string) error
	ListApps(req *ListAppsRequest) ([]*AppSummary, string, error)

	// ResiliencyPolicy operations
	CreateResiliencyPolicy(req *CreateResiliencyPolicyRequest) (*ResiliencyPolicy, error)
	DescribeResiliencyPolicy(policyARN string) (*ResiliencyPolicy, error)
	UpdateResiliencyPolicy(req *UpdateResiliencyPolicyRequest) (*ResiliencyPolicy, error)
	DeleteResiliencyPolicy(policyARN string) error
	ListResiliencyPolicies(req *ListResiliencyPoliciesRequest) ([]*ResiliencyPolicy, string, error)

	// Assessment operations
	StartAppAssessment(req *StartAppAssessmentRequest) (*AppAssessment, error)
	DescribeAppAssessment(assessmentARN string) (*AppAssessment, error)
	DeleteAppAssessment(assessmentARN string) error
	ListAppAssessments(req *ListAppAssessmentsRequest) ([]*AppAssessmentSummary, string, error)

	// Tag operations
	TagResource(resourceARN string, tags map[string]string) error
	UntagResource(resourceARN string, tagKeys []string) error
	ListTagsForResource(resourceARN string) (map[string]string, error)
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

// MemoryStorage provides an in-memory implementation of Storage.
type MemoryStorage struct {
	mu          sync.RWMutex                 `json:"-"`
	Apps        map[string]*App              `json:"apps"`
	Policies    map[string]*ResiliencyPolicy `json:"policies"`
	Assessments map[string]*AppAssessment    `json:"assessments"`
	Tags        map[string]map[string]string `json:"tags"`
	dataDir     string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Apps:        make(map[string]*App),
		Policies:    make(map[string]*ResiliencyPolicy),
		Assessments: make(map[string]*AppAssessment),
		Tags:        make(map[string]map[string]string),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "resiliencehub", s)
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

	if s.Apps == nil {
		s.Apps = make(map[string]*App)
	}

	if s.Policies == nil {
		s.Policies = make(map[string]*ResiliencyPolicy)
	}

	if s.Assessments == nil {
		s.Assessments = make(map[string]*AppAssessment)
	}

	if s.Tags == nil {
		s.Tags = make(map[string]map[string]string)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	type alias MemoryStorage

	data, err := json.Marshal(&struct{ *alias }{alias: (*alias)(s)})
	if err != nil {
		return
	}

	_ = storage.SaveBytes(s.dataDir, "resiliencehub", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "resiliencehub", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateApp creates a new application.
func (s *MemoryStorage) CreateApp(req *CreateAppRequest) (*App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate name
	for _, app := range s.Apps {
		if app.Name == req.Name {
			return nil, &Error{
				Code:    errConflict,
				Message: fmt.Sprintf("App with name %s already exists", req.Name),
			}
		}
	}

	appID := uuid.New().String()
	appARN := fmt.Sprintf("arn:aws:resiliencehub:us-east-1:123456789012:app/%s", appID)
	now := float64(time.Now().Unix())

	app := &App{
		AppARN:             appARN,
		AssessmentSchedule: req.AssessmentSchedule,
		ComplianceStatus:   "NotAssessed",
		CreationTime:       now,
		Description:        req.Description,
		DriftStatus:        "NotChecked",
		EventSubscriptions: req.EventSubscriptions,
		Name:               req.Name,
		PermissionModel:    req.PermissionModel,
		PolicyARN:          req.PolicyARN,
		Status:             "Active",
		Tags:               req.Tags,
	}

	s.Apps[appARN] = app

	if req.Tags != nil {
		s.Tags[appARN] = req.Tags
	}

	s.saveLocked()

	return app, nil
}

// DescribeApp retrieves an application by ARN.
func (s *MemoryStorage) DescribeApp(appARN string) (*App, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, ok := s.Apps[appARN]
	if !ok {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("App not found: %s", appARN),
		}
	}

	return app, nil
}

// UpdateApp updates an existing application.
func (s *MemoryStorage) UpdateApp(req *UpdateAppRequest) (*App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.Apps[req.AppARN]
	if !ok {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("App not found: %s", req.AppARN),
		}
	}

	if req.AssessmentSchedule != "" {
		app.AssessmentSchedule = req.AssessmentSchedule
	}

	if req.Description != "" {
		app.Description = req.Description
	}

	if req.EventSubscriptions != nil {
		app.EventSubscriptions = req.EventSubscriptions
	}

	if req.PermissionModel != nil {
		app.PermissionModel = req.PermissionModel
	}

	if req.PolicyARN != "" {
		app.PolicyARN = req.PolicyARN
	}

	if req.ClearResiliencyPolicyARN {
		app.PolicyARN = ""
	}

	s.saveLocked()

	return app, nil
}

// DeleteApp deletes an application.
func (s *MemoryStorage) DeleteApp(appARN string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.Apps[appARN]; !ok {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("App not found: %s", appARN),
		}
	}

	delete(s.Apps, appARN)
	delete(s.Tags, appARN)

	s.saveLocked()

	return nil
}

// ListApps lists all applications.
func (s *MemoryStorage) ListApps(req *ListAppsRequest) ([]*AppSummary, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summaries := make([]*AppSummary, 0, len(s.Apps))

	for _, app := range s.Apps {
		// Apply filters
		if req.Name != "" && app.Name != req.Name {
			continue
		}

		if req.AppARN != "" && app.AppARN != req.AppARN {
			continue
		}

		summary := &AppSummary{
			AppARN:                    app.AppARN,
			AssessmentSchedule:        app.AssessmentSchedule,
			ComplianceStatus:          app.ComplianceStatus,
			CreationTime:              app.CreationTime,
			Description:               app.Description,
			DriftStatus:               app.DriftStatus,
			LastAppComplianceEvalTime: app.LastAppComplianceEvalTime,
			Name:                      app.Name,
			ResiliencyScore:           app.ResiliencyScore,
			RpoInSecs:                 app.RpoInSecs,
			RtoInSecs:                 app.RtoInSecs,
			Status:                    app.Status,
		}
		summaries = append(summaries, summary)
	}

	return summaries, "", nil
}

// CreateResiliencyPolicy creates a new resiliency policy.
func (s *MemoryStorage) CreateResiliencyPolicy(req *CreateResiliencyPolicyRequest) (*ResiliencyPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate name
	for _, policy := range s.Policies {
		if policy.PolicyName == req.PolicyName {
			return nil, &Error{
				Code:    errConflict,
				Message: fmt.Sprintf("Policy with name %s already exists", req.PolicyName),
			}
		}
	}

	policyID := uuid.New().String()
	policyARN := fmt.Sprintf("arn:aws:resiliencehub:us-east-1:123456789012:resiliency-policy/%s", policyID)
	now := float64(time.Now().Unix())

	policy := &ResiliencyPolicy{
		CreationTime:           now,
		DataLocationConstraint: req.DataLocationConstraint,
		Policy:                 req.Policy,
		PolicyARN:              policyARN,
		PolicyDescription:      req.PolicyDescription,
		PolicyName:             req.PolicyName,
		Tags:                   req.Tags,
		Tier:                   req.Tier,
	}

	s.Policies[policyARN] = policy

	if req.Tags != nil {
		s.Tags[policyARN] = req.Tags
	}

	s.saveLocked()

	return policy, nil
}

// DescribeResiliencyPolicy retrieves a policy by ARN.
func (s *MemoryStorage) DescribeResiliencyPolicy(policyARN string) (*ResiliencyPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policy, ok := s.Policies[policyARN]
	if !ok {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Policy not found: %s", policyARN),
		}
	}

	return policy, nil
}

// UpdateResiliencyPolicy updates an existing policy.
func (s *MemoryStorage) UpdateResiliencyPolicy(req *UpdateResiliencyPolicyRequest) (*ResiliencyPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	policy, ok := s.Policies[req.PolicyARN]
	if !ok {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Policy not found: %s", req.PolicyARN),
		}
	}

	if req.DataLocationConstraint != "" {
		policy.DataLocationConstraint = req.DataLocationConstraint
	}

	if req.Policy != nil {
		policy.Policy = req.Policy
	}

	if req.PolicyDescription != "" {
		policy.PolicyDescription = req.PolicyDescription
	}

	if req.PolicyName != "" {
		policy.PolicyName = req.PolicyName
	}

	if req.Tier != "" {
		policy.Tier = req.Tier
	}

	s.saveLocked()

	return policy, nil
}

// DeleteResiliencyPolicy deletes a policy.
func (s *MemoryStorage) DeleteResiliencyPolicy(policyARN string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.Policies[policyARN]; !ok {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Policy not found: %s", policyARN),
		}
	}

	delete(s.Policies, policyARN)
	delete(s.Tags, policyARN)

	s.saveLocked()

	return nil
}

// ListResiliencyPolicies lists all policies.
func (s *MemoryStorage) ListResiliencyPolicies(req *ListResiliencyPoliciesRequest) ([]*ResiliencyPolicy, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policies := make([]*ResiliencyPolicy, 0, len(s.Policies))

	for _, policy := range s.Policies {
		// Apply filters
		if req.PolicyName != "" && policy.PolicyName != req.PolicyName {
			continue
		}

		policies = append(policies, policy)
	}

	return policies, "", nil
}

// StartAppAssessment starts a new assessment.
func (s *MemoryStorage) StartAppAssessment(req *StartAppAssessmentRequest) (*AppAssessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.Apps[req.AppARN]
	if !ok {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("App not found: %s", req.AppARN),
		}
	}

	assessmentARN := fmt.Sprintf("arn:aws:resiliencehub:us-east-1:123456789012:app-assessment/%s", uuid.New().String())
	now := float64(time.Now().Unix())

	var policy *ResiliencyPolicy
	if app.PolicyARN != "" {
		policy = s.Policies[app.PolicyARN]
	}

	assessment := &AppAssessment{
		AppARN: req.AppARN, AppVersion: req.AppVersion,
		AssessmentARN: assessmentARN, AssessmentName: req.AssessmentName,
		AssessmentStatus: "Success", ComplianceStatus: "PolicyBreached",
		Compliance:      defaultCompliance(),
		EndTime:         now,
		Invoker:         "User",
		Policy:          policy,
		StartTime:       now,
		Tags:            req.Tags,
		ResiliencyScore: defaultResiliencyScore(),
	}

	s.Assessments[assessmentARN] = assessment

	if req.Tags != nil {
		s.Tags[assessmentARN] = req.Tags
	}

	s.saveLocked()

	return assessment, nil
}

// defaultCompliance returns default compliance data for assessments.
func defaultCompliance() map[string]*DisruptionCompliance {
	return map[string]*DisruptionCompliance{
		"Software": {ComplianceStatus: "PolicyBreached", CurrentRpoInSecs: 86400, CurrentRtoInSecs: 86400, AchievableRpoInSecs: 3600, AchievableRtoInSecs: 3600},
		"Hardware": {ComplianceStatus: "PolicyMet", CurrentRpoInSecs: 3600, CurrentRtoInSecs: 3600, AchievableRpoInSecs: 3600, AchievableRtoInSecs: 3600},
	}
}

// defaultResiliencyScore returns default resiliency score for assessments.
func defaultResiliencyScore() *ResiliencyScore {
	return &ResiliencyScore{Score: 75.0, DisruptionScore: map[string]float64{"Software": 50.0, "Hardware": 100.0}}
}

// DescribeAppAssessment retrieves an assessment by ARN.
func (s *MemoryStorage) DescribeAppAssessment(assessmentARN string) (*AppAssessment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	assessment, ok := s.Assessments[assessmentARN]
	if !ok {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Assessment not found: %s", assessmentARN),
		}
	}

	return assessment, nil
}

// DeleteAppAssessment deletes an assessment.
func (s *MemoryStorage) DeleteAppAssessment(assessmentARN string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.Assessments[assessmentARN]; !ok {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Assessment not found: %s", assessmentARN),
		}
	}

	delete(s.Assessments, assessmentARN)
	delete(s.Tags, assessmentARN)

	s.saveLocked()

	return nil
}

// ListAppAssessments lists all assessments.
func (s *MemoryStorage) ListAppAssessments(req *ListAppAssessmentsRequest) ([]*AppAssessmentSummary, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summaries := make([]*AppAssessmentSummary, 0, len(s.Assessments))

	for _, assessment := range s.Assessments {
		// Apply filters
		if req.AppARN != "" && assessment.AppARN != req.AppARN {
			continue
		}

		if req.AssessmentName != "" && assessment.AssessmentName != req.AssessmentName {
			continue
		}

		if req.AssessmentStatus != "" && assessment.AssessmentStatus != req.AssessmentStatus {
			continue
		}

		if req.ComplianceStatus != "" && assessment.ComplianceStatus != req.ComplianceStatus {
			continue
		}

		if req.Invoker != "" && assessment.Invoker != req.Invoker {
			continue
		}

		var score float64
		if assessment.ResiliencyScore != nil {
			score = assessment.ResiliencyScore.Score
		}

		summary := &AppAssessmentSummary{
			AppARN:           assessment.AppARN,
			AppVersion:       assessment.AppVersion,
			AssessmentARN:    assessment.AssessmentARN,
			AssessmentName:   assessment.AssessmentName,
			AssessmentStatus: assessment.AssessmentStatus,
			ComplianceStatus: assessment.ComplianceStatus,
			Cost:             assessment.Cost,
			DriftStatus:      assessment.DriftStatus,
			EndTime:          assessment.EndTime,
			Invoker:          assessment.Invoker,
			Message:          assessment.Message,
			ResiliencyScore:  score,
			StartTime:        assessment.StartTime,
			VersionName:      assessment.VersionName,
		}
		summaries = append(summaries, summary)
	}

	return summaries, "", nil
}

// TagResource adds tags to a resource.
func (s *MemoryStorage) TagResource(resourceARN string, tags map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if resource exists
	_, appOK := s.Apps[resourceARN]
	_, policyOK := s.Policies[resourceARN]
	_, assessmentOK := s.Assessments[resourceARN]

	if !appOK && !policyOK && !assessmentOK {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Resource not found: %s", resourceARN),
		}
	}

	if s.Tags[resourceARN] == nil {
		s.Tags[resourceARN] = make(map[string]string)
	}

	maps.Copy(s.Tags[resourceARN], tags)

	s.saveLocked()

	return nil
}

// UntagResource removes tags from a resource.
func (s *MemoryStorage) UntagResource(resourceARN string, tagKeys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if resource exists
	_, appOK := s.Apps[resourceARN]
	_, policyOK := s.Policies[resourceARN]
	_, assessmentOK := s.Assessments[resourceARN]

	if !appOK && !policyOK && !assessmentOK {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Resource not found: %s", resourceARN),
		}
	}

	if s.Tags[resourceARN] != nil {
		for _, key := range tagKeys {
			delete(s.Tags[resourceARN], key)
		}
	}

	s.saveLocked()

	return nil
}

// ListTagsForResource retrieves tags for a resource.
func (s *MemoryStorage) ListTagsForResource(resourceARN string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if resource exists
	_, appOK := s.Apps[resourceARN]
	_, policyOK := s.Policies[resourceARN]
	_, assessmentOK := s.Assessments[resourceARN]

	if !appOK && !policyOK && !assessmentOK {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Resource not found: %s", resourceARN),
		}
	}

	tags := s.Tags[resourceARN]
	if tags == nil {
		return make(map[string]string), nil
	}

	return tags, nil
}
