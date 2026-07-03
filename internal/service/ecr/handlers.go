package ecr

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

// handlerFunc is a type alias for handler functions.
type handlerFunc func(http.ResponseWriter, *http.Request)

// getActionHandlers returns a map of action names to handler functions.
func (s *Service) getActionHandlers() map[string]handlerFunc {
	return map[string]handlerFunc{
		"CreateRepository":      s.CreateRepository,
		"DeleteRepository":      s.DeleteRepository,
		"DescribeRepositories":  s.DescribeRepositories,
		"ListImages":            s.ListImages,
		"PutImage":              s.PutImage,
		"BatchGetImage":         s.BatchGetImage,
		"BatchDeleteImage":      s.BatchDeleteImage,
		"GetAuthorizationToken": s.GetAuthorizationToken,
		"PutLifecyclePolicy":    s.PutLifecyclePolicy,
		"GetLifecyclePolicy":    s.GetLifecyclePolicy,
		"DeleteLifecyclePolicy": s.DeleteLifecyclePolicy,
		"ListTagsForResource":   s.tagsNoOp,
		"TagResource":           s.tagsNoOp,
		"UntagResource":         s.tagsNoOp,
	}
}

// tagsNoOp returns an empty success response. ECR tags are not modeled but
// AWS SDK clients call ListTagsForResource on every read; returning empty
// keeps the read path from hitting InvalidAction.
func (s *Service) tagsNoOp(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, struct {
		Tags []any `json:"tags"`
	}{Tags: []any{}})
}

// DispatchAction dispatches the request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "AmazonEC2ContainerRegistry_V20150921.")

	handlers := s.getActionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeError(w, "InvalidAction", "The action "+action+" is not valid for this endpoint.", http.StatusBadRequest)
}

// CreateRepository handles the CreateRepository API.
func (s *Service) CreateRepository(w http.ResponseWriter, r *http.Request) {
	var req CreateRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	repo, err := s.storage.CreateRepository(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &CreateRepositoryResponse{
		Repository: toRepositoryOutput(repo),
	}

	writeResponse(w, resp)
}

// DeleteRepository handles the DeleteRepository API.
func (s *Service) DeleteRepository(w http.ResponseWriter, r *http.Request) {
	var req DeleteRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	repo, err := s.storage.DeleteRepository(r.Context(), req.RepositoryName, req.Force)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &DeleteRepositoryResponse{
		Repository: toRepositoryOutput(repo),
	}

	writeResponse(w, resp)
}

// DescribeRepositories handles the DescribeRepositories API.
func (s *Service) DescribeRepositories(w http.ResponseWriter, r *http.Request) {
	var req DescribeRepositoriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	repos, nextToken, err := s.storage.DescribeRepositories(r.Context(), req.RepositoryNames, req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	outputs := make([]RepositoryOutput, len(repos))

	for i, repo := range repos {
		outputs[i] = *toRepositoryOutput(repo)
	}

	resp := &DescribeRepositoriesResponse{
		Repositories: outputs,
		NextToken:    nextToken,
	}

	writeResponse(w, resp)
}

// ListImages handles the ListImages API.
func (s *Service) ListImages(w http.ResponseWriter, r *http.Request) {
	var req ListImagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	imageIDs, nextToken, err := s.storage.ListImages(r.Context(), req.RepositoryName, req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	ids := make([]ImageIdentifier, len(imageIDs))

	for i, id := range imageIDs {
		ids[i] = *id
	}

	resp := &ListImagesResponse{
		ImageIDs:  ids,
		NextToken: nextToken,
	}

	writeResponse(w, resp)
}

// PutImage handles the PutImage API.
func (s *Service) PutImage(w http.ResponseWriter, r *http.Request) {
	var req PutImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	img, err := s.storage.PutImage(r.Context(), req.RepositoryName, req.ImageManifest, req.ImageTag)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &PutImageResponse{
		Image: toImageOutput(img),
	}

	writeResponse(w, resp)
}

// BatchGetImage handles the BatchGetImage API.
func (s *Service) BatchGetImage(w http.ResponseWriter, r *http.Request) {
	var req BatchGetImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	images, failures, err := s.storage.BatchGetImage(r.Context(), req.RepositoryName, req.ImageIDs)
	if err != nil {
		handleError(w, err)

		return
	}

	outputs := make([]ImageOutput, len(images))

	for i, img := range images {
		outputs[i] = *toImageOutput(img)
	}

	resp := &BatchGetImageResponse{
		Images:   outputs,
		Failures: failures,
	}

	writeResponse(w, resp)
}

// BatchDeleteImage handles the BatchDeleteImage API.
func (s *Service) BatchDeleteImage(w http.ResponseWriter, r *http.Request) {
	var req BatchDeleteImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	deleted, failures, err := s.storage.BatchDeleteImage(r.Context(), req.RepositoryName, req.ImageIDs)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &BatchDeleteImageResponse{
		ImageIDs: deleted,
		Failures: failures,
	}

	writeResponse(w, resp)
}

// GetAuthorizationToken handles the GetAuthorizationToken API.
func (s *Service) GetAuthorizationToken(w http.ResponseWriter, r *http.Request) {
	var req GetAuthorizationTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	authData, err := s.storage.GetAuthorizationToken(r.Context())
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &GetAuthorizationTokenResponse{
		AuthorizationData: authData,
	}

	writeResponse(w, resp)
}

// toRepositoryOutput converts a Repository to RepositoryOutput.
func toRepositoryOutput(repo *Repository) *RepositoryOutput {
	return &RepositoryOutput{
		RepositoryArn:              repo.RepositoryArn,
		RegistryID:                 repo.RegistryID,
		RepositoryName:             repo.RepositoryName,
		RepositoryURI:              repo.RepositoryURI,
		CreatedAt:                  float64(repo.CreatedAt.Unix()),
		ImageTagMutability:         repo.ImageTagMutability,
		ImageScanningConfiguration: repo.ImageScanningConfiguration,
		EncryptionConfiguration:    repo.EncryptionConfiguration,
	}
}

// toImageOutput converts an Image to ImageOutput.
func toImageOutput(img *Image) *ImageOutput {
	return &ImageOutput{
		RegistryID:     img.RegistryID,
		RepositoryName: img.RepositoryName,
		ImageID:        img.ImageID,
		ImageManifest:  img.ImageManifest,
	}
}

// PutLifecyclePolicy handles the PutLifecyclePolicy API.
func (s *Service) PutLifecyclePolicy(w http.ResponseWriter, r *http.Request) {
	var req PutLifecyclePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.RepositoryName == "" || req.LifecyclePolicyText == "" {
		writeError(w, "ValidationException", "repositoryName and lifecyclePolicyText are required", http.StatusBadRequest)

		return
	}

	stored, err := s.storage.PutLifecyclePolicy(r.Context(), req.RepositoryName, req.LifecyclePolicyText)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &PutLifecyclePolicyResponse{
		RegistryID:          req.RegistryID,
		RepositoryName:      req.RepositoryName,
		LifecyclePolicyText: stored,
	})
}

// GetLifecyclePolicy handles the GetLifecyclePolicy API.
func (s *Service) GetLifecyclePolicy(w http.ResponseWriter, r *http.Request) {
	var req GetLifecyclePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.RepositoryName == "" {
		writeError(w, "ValidationException", "repositoryName is required", http.StatusBadRequest)

		return
	}

	policy, at, err := s.storage.GetLifecyclePolicy(r.Context(), req.RepositoryName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &GetLifecyclePolicyResponse{
		RegistryID:          req.RegistryID,
		RepositoryName:      req.RepositoryName,
		LifecyclePolicyText: policy,
		LastEvaluatedAt:     epochSeconds(at),
	})
}

// DeleteLifecyclePolicy handles the DeleteLifecyclePolicy API.
func (s *Service) DeleteLifecyclePolicy(w http.ResponseWriter, r *http.Request) {
	var req DeleteLifecyclePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.RepositoryName == "" {
		writeError(w, "ValidationException", "repositoryName is required", http.StatusBadRequest)

		return
	}

	policy, at, err := s.storage.DeleteLifecyclePolicy(r.Context(), req.RepositoryName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DeleteLifecyclePolicyResponse{
		RegistryID:          req.RegistryID,
		RepositoryName:      req.RepositoryName,
		LifecyclePolicyText: policy,
		LastEvaluatedAt:     epochSeconds(at),
	})
}

func epochSeconds(t time.Time) float64 {
	if t.IsZero() {
		return 0
	}

	return float64(t.Unix())
}

// writeResponse writes a JSON response.
func writeResponse(w http.ResponseWriter, resp any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeAmzJSON11, code, message, status)
}

// handleError handles service errors.
func handleError(w http.ResponseWriter, err error) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		status := getErrorStatus(svcErr.Code)
		writeError(w, svcErr.Code, svcErr.Message, status)

		return
	}

	writeError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)
}

// getErrorStatus returns the HTTP status code for a given error code.
func getErrorStatus(code string) int {
	switch code {
	case errRepositoryNotFound, errImageNotFound:
		return http.StatusNotFound
	case errRepositoryAlreadyExists:
		return http.StatusConflict
	case errInvalidParameter:
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}
