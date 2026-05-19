package codegurureviewer

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

func epochNow() float64 {
	return float64(time.Now().Unix())
}

// Storage defines the interface for CodeGuru Reviewer storage operations.
type Storage interface {
	AssociateRepository(input *AssociateRepositoryInput) *RepositoryAssociation
	DescribeRepositoryAssociation(arn string) (*RepositoryAssociation, error)
	DisassociateRepository(arn string) (*RepositoryAssociation, error)
	ListRepositoryAssociations() []RepositoryAssociationSummary

	CreateCodeReview(input *CreateCodeReviewInput) (*CodeReview, error)
	DescribeCodeReview(arn string) (*CodeReview, error)
	ListCodeReviews() []CodeReview

	ListRecommendations(codeReviewArn string) []RecommendationSummary

	PutRecommendationFeedback(input *PutRecommendationFeedbackInput) error
	DescribeRecommendationFeedback(codeReviewArn, recommendationID string) (*RecommendationFeedback, error)
	ListRecommendationFeedback(codeReviewArn string) []RecommendationFeedback
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

// MemoryStorage is an in-memory implementation of Storage.
type MemoryStorage struct {
	mu           sync.RWMutex                                  `json:"-"`
	Associations map[string]*RepositoryAssociation             `json:"associations"`
	CodeReviews  map[string]*CodeReview                        `json:"codeReviews"`
	Feedback     map[string]map[string]*RecommendationFeedback `json:"feedback"` // codeReviewArn -> recommendationID -> feedback
	dataDir      string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Associations: make(map[string]*RepositoryAssociation),
		CodeReviews:  make(map[string]*CodeReview),
		Feedback:     make(map[string]map[string]*RecommendationFeedback),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "codeguru-reviewer", s)
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

	if m.Associations == nil {
		m.Associations = make(map[string]*RepositoryAssociation)
	}

	if m.CodeReviews == nil {
		m.CodeReviews = make(map[string]*CodeReview)
	}

	if m.Feedback == nil {
		m.Feedback = make(map[string]map[string]*RecommendationFeedback)
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

	_ = storage.SaveBytes(m.dataDir, "codeguru-reviewer", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "codeguru-reviewer", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// AssociateRepository creates a new repository association.
func (m *MemoryStorage) AssociateRepository(input *AssociateRepositoryInput) *RepositoryAssociation {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	now := epochNow()
	arn := fmt.Sprintf("arn:aws:codeguru-reviewer:us-east-1:000000000000:association:%s", id)

	name, owner, providerType, connectionArn := extractRepoDetails(input.Repository)

	assoc := &RepositoryAssociation{
		AssociationArn:       arn,
		AssociationID:        id,
		ConnectionArn:        connectionArn,
		CreatedTimeStamp:     now,
		LastUpdatedTimeStamp: now,
		Name:                 name,
		Owner:                owner,
		ProviderType:         providerType,
		State:                "Associated",
		Tags:                 input.Tags,
	}

	m.Associations[arn] = assoc

	m.saveLocked()

	return assoc
}

func extractRepoDetails(repo *RepositoryInput) (name, owner, providerType, connectionArn string) {
	if repo == nil {
		return "", "", "", ""
	}

	if repo.CodeCommit != nil {
		return repo.CodeCommit.Name, "", "CodeCommit", ""
	}

	if repo.Bitbucket != nil {
		return repo.Bitbucket.Name, repo.Bitbucket.Owner, "Bitbucket", repo.Bitbucket.ConnectionArn
	}

	if repo.GitHubEnterpriseServer != nil {
		return repo.GitHubEnterpriseServer.Name, repo.GitHubEnterpriseServer.Owner, "GitHubEnterpriseServer", repo.GitHubEnterpriseServer.ConnectionArn
	}

	if repo.S3Bucket != nil {
		return repo.S3Bucket.Name, "", "S3Bucket", ""
	}

	return "", "", "", ""
}

// DescribeRepositoryAssociation returns a repository association by ARN.
func (m *MemoryStorage) DescribeRepositoryAssociation(arn string) (*RepositoryAssociation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assoc, ok := m.Associations[arn]
	if !ok {
		return nil, fmt.Errorf("NotFoundException: repository association %s not found", arn)
	}

	return assoc, nil
}

// DisassociateRepository removes a repository association.
func (m *MemoryStorage) DisassociateRepository(arn string) (*RepositoryAssociation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	assoc, ok := m.Associations[arn]
	if !ok {
		return nil, fmt.Errorf("NotFoundException: repository association %s not found", arn)
	}

	assoc.State = "Disassociated"
	assoc.LastUpdatedTimeStamp = epochNow()

	delete(m.Associations, arn)

	m.saveLocked()

	return assoc, nil
}

// ListRepositoryAssociations returns all repository associations.
func (m *MemoryStorage) ListRepositoryAssociations() []RepositoryAssociationSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]RepositoryAssociationSummary, 0, len(m.Associations))
	for _, assoc := range m.Associations {
		result = append(result, RepositoryAssociationSummary{
			AssociationArn:       assoc.AssociationArn,
			AssociationID:        assoc.AssociationID,
			ConnectionArn:        assoc.ConnectionArn,
			LastUpdatedTimeStamp: assoc.LastUpdatedTimeStamp,
			Name:                 assoc.Name,
			Owner:                assoc.Owner,
			ProviderType:         assoc.ProviderType,
			State:                assoc.State,
		})
	}

	return result
}

// CreateCodeReview creates a new code review.
func (m *MemoryStorage) CreateCodeReview(input *CreateCodeReviewInput) (*CodeReview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the association.
	assoc, ok := m.Associations[input.RepositoryAssociationArn]
	if !ok {
		return nil, fmt.Errorf("NotFoundException: repository association %s not found", input.RepositoryAssociationArn)
	}

	id := uuid.New().String()
	now := epochNow()
	arn := fmt.Sprintf("arn:aws:codeguru-reviewer:us-east-1:000000000000:association:%s:codereview:%s", assoc.AssociationID, id)

	review := &CodeReview{
		AssociationArn:       assoc.AssociationArn,
		CodeReviewArn:        arn,
		CreatedTimeStamp:     now,
		LastUpdatedTimeStamp: now,
		Name:                 input.Name,
		Owner:                assoc.Owner,
		ProviderType:         assoc.ProviderType,
		RepositoryName:       assoc.Name,
		State:                "Completed",
		Type:                 "RepositoryAnalysis",
	}

	m.CodeReviews[arn] = review

	m.saveLocked()

	return review, nil
}

// DescribeCodeReview returns a code review by ARN.
func (m *MemoryStorage) DescribeCodeReview(arn string) (*CodeReview, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	review, ok := m.CodeReviews[arn]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: code review %s not found", arn)
	}

	return review, nil
}

// ListCodeReviews returns all code reviews.
func (m *MemoryStorage) ListCodeReviews() []CodeReview {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]CodeReview, 0, len(m.CodeReviews))
	for _, review := range m.CodeReviews {
		result = append(result, *review)
	}

	return result
}

// ListRecommendations returns recommendations for a code review.
func (m *MemoryStorage) ListRecommendations(_ string) []RecommendationSummary {
	return []RecommendationSummary{}
}

// PutRecommendationFeedback stores feedback for a recommendation.
func (m *MemoryStorage) PutRecommendationFeedback(input *PutRecommendationFeedbackInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.CodeReviews[input.CodeReviewArn]; !ok {
		return fmt.Errorf("ResourceNotFoundException: code review %s not found", input.CodeReviewArn)
	}

	now := epochNow()

	if m.Feedback[input.CodeReviewArn] == nil {
		m.Feedback[input.CodeReviewArn] = make(map[string]*RecommendationFeedback)
	}

	fb := &RecommendationFeedback{
		CodeReviewArn:        input.CodeReviewArn,
		CreatedTimeStamp:     now,
		LastUpdatedTimeStamp: now,
		Reactions:            input.Reactions,
		RecommendationID:     input.RecommendationID,
		UserID:               "test-user",
	}

	m.Feedback[input.CodeReviewArn][input.RecommendationID] = fb

	m.saveLocked()

	return nil
}

// DescribeRecommendationFeedback returns feedback for a recommendation.
func (m *MemoryStorage) DescribeRecommendationFeedback(codeReviewArn, recommendationID string) (*RecommendationFeedback, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fbMap, ok := m.Feedback[codeReviewArn]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: feedback not found")
	}

	fb, ok := fbMap[recommendationID]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: feedback not found")
	}

	return fb, nil
}

// ListRecommendationFeedback returns all feedback for a code review.
func (m *MemoryStorage) ListRecommendationFeedback(codeReviewArn string) []RecommendationFeedback {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fbMap := m.Feedback[codeReviewArn]
	result := make([]RecommendationFeedback, 0, len(fbMap))

	for _, fb := range fbMap {
		result = append(result, *fb)
	}

	return result
}
