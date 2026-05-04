// Package codecommit provides AWS CodeCommit service emulation.
package codecommit

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes for CodeCommit.
const (
	errRepositoryDoesNotExistException          = "RepositoryDoesNotExistException"
	errRepositoryNameExistsException            = "RepositoryNameExistsException"
	errRepositoryNameRequiredException          = "RepositoryNameRequiredException"
	errInvalidRepositoryNameException           = "InvalidRepositoryNameException"
	errBranchDoesNotExistException              = "BranchDoesNotExistException"
	errBranchNameExistsException                = "BranchNameExistsException"
	errBranchNameRequiredException              = "BranchNameRequiredException"
	errInvalidBranchNameException               = "InvalidBranchNameException"
	errFileDoesNotExistException                = "FileDoesNotExistException"
	errFileContentRequiredException             = "FileContentRequiredException"
	errInvalidPathException                     = "InvalidPathException"
	errPathRequiredException                    = "PathRequiredException"
	errCommitRequiredException                  = "CommitRequiredException"
	errCommitIDRequiredException                = "CommitIdRequiredException"
	errEncryptionIntegrityChecksFailedException = "EncryptionIntegrityChecksFailedException"
	errEncryptionKeyAccessDeniedException       = "EncryptionKeyAccessDeniedException"
	errEncryptionKeyDisabledException           = "EncryptionKeyDisabledException"
	errEncryptionKeyNotFoundException           = "EncryptionKeyNotFoundException"
	errEncryptionKeyUnavailableException        = "EncryptionKeyUnavailableException"
)

// Storage defines the interface for CodeCommit storage.
type Storage interface {
	CreateRepository(ctx context.Context, name, description string, tags map[string]string, kmsKeyID string) (*Repository, error)
	GetRepository(ctx context.Context, name string) (*Repository, error)
	DeleteRepository(ctx context.Context, name string) (string, error)
	ListRepositories(ctx context.Context, nextToken string, sortBy, order string) ([]*Repository, string, error)
	CreateBranch(ctx context.Context, repoName, branchName, commitID string) (*Branch, error)
	GetBranch(ctx context.Context, repoName, branchName string) (*Branch, error)
	ListBranches(ctx context.Context, repoName string, nextToken string) ([]string, string, error)
	GetFile(ctx context.Context, repoName, filePath, commitSpecifier string) (*FileEntry, *Commit, error)
	PutFile(ctx context.Context, repoName, branchName, filePath string, fileContent []byte, fileMode, commitMessage, name, email, parentCommitID string) (*Commit, string, error)
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

// MemoryStorage implements Storage with in-memory data structures.
type MemoryStorage struct {
	mu           sync.RWMutex                     `json:"-"`
	Repositories map[string]*Repository           `json:"repositories"`
	Branches     map[string]map[string]*Branch    `json:"branches"`
	Files        map[string]map[string]*FileEntry `json:"files"`
	Commits      map[string]map[string]*Commit    `json:"commits"`
	accountID    string
	region       string
	dataDir      string
}

// NewMemoryStorage creates a new MemoryStorage with optional configuration.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Repositories: make(map[string]*Repository),
		Branches:     make(map[string]map[string]*Branch),
		Files:        make(map[string]map[string]*FileEntry),
		Commits:      make(map[string]map[string]*Commit),
		accountID:    "000000000000",
		region:       "us-east-1",
	}

	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "codecommit", s)
	}

	return s
}

// MarshalJSON implements json.Marshaler.
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

// UnmarshalJSON implements json.Unmarshaler.
func (s *MemoryStorage) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(s)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if s.Repositories == nil {
		s.Repositories = make(map[string]*Repository)
	}

	if s.Branches == nil {
		s.Branches = make(map[string]map[string]*Branch)
	}

	if s.Files == nil {
		s.Files = make(map[string]map[string]*FileEntry)
	}

	if s.Commits == nil {
		s.Commits = make(map[string]map[string]*Commit)
	}

	return nil
}

// Close saves the storage state if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "codecommit", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

var repositoryNameRegex = regexp.MustCompile(`^[\w.-]+$`)

// validateRepositoryName checks if the repository name meets AWS requirements.
func validateRepositoryName(name string) error {
	if name == "" {
		return &ServiceError{
			Code:    errRepositoryNameRequiredException,
			Message: "RepositoryName is required.",
		}
	}

	if len(name) > 100 {
		return &ServiceError{
			Code:    errInvalidRepositoryNameException,
			Message: "RepositoryName cannot exceed 100 characters.",
		}
	}

	if strings.HasSuffix(name, ".git") {
		return &ServiceError{
			Code:    errInvalidRepositoryNameException,
			Message: fmt.Sprintf("%s is not a valid repository name because it ends with '.git'.", name),
		}
	}

	if !repositoryNameRegex.MatchString(name) {
		return &ServiceError{
			Code:    errInvalidRepositoryNameException,
			Message: fmt.Sprintf("%s is not a valid repository name.", name),
		}
	}

	return nil
}

// CreateRepository creates a new repository.
func (s *MemoryStorage) CreateRepository(_ context.Context, name, description string, tags map[string]string, kmsKeyID string) (*Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateRepositoryName(name); err != nil {
		return nil, err
	}

	if _, ok := s.Repositories[name]; ok {
		return nil, &ServiceError{
			Code:    errRepositoryNameExistsException,
			Message: fmt.Sprintf("A repository named %s already exists.", name),
		}
	}

	repoID := uuid.New().String()
	arn := fmt.Sprintf("arn:aws:codecommit:%s:%s:%s", s.region, s.accountID, name)
	now := time.Now()

	repo := &Repository{
		RepositoryName:        name,
		RepositoryDescription: description,
		DefaultBranch:         "main",
		RepositoryID:          repoID,
		Arn:                   arn,
		KmsKeyID:              kmsKeyID,
		CreatedAt:             now,
		LastModifiedDate:      now,
		CloneURLHTTP:          fmt.Sprintf("https://git-codecommit.%s.amazonaws.com/v1/repos/%s", s.region, name),
		CloneURLSSH:           fmt.Sprintf("ssh://git-codecommit.%s.amazonaws.com/v1/repos/%s", s.region, name),
		Tags:                  tags,
	}

	s.Repositories[name] = repo

	initialCommitID := uuid.New().String()
	s.Branches[name] = map[string]*Branch{
		"main": {
			BranchName:      "main",
			DefaultCommitID: initialCommitID,
			RepositoryName:  name,
		},
	}

	s.Files[name] = make(map[string]*FileEntry)
	s.Commits[name] = map[string]*Commit{
		initialCommitID: {
			CommitID:     initialCommitID,
			TreeID:       uuid.New().String(),
			ParentIDs:    []string{},
			Message:      fmt.Sprintf("Initial commit for %s", name),
			Author:       &UserInfo{Name: "AWS CodeCommit", Email: "codecommit@amazon.com", Date: now.Format(time.RFC3339)},
			Committer:    &UserInfo{Name: "AWS CodeCommit", Email: "codecommit@amazon.com", Date: now.Format(time.RFC3339)},
			CreationDate: now,
		},
	}

	return repo, nil
}

// GetRepository returns a repository by name.
func (s *MemoryStorage) GetRepository(_ context.Context, name string) (*Repository, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := validateRepositoryName(name); err != nil {
		return nil, err
	}

	repo, ok := s.Repositories[name]
	if !ok {
		return nil, &ServiceError{
			Code:    errRepositoryDoesNotExistException,
			Message: fmt.Sprintf("%s does not exist.", name),
		}
	}

	return repo, nil
}

// DeleteRepository deletes a repository by name.
func (s *MemoryStorage) DeleteRepository(_ context.Context, name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateRepositoryName(name); err != nil {
		return "", err
	}

	repo, ok := s.Repositories[name]
	if !ok {
		return "", &ServiceError{
			Code:    errRepositoryDoesNotExistException,
			Message: fmt.Sprintf("%s does not exist.", name),
		}
	}

	delete(s.Repositories, name)
	delete(s.Branches, name)
	delete(s.Files, name)
	delete(s.Commits, name)

	return repo.RepositoryID, nil
}

// ListRepositories lists all repositories with optional sorting.
func (s *MemoryStorage) ListRepositories(_ context.Context, _, sortBy, order string) ([]*Repository, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	repos := make([]*Repository, 0, len(s.Repositories))

	for _, repo := range s.Repositories {
		repos = append(repos, repo)
	}

	switch sortBy {
	case "lastModifiedDate":
		sort.Slice(repos, func(i, j int) bool {
			if order == "descending" {
				return repos[i].LastModifiedDate.After(repos[j].LastModifiedDate)
			}

			return repos[i].LastModifiedDate.Before(repos[j].LastModifiedDate)
		})
	case "repositoryName":
		sort.Slice(repos, func(i, j int) bool {
			if order == "descending" {
				return repos[i].RepositoryName > repos[j].RepositoryName
			}

			return repos[i].RepositoryName < repos[j].RepositoryName
		})
	default:
		sort.Slice(repos, func(i, j int) bool {
			return repos[i].RepositoryName < repos[j].RepositoryName
		})
	}

	return repos, "", nil
}

// CreateBranch creates a new branch in a repository.
func (s *MemoryStorage) CreateBranch(_ context.Context, repoName, branchName, commitID string) (*Branch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateRepositoryName(repoName); err != nil {
		return nil, err
	}

	if branchName == "" {
		return nil, &ServiceError{
			Code:    errBranchNameRequiredException,
			Message: "BranchName is required.",
		}
	}

	branches, ok := s.Branches[repoName]
	if !ok {
		return nil, &ServiceError{
			Code:    errRepositoryDoesNotExistException,
			Message: fmt.Sprintf("%s does not exist.", repoName),
		}
	}

	if _, exists := branches[branchName]; exists {
		return nil, &ServiceError{
			Code:    errBranchNameExistsException,
			Message: fmt.Sprintf("A branch named %s already exists.", branchName),
		}
	}

	if commitID == "" {
		return nil, &ServiceError{
			Code:    errCommitIDRequiredException,
			Message: "CommitId is required.",
		}
	}

	branch := &Branch{
		BranchName:      branchName,
		DefaultCommitID: commitID,
		RepositoryName:  repoName,
	}

	branches[branchName] = branch

	return branch, nil
}

// GetBranch returns a branch in a repository.
func (s *MemoryStorage) GetBranch(_ context.Context, repoName, branchName string) (*Branch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if repoName == "" {
		return nil, &ServiceError{
			Code:    errRepositoryNameRequiredException,
			Message: "RepositoryName is required.",
		}
	}

	if branchName == "" {
		return nil, &ServiceError{
			Code:    errBranchNameRequiredException,
			Message: "BranchName is required.",
		}
	}

	branches, ok := s.Branches[repoName]
	if !ok {
		return nil, &ServiceError{
			Code:    errRepositoryDoesNotExistException,
			Message: fmt.Sprintf("%s does not exist.", repoName),
		}
	}

	branch, exists := branches[branchName]
	if !exists {
		return nil, &ServiceError{
			Code:    errBranchDoesNotExistException,
			Message: fmt.Sprintf("%s does not exist in %s.", branchName, repoName),
		}
	}

	return branch, nil
}

// ListBranches lists all branches in a repository.
func (s *MemoryStorage) ListBranches(_ context.Context, repoName, _ string) ([]string, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if repoName == "" {
		return nil, "", &ServiceError{
			Code:    errRepositoryNameRequiredException,
			Message: "RepositoryName is required.",
		}
	}

	branches, ok := s.Branches[repoName]
	if !ok {
		return nil, "", &ServiceError{
			Code:    errRepositoryDoesNotExistException,
			Message: fmt.Sprintf("%s does not exist.", repoName),
		}
	}

	names := make([]string, 0, len(branches))

	for name := range branches {
		names = append(names, name)
	}

	sort.Strings(names)

	return names, "", nil
}

// GetFile returns a file from a repository.
func (s *MemoryStorage) GetFile(_ context.Context, repoName, filePath, _ string) (*FileEntry, *Commit, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if repoName == "" {
		return nil, nil, &ServiceError{
			Code:    errRepositoryNameRequiredException,
			Message: "RepositoryName is required.",
		}
	}

	if filePath == "" {
		return nil, nil, &ServiceError{
			Code:    errPathRequiredException,
			Message: "FilePath is required.",
		}
	}

	files, ok := s.Files[repoName]
	if !ok {
		return nil, nil, &ServiceError{
			Code:    errRepositoryDoesNotExistException,
			Message: fmt.Sprintf("%s does not exist.", repoName),
		}
	}

	file, exists := files[filePath]
	if !exists {
		return nil, nil, &ServiceError{
			Code:    errFileDoesNotExistException,
			Message: fmt.Sprintf("File %s does not exist.", filePath),
		}
	}

	var latestCommit *Commit

	for _, commit := range s.Commits[repoName] {
		if latestCommit == nil || commit.CreationDate.After(latestCommit.CreationDate) {
			latestCommit = commit
		}
	}

	return file, latestCommit, nil
}

func (s *MemoryStorage) validatePutFile(repoName, branchName, filePath string, fileContent []byte) (map[string]*Branch, error) {
	if repoName == "" {
		return nil, &ServiceError{Code: errRepositoryNameRequiredException, Message: "RepositoryName is required."}
	}

	if branchName == "" {
		return nil, &ServiceError{Code: errBranchNameRequiredException, Message: "BranchName is required."}
	}

	if filePath == "" {
		return nil, &ServiceError{Code: errPathRequiredException, Message: "FilePath is required."}
	}

	if len(fileContent) == 0 {
		return nil, &ServiceError{Code: errFileContentRequiredException, Message: "FileContent is required."}
	}

	branches, ok := s.Branches[repoName]
	if !ok {
		return nil, &ServiceError{
			Code:    errRepositoryDoesNotExistException,
			Message: fmt.Sprintf("%s does not exist.", repoName),
		}
	}

	if _, exists := branches[branchName]; !exists {
		return nil, &ServiceError{
			Code:    errBranchDoesNotExistException,
			Message: fmt.Sprintf("%s does not exist in %s.", branchName, repoName),
		}
	}

	return branches, nil
}

// PutFile adds or updates a file in a repository.
func (s *MemoryStorage) PutFile(_ context.Context, repoName, branchName, filePath string, fileContent []byte, fileMode, commitMessage, name, email, parentCommitID string) (*Commit, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	branches, err := s.validatePutFile(repoName, branchName, filePath, fileContent)
	if err != nil {
		return nil, "", err
	}

	if s.Files[repoName] == nil {
		s.Files[repoName] = make(map[string]*FileEntry)
	}

	blobID := uuid.New().String()

	fileModeVal := "NORMAL"
	if fileMode != "" {
		fileModeVal = fileMode
	}

	s.Files[repoName][filePath] = &FileEntry{
		FilePath:    filePath,
		FileContent: fileContent,
		FileMode:    fileModeVal,
	}

	commit := s.createFileCommit(branches, branchName, filePath, commitMessage, name, email, parentCommitID)

	if s.Commits[repoName] == nil {
		s.Commits[repoName] = make(map[string]*Commit)
	}

	s.Commits[repoName][commit.CommitID] = commit
	branches[branchName].DefaultCommitID = commit.CommitID

	repo := s.Repositories[repoName]
	repo.LastModifiedDate = commit.CreationDate

	return commit, blobID, nil
}

// createFileCommit creates a new commit for a file change.
func (s *MemoryStorage) createFileCommit(branches map[string]*Branch, branchName, filePath, commitMessage, name, email, parentCommitID string) *Commit {
	commitID := uuid.New().String()
	treeID := uuid.New().String()
	now := time.Now()

	if commitMessage == "" {
		commitMessage = fmt.Sprintf("Added %s", filePath)
	}

	if name == "" {
		name = "AWS CodeCommit"
	}

	if email == "" {
		email = "codecommit@amazon.com"
	}

	parentIDs := []string{branches[branchName].DefaultCommitID}
	if parentCommitID != "" {
		parentIDs = []string{parentCommitID}
	}

	return &Commit{
		CommitID:     commitID,
		TreeID:       treeID,
		ParentIDs:    parentIDs,
		Message:      commitMessage,
		Author:       &UserInfo{Name: name, Email: email, Date: now.Format(time.RFC3339)},
		Committer:    &UserInfo{Name: name, Email: email, Date: now.Format(time.RFC3339)},
		CreationDate: now,
	}
}
