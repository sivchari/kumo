package ecr

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/storage"
)

// Default values.
const defaultRegion = "us-east-1"

// Error codes.
const (
	errRepositoryNotFound      = "RepositoryNotFoundException"
	errRepositoryAlreadyExists = "RepositoryAlreadyExistsException"
	errImageNotFound           = "ImageNotFoundException"
	errInvalidParameter        = "InvalidParameterException"
)

// Storage defines the ECR storage interface.
type Storage interface {
	CreateRepository(ctx context.Context, req *CreateRepositoryRequest) (*Repository, error)
	DeleteRepository(ctx context.Context, repositoryName string, force bool) (*Repository, error)
	DescribeRepositories(ctx context.Context, names []string, maxResults int32, nextToken string) ([]*Repository, string, error)
	ListImages(ctx context.Context, repositoryName string, maxResults int32, nextToken string) ([]*ImageIdentifier, string, error)
	PutImage(ctx context.Context, repositoryName, imageManifest, imageTag string) (*Image, error)
	BatchGetImage(ctx context.Context, repositoryName string, imageIDs []ImageIdentifier) ([]*Image, []ImageFailure, error)
	BatchDeleteImage(ctx context.Context, repositoryName string, imageIDs []ImageIdentifier) ([]ImageIdentifier, []ImageFailure, error)
	GetAuthorizationToken(ctx context.Context) ([]AuthorizationData, error)
	DispatchAction(action string) bool

	PutLifecyclePolicy(ctx context.Context, repositoryName, policyText string) (string, error)
	GetLifecyclePolicy(ctx context.Context, repositoryName string) (string, time.Time, error)
	DeleteLifecyclePolicy(ctx context.Context, repositoryName string) (string, time.Time, error)
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
	mu           sync.RWMutex               `json:"-"`
	Repositories map[string]*repositoryData `json:"repositories"`
	region       string
	accountID    string
	dataDir      string
}

// repositoryData holds repository information and its images.
type repositoryData struct {
	Repository          *Repository       `json:"repository"`
	Images              map[string]*Image `json:"images"`
	LifecyclePolicyText string            `json:"lifecyclePolicyText,omitempty"`
	LifecyclePolicyAt   time.Time         `json:"lifecyclePolicyAt,omitempty"`
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Repositories: make(map[string]*repositoryData),
		region:       region,
		accountID:    "000000000000",
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "ecr", s)
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

	if s.Repositories == nil {
		s.Repositories = make(map[string]*repositoryData)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "ecr", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "ecr", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateRepository creates a new repository.
func (s *MemoryStorage) CreateRepository(_ context.Context, req *CreateRepositoryRequest) (*Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Repositories[req.RepositoryName]; exists {
		return nil, &ServiceError{Code: errRepositoryAlreadyExists, Message: "Repository already exists"}
	}

	tagMutability := req.ImageTagMutability
	if tagMutability == "" {
		tagMutability = "MUTABLE"
	}

	now := time.Now()
	repo := &Repository{
		RepositoryArn:              fmt.Sprintf("arn:aws:ecr:%s:%s:repository/%s", s.region, s.accountID, req.RepositoryName),
		RegistryID:                 s.accountID,
		RepositoryName:             req.RepositoryName,
		RepositoryURI:              fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s", s.accountID, s.region, req.RepositoryName),
		CreatedAt:                  now,
		ImageTagMutability:         tagMutability,
		ImageScanningConfiguration: req.ImageScanningConfiguration,
		EncryptionConfiguration:    req.EncryptionConfiguration,
	}

	s.Repositories[req.RepositoryName] = &repositoryData{
		Repository: repo,
		Images:     make(map[string]*Image),
	}

	s.saveLocked()

	return repo, nil
}

// DeleteRepository deletes a repository.
func (s *MemoryStorage) DeleteRepository(_ context.Context, repositoryName string, force bool) (*Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rd, exists := s.Repositories[repositoryName]
	if !exists {
		return nil, &ServiceError{Code: errRepositoryNotFound, Message: "Repository does not exist"}
	}

	if !force && len(rd.Images) > 0 {
		return nil, &ServiceError{Code: errInvalidParameter, Message: "Repository contains images"}
	}

	delete(s.Repositories, repositoryName)

	s.saveLocked()

	return rd.Repository, nil
}

// DescribeRepositories describes repositories.
func (s *MemoryStorage) DescribeRepositories(_ context.Context, names []string, maxResults int32, _ string) ([]*Repository, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults <= 0 {
		maxResults = 100
	}

	var repos []*Repository

	if len(names) > 0 {
		for _, name := range names {
			if rd, exists := s.Repositories[name]; exists {
				repos = append(repos, rd.Repository)
			}
		}
	} else {
		for _, rd := range s.Repositories {
			repos = append(repos, rd.Repository)
		}
	}

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].RepositoryName < repos[j].RepositoryName
	})

	if int32(len(repos)) > maxResults { //nolint:gosec // slice length bounded by maxResults parameter
		repos = repos[:maxResults]
	}

	return repos, "", nil
}

// ListImages lists images in a repository.
func (s *MemoryStorage) ListImages(_ context.Context, repositoryName string, maxResults int32, nextToken string) ([]*ImageIdentifier, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rd, exists := s.Repositories[repositoryName]
	if !exists {
		return nil, "", &ServiceError{Code: errRepositoryNotFound, Message: "Repository does not exist"}
	}

	if maxResults <= 0 {
		maxResults = 100
	}

	// Collect all images into a slice for sorting
	images := make([]*Image, 0, len(rd.Images))
	for _, img := range rd.Images {
		images = append(images, img)
	}

	// Sort by push date (descending - newest first)
	sort.Slice(images, func(i, j int) bool {
		return images[i].PushedAt.After(images[j].PushedAt)
	})

	// Find the starting index based on nextToken (which is the image digest)
	startIdx := 0

	if nextToken != "" {
		for i, img := range images {
			if img.ImageDigest == nextToken {
				startIdx = i + 1

				break
			}
		}
	}

	// Build the result slice with pagination
	var imageIDs []*ImageIdentifier

	var newNextToken string

	for i := startIdx; i < len(images); i++ {
		if int32(len(imageIDs)) >= maxResults { //nolint:gosec // slice length bounded by maxResults parameter
			newNextToken = images[i-1].ImageDigest

			break
		}

		img := images[i]
		imageIDs = append(imageIDs, &ImageIdentifier{
			ImageDigest: img.ImageDigest,
			ImageTag:    img.ImageID.ImageTag,
		})
	}

	return imageIDs, newNextToken, nil
}

// PutImage puts an image into a repository.
func (s *MemoryStorage) PutImage(_ context.Context, repositoryName, imageManifest, imageTag string) (*Image, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rd, exists := s.Repositories[repositoryName]
	if !exists {
		return nil, &ServiceError{Code: errRepositoryNotFound, Message: "Repository does not exist"}
	}

	digest := calculateDigest(imageManifest)

	img := &Image{
		RegistryID:     s.accountID,
		RepositoryName: repositoryName,
		ImageManifest:  imageManifest,
		ImageDigest:    digest,
		ImageID: &ImageIdentifier{
			ImageDigest: digest,
			ImageTag:    imageTag,
		},
		PushedAt: time.Now(),
	}

	rd.Images[digest] = img

	s.saveLocked()

	return img, nil
}

// BatchGetImage gets multiple images.
func (s *MemoryStorage) BatchGetImage(_ context.Context, repositoryName string, imageIDs []ImageIdentifier) ([]*Image, []ImageFailure, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rd, exists := s.Repositories[repositoryName]
	if !exists {
		return nil, nil, &ServiceError{Code: errRepositoryNotFound, Message: "Repository does not exist"}
	}

	var images []*Image

	var failures []ImageFailure

	for _, id := range imageIDs {
		found := false

		for _, img := range rd.Images {
			if (id.ImageDigest != "" && img.ImageDigest == id.ImageDigest) ||
				(id.ImageTag != "" && img.ImageID.ImageTag == id.ImageTag) {
				images = append(images, img)
				found = true

				break
			}
		}

		if !found {
			failures = append(failures, ImageFailure{
				ImageID:       &id,
				FailureCode:   "ImageNotFound",
				FailureReason: "Image not found",
			})
		}
	}

	return images, failures, nil
}

// BatchDeleteImage deletes multiple images.
func (s *MemoryStorage) BatchDeleteImage(_ context.Context, repositoryName string, imageIDs []ImageIdentifier) ([]ImageIdentifier, []ImageFailure, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rd, exists := s.Repositories[repositoryName]
	if !exists {
		return nil, nil, &ServiceError{Code: errRepositoryNotFound, Message: "Repository does not exist"}
	}

	var deleted []ImageIdentifier

	var failures []ImageFailure

	for _, id := range imageIDs {
		found := false

		for digest, img := range rd.Images {
			if (id.ImageDigest != "" && img.ImageDigest == id.ImageDigest) ||
				(id.ImageTag != "" && img.ImageID.ImageTag == id.ImageTag) {
				delete(rd.Images, digest)

				deleted = append(deleted, ImageIdentifier{
					ImageDigest: img.ImageDigest,
					ImageTag:    img.ImageID.ImageTag,
				})
				found = true

				break
			}
		}

		if !found {
			failures = append(failures, ImageFailure{
				ImageID:       &id,
				FailureCode:   "ImageNotFound",
				FailureReason: "Image not found",
			})
		}
	}

	s.saveLocked()

	return deleted, failures, nil
}

// GetAuthorizationToken returns authorization tokens.
func (s *MemoryStorage) GetAuthorizationToken(_ context.Context) ([]AuthorizationData, error) {
	token := base64.StdEncoding.EncodeToString([]byte("AWS:password"))
	expiresAt := time.Now().Add(12 * time.Hour)

	return []AuthorizationData{
		{
			AuthorizationToken: token,
			ExpiresAt:          float64(expiresAt.Unix()),
			ProxyEndpoint:      fmt.Sprintf("https://%s.dkr.ecr.%s.amazonaws.com", s.accountID, s.region),
		},
	}, nil
}

// DispatchAction checks if the action is valid.
func (s *MemoryStorage) DispatchAction(_ string) bool {
	return true
}

// calculateDigest calculates SHA256 digest of the manifest.
func calculateDigest(manifest string) string {
	hash := sha256.Sum256([]byte(manifest))

	return fmt.Sprintf("sha256:%x", hash)
}

// PutLifecyclePolicy stores the lifecycle policy text on the named repository.
// AWS validates the policy JSON shape; kumo stores whatever the caller sent so
// tests can verify policy round-trips without modeling the lifecycle engine.
func (s *MemoryStorage) PutLifecyclePolicy(_ context.Context, repositoryName, policyText string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	repo, ok := s.Repositories[repositoryName]
	if !ok {
		return "", &ServiceError{
			Code:    "RepositoryNotFoundException",
			Message: fmt.Sprintf("The repository with name '%s' does not exist in the registry with id '%s'", repositoryName, s.accountID),
		}
	}

	repo.LifecyclePolicyText = policyText
	repo.LifecyclePolicyAt = time.Now().UTC()

	s.saveLocked()

	return policyText, nil
}

// GetLifecyclePolicy returns the stored policy text and the time it was set.
// AWS returns LifecyclePolicyNotFoundException when the repository has no
// policy; kumo mirrors that.
func (s *MemoryStorage) GetLifecyclePolicy(_ context.Context, repositoryName string) (string, time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	repo, ok := s.Repositories[repositoryName]
	if !ok {
		return "", time.Time{}, &ServiceError{
			Code:    "RepositoryNotFoundException",
			Message: fmt.Sprintf("The repository with name '%s' does not exist in the registry with id '%s'", repositoryName, s.accountID),
		}
	}

	if repo.LifecyclePolicyText == "" {
		return "", time.Time{}, &ServiceError{
			Code:    "LifecyclePolicyNotFoundException",
			Message: fmt.Sprintf("Lifecycle policy does not exist for the repository with name '%s' in the registry with id '%s'", repositoryName, s.accountID),
		}
	}

	return repo.LifecyclePolicyText, repo.LifecyclePolicyAt, nil
}

// DeleteLifecyclePolicy clears the policy text and returns what was removed.
func (s *MemoryStorage) DeleteLifecyclePolicy(_ context.Context, repositoryName string) (string, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	repo, ok := s.Repositories[repositoryName]
	if !ok {
		return "", time.Time{}, &ServiceError{
			Code:    "RepositoryNotFoundException",
			Message: fmt.Sprintf("The repository with name '%s' does not exist in the registry with id '%s'", repositoryName, s.accountID),
		}
	}

	if repo.LifecyclePolicyText == "" {
		return "", time.Time{}, &ServiceError{
			Code:    "LifecyclePolicyNotFoundException",
			Message: fmt.Sprintf("Lifecycle policy does not exist for the repository with name '%s' in the registry with id '%s'", repositoryName, s.accountID),
		}
	}

	prev := repo.LifecyclePolicyText
	at := repo.LifecyclePolicyAt
	repo.LifecyclePolicyText = ""
	repo.LifecyclePolicyAt = time.Time{}

	s.saveLocked()

	return prev, at, nil
}
