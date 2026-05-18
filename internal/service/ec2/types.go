package ec2

import (
	"encoding/xml"
	"time"
)

// Instance represents an EC2 instance.
type Instance struct {
	InstanceID       string
	ImageID          string
	InstanceType     string
	State            InstanceState
	PrivateIPAddress string
	PublicIPAddress  string
	SecurityGroups   []GroupIdentifier
	KeyName          string
	LaunchTime       time.Time
	Tags             []Tag
}

// InstanceState represents the state of an instance.
type InstanceState struct {
	Code int
	Name string
}

// GroupIdentifier represents a security group.
type GroupIdentifier struct {
	GroupID   string
	GroupName string
}

// Tag represents a resource tag.
type Tag struct {
	Key   string
	Value string
}

// SecurityGroup represents an EC2 security group.
type SecurityGroup struct {
	GroupID      string
	GroupName    string
	Description  string
	VpcID        string
	IngressRules []IPPermission
	EgressRules  []IPPermission
	Tags         []Tag
}

// IPPermission represents an ingress or egress rule.
type IPPermission struct {
	IPProtocol string
	FromPort   int
	ToPort     int
	IPRanges   []IPRange
}

// IPRange represents a CIDR IP range.
type IPRange struct {
	CidrIP      string
	Description string
}

// KeyPair represents an EC2 key pair.
type KeyPair struct {
	KeyName        string
	KeyFingerprint string
	KeyPairID      string
	KeyMaterial    string
	CreateTime     time.Time
	Tags           []Tag
}

// RunInstancesRequest represents a RunInstances request.
type RunInstancesRequest struct {
	ImageID          string   `json:"ImageId"`
	InstanceType     string   `json:"InstanceType"`
	MinCount         int      `json:"MinCount"`
	MaxCount         int      `json:"MaxCount"`
	KeyName          string   `json:"KeyName,omitempty"`
	SecurityGroupIDs []string `json:"SecurityGroupIds,omitempty"`
	SecurityGroups   []string `json:"SecurityGroups,omitempty"`
}

// TerminateInstancesRequest represents a TerminateInstances request.
type TerminateInstancesRequest struct {
	InstanceIDs []string `json:"InstanceIds"`
}

// DescribeInstancesRequest represents a DescribeInstances request.
type DescribeInstancesRequest struct {
	InstanceIDs []string `json:"InstanceIds,omitempty"`
}

// StartInstancesRequest represents a StartInstances request.
type StartInstancesRequest struct {
	InstanceIDs []string `json:"InstanceIds"`
}

// StopInstancesRequest represents a StopInstances request.
type StopInstancesRequest struct {
	InstanceIDs []string `json:"InstanceIds"`
}

// CreateSecurityGroupRequest represents a CreateSecurityGroup request.
type CreateSecurityGroupRequest struct {
	GroupName        string `json:"GroupName"`
	GroupDescription string `json:"GroupDescription"`
	VpcID            string `json:"VpcId,omitempty"`
}

// DeleteSecurityGroupRequest represents a DeleteSecurityGroup request.
type DeleteSecurityGroupRequest struct {
	GroupID   string `json:"GroupId,omitempty"`
	GroupName string `json:"GroupName,omitempty"`
}

// AuthorizeSecurityGroupIngressRequest represents an AuthorizeSecurityGroupIngress request.
type AuthorizeSecurityGroupIngressRequest struct {
	GroupID       string         `json:"GroupId,omitempty"`
	GroupName     string         `json:"GroupName,omitempty"`
	IPPermissions []IPPermission `json:"IPPermissions"`
}

// AuthorizeSecurityGroupEgressRequest represents an AuthorizeSecurityGroupEgress request.
type AuthorizeSecurityGroupEgressRequest struct {
	GroupID       string         `json:"GroupId"`
	IPPermissions []IPPermission `json:"IPPermissions"`
}

// CreateKeyPairRequest represents a CreateKeyPair request.
type CreateKeyPairRequest struct {
	KeyName string `json:"KeyName"`
	KeyType string `json:"KeyType,omitempty"`
}

// DeleteKeyPairRequest represents a DeleteKeyPair request.
type DeleteKeyPairRequest struct {
	KeyName   string `json:"KeyName,omitempty"`
	KeyPairID string `json:"KeyPairId,omitempty"`
}

// DescribeKeyPairsRequest represents a DescribeKeyPairs request.
type DescribeKeyPairsRequest struct {
	KeyNames   []string `json:"KeyNames,omitempty"`
	KeyPairIDs []string `json:"KeyPairIds,omitempty"`
}

// XML Response types for EC2.

// XMLRunInstancesResponse is the XML response for RunInstances.
type XMLRunInstancesResponse struct {
	XMLName       xml.Name        `xml:"RunInstancesResponse"`
	Xmlns         string          `xml:"xmlns,attr"`
	RequestID     string          `xml:"requestId"`
	ReservationID string          `xml:"reservationId"`
	OwnerID       string          `xml:"ownerId"`
	InstancesSet  XMLInstancesSet `xml:"instancesSet"`
}

// XMLInstancesSet contains a list of instances.
type XMLInstancesSet struct {
	Items []XMLInstance `xml:"item"`
}

// XMLInstance represents an instance in XML format.
type XMLInstance struct {
	InstanceID       string           `xml:"instanceId"`
	ImageID          string           `xml:"imageId"`
	InstanceType     string           `xml:"instanceType"`
	InstanceState    XMLInstanceState `xml:"instanceState"`
	PrivateIPAddress string           `xml:"privateIpAddress,omitempty"`
	IPAddress        string           `xml:"ipAddress,omitempty"`
	KeyName          string           `xml:"keyName,omitempty"`
	LaunchTime       string           `xml:"launchTime"`
	GroupSet         XMLGroupSet      `xml:"groupSet"`
}

// XMLInstanceState represents the state of an instance in XML format.
type XMLInstanceState struct {
	Code int    `xml:"code"`
	Name string `xml:"name"`
}

// XMLGroupSet contains a list of security groups.
type XMLGroupSet struct {
	Items []XMLGroupIdentifier `xml:"item"`
}

// XMLGroupIdentifier represents a security group in XML format.
type XMLGroupIdentifier struct {
	GroupID   string `xml:"groupId"`
	GroupName string `xml:"groupName"`
}

// XMLTerminateInstancesResponse is the XML response for TerminateInstances.
type XMLTerminateInstancesResponse struct {
	XMLName      xml.Name                  `xml:"TerminateInstancesResponse"`
	Xmlns        string                    `xml:"xmlns,attr"`
	RequestID    string                    `xml:"requestId"`
	InstancesSet XMLInstanceStateChangeSet `xml:"instancesSet"`
}

// XMLInstanceStateChangeSet contains a list of instance state changes.
type XMLInstanceStateChangeSet struct {
	Items []XMLInstanceStateChange `xml:"item"`
}

// XMLInstanceStateChange represents an instance state change in XML format.
type XMLInstanceStateChange struct {
	InstanceID    string           `xml:"instanceId"`
	CurrentState  XMLInstanceState `xml:"currentState"`
	PreviousState XMLInstanceState `xml:"previousState"`
}

// XMLDescribeInstancesResponse is the XML response for DescribeInstances.
type XMLDescribeInstancesResponse struct {
	XMLName        xml.Name          `xml:"DescribeInstancesResponse"`
	Xmlns          string            `xml:"xmlns,attr"`
	RequestID      string            `xml:"requestId"`
	ReservationSet XMLReservationSet `xml:"reservationSet"`
}

// XMLReservationSet contains a list of reservations.
type XMLReservationSet struct {
	Items []XMLReservation `xml:"item"`
}

// XMLReservation represents a reservation in XML format.
type XMLReservation struct {
	ReservationID string          `xml:"reservationId"`
	OwnerID       string          `xml:"ownerId"`
	InstancesSet  XMLInstancesSet `xml:"instancesSet"`
}

// XMLStartInstancesResponse is the XML response for StartInstances.
type XMLStartInstancesResponse struct {
	XMLName      xml.Name                  `xml:"StartInstancesResponse"`
	Xmlns        string                    `xml:"xmlns,attr"`
	RequestID    string                    `xml:"requestId"`
	InstancesSet XMLInstanceStateChangeSet `xml:"instancesSet"`
}

// XMLStopInstancesResponse is the XML response for StopInstances.
type XMLStopInstancesResponse struct {
	XMLName      xml.Name                  `xml:"StopInstancesResponse"`
	Xmlns        string                    `xml:"xmlns,attr"`
	RequestID    string                    `xml:"requestId"`
	InstancesSet XMLInstanceStateChangeSet `xml:"instancesSet"`
}

// XMLCreateSecurityGroupResponse is the XML response for CreateSecurityGroup.
type XMLCreateSecurityGroupResponse struct {
	XMLName   xml.Name `xml:"CreateSecurityGroupResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	RequestID string   `xml:"requestId"`
	Return    bool     `xml:"return"`
	GroupID   string   `xml:"groupId"`
}

// XMLDeleteSecurityGroupResponse is the XML response for DeleteSecurityGroup.
type XMLDeleteSecurityGroupResponse struct {
	XMLName   xml.Name `xml:"DeleteSecurityGroupResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	RequestID string   `xml:"requestId"`
	Return    bool     `xml:"return"`
}

// XMLAuthorizeSecurityGroupIngressResponse is the XML response for AuthorizeSecurityGroupIngress.
type XMLAuthorizeSecurityGroupIngressResponse struct {
	XMLName   xml.Name `xml:"AuthorizeSecurityGroupIngressResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	RequestID string   `xml:"requestId"`
	Return    bool     `xml:"return"`
}

// XMLAuthorizeSecurityGroupEgressResponse is the XML response for AuthorizeSecurityGroupEgress.
type XMLAuthorizeSecurityGroupEgressResponse struct {
	XMLName   xml.Name `xml:"AuthorizeSecurityGroupEgressResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	RequestID string   `xml:"requestId"`
	Return    bool     `xml:"return"`
}

// DescribeSecurityGroupsRequest carries the optional GroupId / GroupName
// filters; an empty pair returns every SG.
type DescribeSecurityGroupsRequest struct {
	GroupIDs   []string `json:"GroupIds,omitempty"`
	GroupNames []string `json:"GroupNames,omitempty"`
}

// XMLDescribeSecurityGroupsResponse is the AWS Query wire shape: a
// securityGroupInfo container of <item> elements.
type XMLDescribeSecurityGroupsResponse struct {
	XMLName           xml.Name             `xml:"DescribeSecurityGroupsResponse"`
	Xmlns             string               `xml:"xmlns,attr"`
	RequestID         string               `xml:"requestId"`
	SecurityGroupInfo XMLSecurityGroupInfo `xml:"securityGroupInfo"`
}

// XMLSecurityGroupInfo wraps the per-SG <item> list.
type XMLSecurityGroupInfo struct {
	Items []XMLSecurityGroup `xml:"item"`
}

// XMLSecurityGroup is one SG on the wire. Field names use the
// lowercase-camel form AWS publishes for the legacy EC2 XML protocol.
type XMLSecurityGroup struct {
	OwnerID             string             `xml:"ownerId"`
	GroupID             string             `xml:"groupId"`
	GroupName           string             `xml:"groupName"`
	GroupDescription    string             `xml:"groupDescription"`
	VpcID               string             `xml:"vpcId,omitempty"`
	IPPermissions       XMLIPPermissionSet `xml:"ipPermissions"`
	IPPermissionsEgress XMLIPPermissionSet `xml:"ipPermissionsEgress"`
	TagSet              XMLTagSet          `xml:"tagSet"`
}

// XMLIPPermissionSet is a list of IP permissions.
type XMLIPPermissionSet struct {
	Items []XMLIPPermission `xml:"item"`
}

// XMLIPPermission is one ingress / egress rule on the wire.
type XMLIPPermission struct {
	IPProtocol string      `xml:"ipProtocol"`
	FromPort   int         `xml:"fromPort"`
	ToPort     int         `xml:"toPort"`
	IPRanges   XMLIPRanges `xml:"ipRanges"`
}

// XMLIPRanges wraps the per-CIDR <item> list.
type XMLIPRanges struct {
	Items []XMLIPRange `xml:"item"`
}

// XMLIPRange is one CIDR entry.
type XMLIPRange struct {
	CidrIP      string `xml:"cidrIp"`
	Description string `xml:"description,omitempty"`
}

// XMLCreateKeyPairResponse is the XML response for CreateKeyPair.
type XMLCreateKeyPairResponse struct {
	XMLName        xml.Name `xml:"CreateKeyPairResponse"`
	Xmlns          string   `xml:"xmlns,attr"`
	RequestID      string   `xml:"requestId"`
	KeyName        string   `xml:"keyName"`
	KeyFingerprint string   `xml:"keyFingerprint"`
	KeyMaterial    string   `xml:"keyMaterial"`
	KeyPairID      string   `xml:"keyPairId"`
}

// XMLDeleteKeyPairResponse is the XML response for DeleteKeyPair.
type XMLDeleteKeyPairResponse struct {
	XMLName   xml.Name `xml:"DeleteKeyPairResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	RequestID string   `xml:"requestId"`
	Return    bool     `xml:"return"`
}

// XMLDescribeKeyPairsResponse is the XML response for DescribeKeyPairs.
type XMLDescribeKeyPairsResponse struct {
	XMLName   xml.Name      `xml:"DescribeKeyPairsResponse"`
	Xmlns     string        `xml:"xmlns,attr"`
	RequestID string        `xml:"requestId"`
	KeySet    XMLKeyPairSet `xml:"keySet"`
}

// XMLKeyPairSet contains a list of key pairs.
type XMLKeyPairSet struct {
	Items []XMLKeyPairInfo `xml:"item"`
}

// XMLKeyPairInfo represents a key pair in XML format.
type XMLKeyPairInfo struct {
	KeyName        string `xml:"keyName"`
	KeyFingerprint string `xml:"keyFingerprint"`
	KeyPairID      string `xml:"keyPairId"`
}

// XMLErrorResponse is the XML error response for EC2.
type XMLErrorResponse struct {
	XMLName   xml.Name  `xml:"Response"`
	Errors    XMLErrors `xml:"Errors"`
	RequestID string    `xml:"RequestID"`
}

// XMLErrors contains a list of errors.
type XMLErrors struct {
	Error XMLError `xml:"Error"`
}

// XMLError represents an error in XML format.
type XMLError struct {
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// Error represents an EC2 error.
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

// VPC Domain Types

// Vpc represents a VPC.
type Vpc struct {
	VpcID              string
	CidrBlock          string
	State              string
	IsDefault          bool
	InstanceTenancy    string
	EnableDNSHostnames bool
	EnableDNSSupport   bool
	Tags               []Tag
}

// VpcAttributeUpdates carries optional VPC attribute changes. nil pointers
// mean "leave as-is" since AWS modifies one attribute per call.
type VpcAttributeUpdates struct {
	EnableDNSHostnames *bool
	EnableDNSSupport   *bool
}

// SubnetAttributeUpdates is the subnet equivalent.
type SubnetAttributeUpdates struct {
	MapPublicIPOnLaunch         *bool
	AssignIPv6AddressOnCreation *bool
}

// Subnet represents a subnet.
type Subnet struct {
	SubnetID                string
	VpcID                   string
	CidrBlock               string
	AvailabilityZone        string
	AvailableIPAddressCount int
	State                   string
	MapPublicIPOnLaunch     bool
	Tags                    []Tag
}

// InternetGateway represents an internet gateway.
type InternetGateway struct {
	InternetGatewayID string
	Attachments       []InternetGatewayAttachment
	Tags              []Tag
}

// InternetGatewayAttachment represents an attachment of an internet gateway to a VPC.
type InternetGatewayAttachment struct {
	VpcID string
	State string
}

// RouteTable represents a route table.
type RouteTable struct {
	RouteTableID string
	VpcID        string
	Routes       []Route
	Associations []RouteTableAssociation
	Tags         []Tag
}

// Route represents a route in a route table.
type Route struct {
	DestinationCidrBlock string
	GatewayID            string
	NatGatewayID         string
	State                string
	Origin               string
}

// RouteTableAssociation represents an association between a route table and a subnet.
type RouteTableAssociation struct {
	RouteTableAssociationID string
	RouteTableID            string
	SubnetID                string
	Main                    bool
}

// NatGateway represents a NAT gateway.
type NatGateway struct {
	NatGatewayID     string
	SubnetID         string
	VpcID            string
	State            string
	ConnectivityType string
	AllocationID     string
	Tags             []Tag
}

// VPC Request Types

// CreateVpcRequest represents a CreateVpc request.
type CreateVpcRequest struct {
	CidrBlock       string `json:"CidrBlock"`
	InstanceTenancy string `json:"InstanceTenancy,omitempty"`
}

// DeleteVpcRequest represents a DeleteVpc request.
type DeleteVpcRequest struct {
	VpcID string `json:"VpcId"`
}

// DescribeVpcsRequest represents a DescribeVpcs request.
type DescribeVpcsRequest struct {
	VpcIDs []string `json:"VpcIds,omitempty"`
}

// CreateSubnetRequest represents a CreateSubnet request.
type CreateSubnetRequest struct {
	VpcID            string `json:"VpcId"`
	CidrBlock        string `json:"CidrBlock"`
	AvailabilityZone string `json:"AvailabilityZone,omitempty"`
}

// DeleteSubnetRequest represents a DeleteSubnet request.
type DeleteSubnetRequest struct {
	SubnetID string `json:"SubnetId"`
}

// DescribeSubnetsRequest represents a DescribeSubnets request.
type DescribeSubnetsRequest struct {
	SubnetIDs []string `json:"SubnetIds,omitempty"`
}

// CreateInternetGatewayRequest represents a CreateInternetGateway request.
type CreateInternetGatewayRequest struct{}

// AttachInternetGatewayRequest represents an AttachInternetGateway request.
type AttachInternetGatewayRequest struct {
	InternetGatewayID string `json:"InternetGatewayId"`
	VpcID             string `json:"VpcId"`
}

// CreateRouteTableRequest represents a CreateRouteTable request.
type CreateRouteTableRequest struct {
	VpcID string `json:"VpcId"`
}

// CreateRouteRequest represents a CreateRoute request.
type CreateRouteRequest struct {
	RouteTableID         string `json:"RouteTableId"`
	DestinationCidrBlock string `json:"DestinationCidrBlock"`
	GatewayID            string `json:"GatewayId,omitempty"`
	NatGatewayID         string `json:"NatGatewayId,omitempty"`
}

// AssociateRouteTableRequest represents an AssociateRouteTable request.
type AssociateRouteTableRequest struct {
	RouteTableID string `json:"RouteTableId"`
	SubnetID     string `json:"SubnetId"`
}

// CreateNatGatewayRequest represents a CreateNatGateway request.
type CreateNatGatewayRequest struct {
	SubnetID         string `json:"SubnetId"`
	AllocationID     string `json:"AllocationId,omitempty"`
	ConnectivityType string `json:"ConnectivityType,omitempty"`
}

// VPC XML Response Types

// XMLCreateVpcResponse is the XML response for CreateVpc.
type XMLCreateVpcResponse struct {
	XMLName   xml.Name `xml:"CreateVpcResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	RequestID string   `xml:"requestId"`
	Vpc       XMLVpc   `xml:"vpc"`
}

// XMLVpc represents a VPC in XML format.
type XMLVpc struct {
	VpcID           string    `xml:"vpcId"`
	CidrBlock       string    `xml:"cidrBlock"`
	State           string    `xml:"state"`
	IsDefault       bool      `xml:"isDefault"`
	InstanceTenancy string    `xml:"instanceTenancy"`
	TagSet          XMLTagSet `xml:"tagSet"`
}

// XMLTagSet contains a list of tags.
type XMLTagSet struct {
	Items []XMLTag `xml:"item"`
}

// XMLTag represents a tag in XML format.
type XMLTag struct {
	Key   string `xml:"key"`
	Value string `xml:"value"`
}

// XMLDeleteVpcResponse is the XML response for DeleteVpc.
type XMLDeleteVpcResponse struct {
	XMLName   xml.Name `xml:"DeleteVpcResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	RequestID string   `xml:"requestId"`
	Return    bool     `xml:"return"`
}

// XMLDescribeVpcsResponse is the XML response for DescribeVpcs.
type XMLDescribeVpcsResponse struct {
	XMLName   xml.Name  `xml:"DescribeVpcsResponse"`
	Xmlns     string    `xml:"xmlns,attr"`
	RequestID string    `xml:"requestId"`
	VpcSet    XMLVpcSet `xml:"vpcSet"`
}

// XMLVpcSet contains a list of VPCs.
type XMLVpcSet struct {
	Items []XMLVpc `xml:"item"`
}

// XMLCreateSubnetResponse is the XML response for CreateSubnet.
type XMLCreateSubnetResponse struct {
	XMLName   xml.Name  `xml:"CreateSubnetResponse"`
	Xmlns     string    `xml:"xmlns,attr"`
	RequestID string    `xml:"requestId"`
	Subnet    XMLSubnet `xml:"subnet"`
}

// XMLSubnet represents a subnet in XML format.
type XMLSubnet struct {
	SubnetID                string    `xml:"subnetId"`
	VpcID                   string    `xml:"vpcId"`
	CidrBlock               string    `xml:"cidrBlock"`
	AvailabilityZone        string    `xml:"availabilityZone"`
	AvailableIPAddressCount int       `xml:"availableIpAddressCount"`
	State                   string    `xml:"state"`
	MapPublicIPOnLaunch     bool      `xml:"mapPublicIpOnLaunch"`
	TagSet                  XMLTagSet `xml:"tagSet"`
}

// XMLDeleteSubnetResponse is the XML response for DeleteSubnet.
type XMLDeleteSubnetResponse struct {
	XMLName   xml.Name `xml:"DeleteSubnetResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	RequestID string   `xml:"requestId"`
	Return    bool     `xml:"return"`
}

// XMLDescribeSubnetsResponse is the XML response for DescribeSubnets.
type XMLDescribeSubnetsResponse struct {
	XMLName   xml.Name     `xml:"DescribeSubnetsResponse"`
	Xmlns     string       `xml:"xmlns,attr"`
	RequestID string       `xml:"requestId"`
	SubnetSet XMLSubnetSet `xml:"subnetSet"`
}

// XMLSubnetSet contains a list of subnets.
type XMLSubnetSet struct {
	Items []XMLSubnet `xml:"item"`
}

// XMLCreateInternetGatewayResponse is the XML response for CreateInternetGateway.
type XMLCreateInternetGatewayResponse struct {
	XMLName         xml.Name           `xml:"CreateInternetGatewayResponse"`
	Xmlns           string             `xml:"xmlns,attr"`
	RequestID       string             `xml:"requestId"`
	InternetGateway XMLInternetGateway `xml:"internetGateway"`
}

// XMLInternetGateway represents an internet gateway in XML format.
type XMLInternetGateway struct {
	InternetGatewayID string                          `xml:"internetGatewayId"`
	AttachmentSet     XMLInternetGatewayAttachmentSet `xml:"attachmentSet"`
	TagSet            XMLTagSet                       `xml:"tagSet"`
}

// XMLInternetGatewayAttachmentSet contains a list of internet gateway attachments.
type XMLInternetGatewayAttachmentSet struct {
	Items []XMLInternetGatewayAttachment `xml:"item"`
}

// XMLInternetGatewayAttachment represents an internet gateway attachment in XML format.
type XMLInternetGatewayAttachment struct {
	VpcID string `xml:"vpcId"`
	State string `xml:"state"`
}

// XMLAttachInternetGatewayResponse is the XML response for AttachInternetGateway.
type XMLAttachInternetGatewayResponse struct {
	XMLName   xml.Name `xml:"AttachInternetGatewayResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	RequestID string   `xml:"requestId"`
	Return    bool     `xml:"return"`
}

// XMLCreateRouteTableResponse is the XML response for CreateRouteTable.
type XMLCreateRouteTableResponse struct {
	XMLName    xml.Name      `xml:"CreateRouteTableResponse"`
	Xmlns      string        `xml:"xmlns,attr"`
	RequestID  string        `xml:"requestId"`
	RouteTable XMLRouteTable `xml:"routeTable"`
}

// XMLRouteTable represents a route table in XML format.
type XMLRouteTable struct {
	RouteTableID   string                      `xml:"routeTableId"`
	VpcID          string                      `xml:"vpcId"`
	RouteSet       XMLRouteSet                 `xml:"routeSet"`
	AssociationSet XMLRouteTableAssociationSet `xml:"associationSet"`
	TagSet         XMLTagSet                   `xml:"tagSet"`
}

// XMLRouteSet contains a list of routes.
type XMLRouteSet struct {
	Items []XMLRoute `xml:"item"`
}

// XMLRoute represents a route in XML format.
type XMLRoute struct {
	DestinationCidrBlock string `xml:"destinationCidrBlock"`
	GatewayID            string `xml:"gatewayId,omitempty"`
	NatGatewayID         string `xml:"natGatewayId,omitempty"`
	State                string `xml:"state"`
	Origin               string `xml:"origin"`
}

// XMLRouteTableAssociationSet contains a list of route table associations.
type XMLRouteTableAssociationSet struct {
	Items []XMLRouteTableAssociation `xml:"item"`
}

// XMLRouteTableAssociation represents a route table association in XML format.
type XMLRouteTableAssociation struct {
	RouteTableAssociationID string `xml:"routeTableAssociationId"`
	RouteTableID            string `xml:"routeTableId"`
	SubnetID                string `xml:"subnetId,omitempty"`
	Main                    bool   `xml:"main"`
}

// XMLCreateRouteResponse is the XML response for CreateRoute.
type XMLCreateRouteResponse struct {
	XMLName   xml.Name `xml:"CreateRouteResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	RequestID string   `xml:"requestId"`
	Return    bool     `xml:"return"`
}

// XMLAssociateRouteTableResponse is the XML response for AssociateRouteTable.
type XMLAssociateRouteTableResponse struct {
	XMLName       xml.Name `xml:"AssociateRouteTableResponse"`
	Xmlns         string   `xml:"xmlns,attr"`
	RequestID     string   `xml:"requestId"`
	AssociationID string   `xml:"associationId"`
}

// XMLCreateNatGatewayResponse is the XML response for CreateNatGateway.
type XMLCreateNatGatewayResponse struct {
	XMLName    xml.Name      `xml:"CreateNatGatewayResponse"`
	Xmlns      string        `xml:"xmlns,attr"`
	RequestID  string        `xml:"requestId"`
	NatGateway XMLNatGateway `xml:"natGateway"`
}

// XMLNatGateway represents a NAT gateway in XML format.
type XMLNatGateway struct {
	NatGatewayID     string    `xml:"natGatewayId"`
	SubnetID         string    `xml:"subnetId"`
	VpcID            string    `xml:"vpcId"`
	State            string    `xml:"state"`
	ConnectivityType string    `xml:"connectivityType"`
	TagSet           XMLTagSet `xml:"tagSet"`
}

// DescribeInternetGatewaysRequest represents a DescribeInternetGateways request.
type DescribeInternetGatewaysRequest struct {
	InternetGatewayIDs []string `json:"InternetGatewayIds,omitempty"`
}

// DescribeRouteTablesRequest represents a DescribeRouteTables request.
type DescribeRouteTablesRequest struct {
	RouteTableIDs []string `json:"RouteTableIds,omitempty"`
}

// DescribeNatGatewaysRequest represents a DescribeNatGateways request.
type DescribeNatGatewaysRequest struct {
	NatGatewayIDs []string `json:"NatGatewayIds,omitempty"`
}

// XMLDescribeInternetGatewaysResponse is the XML response for DescribeInternetGateways.
type XMLDescribeInternetGatewaysResponse struct {
	XMLName            xml.Name              `xml:"DescribeInternetGatewaysResponse"`
	Xmlns              string                `xml:"xmlns,attr"`
	RequestID          string                `xml:"requestId"`
	InternetGatewaySet XMLInternetGatewaySet `xml:"internetGatewaySet"`
}

// XMLInternetGatewaySet contains a list of internet gateways.
type XMLInternetGatewaySet struct {
	Items []XMLInternetGateway `xml:"item"`
}

// XMLDescribeRouteTablesResponse is the XML response for DescribeRouteTables.
type XMLDescribeRouteTablesResponse struct {
	XMLName       xml.Name         `xml:"DescribeRouteTablesResponse"`
	Xmlns         string           `xml:"xmlns,attr"`
	RequestID     string           `xml:"requestId"`
	RouteTableSet XMLRouteTableSet `xml:"routeTableSet"`
}

// XMLRouteTableSet contains a list of route tables.
type XMLRouteTableSet struct {
	Items []XMLRouteTable `xml:"item"`
}

// XMLDescribeNatGatewaysResponse is the XML response for DescribeNatGateways.
type XMLDescribeNatGatewaysResponse struct {
	XMLName       xml.Name         `xml:"DescribeNatGatewaysResponse"`
	Xmlns         string           `xml:"xmlns,attr"`
	RequestID     string           `xml:"requestId"`
	NatGatewaySet XMLNatGatewaySet `xml:"natGatewaySet"`
}

// XMLNatGatewaySet contains a list of NAT gateways.
type XMLNatGatewaySet struct {
	Items []XMLNatGateway `xml:"item"`
}

// TagDescription represents a tag association with a resource.
type TagDescription struct {
	ResourceID   string
	ResourceType string
	Key          string
	Value        string
}

// CreateTagsRequest represents a CreateTags request.
type CreateTagsRequest struct {
	Resources []string
	Tags      []Tag
}

// DeleteTagsRequest represents a DeleteTags request.
type DeleteTagsRequest struct {
	Resources []string
	// Tags lists the tags to remove. An empty slice removes all tags from the
	// listed resources. A tag with an empty Value removes the tag for any value.
	Tags []Tag
}

// DescribeTagsRequest represents a DescribeTags request.
type DescribeTagsRequest struct {
	// Filters supports the standard EC2 tag filters: resource-id, resource-type,
	// key, value, tag:<key>. Only resource-id and key are honored today; the rest
	// are accepted but ignored.
	Filters map[string][]string
}

// XMLCreateTagsResponse is the XML response for CreateTags.
type XMLCreateTagsResponse struct {
	XMLName   xml.Name `xml:"CreateTagsResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	RequestID string   `xml:"requestId"`
	Return    bool     `xml:"return"`
}

// XMLDeleteTagsResponse is the XML response for DeleteTags.
type XMLDeleteTagsResponse struct {
	XMLName   xml.Name `xml:"DeleteTagsResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	RequestID string   `xml:"requestId"`
	Return    bool     `xml:"return"`
}

// XMLDescribeTagsResponse is the XML response for DescribeTags.
type XMLDescribeTagsResponse struct {
	XMLName   xml.Name             `xml:"DescribeTagsResponse"`
	Xmlns     string               `xml:"xmlns,attr"`
	RequestID string               `xml:"requestId"`
	TagSet    XMLTagDescriptionSet `xml:"tagSet"`
}

// XMLTagDescriptionSet contains a list of tag descriptions.
type XMLTagDescriptionSet struct {
	Items []XMLTagDescription `xml:"item"`
}

// XMLTagDescription represents a tag description in XML format.
type XMLTagDescription struct {
	ResourceID   string `xml:"resourceId"`
	ResourceType string `xml:"resourceType"`
	Key          string `xml:"key"`
	Value        string `xml:"value"`
}

// DeleteInternetGatewayRequest represents a DeleteInternetGateway request.
type DeleteInternetGatewayRequest struct {
	InternetGatewayID string `json:"InternetGatewayId"`
}

// XMLDetachInternetGatewayResponse is the XML response for DetachInternetGateway.
type XMLDetachInternetGatewayResponse struct {
	XMLName   xml.Name `xml:"DetachInternetGatewayResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	RequestID string   `xml:"requestId"`
	Return    bool     `xml:"return"`
}

// XMLDeleteInternetGatewayResponse is the XML response for DeleteInternetGateway.
type XMLDeleteInternetGatewayResponse struct {
	XMLName   xml.Name `xml:"DeleteInternetGatewayResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	RequestID string   `xml:"requestId"`
	Return    bool     `xml:"return"`
}

// XMLDescribeNetworkInterfacesResponse is the XML response for DescribeNetworkInterfaces.
type XMLDescribeNetworkInterfacesResponse struct {
	XMLName             xml.Name               `xml:"DescribeNetworkInterfacesResponse"`
	Xmlns               string                 `xml:"xmlns,attr"`
	RequestID           string                 `xml:"requestId"`
	NetworkInterfaceSet XMLNetworkInterfaceSet `xml:"networkInterfaceSet"`
}

// XMLNetworkInterfaceSet wraps the network interface members.
type XMLNetworkInterfaceSet struct {
	Items []XMLNetworkInterface `xml:"item"`
}

// XMLNetworkInterface is a single network interface description.
type XMLNetworkInterface struct {
	NetworkInterfaceID string `xml:"networkInterfaceId"`
	VpcID              string `xml:"vpcId,omitempty"`
	SubnetID           string `xml:"subnetId,omitempty"`
	Status             string `xml:"status,omitempty"`
}
