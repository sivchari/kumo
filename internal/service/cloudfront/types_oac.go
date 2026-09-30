package cloudfront

import "encoding/xml"

// Origin access control error codes (CloudFront API reference).
const (
	errNoSuchOriginAccessControl               = "NoSuchOriginAccessControl"
	errOriginAccessControlAlreadyExists        = "OriginAccessControlAlreadyExists"
	errOriginAccessControlInUse                = "OriginAccessControlInUse"
	errInvalidOriginAccessControl              = "InvalidOriginAccessControl"
	errInvalidDomainNameForOriginAccessControl = "InvalidDomainNameForOriginAccessControl"
	errIllegalOriginAccessConfiguration        = "IllegalOriginAccessConfiguration"
)

// Origin access control enumerations.
const (
	oacOriginTypeS3             = "s3"
	oacOriginTypeMediaStore     = "mediastore"
	oacOriginTypeMediaPackageV2 = "mediapackagev2"
	oacOriginTypeLambda         = "lambda"

	oacSigningAlways     = "always"
	oacSigningNever      = "never"
	oacSigningNoOverride = "no-override"

	oacSigningProtocolSigV4 = "sigv4"

	oacNameMaxLength = 64
	oacListMaxItems  = 100
)

// OriginAccessControl is a CloudFront origin access control: the signing
// policy CloudFront applies to origin requests of the origins that reference
// it by OriginAccessControlId.
type OriginAccessControl struct {
	ID     string
	ETag   string
	Config OriginAccessControlConfig
}

// OriginAccessControlConfig is the user-supplied configuration.
type OriginAccessControlConfig struct {
	Name                          string
	Description                   string
	SigningProtocol               string
	SigningBehavior               string
	OriginAccessControlOriginType string
}

// OriginAccessControlConfigXML is the XML view of OriginAccessControlConfig;
// also the request body of Create/UpdateOriginAccessControl and the response
// body of GetOriginAccessControlConfig.
type OriginAccessControlConfigXML struct {
	XMLName                       xml.Name `xml:"OriginAccessControlConfig"`
	Xmlns                         string   `xml:"xmlns,attr,omitempty"`
	Name                          string   `xml:"Name"`
	Description                   string   `xml:"Description,omitempty"`
	SigningProtocol               string   `xml:"SigningProtocol"`
	SigningBehavior               string   `xml:"SigningBehavior"`
	OriginAccessControlOriginType string   `xml:"OriginAccessControlOriginType"`
}

// OriginAccessControlResultXML is the body of Create/Get/UpdateOriginAccessControl.
type OriginAccessControlResultXML struct {
	XMLName                   xml.Name                      `xml:"OriginAccessControl"`
	Xmlns                     string                        `xml:"xmlns,attr"`
	ID                        string                        `xml:"Id"`
	OriginAccessControlConfig *OriginAccessControlConfigXML `xml:"OriginAccessControlConfig"`
}

// OriginAccessControlListXML is the body of ListOriginAccessControls. Items
// is omitted when the account has none, as on AWS.
type OriginAccessControlListXML struct {
	XMLName     xml.Name                        `xml:"OriginAccessControlList"`
	Xmlns       string                          `xml:"xmlns,attr"`
	Marker      string                          `xml:"Marker"`
	NextMarker  string                          `xml:"NextMarker,omitempty"`
	MaxItems    int                             `xml:"MaxItems"`
	IsTruncated bool                            `xml:"IsTruncated"`
	Quantity    int                             `xml:"Quantity"`
	Items       []OriginAccessControlSummaryXML `xml:"Items>OriginAccessControlSummary,omitempty"`
}

// OriginAccessControlSummaryXML is one item of OriginAccessControlList.
type OriginAccessControlSummaryXML struct {
	ID                            string `xml:"Id"`
	Description                   string `xml:"Description"`
	Name                          string `xml:"Name"`
	SigningProtocol               string `xml:"SigningProtocol"`
	SigningBehavior               string `xml:"SigningBehavior"`
	OriginAccessControlOriginType string `xml:"OriginAccessControlOriginType"`
}
