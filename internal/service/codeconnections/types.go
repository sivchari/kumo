package codeconnections

import (
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// ConnectionStatus represents the status of a connection.
type ConnectionStatus string

// Connection statuses.
const (
	ConnectionStatusPending   ConnectionStatus = "PENDING"
	ConnectionStatusAvailable ConnectionStatus = "AVAILABLE"
	ConnectionStatusError     ConnectionStatus = "ERROR"
)

// ProviderType represents the provider type.
type ProviderType string

// Provider types.
const (
	ProviderTypeBitbucket              ProviderType = "Bitbucket"
	ProviderTypeGitHub                 ProviderType = "GitHub"
	ProviderTypeGitHubEnterpriseServer ProviderType = "GitHubEnterpriseServer"
	ProviderTypeGitLab                 ProviderType = "GitLab"
	ProviderTypeGitLabSelfManaged      ProviderType = "GitLabSelfManaged"
)

// Connection represents a CodeConnections connection.
type Connection struct {
	ConnectionArn    string
	ConnectionName   string
	ConnectionStatus ConnectionStatus
	OwnerAccountID   string
	ProviderType     ProviderType
	HostArn          string
	CreatedAt        time.Time
	Tags             map[string]string
}

// Host represents a CodeConnections host.
type Host struct {
	HostArn          string
	Name             string
	Status           string
	ProviderType     ProviderType
	ProviderEndpoint string
	VpcConfiguration *VpcConfiguration
	StatusMessage    string
	CreatedAt        time.Time
	Tags             map[string]string
}

// VpcConfiguration represents VPC configuration for a host.
type VpcConfiguration struct {
	VpcID            string
	SubnetIDs        []string
	SecurityGroupIDs []string
	TLSCertificate   string
}

// RepositoryLink represents a repository link.
type RepositoryLink struct {
	RepositoryLinkArn string
	RepositoryLinkID  string
	ConnectionArn     string
	OwnerID           string
	ProviderType      ProviderType
	RepositoryName    string
	EncryptionKeyArn  string
	CreatedAt         time.Time
	Tags              map[string]string
}

// CreateConnectionRequest is the request for CreateConnection.
type CreateConnectionRequest struct {
	ConnectionName string `json:"ConnectionName"`
	ProviderType   string `json:"ProviderType,omitempty"`
	Tags           []Tag  `json:"Tags,omitempty"`
	HostArn        string `json:"HostArn,omitempty"`
}

// Tag represents a tag.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// CreateConnectionResponse is the response for CreateConnection.
type CreateConnectionResponse struct {
	ConnectionArn string `json:"ConnectionArn"`
	Tags          []Tag  `json:"Tags,omitempty"`
}

// GetConnectionRequest is the request for GetConnection.
type GetConnectionRequest struct {
	ConnectionArn string `json:"ConnectionArn"`
}

// GetConnectionResponse is the response for GetConnection.
type GetConnectionResponse struct {
	Connection *ConnectionOutput `json:"Connection,omitempty"`
}

// ConnectionOutput represents connection in API response.
type ConnectionOutput struct {
	ConnectionArn    string `json:"ConnectionArn,omitempty"`
	ConnectionName   string `json:"ConnectionName,omitempty"`
	ConnectionStatus string `json:"ConnectionStatus,omitempty"`
	OwnerAccountID   string `json:"OwnerAccountId,omitempty"`
	ProviderType     string `json:"ProviderType,omitempty"`
	HostArn          string `json:"HostArn,omitempty"`
}

// DeleteConnectionRequest is the request for DeleteConnection.
type DeleteConnectionRequest struct {
	ConnectionArn string `json:"ConnectionArn"`
}

// DeleteConnectionResponse is the response for DeleteConnection.
type DeleteConnectionResponse struct{}

// ListConnectionsRequest is the request for ListConnections.
type ListConnectionsRequest struct {
	ProviderTypeFilter string `json:"ProviderTypeFilter,omitempty"`
	HostArnFilter      string `json:"HostArnFilter,omitempty"`
	MaxResults         int32  `json:"MaxResults,omitempty"`
	NextToken          string `json:"NextToken,omitempty"`
}

// ListConnectionsResponse is the response for ListConnections.
type ListConnectionsResponse struct {
	Connections []ConnectionOutput `json:"Connections,omitempty"`
	NextToken   string             `json:"NextToken,omitempty"`
}

// CreateHostRequest is the request for CreateHost.
type CreateHostRequest struct {
	Name             string          `json:"Name"`
	ProviderType     string          `json:"ProviderType"`
	ProviderEndpoint string          `json:"ProviderEndpoint"`
	VpcConfiguration *VpcConfigInput `json:"VpcConfiguration,omitempty"`
	Tags             []Tag           `json:"Tags,omitempty"`
}

// VpcConfigInput represents VPC configuration input.
type VpcConfigInput struct {
	VpcID            string   `json:"VpcId"`
	SubnetIDs        []string `json:"SubnetIds"`
	SecurityGroupIDs []string `json:"SecurityGroupIds"`
	TLSCertificate   string   `json:"TLSCertificate,omitempty"`
}

// CreateHostResponse is the response for CreateHost.
type CreateHostResponse struct {
	HostArn string `json:"HostArn"`
	Tags    []Tag  `json:"Tags,omitempty"`
}

// GetHostRequest is the request for GetHost.
type GetHostRequest struct {
	HostArn string `json:"HostArn"`
}

// GetHostResponse is the response for GetHost.
type GetHostResponse struct {
	Name             string           `json:"Name,omitempty"`
	Status           string           `json:"Status,omitempty"`
	ProviderType     string           `json:"ProviderType,omitempty"`
	ProviderEndpoint string           `json:"ProviderEndpoint,omitempty"`
	VpcConfiguration *VpcConfigOutput `json:"VpcConfiguration,omitempty"`
}

// VpcConfigOutput represents VPC configuration output.
type VpcConfigOutput struct {
	VpcID            string   `json:"VpcId,omitempty"`
	SubnetIDs        []string `json:"SubnetIds,omitempty"`
	SecurityGroupIDs []string `json:"SecurityGroupIds,omitempty"`
	TLSCertificate   string   `json:"TLSCertificate,omitempty"`
}

// DeleteHostRequest is the request for DeleteHost.
type DeleteHostRequest struct {
	HostArn string `json:"HostArn"`
}

// DeleteHostResponse is the response for DeleteHost.
type DeleteHostResponse struct{}

// ListHostsRequest is the request for ListHosts.
type ListHostsRequest struct {
	MaxResults int32  `json:"MaxResults,omitempty"`
	NextToken  string `json:"NextToken,omitempty"`
}

// ListHostsResponse is the response for ListHosts.
type ListHostsResponse struct {
	Hosts     []HostOutput `json:"Hosts,omitempty"`
	NextToken string       `json:"NextToken,omitempty"`
}

// HostOutput represents host in API response.
type HostOutput struct {
	HostArn          string           `json:"HostArn,omitempty"`
	Name             string           `json:"Name,omitempty"`
	Status           string           `json:"Status,omitempty"`
	ProviderType     string           `json:"ProviderType,omitempty"`
	ProviderEndpoint string           `json:"ProviderEndpoint,omitempty"`
	VpcConfiguration *VpcConfigOutput `json:"VpcConfiguration,omitempty"`
	StatusMessage    string           `json:"StatusMessage,omitempty"`
}

// UpdateHostRequest is the request for UpdateHost.
type UpdateHostRequest struct {
	HostArn          string          `json:"HostArn"`
	ProviderEndpoint string          `json:"ProviderEndpoint,omitempty"`
	VpcConfiguration *VpcConfigInput `json:"VpcConfiguration,omitempty"`
}

// UpdateHostResponse is the response for UpdateHost.
type UpdateHostResponse struct{}

// CreateRepositoryLinkRequest is the request for CreateRepositoryLink.
type CreateRepositoryLinkRequest struct {
	ConnectionArn    string `json:"ConnectionArn"`
	OwnerID          string `json:"OwnerId"`
	RepositoryName   string `json:"RepositoryName"`
	EncryptionKeyArn string `json:"EncryptionKeyArn,omitempty"`
	Tags             []Tag  `json:"Tags,omitempty"`
}

// CreateRepositoryLinkResponse is the response for CreateRepositoryLink.
type CreateRepositoryLinkResponse struct {
	RepositoryLinkInfo *RepositoryLinkOutput `json:"RepositoryLinkInfo,omitempty"`
}

// RepositoryLinkOutput represents repository link in API response.
type RepositoryLinkOutput struct {
	RepositoryLinkArn string `json:"RepositoryLinkArn,omitempty"`
	RepositoryLinkID  string `json:"RepositoryLinkId,omitempty"`
	ConnectionArn     string `json:"ConnectionArn,omitempty"`
	OwnerID           string `json:"OwnerId,omitempty"`
	ProviderType      string `json:"ProviderType,omitempty"`
	RepositoryName    string `json:"RepositoryName,omitempty"`
	EncryptionKeyArn  string `json:"EncryptionKeyArn,omitempty"`
}

// GetRepositoryLinkRequest is the request for GetRepositoryLink.
type GetRepositoryLinkRequest struct {
	RepositoryLinkID string `json:"RepositoryLinkId"`
}

// GetRepositoryLinkResponse is the response for GetRepositoryLink.
type GetRepositoryLinkResponse struct {
	RepositoryLinkInfo *RepositoryLinkOutput `json:"RepositoryLinkInfo,omitempty"`
}

// DeleteRepositoryLinkRequest is the request for DeleteRepositoryLink.
type DeleteRepositoryLinkRequest struct {
	RepositoryLinkID string `json:"RepositoryLinkId"`
}

// DeleteRepositoryLinkResponse is the response for DeleteRepositoryLink.
type DeleteRepositoryLinkResponse struct{}

// ListRepositoryLinksRequest is the request for ListRepositoryLinks.
type ListRepositoryLinksRequest struct {
	MaxResults int32  `json:"MaxResults,omitempty"`
	NextToken  string `json:"NextToken,omitempty"`
}

// ListRepositoryLinksResponse is the response for ListRepositoryLinks.
type ListRepositoryLinksResponse struct {
	RepositoryLinks []RepositoryLinkOutput `json:"RepositoryLinks,omitempty"`
	NextToken       string                 `json:"NextToken,omitempty"`
}

// UpdateRepositoryLinkRequest is the request for UpdateRepositoryLink.
type UpdateRepositoryLinkRequest struct {
	RepositoryLinkID string `json:"RepositoryLinkId"`
	ConnectionArn    string `json:"ConnectionArn,omitempty"`
	EncryptionKeyArn string `json:"EncryptionKeyArn,omitempty"`
}

// UpdateRepositoryLinkResponse is the response for UpdateRepositoryLink.
type UpdateRepositoryLinkResponse struct {
	RepositoryLinkInfo *RepositoryLinkOutput `json:"RepositoryLinkInfo,omitempty"`
}

// ListTagsForResourceRequest is the request for ListTagsForResource.
type ListTagsForResourceRequest struct {
	ResourceArn string `json:"ResourceArn"`
}

// ListTagsForResourceResponse is the response for ListTagsForResource.
type ListTagsForResourceResponse struct {
	Tags []Tag `json:"Tags,omitempty"`
}

// TagResourceRequest is the request for TagResource.
type TagResourceRequest struct {
	ResourceArn string `json:"ResourceArn"`
	Tags        []Tag  `json:"Tags"`
}

// TagResourceResponse is the response for TagResource.
type TagResourceResponse struct{}

// UntagResourceRequest is the request for UntagResource.
type UntagResourceRequest struct {
	ResourceArn string   `json:"ResourceArn"`
	TagKeys     []string `json:"TagKeys"`
}

// UntagResourceResponse is the response for UntagResource.
type UntagResourceResponse struct{}

// ErrorResponse represents a CodeConnections error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// ServiceError represents a CodeConnections service error.
type ServiceError = service.CodedError
