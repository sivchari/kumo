// Package elbv2 provides ELB v2 service emulation for kumo.
package elbv2

import (
	"encoding/xml"
	"time"
)

const elbXMLNS = "http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/"

// LoadBalancer represents an ELB load balancer.
type LoadBalancer struct {
	LoadBalancerArn       string
	DNSName               string
	CanonicalHostedZoneID string
	CreatedTime           time.Time
	LoadBalancerName      string
	Scheme                string // internet-facing | internal
	VpcID                 string
	State                 LoadBalancerState
	Type                  string // application | network | gateway
	AvailabilityZones     []AvailabilityZone
	SecurityGroups        []string
	IPAddressType         string
	Attributes            map[string]string
}

// LoadBalancerState represents the state of a load balancer.
type LoadBalancerState struct {
	Code   string
	Reason string
}

// AvailabilityZone represents an availability zone.
type AvailabilityZone struct {
	ZoneName         string
	SubnetID         string
	LoadBalancerAddr []LoadBalancerAddress
}

// LoadBalancerAddress represents a load balancer address.
type LoadBalancerAddress struct {
	IPAddress    string
	AllocationID string
}

// TargetGroup represents an ELB target group.
type TargetGroup struct {
	TargetGroupArn             string
	TargetGroupName            string
	Protocol                   string
	Port                       int
	VpcID                      string
	HealthCheckEnabled         bool
	HealthCheckIntervalSeconds int
	HealthCheckPath            string
	HealthCheckPort            string
	HealthCheckProtocol        string
	HealthCheckTimeoutSeconds  int
	HealthyThresholdCount      int
	UnhealthyThresholdCount    int
	TargetType                 string // instance | ip | lambda | alb
	LoadBalancerArns           []string
	Attributes                 map[string]string
}

// Listener represents an ELB listener.
type Listener struct {
	ListenerArn     string
	LoadBalancerArn string
	Port            int
	Protocol        string
	DefaultActions  []Action
	Rules           []Rule
}

// Action represents a listener action. The legacy single-target form
// (Type=forward + TargetGroupArn) and the weighted form (Type=forward +
// ForwardConfig with multiple TargetGroupTuples) are both expressible:
// AWS clients send one or the other, never both, and Describe responses
// echo back whichever was supplied.
type Action struct {
	Type           string
	TargetGroupArn string
	Order          int
	// ForwardConfig is set when the action uses weighted target groups
	// (the canary / blue-green pattern). nil means "use TargetGroupArn".
	ForwardConfig *ForwardActionConfig
}

// ForwardActionConfig groups multiple target groups under one forward
// action with relative weights. Used by ALB canary / blue-green
// deployments — the listener splits traffic across TargetGroups by the
// supplied Weight ratio.
type ForwardActionConfig struct {
	TargetGroups     []TargetGroupTuple
	StickinessConfig *TargetGroupStickinessConfig
}

// TargetGroupTuple is one (target group, weight) pair inside a
// ForwardActionConfig. Weight is the relative share of traffic routed
// to TargetGroupArn; the AWS console expresses the canonical 100-total
// form but real ALB normalises across whatever weights are supplied.
type TargetGroupTuple struct {
	TargetGroupArn string
	Weight         int
}

// TargetGroupStickinessConfig controls whether requests for the same
// client are pinned to the same target group during a weighted forward.
// Without it, every request is independently weight-sampled.
type TargetGroupStickinessConfig struct {
	Enabled         bool
	DurationSeconds int
}

// Rule represents a listener rule for path/host-based routing.
type Rule struct {
	RuleArn    string
	Priority   string // AWS treats priority as string ("default" or 1-50000)
	Conditions []RuleCondition
	Actions    []Action
	IsDefault  bool
}

// RuleCondition represents one condition on a rule (host-header,
// path-pattern, http-header, etc.). Field + Values is the legacy form;
// modern SDKs may also send <Field>Config.Values.member.N. The handler
// accepts both and stores them in Field/Values.
type RuleCondition struct {
	Field  string
	Values []string
}

// Target represents a target in a target group.
type Target struct {
	ID               string
	Port             int
	AvailabilityZone string
}

// TargetDescription represents a target with its health status.
type TargetDescription struct {
	Target      Target
	HealthState string
}

// Request types.

// CreateLoadBalancerRequest represents a CreateLoadBalancer request.
type CreateLoadBalancerRequest struct {
	Name           string   `json:"Name"`
	Subnets        []string `json:"Subnets,omitempty"`
	SecurityGroups []string `json:"SecurityGroups,omitempty"`
	Scheme         string   `json:"Scheme,omitempty"`
	Type           string   `json:"Type,omitempty"`
	IPAddressType  string   `json:"IpAddressType,omitempty"`
}

// DeleteLoadBalancerRequest represents a DeleteLoadBalancer request.
type DeleteLoadBalancerRequest struct {
	LoadBalancerArn string `json:"LoadBalancerArn"`
}

// DescribeLoadBalancersRequest represents a DescribeLoadBalancers request.
type DescribeLoadBalancersRequest struct {
	LoadBalancerArns []string `json:"LoadBalancerArns,omitempty"`
	Names            []string `json:"Names,omitempty"`
}

// CreateTargetGroupRequest represents a CreateTargetGroup request.
type CreateTargetGroupRequest struct {
	Name                       string `json:"Name"`
	Protocol                   string `json:"Protocol,omitempty"`
	Port                       int    `json:"Port,omitempty"`
	VpcID                      string `json:"VpcId,omitempty"`
	HealthCheckProtocol        string `json:"HealthCheckProtocol,omitempty"`
	HealthCheckPort            string `json:"HealthCheckPort,omitempty"`
	HealthCheckEnabled         bool   `json:"HealthCheckEnabled,omitempty"`
	HealthCheckPath            string `json:"HealthCheckPath,omitempty"`
	HealthCheckIntervalSeconds int    `json:"HealthCheckIntervalSeconds,omitempty"`
	HealthCheckTimeoutSeconds  int    `json:"HealthCheckTimeoutSeconds,omitempty"`
	HealthyThresholdCount      int    `json:"HealthyThresholdCount,omitempty"`
	UnhealthyThresholdCount    int    `json:"UnhealthyThresholdCount,omitempty"`
	TargetType                 string `json:"TargetType,omitempty"`
}

// DeleteTargetGroupRequest represents a DeleteTargetGroup request.
type DeleteTargetGroupRequest struct {
	TargetGroupArn string `json:"TargetGroupArn"`
}

// DescribeTargetGroupsRequest represents a DescribeTargetGroups request.
type DescribeTargetGroupsRequest struct {
	TargetGroupArns []string `json:"TargetGroupArns,omitempty"`
	Names           []string `json:"Names,omitempty"`
	LoadBalancerArn string   `json:"LoadBalancerArn,omitempty"`
}

// RegisterTargetsRequest represents a RegisterTargets request.
type RegisterTargetsRequest struct {
	TargetGroupArn string   `json:"TargetGroupArn"`
	Targets        []Target `json:"Targets"`
}

// DeregisterTargetsRequest represents a DeregisterTargets request.
type DeregisterTargetsRequest struct {
	TargetGroupArn string   `json:"TargetGroupArn"`
	Targets        []Target `json:"Targets"`
}

// CreateListenerRequest represents a CreateListener request.
type CreateListenerRequest struct {
	LoadBalancerArn string   `json:"LoadBalancerArn"`
	Port            int      `json:"Port"`
	Protocol        string   `json:"Protocol"`
	DefaultActions  []Action `json:"DefaultActions"`
}

// DeleteListenerRequest represents a DeleteListener request.
type DeleteListenerRequest struct {
	ListenerArn string `json:"ListenerArn"`
}

// XML Response types.

// XMLCreateLoadBalancerResponse is the XML response for CreateLoadBalancer.
type XMLCreateLoadBalancerResponse struct {
	XMLName          xml.Name                    `xml:"CreateLoadBalancerResponse"`
	Xmlns            string                      `xml:"xmlns,attr"`
	Result           XMLCreateLoadBalancerResult `xml:"CreateLoadBalancerResult"`
	ResponseMetadata XMLResponseMetadata         `xml:"ResponseMetadata"`
}

// XMLCreateLoadBalancerResult contains the result of CreateLoadBalancer.
type XMLCreateLoadBalancerResult struct {
	LoadBalancers XMLLoadBalancers `xml:"LoadBalancers"`
}

// XMLDeleteLoadBalancerResponse is the XML response for DeleteLoadBalancer.
type XMLDeleteLoadBalancerResponse struct {
	XMLName          xml.Name                    `xml:"DeleteLoadBalancerResponse"`
	Xmlns            string                      `xml:"xmlns,attr"`
	Result           XMLDeleteLoadBalancerResult `xml:"DeleteLoadBalancerResult"`
	ResponseMetadata XMLResponseMetadata         `xml:"ResponseMetadata"`
}

// XMLDeleteLoadBalancerResult is an empty result for DeleteLoadBalancer.
type XMLDeleteLoadBalancerResult struct{}

// XMLDescribeLoadBalancersResponse is the XML response for DescribeLoadBalancers.
type XMLDescribeLoadBalancersResponse struct {
	XMLName          xml.Name                       `xml:"DescribeLoadBalancersResponse"`
	Xmlns            string                         `xml:"xmlns,attr"`
	Result           XMLDescribeLoadBalancersResult `xml:"DescribeLoadBalancersResult"`
	ResponseMetadata XMLResponseMetadata            `xml:"ResponseMetadata"`
}

// XMLDescribeLoadBalancersResult contains the result of DescribeLoadBalancers.
type XMLDescribeLoadBalancersResult struct {
	LoadBalancers XMLLoadBalancers `xml:"LoadBalancers"`
}

// XMLLoadBalancers contains a list of load balancers.
type XMLLoadBalancers struct {
	Members []XMLLoadBalancer `xml:"member"`
}

// XMLLoadBalancer represents a load balancer in XML format.
type XMLLoadBalancer struct {
	LoadBalancerArn       string               `xml:"LoadBalancerArn"`
	DNSName               string               `xml:"DNSName"`
	CanonicalHostedZoneID string               `xml:"CanonicalHostedZoneId"`
	CreatedTime           string               `xml:"CreatedTime"`
	LoadBalancerName      string               `xml:"LoadBalancerName"`
	Scheme                string               `xml:"Scheme"`
	VpcID                 string               `xml:"VpcId"`
	State                 XMLLoadBalancerState `xml:"State"`
	Type                  string               `xml:"Type"`
	AvailabilityZones     XMLAvailabilityZones `xml:"AvailabilityZones"`
	SecurityGroups        XMLSecurityGroups    `xml:"SecurityGroups"`
	IPAddressType         string               `xml:"IpAddressType"`
}

// XMLLoadBalancerState represents a load balancer state in XML format.
type XMLLoadBalancerState struct {
	Code   string `xml:"Code"`
	Reason string `xml:"Reason,omitempty"`
}

// XMLAvailabilityZones contains a list of availability zones.
type XMLAvailabilityZones struct {
	Members []XMLAvailabilityZone `xml:"member"`
}

// XMLAvailabilityZone represents an availability zone in XML format.
type XMLAvailabilityZone struct {
	ZoneName string `xml:"ZoneName"`
	SubnetID string `xml:"SubnetId"`
}

// XMLSecurityGroups contains a list of security groups.
type XMLSecurityGroups struct {
	Members []string `xml:"member"`
}

// XMLCreateTargetGroupResponse is the XML response for CreateTargetGroup.
type XMLCreateTargetGroupResponse struct {
	XMLName          xml.Name                   `xml:"CreateTargetGroupResponse"`
	Xmlns            string                     `xml:"xmlns,attr"`
	Result           XMLCreateTargetGroupResult `xml:"CreateTargetGroupResult"`
	ResponseMetadata XMLResponseMetadata        `xml:"ResponseMetadata"`
}

// XMLCreateTargetGroupResult contains the result of CreateTargetGroup.
type XMLCreateTargetGroupResult struct {
	TargetGroups XMLTargetGroups `xml:"TargetGroups"`
}

// XMLDeleteTargetGroupResponse is the XML response for DeleteTargetGroup.
type XMLDeleteTargetGroupResponse struct {
	XMLName          xml.Name                   `xml:"DeleteTargetGroupResponse"`
	Xmlns            string                     `xml:"xmlns,attr"`
	Result           XMLDeleteTargetGroupResult `xml:"DeleteTargetGroupResult"`
	ResponseMetadata XMLResponseMetadata        `xml:"ResponseMetadata"`
}

// XMLDeleteTargetGroupResult is an empty result for DeleteTargetGroup.
type XMLDeleteTargetGroupResult struct{}

// XMLDescribeTargetGroupsResponse is the XML response for DescribeTargetGroups.
type XMLDescribeTargetGroupsResponse struct {
	XMLName          xml.Name                      `xml:"DescribeTargetGroupsResponse"`
	Xmlns            string                        `xml:"xmlns,attr"`
	Result           XMLDescribeTargetGroupsResult `xml:"DescribeTargetGroupsResult"`
	ResponseMetadata XMLResponseMetadata           `xml:"ResponseMetadata"`
}

// XMLDescribeTargetGroupsResult contains the result of DescribeTargetGroups.
type XMLDescribeTargetGroupsResult struct {
	TargetGroups XMLTargetGroups `xml:"TargetGroups"`
}

// XMLTargetGroups contains a list of target groups.
type XMLTargetGroups struct {
	Members []XMLTargetGroup `xml:"member"`
}

// XMLTargetGroup represents a target group in XML format.
type XMLTargetGroup struct {
	TargetGroupArn             string              `xml:"TargetGroupArn"`
	TargetGroupName            string              `xml:"TargetGroupName"`
	Protocol                   string              `xml:"Protocol,omitempty"`
	Port                       int                 `xml:"Port,omitempty"`
	VpcID                      string              `xml:"VpcId,omitempty"`
	HealthCheckEnabled         bool                `xml:"HealthCheckEnabled"`
	HealthCheckIntervalSeconds int                 `xml:"HealthCheckIntervalSeconds"`
	HealthCheckPath            string              `xml:"HealthCheckPath,omitempty"`
	HealthCheckPort            string              `xml:"HealthCheckPort"`
	HealthCheckProtocol        string              `xml:"HealthCheckProtocol"`
	HealthCheckTimeoutSeconds  int                 `xml:"HealthCheckTimeoutSeconds"`
	HealthyThresholdCount      int                 `xml:"HealthyThresholdCount"`
	UnhealthyThresholdCount    int                 `xml:"UnhealthyThresholdCount"`
	TargetType                 string              `xml:"TargetType"`
	LoadBalancerArns           XMLLoadBalancerArns `xml:"LoadBalancerArns"`
}

// XMLLoadBalancerArns contains a list of load balancer ARNs.
type XMLLoadBalancerArns struct {
	Members []string `xml:"member"`
}

// XMLRegisterTargetsResponse is the XML response for RegisterTargets.
type XMLRegisterTargetsResponse struct {
	XMLName          xml.Name                 `xml:"RegisterTargetsResponse"`
	Xmlns            string                   `xml:"xmlns,attr"`
	Result           XMLRegisterTargetsResult `xml:"RegisterTargetsResult"`
	ResponseMetadata XMLResponseMetadata      `xml:"ResponseMetadata"`
}

// XMLRegisterTargetsResult is an empty result for RegisterTargets.
type XMLRegisterTargetsResult struct{}

// XMLDeregisterTargetsResponse is the XML response for DeregisterTargets.
type XMLDeregisterTargetsResponse struct {
	XMLName          xml.Name                   `xml:"DeregisterTargetsResponse"`
	Xmlns            string                     `xml:"xmlns,attr"`
	Result           XMLDeregisterTargetsResult `xml:"DeregisterTargetsResult"`
	ResponseMetadata XMLResponseMetadata        `xml:"ResponseMetadata"`
}

// XMLDeregisterTargetsResult is an empty result for DeregisterTargets.
type XMLDeregisterTargetsResult struct{}

// XMLCreateListenerResponse is the XML response for CreateListener.
type XMLCreateListenerResponse struct {
	XMLName          xml.Name                `xml:"CreateListenerResponse"`
	Xmlns            string                  `xml:"xmlns,attr"`
	Result           XMLCreateListenerResult `xml:"CreateListenerResult"`
	ResponseMetadata XMLResponseMetadata     `xml:"ResponseMetadata"`
}

// XMLCreateListenerResult contains the result of CreateListener.
type XMLCreateListenerResult struct {
	Listeners XMLListeners `xml:"Listeners"`
}

// XMLDeleteListenerResponse is the XML response for DeleteListener.
type XMLDeleteListenerResponse struct {
	XMLName          xml.Name                `xml:"DeleteListenerResponse"`
	Xmlns            string                  `xml:"xmlns,attr"`
	Result           XMLDeleteListenerResult `xml:"DeleteListenerResult"`
	ResponseMetadata XMLResponseMetadata     `xml:"ResponseMetadata"`
}

// XMLDeleteListenerResult is an empty result for DeleteListener.
type XMLDeleteListenerResult struct{}

// XMLListeners contains a list of listeners.
type XMLListeners struct {
	Members []XMLListener `xml:"member"`
}

// XMLListener represents a listener in XML format.
type XMLListener struct {
	ListenerArn     string     `xml:"ListenerArn"`
	LoadBalancerArn string     `xml:"LoadBalancerArn"`
	Port            int        `xml:"Port"`
	Protocol        string     `xml:"Protocol"`
	DefaultActions  XMLActions `xml:"DefaultActions"`
}

// XMLActions contains a list of actions.
type XMLActions struct {
	Members []XMLAction `xml:"member"`
}

// XMLAction represents an action in XML format. ForwardConfig is a
// pointer so it serialises out only when an explicit weighted forward
// was supplied; clients using the legacy single-target form (just
// TargetGroupArn) get a response that matches what they sent.
type XMLAction struct {
	Type           string                  `xml:"Type"`
	TargetGroupArn string                  `xml:"TargetGroupArn,omitempty"`
	Order          int                     `xml:"Order,omitempty"`
	ForwardConfig  *XMLForwardActionConfig `xml:"ForwardConfig,omitempty"`
}

// XMLForwardActionConfig is the XML wire shape for a multi-target-group
// forward (the canary / blue-green pattern). TargetGroups is required;
// TargetGroupStickinessConfig is optional.
type XMLForwardActionConfig struct {
	TargetGroups                XMLTargetGroupTuples            `xml:"TargetGroups"`
	TargetGroupStickinessConfig *XMLTargetGroupStickinessConfig `xml:"TargetGroupStickinessConfig,omitempty"`
}

// XMLTargetGroupTuples wraps the member-list shape AWS uses for arrays.
type XMLTargetGroupTuples struct {
	Members []XMLTargetGroupTuple `xml:"member"`
}

// XMLTargetGroupTuple is one (target group, weight) pair on the wire.
type XMLTargetGroupTuple struct {
	TargetGroupArn string `xml:"TargetGroupArn"`
	Weight         int    `xml:"Weight"`
}

// XMLTargetGroupStickinessConfig is the wire shape for the sticky-target
// option on a weighted forward. AWS surfaces DurationSeconds even when
// Enabled is false, but the value is only meaningful when Enabled=true.
type XMLTargetGroupStickinessConfig struct {
	Enabled         bool `xml:"Enabled"`
	DurationSeconds int  `xml:"DurationSeconds,omitempty"`
}

// XMLCreateRuleResponse is the XML response for CreateRule.
type XMLCreateRuleResponse struct {
	XMLName          xml.Name            `xml:"CreateRuleResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	Result           XMLCreateRuleResult `xml:"CreateRuleResult"`
	ResponseMetadata XMLResponseMetadata `xml:"ResponseMetadata"`
}

// XMLCreateRuleResult contains the created rules.
type XMLCreateRuleResult struct {
	Rules XMLRules `xml:"Rules"`
}

// XMLDescribeRulesResponse is the XML response for DescribeRules.
type XMLDescribeRulesResponse struct {
	XMLName          xml.Name               `xml:"DescribeRulesResponse"`
	Xmlns            string                 `xml:"xmlns,attr"`
	Result           XMLDescribeRulesResult `xml:"DescribeRulesResult"`
	ResponseMetadata XMLResponseMetadata    `xml:"ResponseMetadata"`
}

// XMLDescribeRulesResult contains the listed rules.
type XMLDescribeRulesResult struct {
	Rules XMLRules `xml:"Rules"`
}

// XMLModifyRuleResponse is the XML response for ModifyRule.
type XMLModifyRuleResponse struct {
	XMLName          xml.Name            `xml:"ModifyRuleResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	Result           XMLModifyRuleResult `xml:"ModifyRuleResult"`
	ResponseMetadata XMLResponseMetadata `xml:"ResponseMetadata"`
}

// XMLModifyRuleResult contains the modified rule.
type XMLModifyRuleResult struct {
	Rules XMLRules `xml:"Rules"`
}

// XMLDeleteRuleResponse is the XML response for DeleteRule.
type XMLDeleteRuleResponse struct {
	XMLName          xml.Name            `xml:"DeleteRuleResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	Result           XMLDeleteRuleResult `xml:"DeleteRuleResult"`
	ResponseMetadata XMLResponseMetadata `xml:"ResponseMetadata"`
}

// XMLDeleteRuleResult is an empty result for DeleteRule.
type XMLDeleteRuleResult struct{}

// XMLSetRulePrioritiesResponse is the XML response for SetRulePriorities.
type XMLSetRulePrioritiesResponse struct {
	XMLName          xml.Name                   `xml:"SetRulePrioritiesResponse"`
	Xmlns            string                     `xml:"xmlns,attr"`
	Result           XMLSetRulePrioritiesResult `xml:"SetRulePrioritiesResult"`
	ResponseMetadata XMLResponseMetadata        `xml:"ResponseMetadata"`
}

// XMLSetRulePrioritiesResult contains the rules with their new priorities.
type XMLSetRulePrioritiesResult struct {
	Rules XMLRules `xml:"Rules"`
}

// XMLRules contains a list of rules.
type XMLRules struct {
	Members []XMLRule `xml:"member"`
}

// XMLRule represents a rule in XML format.
type XMLRule struct {
	RuleArn    string            `xml:"RuleArn"`
	Priority   string            `xml:"Priority"`
	Conditions XMLRuleConditions `xml:"Conditions"`
	Actions    XMLActions        `xml:"Actions"`
	IsDefault  bool              `xml:"IsDefault"`
}

// XMLRuleConditions contains a list of conditions.
type XMLRuleConditions struct {
	Members []XMLRuleCondition `xml:"member"`
}

// XMLRuleCondition represents a condition in XML format. AWS returns each
// condition with both the legacy Values element AND a typed config child
// (PathPatternConfig, HostHeaderConfig, …). The AWS provider dereferences
// the typed config without nil-checking, so we surface a non-nil pointer
// for whichever field matches the condition's Field, even when its values
// are empty.
type XMLRuleCondition struct {
	Field                   string                `xml:"Field"`
	Values                  XMLRuleValues         `xml:"Values"`
	HostHeaderConfig        *XMLRuleValuesConfig  `xml:"HostHeaderConfig,omitempty"`
	PathPatternConfig       *XMLRuleValuesConfig  `xml:"PathPatternConfig,omitempty"`
	HTTPHeaderConfig        *XMLHTTPHeaderConfig  `xml:"HttpHeaderConfig,omitempty"`
	HTTPRequestMethodConfig *XMLRuleValuesConfig  `xml:"HttpRequestMethodConfig,omitempty"`
	QueryStringConfig       *XMLQueryStringConfig `xml:"QueryStringConfig,omitempty"`
	SourceIPConfig          *XMLRuleValuesConfig  `xml:"SourceIpConfig,omitempty"`
}

// XMLRuleValues contains a list of values for a condition.
type XMLRuleValues struct {
	Members []string `xml:"member"`
}

// XMLRuleValuesConfig is the typed value-list wrapper used by the
// host-header / path-pattern / http-request-method / source-ip configs.
type XMLRuleValuesConfig struct {
	Values XMLRuleValues `xml:"Values"`
}

// XMLHTTPHeaderConfig is the typed config for http-header conditions.
type XMLHTTPHeaderConfig struct {
	HTTPHeaderName string        `xml:"HttpHeaderName"`
	Values         XMLRuleValues `xml:"Values"`
}

// XMLQueryStringConfig is the typed config for query-string conditions.
type XMLQueryStringConfig struct {
	Values XMLQueryStringKVPairs `xml:"Values"`
}

// XMLQueryStringKVPairs holds the key/value pairs for a query-string match.
type XMLQueryStringKVPairs struct {
	Members []XMLQueryStringKV `xml:"member"`
}

// XMLQueryStringKV is one key=value pair in a query-string condition.
type XMLQueryStringKV struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// XMLResponseMetadata contains response metadata.
type XMLResponseMetadata struct {
	RequestID string `xml:"RequestId"`
}

// XMLErrorResponse is the XML error response.
type XMLErrorResponse struct {
	XMLName   xml.Name `xml:"ErrorResponse"`
	Error     XMLError `xml:"Error"`
	RequestID string   `xml:"RequestId"`
}

// XMLError represents an error in XML format.
type XMLError struct {
	Type    string `xml:"Type"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// Error represents an ELB error.
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

// XMLAttributePair represents a single attribute key/value entry.
type XMLAttributePair struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// XMLAttributePairs contains a list of attribute key/value entries.
type XMLAttributePairs struct {
	Members []XMLAttributePair `xml:"member"`
}

// XMLModifyLoadBalancerAttributesResponse is the XML response.
type XMLModifyLoadBalancerAttributesResponse struct {
	XMLName          xml.Name                              `xml:"ModifyLoadBalancerAttributesResponse"`
	Xmlns            string                                `xml:"xmlns,attr"`
	Result           XMLModifyLoadBalancerAttributesResult `xml:"ModifyLoadBalancerAttributesResult"`
	ResponseMetadata XMLResponseMetadata                   `xml:"ResponseMetadata"`
}

// XMLModifyLoadBalancerAttributesResult contains the updated attributes.
type XMLModifyLoadBalancerAttributesResult struct {
	Attributes XMLAttributePairs `xml:"Attributes"`
}

// XMLDescribeLoadBalancerAttributesResponse is the XML response.
type XMLDescribeLoadBalancerAttributesResponse struct {
	XMLName          xml.Name                                `xml:"DescribeLoadBalancerAttributesResponse"`
	Xmlns            string                                  `xml:"xmlns,attr"`
	Result           XMLDescribeLoadBalancerAttributesResult `xml:"DescribeLoadBalancerAttributesResult"`
	ResponseMetadata XMLResponseMetadata                     `xml:"ResponseMetadata"`
}

// XMLDescribeLoadBalancerAttributesResult contains the requested attributes.
type XMLDescribeLoadBalancerAttributesResult struct {
	Attributes XMLAttributePairs `xml:"Attributes"`
}

// XMLModifyTargetGroupAttributesResponse is the XML response.
type XMLModifyTargetGroupAttributesResponse struct {
	XMLName          xml.Name                             `xml:"ModifyTargetGroupAttributesResponse"`
	Xmlns            string                               `xml:"xmlns,attr"`
	Result           XMLModifyTargetGroupAttributesResult `xml:"ModifyTargetGroupAttributesResult"`
	ResponseMetadata XMLResponseMetadata                  `xml:"ResponseMetadata"`
}

// XMLModifyTargetGroupAttributesResult contains the updated attributes.
type XMLModifyTargetGroupAttributesResult struct {
	Attributes XMLAttributePairs `xml:"Attributes"`
}

// XMLDescribeTargetGroupAttributesResponse is the XML response.
type XMLDescribeTargetGroupAttributesResponse struct {
	XMLName          xml.Name                               `xml:"DescribeTargetGroupAttributesResponse"`
	Xmlns            string                                 `xml:"xmlns,attr"`
	Result           XMLDescribeTargetGroupAttributesResult `xml:"DescribeTargetGroupAttributesResult"`
	ResponseMetadata XMLResponseMetadata                    `xml:"ResponseMetadata"`
}

// XMLDescribeTargetGroupAttributesResult contains the requested attributes.
type XMLDescribeTargetGroupAttributesResult struct {
	Attributes XMLAttributePairs `xml:"Attributes"`
}

// XMLDescribeTargetHealthResponse is the XML response for DescribeTargetHealth.
type XMLDescribeTargetHealthResponse struct {
	XMLName          xml.Name                      `xml:"DescribeTargetHealthResponse"`
	Xmlns            string                        `xml:"xmlns,attr"`
	Result           XMLDescribeTargetHealthResult `xml:"DescribeTargetHealthResult"`
	ResponseMetadata XMLResponseMetadata           `xml:"ResponseMetadata"`
}

// XMLDescribeTargetHealthResult contains the per-target health.
type XMLDescribeTargetHealthResult struct {
	TargetHealthDescriptions XMLTargetHealthDescriptions `xml:"TargetHealthDescriptions"`
}

// XMLTargetHealthDescriptions contains a list of target health descriptions.
type XMLTargetHealthDescriptions struct {
	Members []XMLTargetHealthDescription `xml:"member"`
}

// XMLTargetHealthDescription represents one target's health.
type XMLTargetHealthDescription struct {
	Target       XMLTargetForHealth `xml:"Target"`
	TargetHealth XMLTargetHealth    `xml:"TargetHealth"`
}

// XMLTargetForHealth is the Target as wrapped inside a health description.
type XMLTargetForHealth struct {
	ID               string `xml:"Id"`
	Port             int    `xml:"Port"`
	AvailabilityZone string `xml:"AvailabilityZone,omitempty"`
}

// XMLTargetHealth represents a target's health state.
type XMLTargetHealth struct {
	State       string `xml:"State"`
	Reason      string `xml:"Reason,omitempty"`
	Description string `xml:"Description,omitempty"`
}

// XMLDescribeListenersResponse is the XML response for DescribeListeners.
type XMLDescribeListenersResponse struct {
	XMLName          xml.Name                   `xml:"DescribeListenersResponse"`
	Xmlns            string                     `xml:"xmlns,attr"`
	Result           XMLDescribeListenersResult `xml:"DescribeListenersResult"`
	ResponseMetadata XMLResponseMetadata        `xml:"ResponseMetadata"`
}

// XMLDescribeListenersResult contains the listed listeners.
type XMLDescribeListenersResult struct {
	Listeners XMLListeners `xml:"Listeners"`
}

// XMLModifyListenerResponse is the XML response for ModifyListener.
type XMLModifyListenerResponse struct {
	XMLName          xml.Name                `xml:"ModifyListenerResponse"`
	Xmlns            string                  `xml:"xmlns,attr"`
	Result           XMLModifyListenerResult `xml:"ModifyListenerResult"`
	ResponseMetadata XMLResponseMetadata     `xml:"ResponseMetadata"`
}

// XMLModifyListenerResult contains the modified listener.
type XMLModifyListenerResult struct {
	Listeners XMLListeners `xml:"Listeners"`
}

// XMLDescribeTagsResponse is the XML response for DescribeTags.
type XMLDescribeTagsResponse struct {
	XMLName            xml.Name              `xml:"DescribeTagsResponse"`
	Xmlns              string                `xml:"xmlns,attr"`
	DescribeTagsResult XMLDescribeTagsResult `xml:"DescribeTagsResult"`
	ResponseMetadata   XMLResponseMetadata   `xml:"ResponseMetadata"`
}

// XMLDescribeTagsResult contains tag descriptions.
type XMLDescribeTagsResult struct {
	TagDescriptions XMLTagDescriptions `xml:"TagDescriptions"`
}

// XMLTagDescriptions contains a list of tag descriptions.
type XMLTagDescriptions struct {
	Members []XMLTagDescription `xml:"member"`
}

// XMLTagDescription represents tags attached to a resource.
type XMLTagDescription struct {
	ResourceArn string          `xml:"ResourceArn"`
	Tags        XMLEmptyMembers `xml:"Tags"`
}

// XMLEmptyMembers represents an empty AWS Query member list.
type XMLEmptyMembers struct {
	Members []struct{} `xml:"member"`
}

// XMLDescribeCapacityReservationResponse is the XML response for DescribeCapacityReservation.
type XMLDescribeCapacityReservationResponse struct {
	XMLName                           xml.Name                     `xml:"DescribeCapacityReservationResponse"`
	Xmlns                             string                       `xml:"xmlns,attr"`
	DescribeCapacityReservationResult XMLCapacityReservationResult `xml:"DescribeCapacityReservationResult"`
	ResponseMetadata                  XMLResponseMetadata          `xml:"ResponseMetadata"`
}

// XMLCapacityReservationResult contains capacity reservation metadata.
type XMLCapacityReservationResult struct {
	LastModifiedTime          string          `xml:"LastModifiedTime,omitempty"`
	DecreaseRequestsRemaining int             `xml:"DecreaseRequestsRemaining"`
	CapacityReservationState  XMLEmptyMembers `xml:"CapacityReservationState"`
}

// XMLDescribeListenerAttributesResponse is the XML response for DescribeListenerAttributes.
type XMLDescribeListenerAttributesResponse struct {
	XMLName          xml.Name                    `xml:"DescribeListenerAttributesResponse"`
	Xmlns            string                      `xml:"xmlns,attr"`
	Result           XMLListenerAttributesResult `xml:"DescribeListenerAttributesResult"`
	ResponseMetadata XMLResponseMetadata         `xml:"ResponseMetadata"`
}

// XMLListenerAttributesResult contains listener attributes.
type XMLListenerAttributesResult struct {
	Attributes XMLAttributePairs `xml:"Attributes"`
}

// XMLAddTagsResponse is the XML response for AddTags.
type XMLAddTagsResponse struct {
	XMLName          xml.Name            `xml:"AddTagsResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	ResponseMetadata XMLResponseMetadata `xml:"ResponseMetadata"`
}

// XMLRemoveTagsResponse is the XML response for RemoveTags.
type XMLRemoveTagsResponse struct {
	XMLName          xml.Name            `xml:"RemoveTagsResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	ResponseMetadata XMLResponseMetadata `xml:"ResponseMetadata"`
}
