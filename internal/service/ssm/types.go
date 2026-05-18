// Package ssm provides SSM Parameter Store service emulation for kumo.
package ssm

import (
	"time"
)

// Parameter types.
const (
	ParameterTypeString       = "String"
	ParameterTypeStringList   = "StringList"
	ParameterTypeSecureString = "SecureString"
)

// Parameter tiers.
const (
	ParameterTierStandard        = "Standard"
	ParameterTierAdvanced        = "Advanced"
	ParameterTierIntelligentTier = "Intelligent-Tiering"
)

// Parameter represents an SSM parameter.
type Parameter struct {
	Name             string
	Type             string
	Value            string
	Version          int64
	LastModifiedDate time.Time
	ARN              string
	DataType         string
	Tier             string
	Description      string
}

// ParameterMetadata represents parameter metadata for DescribeParameters.
type ParameterMetadata struct {
	Name             string  `json:"Name"`
	Type             string  `json:"Type"`
	Description      string  `json:"Description,omitempty"`
	Version          int64   `json:"Version"`
	LastModifiedDate float64 `json:"LastModifiedDate"`
	Tier             string  `json:"Tier,omitempty"`
	DataType         string  `json:"DataType,omitempty"`
}

// PutParameterRequest is the request for PutParameter.
type PutParameterRequest struct {
	Name        string `json:"Name"`
	Value       string `json:"Value"`
	Type        string `json:"Type,omitempty"`
	Description string `json:"Description,omitempty"`
	Overwrite   bool   `json:"Overwrite,omitempty"`
	Tier        string `json:"Tier,omitempty"`
	DataType    string `json:"DataType,omitempty"`
}

// PutParameterResponse is the response for PutParameter.
type PutParameterResponse struct {
	Version int64  `json:"Version"`
	Tier    string `json:"Tier,omitempty"`
}

// GetParameterRequest is the request for GetParameter.
type GetParameterRequest struct {
	Name           string `json:"Name"`
	WithDecryption bool   `json:"WithDecryption,omitempty"`
}

// GetParameterResponse is the response for GetParameter.
type GetParameterResponse struct {
	Parameter *ParameterValue `json:"Parameter"`
}

// ParameterValue represents a parameter value in responses.
type ParameterValue struct {
	Name             string  `json:"Name"`
	Type             string  `json:"Type"`
	Value            string  `json:"Value"`
	Version          int64   `json:"Version"`
	LastModifiedDate float64 `json:"LastModifiedDate"`
	ARN              string  `json:"ARN"`
	DataType         string  `json:"DataType,omitempty"`
}

// GetParametersRequest is the request for GetParameters.
type GetParametersRequest struct {
	Names          []string `json:"Names"`
	WithDecryption bool     `json:"WithDecryption,omitempty"`
}

// GetParametersResponse is the response for GetParameters.
type GetParametersResponse struct {
	Parameters        []*ParameterValue `json:"Parameters"`
	InvalidParameters []string          `json:"InvalidParameters,omitempty"`
}

// GetParametersByPathRequest is the request for GetParametersByPath.
type GetParametersByPathRequest struct {
	Path           string `json:"Path"`
	Recursive      bool   `json:"Recursive,omitempty"`
	WithDecryption bool   `json:"WithDecryption,omitempty"`
	MaxResults     int    `json:"MaxResults,omitempty"`
	NextToken      string `json:"NextToken,omitempty"`
}

// GetParametersByPathResponse is the response for GetParametersByPath.
type GetParametersByPathResponse struct {
	Parameters []*ParameterValue `json:"Parameters"`
	NextToken  string            `json:"NextToken,omitempty"`
}

// DeleteParameterRequest is the request for DeleteParameter.
type DeleteParameterRequest struct {
	Name string `json:"Name"`
}

// DeleteParametersRequest is the request for DeleteParameters.
type DeleteParametersRequest struct {
	Names []string `json:"Names"`
}

// DeleteParametersResponse is the response for DeleteParameters.
type DeleteParametersResponse struct {
	DeletedParameters []string `json:"DeletedParameters,omitempty"`
	InvalidParameters []string `json:"InvalidParameters,omitempty"`
}

// ParameterFilter is a filter for DescribeParameters.
type ParameterFilter struct {
	Key    string   `json:"Key"`
	Values []string `json:"Values,omitempty"`
	Option string   `json:"Option,omitempty"`
}

// DescribeParametersRequest is the request for DescribeParameters.
type DescribeParametersRequest struct {
	ParameterFilters []ParameterFilter `json:"ParameterFilters,omitempty"`
	MaxResults       int               `json:"MaxResults,omitempty"`
	NextToken        string            `json:"NextToken,omitempty"`
}

// DescribeParametersResponse is the response for DescribeParameters.
type DescribeParametersResponse struct {
	Parameters []*ParameterMetadata `json:"Parameters"`
	NextToken  string               `json:"NextToken,omitempty"`
}

// ParameterError represents an SSM error.
type ParameterError struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (e *ParameterError) Error() string {
	return e.Message
}

// Error codes for SSM.
const (
	ErrParameterNotFound      = "ParameterNotFound"
	ErrParameterAlreadyExists = "ParameterAlreadyExists"
	ErrInvalidParameterValue  = "ValidationException"
	ErrServiceException       = "InternalServerError"
)

// Tag represents an SSM resource tag.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// ListTagsForResourceRequest is the request for ListTagsForResource.
type ListTagsForResourceRequest struct {
	ResourceType string `json:"ResourceType"`
	ResourceID   string `json:"ResourceId"`
}

// ListTagsForResourceResponse is the response for ListTagsForResource.
type ListTagsForResourceResponse struct {
	TagList []Tag `json:"TagList"`
}

// AddTagsToResourceRequest is the request for AddTagsToResource.
type AddTagsToResourceRequest struct {
	ResourceType string `json:"ResourceType"`
	ResourceID   string `json:"ResourceId"`
	Tags         []Tag  `json:"Tags"`
}

// RemoveTagsFromResourceRequest is the request for RemoveTagsFromResource.
type RemoveTagsFromResourceRequest struct {
	ResourceType string   `json:"ResourceType"`
	ResourceID   string   `json:"ResourceId"`
	TagKeys      []string `json:"TagKeys"`
}
