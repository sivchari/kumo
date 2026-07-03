// Package route53resolver provides Route 53 Resolver service emulation for kumo.
package route53resolver

// ResolverEndpoint represents a Route 53 Resolver endpoint.
type ResolverEndpoint struct {
	ID                    string
	CreatorRequestID      string
	ARN                   string
	Name                  string
	SecurityGroupIDs      []string
	Direction             string // INBOUND or OUTBOUND
	IPAddressCount        int
	HostVPCID             string
	Status                string
	StatusMessage         string
	CreationTime          string
	ModificationTime      string
	OutpostArn            string
	PreferredInstanceType string
	ResolverEndpointType  string
	Protocols             []string
	IPAddresses           []*IPAddressResponse
}

// IPAddressRequest represents an IP address request.
type IPAddressRequest struct {
	SubnetID string `json:"SubnetId"`
	IP       string `json:"Ip,omitempty"`
	IPv6     string `json:"Ipv6,omitempty"`
}

// IPAddressResponse represents an IP address response.
type IPAddressResponse struct {
	IPAddressID      string `json:"IpId"`
	SubnetID         string `json:"SubnetId"`
	IP               string `json:"Ip,omitempty"`
	IPv6             string `json:"Ipv6,omitempty"`
	Status           string `json:"Status"`
	StatusMessage    string `json:"StatusMessage,omitempty"`
	CreationTime     string `json:"CreationTime"`
	ModificationTime string `json:"ModificationTime"`
}

// ResolverRule represents a Route 53 Resolver rule.
type ResolverRule struct {
	ID                 string
	CreatorRequestID   string
	ARN                string
	DomainName         string
	Status             string
	StatusMessage      string
	RuleType           string // FORWARD, SYSTEM, or RECURSIVE
	Name               string
	TargetIPs          []*TargetAddress
	ResolverEndpointID string
	OwnerID            string
	ShareStatus        string
	CreationTime       string
	ModificationTime   string
}

// TargetAddress represents a target IP address for forwarding.
type TargetAddress struct {
	IP       string `json:"Ip,omitempty"`
	Port     int    `json:"Port,omitempty"`
	IPv6     string `json:"Ipv6,omitempty"`
	Protocol string `json:"Protocol,omitempty"`
}

// ResolverRuleAssociation represents an association between a resolver rule and a VPC.
type ResolverRuleAssociation struct {
	ID             string
	ResolverRuleID string
	Name           string
	VPCID          string
	Status         string
	StatusMessage  string
}

// Tag represents a resource tag.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// CreateResolverEndpointRequest is the request for CreateResolverEndpoint.
type CreateResolverEndpointRequest struct {
	CreatorRequestID      string             `json:"CreatorRequestId"`
	Name                  string             `json:"Name,omitempty"`
	SecurityGroupIDs      []string           `json:"SecurityGroupIds"`
	Direction             string             `json:"Direction"`
	IPAddresses           []IPAddressRequest `json:"IpAddresses"`
	OutpostArn            string             `json:"OutpostArn,omitempty"`
	PreferredInstanceType string             `json:"PreferredInstanceType,omitempty"`
	ResolverEndpointType  string             `json:"ResolverEndpointType,omitempty"`
	Protocols             []string           `json:"Protocols,omitempty"`
	Tags                  []Tag              `json:"Tags,omitempty"`
}

// CreateResolverEndpointResponse is the response for CreateResolverEndpoint.
type CreateResolverEndpointResponse struct {
	ResolverEndpoint *ResolverEndpointOutput `json:"ResolverEndpoint"`
}

// ResolverEndpointOutput represents a resolver endpoint in API responses.
type ResolverEndpointOutput struct {
	ID                    string   `json:"Id"`
	CreatorRequestID      string   `json:"CreatorRequestId"`
	ARN                   string   `json:"Arn"`
	Name                  string   `json:"Name,omitempty"`
	SecurityGroupIDs      []string `json:"SecurityGroupIds"`
	Direction             string   `json:"Direction"`
	IPAddressCount        int      `json:"IpAddressCount"`
	HostVPCID             string   `json:"HostVPCId"`
	Status                string   `json:"Status"`
	StatusMessage         string   `json:"StatusMessage,omitempty"`
	CreationTime          string   `json:"CreationTime"`
	ModificationTime      string   `json:"ModificationTime"`
	OutpostArn            string   `json:"OutpostArn,omitempty"`
	PreferredInstanceType string   `json:"PreferredInstanceType,omitempty"`
	ResolverEndpointType  string   `json:"ResolverEndpointType,omitempty"`
	Protocols             []string `json:"Protocols,omitempty"`
}

// GetResolverEndpointRequest is the request for GetResolverEndpoint.
type GetResolverEndpointRequest struct {
	ResolverEndpointID string `json:"ResolverEndpointId"`
}

// GetResolverEndpointResponse is the response for GetResolverEndpoint.
type GetResolverEndpointResponse struct {
	ResolverEndpoint *ResolverEndpointOutput `json:"ResolverEndpoint"`
}

// DeleteResolverEndpointRequest is the request for DeleteResolverEndpoint.
type DeleteResolverEndpointRequest struct {
	ResolverEndpointID string `json:"ResolverEndpointId"`
}

// DeleteResolverEndpointResponse is the response for DeleteResolverEndpoint.
type DeleteResolverEndpointResponse struct {
	ResolverEndpoint *ResolverEndpointOutput `json:"ResolverEndpoint"`
}

// ListResolverEndpointsRequest is the request for ListResolverEndpoints.
type ListResolverEndpointsRequest struct {
	MaxResults int      `json:"MaxResults,omitempty"`
	NextToken  string   `json:"NextToken,omitempty"`
	Filters    []Filter `json:"Filters,omitempty"`
}

// Filter represents a filter for list operations.
type Filter struct {
	Name   string   `json:"Name"`
	Values []string `json:"Values"`
}

// ListResolverEndpointsResponse is the response for ListResolverEndpoints.
type ListResolverEndpointsResponse struct {
	NextToken         string                    `json:"NextToken,omitempty"`
	MaxResults        int                       `json:"MaxResults"`
	ResolverEndpoints []*ResolverEndpointOutput `json:"ResolverEndpoints"`
}

// CreateResolverRuleRequest is the request for CreateResolverRule.
type CreateResolverRuleRequest struct {
	CreatorRequestID   string          `json:"CreatorRequestId"`
	Name               string          `json:"Name,omitempty"`
	RuleType           string          `json:"RuleType"`
	DomainName         string          `json:"DomainName"`
	TargetIPs          []TargetAddress `json:"TargetIps,omitempty"`
	ResolverEndpointID string          `json:"ResolverEndpointId,omitempty"`
	Tags               []Tag           `json:"Tags,omitempty"`
}

// CreateResolverRuleResponse is the response for CreateResolverRule.
type CreateResolverRuleResponse struct {
	ResolverRule *ResolverRuleOutput `json:"ResolverRule"`
}

// ResolverRuleOutput represents a resolver rule in API responses.
type ResolverRuleOutput struct {
	ID                 string          `json:"Id"`
	CreatorRequestID   string          `json:"CreatorRequestId"`
	ARN                string          `json:"Arn"`
	DomainName         string          `json:"DomainName"`
	Status             string          `json:"Status"`
	StatusMessage      string          `json:"StatusMessage,omitempty"`
	RuleType           string          `json:"RuleType"`
	Name               string          `json:"Name,omitempty"`
	TargetIPs          []TargetAddress `json:"TargetIps,omitempty"`
	ResolverEndpointID string          `json:"ResolverEndpointId,omitempty"`
	OwnerID            string          `json:"OwnerId"`
	ShareStatus        string          `json:"ShareStatus"`
	CreationTime       string          `json:"CreationTime"`
	ModificationTime   string          `json:"ModificationTime"`
}

// GetResolverRuleRequest is the request for GetResolverRule.
type GetResolverRuleRequest struct {
	ResolverRuleID string `json:"ResolverRuleId"`
}

// GetResolverRuleResponse is the response for GetResolverRule.
type GetResolverRuleResponse struct {
	ResolverRule *ResolverRuleOutput `json:"ResolverRule"`
}

// DeleteResolverRuleRequest is the request for DeleteResolverRule.
type DeleteResolverRuleRequest struct {
	ResolverRuleID string `json:"ResolverRuleId"`
}

// DeleteResolverRuleResponse is the response for DeleteResolverRule.
type DeleteResolverRuleResponse struct {
	ResolverRule *ResolverRuleOutput `json:"ResolverRule"`
}

// ListResolverRulesRequest is the request for ListResolverRules.
type ListResolverRulesRequest struct {
	MaxResults int      `json:"MaxResults,omitempty"`
	NextToken  string   `json:"NextToken,omitempty"`
	Filters    []Filter `json:"Filters,omitempty"`
}

// ListResolverRulesResponse is the response for ListResolverRules.
type ListResolverRulesResponse struct {
	NextToken     string                `json:"NextToken,omitempty"`
	MaxResults    int                   `json:"MaxResults"`
	ResolverRules []*ResolverRuleOutput `json:"ResolverRules"`
}

// AssociateResolverRuleRequest is the request for AssociateResolverRule.
type AssociateResolverRuleRequest struct {
	ResolverRuleID string `json:"ResolverRuleId"`
	Name           string `json:"Name,omitempty"`
	VPCID          string `json:"VPCId"`
}

// AssociateResolverRuleResponse is the response for AssociateResolverRule.
type AssociateResolverRuleResponse struct {
	ResolverRuleAssociation *ResolverRuleAssociationOutput `json:"ResolverRuleAssociation"`
}

// ResolverRuleAssociationOutput represents a resolver rule association in API responses.
type ResolverRuleAssociationOutput struct {
	ID             string `json:"Id"`
	ResolverRuleID string `json:"ResolverRuleId"`
	Name           string `json:"Name,omitempty"`
	VPCID          string `json:"VPCId"`
	Status         string `json:"Status"`
	StatusMessage  string `json:"StatusMessage,omitempty"`
}

// DisassociateResolverRuleRequest is the request for DisassociateResolverRule.
type DisassociateResolverRuleRequest struct {
	ResolverRuleID string `json:"ResolverRuleId"`
	VPCID          string `json:"VPCId"`
}

// DisassociateResolverRuleResponse is the response for DisassociateResolverRule.
type DisassociateResolverRuleResponse struct {
	ResolverRuleAssociation *ResolverRuleAssociationOutput `json:"ResolverRuleAssociation"`
}

// ListResolverRuleAssociationsRequest is the request for ListResolverRuleAssociations.
type ListResolverRuleAssociationsRequest struct {
	MaxResults int      `json:"MaxResults,omitempty"`
	NextToken  string   `json:"NextToken,omitempty"`
	Filters    []Filter `json:"Filters,omitempty"`
}

// ListResolverRuleAssociationsResponse is the response for ListResolverRuleAssociations.
type ListResolverRuleAssociationsResponse struct {
	NextToken                string                           `json:"NextToken,omitempty"`
	MaxResults               int                              `json:"MaxResults"`
	ResolverRuleAssociations []*ResolverRuleAssociationOutput `json:"ResolverRuleAssociations"`
}

// ErrorResponse represents a Route 53 Resolver error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// ResolverError represents a Route 53 Resolver error.
type ResolverError struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *ResolverError) Error() string {
	return e.Message
}
