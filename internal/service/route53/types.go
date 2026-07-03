package route53

import "encoding/xml"

const xmlns = "https://route53.amazonaws.com/doc/2013-04-01/"

// HostedZone represents a Route 53 hosted zone.
type HostedZone struct {
	ID                     string            `xml:"Id"`
	Name                   string            `xml:"Name"`
	CallerReference        string            `xml:"CallerReference"`
	Config                 *HostedZoneConfig `xml:"Config,omitempty"`
	ResourceRecordSetCount int64             `xml:"ResourceRecordSetCount"`
}

// HostedZoneConfig represents the configuration for a hosted zone.
type HostedZoneConfig struct {
	Comment     string `xml:"Comment,omitempty"`
	PrivateZone bool   `xml:"PrivateZone"`
}

// ResourceRecordSet represents a DNS record set.
type ResourceRecordSet struct {
	Name             string           `xml:"Name"`
	Type             string           `xml:"Type"`
	TTL              *int64           `xml:"TTL,omitempty"`
	ResourceRecords  []ResourceRecord `xml:"ResourceRecords>ResourceRecord,omitempty"`
	AliasTarget      *AliasTarget     `xml:"AliasTarget,omitempty"`
	SetIdentifier    string           `xml:"SetIdentifier,omitempty"`
	Weight           *int64           `xml:"Weight,omitempty"`
	Region           string           `xml:"Region,omitempty"`
	Failover         string           `xml:"Failover,omitempty"`
	HealthCheckID    string           `xml:"HealthCheckId,omitempty"`
	MultiValueAnswer *bool            `xml:"MultiValueAnswer,omitempty"`
}

// ResourceRecord represents a single resource record value.
type ResourceRecord struct {
	Value string `xml:"Value"`
}

// AliasTarget represents an alias target for a record set.
type AliasTarget struct {
	HostedZoneID         string `xml:"HostedZoneId"`
	DNSName              string `xml:"DNSName"`
	EvaluateTargetHealth bool   `xml:"EvaluateTargetHealth"`
}

// Change represents a change to a resource record set.
type Change struct {
	Action            string            `xml:"Action"`
	ResourceRecordSet ResourceRecordSet `xml:"ResourceRecordSet"`
}

// ChangeBatch represents a batch of changes.
type ChangeBatch struct {
	Comment string   `xml:"Comment,omitempty"`
	Changes []Change `xml:"Changes>Change"`
}

// ChangeInfo represents information about a change.
type ChangeInfo struct {
	ID          string `xml:"Id"`
	Status      string `xml:"Status"`
	SubmittedAt string `xml:"SubmittedAt"`
	Comment     string `xml:"Comment,omitempty"`
}

// DelegationSet represents the name servers for a hosted zone.
type DelegationSet struct {
	NameServers []string `xml:"NameServers>NameServer"`
}

// CreateHostedZoneRequest represents a request to create a hosted zone.
type CreateHostedZoneRequest struct {
	XMLName          xml.Name          `xml:"CreateHostedZoneRequest"`
	Name             string            `xml:"Name"`
	CallerReference  string            `xml:"CallerReference"`
	HostedZoneConfig *HostedZoneConfig `xml:"HostedZoneConfig,omitempty"`
}

// CreateHostedZoneResponse represents a response to create hosted zone.
type CreateHostedZoneResponse struct {
	XMLName       xml.Name      `xml:"CreateHostedZoneResponse"`
	XMLNS         string        `xml:"xmlns,attr"`
	HostedZone    HostedZone    `xml:"HostedZone"`
	ChangeInfo    ChangeInfo    `xml:"ChangeInfo"`
	DelegationSet DelegationSet `xml:"DelegationSet"`
	Location      string        `xml:"Location,omitempty"`
}

// GetHostedZoneResponse represents a response to get hosted zone.
type GetHostedZoneResponse struct {
	XMLName       xml.Name      `xml:"GetHostedZoneResponse"`
	XMLNS         string        `xml:"xmlns,attr"`
	HostedZone    HostedZone    `xml:"HostedZone"`
	DelegationSet DelegationSet `xml:"DelegationSet"`
}

// ListHostedZonesResponse represents a response to list hosted zones.
type ListHostedZonesResponse struct {
	XMLName     xml.Name     `xml:"ListHostedZonesResponse"`
	XMLNS       string       `xml:"xmlns,attr"`
	HostedZones []HostedZone `xml:"HostedZones>HostedZone"`
	Marker      string       `xml:"Marker,omitempty"`
	IsTruncated bool         `xml:"IsTruncated"`
	NextMarker  string       `xml:"NextMarker,omitempty"`
	MaxItems    string       `xml:"MaxItems"`
}

// ListHostedZonesByNameResponse represents a response to list hosted zones
// by name (used by the AWS data source aws_route53_zone among others).
type ListHostedZonesByNameResponse struct {
	XMLName          xml.Name     `xml:"ListHostedZonesByNameResponse"`
	XMLNS            string       `xml:"xmlns,attr"`
	HostedZones      []HostedZone `xml:"HostedZones>HostedZone"`
	DNSName          string       `xml:"DNSName,omitempty"`
	HostedZoneID     string       `xml:"HostedZoneId,omitempty"`
	IsTruncated      bool         `xml:"IsTruncated"`
	NextDNSName      string       `xml:"NextDNSName,omitempty"`
	NextHostedZoneID string       `xml:"NextHostedZoneId,omitempty"`
	MaxItems         string       `xml:"MaxItems"`
}

// DeleteHostedZoneResponse represents a response to delete hosted zone.
type DeleteHostedZoneResponse struct {
	XMLName    xml.Name   `xml:"DeleteHostedZoneResponse"`
	XMLNS      string     `xml:"xmlns,attr"`
	ChangeInfo ChangeInfo `xml:"ChangeInfo"`
}

// ChangeResourceRecordSetsRequest represents a request to change record sets.
type ChangeResourceRecordSetsRequest struct {
	XMLName     xml.Name    `xml:"ChangeResourceRecordSetsRequest"`
	ChangeBatch ChangeBatch `xml:"ChangeBatch"`
}

// ChangeResourceRecordSetsResponse represents a response to change record sets.
type ChangeResourceRecordSetsResponse struct {
	XMLName    xml.Name   `xml:"ChangeResourceRecordSetsResponse"`
	XMLNS      string     `xml:"xmlns,attr"`
	ChangeInfo ChangeInfo `xml:"ChangeInfo"`
}

// GetChangeResponse represents a response to get change.
type GetChangeResponse struct {
	XMLName    xml.Name   `xml:"GetChangeResponse"`
	XMLNS      string     `xml:"xmlns,attr"`
	ChangeInfo ChangeInfo `xml:"ChangeInfo"`
}

// ListResourceRecordSetsResponse represents a response to list record sets.
type ListResourceRecordSetsResponse struct {
	XMLName              xml.Name            `xml:"ListResourceRecordSetsResponse"`
	XMLNS                string              `xml:"xmlns,attr"`
	ResourceRecordSets   []ResourceRecordSet `xml:"ResourceRecordSets>ResourceRecordSet"`
	IsTruncated          bool                `xml:"IsTruncated"`
	MaxItems             string              `xml:"MaxItems"`
	NextRecordName       string              `xml:"NextRecordName,omitempty"`
	NextRecordType       string              `xml:"NextRecordType,omitempty"`
	NextRecordIdentifier string              `xml:"NextRecordIdentifier,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	XMLName   xml.Name `xml:"ErrorResponse"`
	XMLNS     string   `xml:"xmlns,attr"`
	Error     Error    `xml:"Error"`
	RequestID string   `xml:"RequestId"`
}

// Error represents an error detail.
type Error struct {
	Type    string `xml:"Type"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// ListTagsForResourceXMLResponse is the XML response for ListTagsForResource.
type ListTagsForResourceXMLResponse struct {
	XMLName        struct{}       `xml:"ListTagsForResourceResponse"`
	XMLNS          string         `xml:"xmlns,attr"`
	ResourceTagSet ResourceTagSet `xml:"ResourceTagSet"`
}

// ChangeTagsForResourceXMLResponse is the XML response for ChangeTagsForResource.
type ChangeTagsForResourceXMLResponse struct {
	XMLName struct{} `xml:"ChangeTagsForResourceResponse"`
	XMLNS   string   `xml:"xmlns,attr"`
}

// ChangeTagsForResourceRequest is the XML request for ChangeTagsForResource.
type ChangeTagsForResourceRequest struct {
	XMLName       xml.Name `xml:"ChangeTagsForResourceRequest"`
	AddTags       *AddTags `xml:"AddTags,omitempty"`
	RemoveTagKeys []string `xml:"RemoveTagKeys>Key,omitempty"`
}

// AddTags wraps the list of tags to add.
type AddTags struct {
	Tags []Tag `xml:"Tag"`
}

// ResourceTagSet is the tag set for a Route53 resource.
type ResourceTagSet struct {
	ResourceType string  `xml:"ResourceType"`
	ResourceID   string  `xml:"ResourceId"`
	Tags         TagList `xml:"Tags"`
}

// TagList wraps tag members.
type TagList struct {
	Tag []Tag `xml:"Tag"`
}

// Tag is a single Route53 tag.
type Tag struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}
