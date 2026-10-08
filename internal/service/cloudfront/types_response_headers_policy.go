package cloudfront

import (
	"encoding/xml"
	"time"
)

// Response headers policy error codes (CloudFront API reference).
const (
	errNoSuchResponseHeadersPolicy        = "NoSuchResponseHeadersPolicy"
	errResponseHeadersPolicyAlreadyExists = "ResponseHeadersPolicyAlreadyExists"
	errResponseHeadersPolicyInUse         = "ResponseHeadersPolicyInUse"
	errTooLongCSPInResponseHeadersPolicy  = "TooLongCSPInResponseHeadersPolicy"
)

// Response headers policy enumerations and documented limits.
const (
	responseHeadersPolicyTypeCustom  = "custom"
	responseHeadersPolicyTypeManaged = "managed"

	responseHeadersPolicyCommentMaxLength = 128
	responseHeadersPolicyCSPMaxLength     = 1783
	responseHeadersPolicyListMaxItems     = 100
	serverTimingSamplingRateMax           = 100
	serverTimingSamplingRateScale         = 10000 // up to four decimal places
)

var (
	responseHeadersPolicyAllowMethods   = []string{"GET", "POST", "OPTIONS", "PUT", "DELETE", "PATCH", "HEAD", "ALL"}
	responseHeadersPolicyFrameOptions   = []string{"DENY", "SAMEORIGIN"}
	responseHeadersPolicyReferrerPolicy = []string{
		"no-referrer", "no-referrer-when-downgrade", "origin", "origin-when-cross-origin",
		"same-origin", "strict-origin", "strict-origin-when-cross-origin", "unsafe-url",
	}

	// Headers that RemoveHeadersConfig cannot remove, matched case-insensitively:
	// exact names plus the X-Amz-Cf-.* and X-Edge-.* patterns.
	responseHeadersPolicyUnremovableHeaders = []string{
		"Connection", "Content-Encoding", "Content-Length", "Expect", "Host", "Keep-Alive",
		"Proxy-Authenticate", "Proxy-Authorization", "Proxy-Connection", "Trailer",
		"Transfer-Encoding", "Upgrade", "Via", "Warning", "X-Accel-Buffering", "X-Accel-Charset",
		"X-Accel-Limit-Rate", "X-Accel-Redirect", "X-Amzn-Auth", "X-Amzn-Cf-Billing", "X-Amzn-Cf-Id",
		"X-Amzn-Cf-Xff", "X-Amzn-ErrorType", "X-Amzn-Fle-Profile", "X-Amzn-Header-Count",
		"X-Amzn-Header-Order", "X-Amzn-Lambda-Integration-Tag", "X-Amzn-RequestId",
		"X-Forwarded-Proto", "X-Real-Ip",
	}
	responseHeadersPolicyUnremovablePrefixes = []string{"X-Amz-Cf-", "X-Edge-"}
)

// ResponseHeadersPolicy is a custom CloudFront response headers policy: the
// HTTP headers added to or removed from the responses of the cache behaviors
// that reference it by ResponseHeadersPolicyId.
type ResponseHeadersPolicy struct {
	ID               string
	ETag             string
	LastModifiedTime time.Time
	Config           ResponseHeadersPolicyConfig
}

// ResponseHeadersPolicyConfig is the user-supplied configuration. It doubles
// as the request body of Create/UpdateResponseHeadersPolicy and the response
// body of GetResponseHeadersPolicyConfig. Optional members are pointers so an
// explicit false or zero round-trips and an omitted one stays omitted.
type ResponseHeadersPolicyConfig struct {
	XMLName                   xml.Name                                        `xml:"ResponseHeadersPolicyConfig" json:"-"`
	Xmlns                     string                                          `xml:"xmlns,attr,omitempty" json:"-"`
	Comment                   string                                          `xml:"Comment,omitempty"`
	CorsConfig                *ResponseHeadersPolicyCorsConfig                `xml:"CorsConfig,omitempty"`
	CustomHeadersConfig       *ResponseHeadersPolicyCustomHeadersConfig       `xml:"CustomHeadersConfig,omitempty"`
	Name                      string                                          `xml:"Name"`
	RemoveHeadersConfig       *ResponseHeadersPolicyRemoveHeadersConfig       `xml:"RemoveHeadersConfig,omitempty"`
	SecurityHeadersConfig     *ResponseHeadersPolicySecurityHeadersConfig     `xml:"SecurityHeadersConfig,omitempty"`
	ServerTimingHeadersConfig *ResponseHeadersPolicyServerTimingHeadersConfig `xml:"ServerTimingHeadersConfig,omitempty"`
}

// ResponseHeadersPolicyCorsConfig is the CORS part of the policy.
type ResponseHeadersPolicyCorsConfig struct {
	AccessControlAllowCredentials *bool                            `xml:"AccessControlAllowCredentials"`
	AccessControlAllowHeaders     *ResponseHeadersPolicyHeaderList `xml:"AccessControlAllowHeaders"`
	AccessControlAllowMethods     *ResponseHeadersPolicyMethodList `xml:"AccessControlAllowMethods"`
	AccessControlAllowOrigins     *ResponseHeadersPolicyOriginList `xml:"AccessControlAllowOrigins"`
	AccessControlExposeHeaders    *ResponseHeadersPolicyHeaderList `xml:"AccessControlExposeHeaders,omitempty"`
	AccessControlMaxAgeSec        *int32                           `xml:"AccessControlMaxAgeSec,omitempty"`
	OriginOverride                *bool                            `xml:"OriginOverride"`
}

// ResponseHeadersPolicyHeaderList is the Quantity + Items>Header shape of
// AccessControlAllowHeaders and AccessControlExposeHeaders. Items is a
// pointer because it is required in the former, even when empty.
type ResponseHeadersPolicyHeaderList struct {
	Quantity *int                          `xml:"Quantity"`
	Items    *ResponseHeadersPolicyHeaders `xml:"Items,omitempty"`
}

// ResponseHeadersPolicyHeaders is the Items element of a header list.
type ResponseHeadersPolicyHeaders struct {
	Header []string `xml:"Header"`
}

// ResponseHeadersPolicyMethodList is AccessControlAllowMethods.
type ResponseHeadersPolicyMethodList struct {
	Quantity *int                          `xml:"Quantity"`
	Items    *ResponseHeadersPolicyMethods `xml:"Items,omitempty"`
}

// ResponseHeadersPolicyMethods is the Items element of AccessControlAllowMethods.
type ResponseHeadersPolicyMethods struct {
	Method []string `xml:"Method"`
}

// ResponseHeadersPolicyOriginList is AccessControlAllowOrigins.
type ResponseHeadersPolicyOriginList struct {
	Quantity *int                          `xml:"Quantity"`
	Items    *ResponseHeadersPolicyOrigins `xml:"Items,omitempty"`
}

// ResponseHeadersPolicyOrigins is the Items element of AccessControlAllowOrigins.
type ResponseHeadersPolicyOrigins struct {
	Origin []string `xml:"Origin"`
}

// ResponseHeadersPolicyCustomHeadersConfig lists the headers to add. Items
// is a pointer so that an omitted Items stays omitted while an explicit
// <Items/> is echoed.
type ResponseHeadersPolicyCustomHeadersConfig struct {
	Quantity *int                                   `xml:"Quantity"`
	Items    *ResponseHeadersPolicyCustomHeaderList `xml:"Items,omitempty"`
}

// ResponseHeadersPolicyCustomHeaderList is the Items element of CustomHeadersConfig.
type ResponseHeadersPolicyCustomHeaderList struct {
	ResponseHeadersPolicyCustomHeader []ResponseHeadersPolicyCustomHeader `xml:"ResponseHeadersPolicyCustomHeader"`
}

// ResponseHeadersPolicyCustomHeader is one header to add.
type ResponseHeadersPolicyCustomHeader struct {
	Header   *string `xml:"Header"`
	Override *bool   `xml:"Override"`
	Value    *string `xml:"Value"`
}

// ResponseHeadersPolicyRemoveHeadersConfig lists the headers to remove; Items
// is a pointer like in CustomHeadersConfig.
type ResponseHeadersPolicyRemoveHeadersConfig struct {
	Quantity *int                                   `xml:"Quantity"`
	Items    *ResponseHeadersPolicyRemoveHeaderList `xml:"Items,omitempty"`
}

// ResponseHeadersPolicyRemoveHeaderList is the Items element of RemoveHeadersConfig.
type ResponseHeadersPolicyRemoveHeaderList struct {
	ResponseHeadersPolicyRemoveHeader []ResponseHeadersPolicyRemoveHeader `xml:"ResponseHeadersPolicyRemoveHeader"`
}

// ResponseHeadersPolicyRemoveHeader is one header to remove.
type ResponseHeadersPolicyRemoveHeader struct {
	Header *string `xml:"Header"`
}

// ResponseHeadersPolicySecurityHeadersConfig is the security headers part of
// the policy; each header is optional.
type ResponseHeadersPolicySecurityHeadersConfig struct {
	ContentSecurityPolicy   *ResponseHeadersPolicyContentSecurityPolicy   `xml:"ContentSecurityPolicy,omitempty"`
	ContentTypeOptions      *ResponseHeadersPolicyContentTypeOptions      `xml:"ContentTypeOptions,omitempty"`
	FrameOptions            *ResponseHeadersPolicyFrameOptions            `xml:"FrameOptions,omitempty"`
	ReferrerPolicy          *ResponseHeadersPolicyReferrerPolicy          `xml:"ReferrerPolicy,omitempty"`
	StrictTransportSecurity *ResponseHeadersPolicyStrictTransportSecurity `xml:"StrictTransportSecurity,omitempty"`
	XSSProtection           *ResponseHeadersPolicyXSSProtection           `xml:"XSSProtection,omitempty"`
}

// ResponseHeadersPolicyContentSecurityPolicy is the Content-Security-Policy header.
type ResponseHeadersPolicyContentSecurityPolicy struct {
	ContentSecurityPolicy *string `xml:"ContentSecurityPolicy"`
	Override              *bool   `xml:"Override"`
}

// ResponseHeadersPolicyContentTypeOptions is the X-Content-Type-Options header.
type ResponseHeadersPolicyContentTypeOptions struct {
	Override *bool `xml:"Override"`
}

// ResponseHeadersPolicyFrameOptions is the X-Frame-Options header.
type ResponseHeadersPolicyFrameOptions struct {
	FrameOption string `xml:"FrameOption"`
	Override    *bool  `xml:"Override"`
}

// ResponseHeadersPolicyReferrerPolicy is the Referrer-Policy header.
type ResponseHeadersPolicyReferrerPolicy struct {
	Override       *bool  `xml:"Override"`
	ReferrerPolicy string `xml:"ReferrerPolicy"`
}

// ResponseHeadersPolicyStrictTransportSecurity is the Strict-Transport-Security header.
type ResponseHeadersPolicyStrictTransportSecurity struct {
	AccessControlMaxAgeSec *int32 `xml:"AccessControlMaxAgeSec"`
	IncludeSubdomains      *bool  `xml:"IncludeSubdomains,omitempty"`
	Override               *bool  `xml:"Override"`
	Preload                *bool  `xml:"Preload,omitempty"`
}

// ResponseHeadersPolicyXSSProtection is the X-XSS-Protection header.
type ResponseHeadersPolicyXSSProtection struct {
	ModeBlock  *bool   `xml:"ModeBlock,omitempty"`
	Override   *bool   `xml:"Override"`
	Protection *bool   `xml:"Protection"`
	ReportURI  *string `xml:"ReportUri,omitempty"`
}

// ResponseHeadersPolicyServerTimingHeadersConfig is the Server-Timing header.
type ResponseHeadersPolicyServerTimingHeadersConfig struct {
	Enabled      *bool    `xml:"Enabled"`
	SamplingRate *float64 `xml:"SamplingRate,omitempty"`
}

// ResponseHeadersPolicyXML is the ResponseHeadersPolicy element of
// Create/Get/UpdateResponseHeadersPolicy and of a ResponseHeadersPolicySummary.
type ResponseHeadersPolicyXML struct {
	XMLName                     xml.Name                     `xml:"ResponseHeadersPolicy"`
	Xmlns                       string                       `xml:"xmlns,attr,omitempty"`
	ID                          string                       `xml:"Id"`
	LastModifiedTime            string                       `xml:"LastModifiedTime"`
	ResponseHeadersPolicyConfig *ResponseHeadersPolicyConfig `xml:"ResponseHeadersPolicyConfig"`
}

// ResponseHeadersPolicyListXML is the body of ListResponseHeadersPolicies;
// like CachePolicyList it carries no Marker or IsTruncated, and Items is
// omitted when empty.
type ResponseHeadersPolicyListXML struct {
	XMLName    xml.Name                          `xml:"ResponseHeadersPolicyList"`
	Xmlns      string                            `xml:"xmlns,attr"`
	NextMarker string                            `xml:"NextMarker,omitempty"`
	MaxItems   int                               `xml:"MaxItems"`
	Quantity   int                               `xml:"Quantity"`
	Items      *ResponseHeadersPolicySummaryList `xml:"Items,omitempty"`
}

// ResponseHeadersPolicySummaryList is the Items element of ResponseHeadersPolicyList.
type ResponseHeadersPolicySummaryList struct {
	ResponseHeadersPolicySummary []ResponseHeadersPolicySummaryXML `xml:"ResponseHeadersPolicySummary"`
}

// ResponseHeadersPolicySummaryXML is one item of ResponseHeadersPolicyList.
type ResponseHeadersPolicySummaryXML struct {
	Type                  string                    `xml:"Type"`
	ResponseHeadersPolicy *ResponseHeadersPolicyXML `xml:"ResponseHeadersPolicy"`
}
