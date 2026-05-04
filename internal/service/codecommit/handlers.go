// Package codecommit provides AWS CodeCommit service emulation.
package codecommit

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Error codes for CodeCommit handlers.
const (
	errInvalidAction           = "InvalidAction"
	errInternalServerException = "InternalServerException"
	errInvalidInputException   = "InvalidInputException"
)

// handlerFunc is the type for action handler functions.
type handlerFunc func(http.ResponseWriter, *http.Request)

// getActionHandlers returns the action handlers map.
func (s *Service) getActionHandlers() map[string]handlerFunc {
	return map[string]handlerFunc{
		"CreateRepository": s.CreateRepository,
		"DeleteRepository": s.DeleteRepository,
		"GetRepository":    s.GetRepository,
		"ListRepositories": s.ListRepositories,
		"CreateBranch":     s.CreateBranch,
		"GetBranch":        s.GetBranch,
		"ListBranches":     s.ListBranches,
		"GetFile":          s.GetFile,
		"PutFile":          s.PutFile,
	}
}

// DispatchAction dispatches an incoming request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "CodeCommit_20150413.")

	handlers := s.getActionHandlers()

	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeCodeCommitError(w, errInvalidAction, "The action "+action+" is not valid", http.StatusBadRequest)
}

// CreateRepository handles the CreateRepository action.
func (s *Service) CreateRepository(w http.ResponseWriter, r *http.Request) {
	var req CreateRepositoryRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeCodeCommitError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	repo, err := s.storage.CreateRepository(r.Context(), req.RepositoryName, req.RepositoryDescription, req.Tags, req.KmsKeyID)
	if err != nil {
		handleCodeCommitError(w, err)

		return
	}

	writeJSONResponse(w, CreateRepositoryResponse{
		RepositoryMetadata: convertRepoToOutput(repo),
	})
}

// DeleteRepository handles the DeleteRepository action.
func (s *Service) DeleteRepository(w http.ResponseWriter, r *http.Request) {
	var req DeleteRepositoryRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeCodeCommitError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	repoID, err := s.storage.DeleteRepository(r.Context(), req.RepositoryName)
	if err != nil {
		handleCodeCommitError(w, err)

		return
	}

	writeJSONResponse(w, DeleteRepositoryResponse{
		RepositoryID: repoID,
	})
}

// GetRepository handles the GetRepository action.
func (s *Service) GetRepository(w http.ResponseWriter, r *http.Request) {
	var req GetRepositoryRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeCodeCommitError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	repo, err := s.storage.GetRepository(r.Context(), req.RepositoryName)
	if err != nil {
		handleCodeCommitError(w, err)

		return
	}

	writeJSONResponse(w, GetRepositoryResponse{
		RepositoryMetadata: convertRepoToOutput(repo),
	})
}

// ListRepositories handles the ListRepositories action.
func (s *Service) ListRepositories(w http.ResponseWriter, r *http.Request) {
	var req ListRepositoriesRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeCodeCommitError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	repos, nextToken, err := s.storage.ListRepositories(r.Context(), req.NextToken, req.SortBy, req.Order)
	if err != nil {
		handleCodeCommitError(w, err)

		return
	}

	outputs := make([]RepositoryNameIDPairOutput, 0, len(repos))

	for _, repo := range repos {
		outputs = append(outputs, RepositoryNameIDPairOutput{
			RepositoryName: repo.RepositoryName,
			RepositoryID:   repo.RepositoryID,
		})
	}

	writeJSONResponse(w, ListRepositoriesResponse{
		Repositories: outputs,
		NextToken:    nextToken,
	})
}

// CreateBranch handles the CreateBranch action.
func (s *Service) CreateBranch(w http.ResponseWriter, r *http.Request) {
	var req CreateBranchRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeCodeCommitError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if _, err := s.storage.CreateBranch(r.Context(), req.RepositoryName, req.BranchName, req.CommitID); err != nil {
		handleCodeCommitError(w, err)

		return
	}

	writeJSONResponse(w, CreateBranchResponse{})
}

// GetBranch handles the GetBranch action.
func (s *Service) GetBranch(w http.ResponseWriter, r *http.Request) {
	var req GetBranchRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeCodeCommitError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	branch, err := s.storage.GetBranch(r.Context(), req.RepositoryName, req.BranchName)
	if err != nil {
		handleCodeCommitError(w, err)

		return
	}

	writeJSONResponse(w, GetBranchResponse{
		Branch: &BranchInfoOutput{
			BranchName: branch.BranchName,
			CommitID:   branch.DefaultCommitID,
		},
	})
}

// ListBranches handles the ListBranches action.
func (s *Service) ListBranches(w http.ResponseWriter, r *http.Request) {
	var req ListBranchesRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeCodeCommitError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	branchNames, nextToken, err := s.storage.ListBranches(r.Context(), req.RepositoryName, req.NextToken)
	if err != nil {
		handleCodeCommitError(w, err)

		return
	}

	writeJSONResponse(w, ListBranchesResponse{
		Branches:  branchNames,
		NextToken: nextToken,
	})
}

// GetFile handles the GetFile action.
func (s *Service) GetFile(w http.ResponseWriter, r *http.Request) {
	var req GetFileRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeCodeCommitError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	file, commit, err := s.storage.GetFile(r.Context(), req.RepositoryName, req.FilePath, req.CommitSpecifier)
	if err != nil {
		handleCodeCommitError(w, err)

		return
	}

	commitID := ""
	if commit != nil {
		commitID = commit.CommitID
	}

	writeJSONResponse(w, GetFileResponse{
		CommitID:    commitID,
		BlobID:      uuid.New().String(),
		FilePath:    file.FilePath,
		FileMode:    file.FileMode,
		FileSize:    int64(len(file.FileContent)),
		FileContent: base64.StdEncoding.EncodeToString(file.FileContent),
	})
}

// PutFile handles the PutFile action.
func (s *Service) PutFile(w http.ResponseWriter, r *http.Request) {
	var req PutFileRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeCodeCommitError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	decodedContent, err := base64.StdEncoding.DecodeString(req.FileContent)
	if err != nil {
		writeCodeCommitError(w, errInvalidInputException, "FileContent must be base64-encoded.", http.StatusBadRequest)

		return
	}

	commit, blobID, err := s.storage.PutFile(
		r.Context(),
		req.RepositoryName,
		req.BranchName,
		req.FilePath,
		decodedContent,
		req.FileMode,
		req.CommitMessage,
		req.Name,
		req.Email,
		req.ParentCommitID,
	)
	if err != nil {
		handleCodeCommitError(w, err)

		return
	}

	writeJSONResponse(w, PutFileResponse{
		CommitID: commit.CommitID,
		BlobID:   blobID,
		TreeID:   commit.TreeID,
	})
}

// toEpochFloat converts a time.Time to epoch seconds as float64.
func toEpochFloat(t time.Time) float64 {
	return float64(t.UnixMilli()) / 1000.0
}

// convertRepoToOutput converts a Repository to RepositoryMetadataOutput.
func convertRepoToOutput(repo *Repository) *RepositoryMetadataOutput {
	return &RepositoryMetadataOutput{
		AccountID:             "000000000000",
		RepositoryID:          repo.RepositoryID,
		RepositoryName:        repo.RepositoryName,
		RepositoryDescription: repo.RepositoryDescription,
		DefaultBranch:         repo.DefaultBranch,
		LastModifiedDate:      toEpochFloat(repo.LastModifiedDate),
		CreationDate:          toEpochFloat(repo.CreatedAt),
		CloneURLHTTP:          repo.CloneURLHTTP,
		CloneURLSSH:           repo.CloneURLSSH,
		Arn:                   repo.Arn,
		KmsKeyID:              repo.KmsKeyID,
	}
}

// readJSONRequest reads and unmarshals a JSON request body.
func readJSONRequest(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	if len(body) == 0 {
		return nil
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// writeJSONResponse writes a JSON response.
func writeJSONResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// writeCodeCommitError writes a CodeCommit error response.
func writeCodeCommitError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Type:    code,
		Message: message,
	})
}

// handleCodeCommitError handles a CodeCommit error and writes the response.
func handleCodeCommitError(w http.ResponseWriter, err error) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		status := http.StatusBadRequest

		if svcErr.Code == errInternalServerException {
			status = http.StatusInternalServerError
		}

		writeCodeCommitError(w, svcErr.Code, svcErr.Message, status)

		return
	}

	writeCodeCommitError(w, errInternalServerException, "Internal server error", http.StatusInternalServerError)
}
