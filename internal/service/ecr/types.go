package ecr

import "time"

// Repository represents an ECR repository.
type Repository struct {
	RepositoryArn              string
	RegistryID                 string
	RepositoryName             string
	RepositoryURI              string
	CreatedAt                  time.Time
	ImageTagMutability         string
	ImageScanningConfiguration *ImageScanningConfiguration
	EncryptionConfiguration    *EncryptionConfiguration
}

// ImageScanningConfiguration represents image scanning configuration.
type ImageScanningConfiguration struct {
	ScanOnPush bool `json:"scanOnPush,omitempty"`
}

// EncryptionConfiguration represents encryption configuration.
type EncryptionConfiguration struct {
	EncryptionType string `json:"encryptionType,omitempty"`
	KmsKey         string `json:"kmsKey,omitempty"`
}

// Image represents an image in ECR.
type Image struct {
	RegistryID     string
	RepositoryName string
	ImageID        *ImageIdentifier
	ImageManifest  string
	ImageDigest    string
	PushedAt       time.Time
}

// ImageIdentifier identifies an image.
type ImageIdentifier struct {
	ImageDigest string `json:"imageDigest,omitempty"`
	ImageTag    string `json:"imageTag,omitempty"`
}

// ImageDetail represents detailed information about an image.
type ImageDetail struct {
	RegistryID        string   `json:"registryId,omitempty"`
	RepositoryName    string   `json:"repositoryName,omitempty"`
	ImageDigest       string   `json:"imageDigest,omitempty"`
	ImageTags         []string `json:"imageTags,omitempty"`
	ImageSizeInBytes  int64    `json:"imageSizeInBytes,omitempty"`
	ImagePushedAt     float64  `json:"imagePushedAt,omitempty"`
	ImageManifest     string   `json:"imageManifest,omitempty"`
	ArtifactMediaType string   `json:"artifactMediaType,omitempty"`
}

// PutLifecyclePolicyRequest is the request for PutLifecyclePolicy.
type PutLifecyclePolicyRequest struct {
	RegistryID          string `json:"registryId,omitempty"`
	RepositoryName      string `json:"repositoryName"`
	LifecyclePolicyText string `json:"lifecyclePolicyText"`
}

// PutLifecyclePolicyResponse is the response for PutLifecyclePolicy.
type PutLifecyclePolicyResponse struct {
	RegistryID          string `json:"registryId,omitempty"`
	RepositoryName      string `json:"repositoryName,omitempty"`
	LifecyclePolicyText string `json:"lifecyclePolicyText,omitempty"`
}

// GetLifecyclePolicyRequest is the request for GetLifecyclePolicy.
type GetLifecyclePolicyRequest struct {
	RegistryID     string `json:"registryId,omitempty"`
	RepositoryName string `json:"repositoryName"`
}

// GetLifecyclePolicyResponse is the response for GetLifecyclePolicy.
type GetLifecyclePolicyResponse struct {
	RegistryID          string  `json:"registryId,omitempty"`
	RepositoryName      string  `json:"repositoryName,omitempty"`
	LifecyclePolicyText string  `json:"lifecyclePolicyText,omitempty"`
	LastEvaluatedAt     float64 `json:"lastEvaluatedAt,omitempty"`
}

// DeleteLifecyclePolicyRequest is the request for DeleteLifecyclePolicy.
type DeleteLifecyclePolicyRequest struct {
	RegistryID     string `json:"registryId,omitempty"`
	RepositoryName string `json:"repositoryName"`
}

// DeleteLifecyclePolicyResponse is the response for DeleteLifecyclePolicy.
type DeleteLifecyclePolicyResponse struct {
	RegistryID          string  `json:"registryId,omitempty"`
	RepositoryName      string  `json:"repositoryName,omitempty"`
	LifecyclePolicyText string  `json:"lifecyclePolicyText,omitempty"`
	LastEvaluatedAt     float64 `json:"lastEvaluatedAt,omitempty"`
}

// CreateRepositoryRequest is the request for CreateRepository.
type CreateRepositoryRequest struct {
	RepositoryName             string                      `json:"repositoryName"`
	Tags                       []Tag                       `json:"tags,omitempty"`
	ImageTagMutability         string                      `json:"imageTagMutability,omitempty"`
	ImageScanningConfiguration *ImageScanningConfiguration `json:"imageScanningConfiguration,omitempty"`
	EncryptionConfiguration    *EncryptionConfiguration    `json:"encryptionConfiguration,omitempty"`
}

// Tag represents a tag.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// CreateRepositoryResponse is the response for CreateRepository.
type CreateRepositoryResponse struct {
	Repository *RepositoryOutput `json:"repository"`
}

// RepositoryOutput is the output representation of a repository.
type RepositoryOutput struct {
	RepositoryArn              string                      `json:"repositoryArn"`
	RegistryID                 string                      `json:"registryId"`
	RepositoryName             string                      `json:"repositoryName"`
	RepositoryURI              string                      `json:"repositoryUri"`
	CreatedAt                  float64                     `json:"createdAt"`
	ImageTagMutability         string                      `json:"imageTagMutability,omitempty"`
	ImageScanningConfiguration *ImageScanningConfiguration `json:"imageScanningConfiguration,omitempty"`
	EncryptionConfiguration    *EncryptionConfiguration    `json:"encryptionConfiguration,omitempty"`
}

// DeleteRepositoryRequest is the request for DeleteRepository.
type DeleteRepositoryRequest struct {
	RepositoryName string `json:"repositoryName"`
	RegistryID     string `json:"registryId,omitempty"`
	Force          bool   `json:"force,omitempty"`
}

// DeleteRepositoryResponse is the response for DeleteRepository.
type DeleteRepositoryResponse struct {
	Repository *RepositoryOutput `json:"repository"`
}

// DescribeRepositoriesRequest is the request for DescribeRepositories.
type DescribeRepositoriesRequest struct {
	RepositoryNames []string `json:"repositoryNames,omitempty"`
	RegistryID      string   `json:"registryId,omitempty"`
	NextToken       string   `json:"nextToken,omitempty"`
	MaxResults      int32    `json:"maxResults,omitempty"`
}

// DescribeRepositoriesResponse is the response for DescribeRepositories.
type DescribeRepositoriesResponse struct {
	Repositories []RepositoryOutput `json:"repositories"`
	NextToken    string             `json:"nextToken,omitempty"`
}

// ListImagesRequest is the request for ListImages.
type ListImagesRequest struct {
	RepositoryName string            `json:"repositoryName"`
	RegistryID     string            `json:"registryId,omitempty"`
	NextToken      string            `json:"nextToken,omitempty"`
	MaxResults     int32             `json:"maxResults,omitempty"`
	Filter         *ListImagesFilter `json:"filter,omitempty"`
}

// ListImagesFilter is the filter for ListImages.
type ListImagesFilter struct {
	TagStatus string `json:"tagStatus,omitempty"`
}

// ListImagesResponse is the response for ListImages.
type ListImagesResponse struct {
	ImageIDs  []ImageIdentifier `json:"imageIds"`
	NextToken string            `json:"nextToken,omitempty"`
}

// PutImageRequest is the request for PutImage.
type PutImageRequest struct {
	RepositoryName string `json:"repositoryName"`
	ImageManifest  string `json:"imageManifest"`
	ImageTag       string `json:"imageTag,omitempty"`
	RegistryID     string `json:"registryId,omitempty"`
	ImageDigest    string `json:"imageDigest,omitempty"`
}

// PutImageResponse is the response for PutImage.
type PutImageResponse struct {
	Image *ImageOutput `json:"image"`
}

// ImageOutput is the output representation of an image.
type ImageOutput struct {
	RegistryID     string           `json:"registryId"`
	RepositoryName string           `json:"repositoryName"`
	ImageID        *ImageIdentifier `json:"imageId"`
	ImageManifest  string           `json:"imageManifest,omitempty"`
}

// BatchGetImageRequest is the request for BatchGetImage.
type BatchGetImageRequest struct {
	RepositoryName     string            `json:"repositoryName"`
	ImageIDs           []ImageIdentifier `json:"imageIds"`
	RegistryID         string            `json:"registryId,omitempty"`
	AcceptedMediaTypes []string          `json:"acceptedMediaTypes,omitempty"`
}

// BatchGetImageResponse is the response for BatchGetImage.
type BatchGetImageResponse struct {
	Images   []ImageOutput  `json:"images"`
	Failures []ImageFailure `json:"failures,omitempty"`
}

// ImageFailure represents a failure to retrieve an image.
type ImageFailure struct {
	ImageID       *ImageIdentifier `json:"imageId,omitempty"`
	FailureCode   string           `json:"failureCode,omitempty"`
	FailureReason string           `json:"failureReason,omitempty"`
}

// BatchDeleteImageRequest is the request for BatchDeleteImage.
type BatchDeleteImageRequest struct {
	RepositoryName string            `json:"repositoryName"`
	ImageIDs       []ImageIdentifier `json:"imageIds"`
	RegistryID     string            `json:"registryId,omitempty"`
}

// BatchDeleteImageResponse is the response for BatchDeleteImage.
type BatchDeleteImageResponse struct {
	ImageIDs []ImageIdentifier `json:"imageIds,omitempty"`
	Failures []ImageFailure    `json:"failures,omitempty"`
}

// GetAuthorizationTokenRequest is the request for GetAuthorizationToken.
type GetAuthorizationTokenRequest struct {
	RegistryIDs []string `json:"registryIds,omitempty"`
}

// GetAuthorizationTokenResponse is the response for GetAuthorizationToken.
type GetAuthorizationTokenResponse struct {
	AuthorizationData []AuthorizationData `json:"authorizationData"`
}

// AuthorizationData contains authorization data for ECR.
type AuthorizationData struct {
	AuthorizationToken string  `json:"authorizationToken"`
	ExpiresAt          float64 `json:"expiresAt"`
	ProxyEndpoint      string  `json:"proxyEndpoint"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// ServiceError represents a service-level error.
type ServiceError struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *ServiceError) Error() string {
	return e.Message
}
