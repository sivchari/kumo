// Package codecommit provides AWS CodeCommit service emulation.
package codecommit

import (
	"time"
)

// Repository represents a CodeCommit repository.
type Repository struct {
	RepositoryName        string
	RepositoryDescription string
	DefaultBranch         string
	RepositoryID          string
	Arn                   string
	KmsKeyID              string
	CreatedAt             time.Time
	LastModifiedDate      time.Time
	CloneURLHTTP          string
	CloneURLSSH           string
	Tags                  map[string]string
}

// Branch represents a branch in a repository.
type Branch struct {
	BranchName      string
	DefaultCommitID string
	RepositoryName  string
}

// FileEntry represents a file in a repository.
type FileEntry struct {
	FilePath    string
	FileContent []byte
	FileMode    string
}

// UserInfo represents author or committer information.
type UserInfo struct {
	Name  string
	Email string
	Date  string
}

// Commit represents a commit in a repository.
type Commit struct {
	CommitID     string
	TreeID       string
	ParentIDs    []string
	Message      string
	Author       *UserInfo
	Committer    *UserInfo
	CreationDate time.Time
}

// CreateRepositoryRequest represents a CreateRepository request.
type CreateRepositoryRequest struct {
	RepositoryName        string            `json:"repositoryName"`
	RepositoryDescription string            `json:"repositoryDescription,omitempty"`
	Tags                  map[string]string `json:"tags,omitempty"`
	KmsKeyID              string            `json:"kmsKeyId,omitempty"`
}

// CreateRepositoryResponse represents a CreateRepository response.
type CreateRepositoryResponse struct {
	RepositoryMetadata *RepositoryMetadataOutput `json:"repositoryMetadata,omitempty"`
}

// DeleteRepositoryRequest represents a DeleteRepository request.
type DeleteRepositoryRequest struct {
	RepositoryName string `json:"repositoryName"`
}

// DeleteRepositoryResponse represents a DeleteRepository response.
type DeleteRepositoryResponse struct {
	RepositoryID string `json:"repositoryId,omitempty"`
}

// GetRepositoryRequest represents a GetRepository request.
type GetRepositoryRequest struct {
	RepositoryName string `json:"repositoryName"`
}

// GetRepositoryResponse represents a GetRepository response.
type GetRepositoryResponse struct {
	RepositoryMetadata *RepositoryMetadataOutput `json:"repositoryMetadata,omitempty"`
}

// ListRepositoriesRequest represents a ListRepositories request.
type ListRepositoriesRequest struct {
	NextToken string `json:"nextToken,omitempty"`
	Order     string `json:"order,omitempty"`
	SortBy    string `json:"sortBy,omitempty"`
}

// ListRepositoriesResponse represents a ListRepositories response.
type ListRepositoriesResponse struct {
	Repositories []RepositoryNameIDPairOutput `json:"repositories,omitempty"`
	NextToken    string                       `json:"nextToken,omitempty"`
}

// RepositoryNameIDPairOutput represents a repository name and ID pair.
type RepositoryNameIDPairOutput struct {
	RepositoryName string `json:"repositoryName,omitempty"`
	RepositoryID   string `json:"repositoryId,omitempty"`
}

// RepositoryMetadataOutput represents repository metadata in API responses.
type RepositoryMetadataOutput struct {
	AccountID             string  `json:"accountId,omitempty"`
	Arn                   string  `json:"Arn,omitempty"`
	CloneURLHTTP          string  `json:"cloneUrlHttp,omitempty"`
	CloneURLSSH           string  `json:"cloneUrlSsh,omitempty"`
	CreationDate          float64 `json:"creationDate,omitempty"`
	DefaultBranch         string  `json:"defaultBranch,omitempty"`
	KmsKeyID              string  `json:"kmsKeyId,omitempty"`
	LastModifiedDate      float64 `json:"lastModifiedDate,omitempty"`
	RepositoryDescription string  `json:"repositoryDescription,omitempty"`
	RepositoryID          string  `json:"repositoryId,omitempty"`
	RepositoryName        string  `json:"repositoryName,omitempty"`
}

// CreateBranchRequest represents a CreateBranch request.
type CreateBranchRequest struct {
	RepositoryName string `json:"repositoryName"`
	BranchName     string `json:"branchName"`
	CommitID       string `json:"commitId,omitempty"`
}

// CreateBranchResponse represents a CreateBranch response.
type CreateBranchResponse struct{}

// GetBranchRequest represents a GetBranch request.
type GetBranchRequest struct {
	RepositoryName string `json:"repositoryName"`
	BranchName     string `json:"branchName"`
}

// GetBranchResponse represents a GetBranch response.
type GetBranchResponse struct {
	Branch *BranchInfoOutput `json:"branch,omitempty"`
}

// BranchInfoOutput represents branch information in API responses.
type BranchInfoOutput struct {
	BranchName string `json:"branchName,omitempty"`
	CommitID   string `json:"commitId,omitempty"`
}

// ListBranchesRequest represents a ListBranches request.
type ListBranchesRequest struct {
	RepositoryName string `json:"repositoryName"`
	NextToken      string `json:"nextToken,omitempty"`
}

// ListBranchesResponse represents a ListBranches response.
type ListBranchesResponse struct {
	Branches  []string `json:"branches,omitempty"`
	NextToken string   `json:"nextToken,omitempty"`
}

// GetFileRequest represents a GetFile request.
type GetFileRequest struct {
	RepositoryName  string `json:"repositoryName"`
	FilePath        string `json:"filePath"`
	CommitSpecifier string `json:"commitSpecifier,omitempty"`
}

// GetFileResponse represents a GetFile response.
type GetFileResponse struct {
	BlobID      string `json:"blobId,omitempty"`
	CommitID    string `json:"commitId,omitempty"`
	FileContent string `json:"fileContent,omitempty"`
	FileMode    string `json:"fileMode,omitempty"`
	FilePath    string `json:"filePath,omitempty"`
	FileSize    int64  `json:"fileSize,omitempty"`
}

// PutFileRequest represents a PutFile request.
type PutFileRequest struct {
	RepositoryName string `json:"repositoryName"`
	BranchName     string `json:"branchName"`
	FileContent    string `json:"fileContent"`
	FilePath       string `json:"filePath"`
	FileMode       string `json:"fileMode,omitempty"`
	CommitMessage  string `json:"commitMessage,omitempty"`
	Name           string `json:"name,omitempty"`
	Email          string `json:"email,omitempty"`
	ParentCommitID string `json:"parentCommitId,omitempty"`
}

// PutFileResponse represents a PutFile response.
type PutFileResponse struct {
	BlobID   string `json:"blobId,omitempty"`
	CommitID string `json:"commitId,omitempty"`
	TreeID   string `json:"treeId,omitempty"`
}

// ErrorResponse represents a CodeCommit error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// ServiceError represents a CodeCommit service error.
type ServiceError struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *ServiceError) Error() string {
	return e.Message
}
