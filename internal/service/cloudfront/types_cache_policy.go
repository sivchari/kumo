package cloudfront

import (
	"encoding/xml"
	"time"
)

// Cache policy error codes (CloudFront API reference).
const (
	errNoSuchCachePolicy        = "NoSuchCachePolicy"
	errCachePolicyAlreadyExists = "CachePolicyAlreadyExists"
	errCachePolicyInUse         = "CachePolicyInUse"
	errInconsistentQuantities   = "InconsistentQuantities"
)

// Cache policy enumerations and documented defaults.
const (
	cachePolicyTypeCustom  = "custom"
	cachePolicyTypeManaged = "managed"

	cachePolicyBehaviorNone      = "none"
	cachePolicyBehaviorWhitelist = "whitelist"
	cachePolicyBehaviorAllExcept = "allExcept"
	cachePolicyBehaviorAll       = "all"

	cachePolicyDefaultTTL       = 86400
	cachePolicyDefaultMaxTTL    = 31536000
	cachePolicyCommentMaxLength = 128
	cachePolicyListMaxItems     = 100
)

// CachePolicy is a custom CloudFront cache policy: the cache key and TTLs of
// the cache behaviors that reference it by CachePolicyId.
type CachePolicy struct {
	ID               string
	ETag             string
	LastModifiedTime time.Time
	Config           CachePolicyConfig
}

// CachePolicyConfig is the user-supplied configuration. It doubles as the
// request body of Create/UpdateCachePolicy and the response body of
// GetCachePolicyConfig. DefaultTTL and MaxTTL are stored with the documented
// defaults applied, so they are always returned.
type CachePolicyConfig struct {
	XMLName    xml.Name               `xml:"CachePolicyConfig" json:"-"`
	Xmlns      string                 `xml:"xmlns,attr,omitempty" json:"-"`
	Comment    string                 `xml:"Comment,omitempty"`
	Name       string                 `xml:"Name"`
	DefaultTTL *int64                 `xml:"DefaultTTL"`
	MaxTTL     *int64                 `xml:"MaxTTL"`
	MinTTL     *int64                 `xml:"MinTTL"`
	Parameters *CachePolicyParameters `xml:"ParametersInCacheKeyAndForwardedToOrigin,omitempty"`
}

// CachePolicyParameters is ParametersInCacheKeyAndForwardedToOrigin.
// EnableAcceptEncodingBrotli is optional and echoed only when it was sent.
type CachePolicyParameters struct {
	EnableAcceptEncodingGzip   *bool                     `xml:"EnableAcceptEncodingGzip"`
	EnableAcceptEncodingBrotli *bool                     `xml:"EnableAcceptEncodingBrotli,omitempty"`
	HeadersConfig              *CachePolicyHeadersConfig `xml:"HeadersConfig"`
	CookiesConfig              *CachePolicyCookiesConfig `xml:"CookiesConfig"`
	QueryStringsConfig         *CachePolicyQueryStrings  `xml:"QueryStringsConfig"`
}

// CachePolicyHeadersConfig is the header part of the cache key.
type CachePolicyHeadersConfig struct {
	HeaderBehavior string            `xml:"HeaderBehavior"`
	Headers        *CachePolicyNames `xml:"Headers,omitempty"`
}

// CachePolicyCookiesConfig is the cookie part of the cache key.
type CachePolicyCookiesConfig struct {
	CookieBehavior string            `xml:"CookieBehavior"`
	Cookies        *CachePolicyNames `xml:"Cookies,omitempty"`
}

// CachePolicyQueryStrings is the query string part of the cache key.
type CachePolicyQueryStrings struct {
	QueryStringBehavior string            `xml:"QueryStringBehavior"`
	QueryStrings        *CachePolicyNames `xml:"QueryStrings,omitempty"`
}

// CachePolicyNames is the Quantity + Items>Name shape shared by the Headers,
// CookieNames and QueryStringNames lists. Items is a pointer so that an
// omitted Items stays omitted while an explicit <Items/> is echoed.
type CachePolicyNames struct {
	Quantity *int                  `xml:"Quantity"`
	Items    *CachePolicyNameItems `xml:"Items,omitempty"`
}

// CachePolicyNameItems is the Items element of CachePolicyNames.
type CachePolicyNameItems struct {
	Name []string `xml:"Name"`
}

// CachePolicyXML is the CachePolicy element of Create/Get/UpdateCachePolicy
// and of a CachePolicySummary.
type CachePolicyXML struct {
	XMLName           xml.Name           `xml:"CachePolicy"`
	Xmlns             string             `xml:"xmlns,attr,omitempty"`
	ID                string             `xml:"Id"`
	LastModifiedTime  string             `xml:"LastModifiedTime"`
	CachePolicyConfig *CachePolicyConfig `xml:"CachePolicyConfig"`
}

// CachePolicyListXML is the body of ListCachePolicies. Unlike
// OriginAccessControlList it carries no Marker or IsTruncated; Items is
// omitted when empty.
type CachePolicyListXML struct {
	XMLName    xml.Name                `xml:"CachePolicyList"`
	Xmlns      string                  `xml:"xmlns,attr"`
	NextMarker string                  `xml:"NextMarker,omitempty"`
	MaxItems   int                     `xml:"MaxItems"`
	Quantity   int                     `xml:"Quantity"`
	Items      *CachePolicySummaryList `xml:"Items,omitempty"`
}

// CachePolicySummaryList is the Items element of CachePolicyList.
type CachePolicySummaryList struct {
	CachePolicySummary []CachePolicySummaryXML `xml:"CachePolicySummary"`
}

// CachePolicySummaryXML is one item of CachePolicyList.
type CachePolicySummaryXML struct {
	Type        string          `xml:"Type"`
	CachePolicy *CachePolicyXML `xml:"CachePolicy"`
}
