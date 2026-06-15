package globalaccelerator

import (
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// AcceleratorStatus represents the status of an accelerator.
type AcceleratorStatus string

// Accelerator statuses.
const (
	AcceleratorStatusDeployed   AcceleratorStatus = "DEPLOYED"
	AcceleratorStatusInProgress AcceleratorStatus = "IN_PROGRESS"
)

// IPAddressType represents the IP address type.
type IPAddressType string

// IP address types.
const (
	IPAddressTypeIPv4      IPAddressType = "IPV4"
	IPAddressTypeDualStack IPAddressType = "DUAL_STACK"
)

// Protocol represents the protocol type.
type Protocol string

// Protocol types.
const (
	ProtocolTCP Protocol = "TCP"
	ProtocolUDP Protocol = "UDP"
)

// ClientAffinity represents client affinity settings.
type ClientAffinity string

// Client affinity types.
const (
	ClientAffinityNone     ClientAffinity = "NONE"
	ClientAffinitySourceIP ClientAffinity = "SOURCE_IP"
)

// HealthState represents the health state of an endpoint.
type HealthState string

// Health states.
const (
	HealthStateInitial   HealthState = "INITIAL"
	HealthStateHealthy   HealthState = "HEALTHY"
	HealthStateUnhealthy HealthState = "UNHEALTHY"
)

// HealthCheckProtocol represents the health check protocol.
type HealthCheckProtocol string

// Health check protocols.
const (
	HealthCheckProtocolTCP   HealthCheckProtocol = "TCP"
	HealthCheckProtocolHTTP  HealthCheckProtocol = "HTTP"
	HealthCheckProtocolHTTPS HealthCheckProtocol = "HTTPS"
)

// Accelerator represents a Global Accelerator accelerator.
type Accelerator struct {
	AcceleratorArn string
	Name           string
	IPAddressType  IPAddressType
	Enabled        bool
	IPSets         []IPSet
	DNSName        string
	Status         AcceleratorStatus
	CreatedTime    time.Time
	LastModified   time.Time
	DualStackDNS   string
	Events         []AcceleratorEvent
}

// IPSet represents an IP set.
type IPSet struct {
	IPFamily        string
	IPAddresses     []string
	IPAddressFamily string
}

// AcceleratorEvent represents an event for an accelerator.
type AcceleratorEvent struct {
	Message   string
	Timestamp time.Time
}

// AcceleratorAttributes represents accelerator attributes.
type AcceleratorAttributes struct {
	FlowLogsEnabled  bool
	FlowLogsS3Bucket string
	FlowLogsS3Prefix string
}

// Listener represents a listener.
type Listener struct {
	ListenerArn    string
	AcceleratorArn string
	PortRanges     []PortRange
	Protocol       Protocol
	ClientAffinity ClientAffinity
}

// PortRange represents a port range.
type PortRange struct {
	FromPort int32
	ToPort   int32
}

// EndpointGroup represents an endpoint group.
type EndpointGroup struct {
	EndpointGroupArn           string
	ListenerArn                string
	EndpointGroupRegion        string
	EndpointDescriptions       []EndpointDescription
	TrafficDialPercentage      float64
	HealthCheckPort            *int32
	HealthCheckProtocol        HealthCheckProtocol
	HealthCheckPath            string
	HealthCheckIntervalSeconds int32
	ThresholdCount             int32
	PortOverrides              []PortOverride
}

// EndpointDescription represents an endpoint.
type EndpointDescription struct {
	EndpointID                  string
	Weight                      int32
	HealthState                 HealthState
	HealthReason                string
	ClientIPPreservationEnabled bool
}

// PortOverride represents a port override.
type PortOverride struct {
	ListenerPort int32
	EndpointPort int32
}

// CreateAcceleratorRequest is the request for CreateAccelerator.
type CreateAcceleratorRequest struct {
	Name             string   `json:"Name"`
	IPAddressType    string   `json:"IpAddressType,omitempty"`
	IPAddresses      []string `json:"IpAddresses,omitempty"`
	Enabled          *bool    `json:"Enabled,omitempty"`
	IdempotencyToken string   `json:"IdempotencyToken"`
	Tags             []Tag    `json:"Tags,omitempty"`
}

// Tag represents a tag.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// CreateAcceleratorResponse is the response for CreateAccelerator.
type CreateAcceleratorResponse struct {
	Accelerator *AcceleratorOutput `json:"Accelerator"`
}

// AcceleratorOutput represents an accelerator in API responses.
type AcceleratorOutput struct {
	AcceleratorArn string        `json:"AcceleratorArn"`
	Name           string        `json:"Name"`
	IPAddressType  string        `json:"IpAddressType"`
	Enabled        bool          `json:"Enabled"`
	IPSets         []IPSetOutput `json:"IpSets,omitempty"`
	DNSName        string        `json:"DnsName"`
	Status         string        `json:"Status"`
	CreatedTime    float64       `json:"CreatedTime"`
	LastModified   float64       `json:"LastModifiedTime"`
	DualStackDNS   string        `json:"DualStackDnsName,omitempty"`
	Events         []EventOutput `json:"Events,omitempty"`
}

// IPSetOutput represents an IP set in API responses.
type IPSetOutput struct {
	IPFamily        string   `json:"IpFamily,omitempty"`
	IPAddresses     []string `json:"IpAddresses"`
	IPAddressFamily string   `json:"IpAddressFamily,omitempty"`
}

// EventOutput represents an event in API responses.
type EventOutput struct {
	Message   string  `json:"Message"`
	Timestamp float64 `json:"Timestamp"`
}

// DescribeAcceleratorRequest is the request for DescribeAccelerator.
type DescribeAcceleratorRequest struct {
	AcceleratorArn string `json:"AcceleratorArn"`
}

// DescribeAcceleratorResponse is the response for DescribeAccelerator.
type DescribeAcceleratorResponse struct {
	Accelerator *AcceleratorOutput `json:"Accelerator"`
}

// ListAcceleratorsRequest is the request for ListAccelerators.
type ListAcceleratorsRequest struct {
	MaxResults int32  `json:"MaxResults,omitempty"`
	NextToken  string `json:"NextToken,omitempty"`
}

// ListAcceleratorsResponse is the response for ListAccelerators.
type ListAcceleratorsResponse struct {
	Accelerators []AcceleratorOutput `json:"Accelerators"`
	NextToken    string              `json:"NextToken,omitempty"`
}

// UpdateAcceleratorRequest is the request for UpdateAccelerator.
type UpdateAcceleratorRequest struct {
	AcceleratorArn string `json:"AcceleratorArn"`
	Name           string `json:"Name,omitempty"`
	IPAddressType  string `json:"IpAddressType,omitempty"`
	Enabled        *bool  `json:"Enabled,omitempty"`
}

// UpdateAcceleratorResponse is the response for UpdateAccelerator.
type UpdateAcceleratorResponse struct {
	Accelerator *AcceleratorOutput `json:"Accelerator"`
}

// DeleteAcceleratorRequest is the request for DeleteAccelerator.
type DeleteAcceleratorRequest struct {
	AcceleratorArn string `json:"AcceleratorArn"`
}

// DeleteAcceleratorResponse is the response for DeleteAccelerator.
type DeleteAcceleratorResponse struct{}

// CreateListenerRequest is the request for CreateListener.
type CreateListenerRequest struct {
	AcceleratorArn   string           `json:"AcceleratorArn"`
	PortRanges       []PortRangeInput `json:"PortRanges"`
	Protocol         string           `json:"Protocol"`
	ClientAffinity   string           `json:"ClientAffinity,omitempty"`
	IdempotencyToken string           `json:"IdempotencyToken"`
}

// PortRangeInput represents a port range in requests.
type PortRangeInput struct {
	FromPort int32 `json:"FromPort"`
	ToPort   int32 `json:"ToPort"`
}

// CreateListenerResponse is the response for CreateListener.
type CreateListenerResponse struct {
	Listener *ListenerOutput `json:"Listener"`
}

// ListenerOutput represents a listener in API responses.
type ListenerOutput struct {
	ListenerArn    string            `json:"ListenerArn"`
	PortRanges     []PortRangeOutput `json:"PortRanges"`
	Protocol       string            `json:"Protocol"`
	ClientAffinity string            `json:"ClientAffinity"`
}

// PortRangeOutput represents a port range in API responses.
type PortRangeOutput struct {
	FromPort int32 `json:"FromPort"`
	ToPort   int32 `json:"ToPort"`
}

// DescribeListenerRequest is the request for DescribeListener.
type DescribeListenerRequest struct {
	ListenerArn string `json:"ListenerArn"`
}

// DescribeListenerResponse is the response for DescribeListener.
type DescribeListenerResponse struct {
	Listener *ListenerOutput `json:"Listener"`
}

// ListListenersRequest is the request for ListListeners.
type ListListenersRequest struct {
	AcceleratorArn string `json:"AcceleratorArn"`
	MaxResults     int32  `json:"MaxResults,omitempty"`
	NextToken      string `json:"NextToken,omitempty"`
}

// ListListenersResponse is the response for ListListeners.
type ListListenersResponse struct {
	Listeners []ListenerOutput `json:"Listeners"`
	NextToken string           `json:"NextToken,omitempty"`
}

// UpdateListenerRequest is the request for UpdateListener.
type UpdateListenerRequest struct {
	ListenerArn    string           `json:"ListenerArn"`
	PortRanges     []PortRangeInput `json:"PortRanges,omitempty"`
	Protocol       string           `json:"Protocol,omitempty"`
	ClientAffinity string           `json:"ClientAffinity,omitempty"`
}

// UpdateListenerResponse is the response for UpdateListener.
type UpdateListenerResponse struct {
	Listener *ListenerOutput `json:"Listener"`
}

// DeleteListenerRequest is the request for DeleteListener.
type DeleteListenerRequest struct {
	ListenerArn string `json:"ListenerArn"`
}

// DeleteListenerResponse is the response for DeleteListener.
type DeleteListenerResponse struct{}

// CreateEndpointGroupRequest is the request for CreateEndpointGroup.
type CreateEndpointGroupRequest struct {
	ListenerArn                string                `json:"ListenerArn"`
	EndpointGroupRegion        string                `json:"EndpointGroupRegion"`
	EndpointConfigurations     []EndpointConfigInput `json:"EndpointConfigurations,omitempty"`
	TrafficDialPercentage      *float64              `json:"TrafficDialPercentage,omitempty"`
	HealthCheckPort            *int32                `json:"HealthCheckPort,omitempty"`
	HealthCheckProtocol        string                `json:"HealthCheckProtocol,omitempty"`
	HealthCheckPath            string                `json:"HealthCheckPath,omitempty"`
	HealthCheckIntervalSeconds *int32                `json:"HealthCheckIntervalSeconds,omitempty"`
	ThresholdCount             *int32                `json:"ThresholdCount,omitempty"`
	IdempotencyToken           string                `json:"IdempotencyToken"`
	PortOverrides              []PortOverrideInput   `json:"PortOverrides,omitempty"`
}

// EndpointConfigInput represents an endpoint configuration in requests.
type EndpointConfigInput struct {
	EndpointID                  string `json:"EndpointId"`
	Weight                      int32  `json:"Weight,omitempty"`
	ClientIPPreservationEnabled *bool  `json:"ClientIPPreservationEnabled,omitempty"`
}

// PortOverrideInput represents a port override in requests.
type PortOverrideInput struct {
	ListenerPort int32 `json:"ListenerPort"`
	EndpointPort int32 `json:"EndpointPort"`
}

// CreateEndpointGroupResponse is the response for CreateEndpointGroup.
type CreateEndpointGroupResponse struct {
	EndpointGroup *EndpointGroupOutput `json:"EndpointGroup"`
}

// EndpointGroupOutput represents an endpoint group in API responses.
type EndpointGroupOutput struct {
	EndpointGroupArn           string                      `json:"EndpointGroupArn"`
	EndpointGroupRegion        string                      `json:"EndpointGroupRegion"`
	EndpointDescriptions       []EndpointDescriptionOutput `json:"EndpointDescriptions,omitempty"`
	TrafficDialPercentage      float64                     `json:"TrafficDialPercentage"`
	HealthCheckPort            *int32                      `json:"HealthCheckPort,omitempty"`
	HealthCheckProtocol        string                      `json:"HealthCheckProtocol"`
	HealthCheckPath            string                      `json:"HealthCheckPath,omitempty"`
	HealthCheckIntervalSeconds int32                       `json:"HealthCheckIntervalSeconds"`
	ThresholdCount             int32                       `json:"ThresholdCount"`
	PortOverrides              []PortOverrideOutput        `json:"PortOverrides,omitempty"`
}

// EndpointDescriptionOutput represents an endpoint in API responses.
type EndpointDescriptionOutput struct {
	EndpointID                  string `json:"EndpointId"`
	Weight                      int32  `json:"Weight"`
	HealthState                 string `json:"HealthState"`
	HealthReason                string `json:"HealthReason,omitempty"`
	ClientIPPreservationEnabled bool   `json:"ClientIPPreservationEnabled"`
}

// PortOverrideOutput represents a port override in API responses.
type PortOverrideOutput struct {
	ListenerPort int32 `json:"ListenerPort"`
	EndpointPort int32 `json:"EndpointPort"`
}

// DescribeEndpointGroupRequest is the request for DescribeEndpointGroup.
type DescribeEndpointGroupRequest struct {
	EndpointGroupArn string `json:"EndpointGroupArn"`
}

// DescribeEndpointGroupResponse is the response for DescribeEndpointGroup.
type DescribeEndpointGroupResponse struct {
	EndpointGroup *EndpointGroupOutput `json:"EndpointGroup"`
}

// ListEndpointGroupsRequest is the request for ListEndpointGroups.
type ListEndpointGroupsRequest struct {
	ListenerArn string `json:"ListenerArn"`
	MaxResults  int32  `json:"MaxResults,omitempty"`
	NextToken   string `json:"NextToken,omitempty"`
}

// ListEndpointGroupsResponse is the response for ListEndpointGroups.
type ListEndpointGroupsResponse struct {
	EndpointGroups []EndpointGroupOutput `json:"EndpointGroups"`
	NextToken      string                `json:"NextToken,omitempty"`
}

// UpdateEndpointGroupRequest is the request for UpdateEndpointGroup.
type UpdateEndpointGroupRequest struct {
	EndpointGroupArn           string                `json:"EndpointGroupArn"`
	EndpointConfigurations     []EndpointConfigInput `json:"EndpointConfigurations,omitempty"`
	TrafficDialPercentage      *float64              `json:"TrafficDialPercentage,omitempty"`
	HealthCheckPort            *int32                `json:"HealthCheckPort,omitempty"`
	HealthCheckProtocol        string                `json:"HealthCheckProtocol,omitempty"`
	HealthCheckPath            string                `json:"HealthCheckPath,omitempty"`
	HealthCheckIntervalSeconds *int32                `json:"HealthCheckIntervalSeconds,omitempty"`
	ThresholdCount             *int32                `json:"ThresholdCount,omitempty"`
	PortOverrides              []PortOverrideInput   `json:"PortOverrides,omitempty"`
}

// UpdateEndpointGroupResponse is the response for UpdateEndpointGroup.
type UpdateEndpointGroupResponse struct {
	EndpointGroup *EndpointGroupOutput `json:"EndpointGroup"`
}

// DeleteEndpointGroupRequest is the request for DeleteEndpointGroup.
type DeleteEndpointGroupRequest struct {
	EndpointGroupArn string `json:"EndpointGroupArn"`
}

// DeleteEndpointGroupResponse is the response for DeleteEndpointGroup.
type DeleteEndpointGroupResponse struct{}

// ErrorResponse represents a Global Accelerator error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// ServiceError represents a Global Accelerator service error.
type ServiceError = service.CodedError
