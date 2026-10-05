// Package cloudfront provides CloudFront service emulation for kumo.
package cloudfront

import (
	"encoding/xml"
	"time"
)

// Distribution represents a CloudFront distribution.
type Distribution struct {
	ID                     string
	ARN                    string
	Status                 string
	LastModifiedTime       time.Time
	DomainName             string
	ETag                   string
	DistributionConfig     *DistributionConfig
	ActiveTrustedSigners   *ActiveTrustedSigners
	ActiveTrustedKeyGroups *ActiveTrustedKeyGroups
	Tags                   map[string]string
}

// DistributionConfig represents CloudFront distribution configuration.
type DistributionConfig struct {
	CallerReference      string
	Origins              *Origins
	DefaultCacheBehavior *DefaultCacheBehavior
	Comment              string
	Enabled              bool
	PriceClass           string
	Aliases              *Aliases
	DefaultRootObject    string
	CacheBehaviors       *CacheBehaviors
	ViewerCertificate    *ViewerCertificate
	HTTPVersion          string
	IsIPV6Enabled        bool
}

// Origins represents the origins for a distribution.
type Origins struct {
	Quantity int
	Items    []Origin
}

// Origin represents a CloudFront origin.
type Origin struct {
	ID                    string
	DomainName            string
	OriginPath            string
	S3OriginConfig        *S3OriginConfig
	CustomOriginConfig    *CustomOriginConfig
	ConnectionAttempts    int
	ConnectionTimeout     int
	OriginAccessControlID string
}

// S3OriginConfig represents S3 origin configuration.
type S3OriginConfig struct {
	OriginAccessIdentity string
}

// CustomOriginConfig represents custom origin configuration.
type CustomOriginConfig struct {
	HTTPPort               int
	HTTPSPort              int
	OriginProtocolPolicy   string
	OriginSSLProtocols     *OriginSSLProtocols
	OriginReadTimeout      int
	OriginKeepaliveTimeout int
}

// OriginSSLProtocols represents allowed SSL protocols for origin.
type OriginSSLProtocols struct {
	Quantity int
	Items    []string
}

// DefaultCacheBehavior represents the default cache behavior.
type DefaultCacheBehavior struct {
	TargetOriginID          string
	ViewerProtocolPolicy    string
	AllowedMethods          *AllowedMethods
	CachedMethods           *CachedMethods
	ForwardedValues         *ForwardedValues
	MinTTL                  int64
	DefaultTTL              int64
	MaxTTL                  int64
	Compress                bool
	SmoothStreaming         bool
	CachePolicyID           string
	OriginRequestPolicyID   string
	ResponseHeadersPolicyID string
	TrustedSigners          *TrustedSigners
	TrustedKeyGroups        *TrustedKeyGroups
	FieldLevelEncryptionID  string
	RealtimeLogConfigArn    string
}

// AllowedMethods represents allowed HTTP methods.
type AllowedMethods struct {
	Quantity int
	Items    []string
}

// CachedMethods represents cached HTTP methods.
type CachedMethods struct {
	Quantity int
	Items    []string
}

// ForwardedValues represents forwarded values configuration.
type ForwardedValues struct {
	QueryString          bool
	Cookies              *CookiePreference
	Headers              *Headers
	QueryStringCacheKeys *QueryStringCacheKeys
}

// CookiePreference represents cookie forwarding preference.
type CookiePreference struct {
	Forward          string
	WhitelistedNames *CookieNames
}

// CookieNames represents a list of cookie names.
type CookieNames struct {
	Quantity int
	Items    []string
}

// Headers represents a list of headers.
type Headers struct {
	Quantity int
	Items    []string
}

// QueryStringCacheKeys represents query string cache keys.
type QueryStringCacheKeys struct {
	Quantity int
	Items    []string
}

// TrustedSigners represents trusted signers.
type TrustedSigners struct {
	Enabled  bool
	Quantity int
	Items    []string
}

// TrustedKeyGroups represents trusted key groups.
type TrustedKeyGroups struct {
	Enabled  bool
	Quantity int
	Items    []string
}

// ActiveTrustedSigners represents active trusted signers.
type ActiveTrustedSigners struct {
	Enabled  bool
	Quantity int
}

// ActiveTrustedKeyGroups represents active trusted key groups.
type ActiveTrustedKeyGroups struct {
	Enabled  bool
	Quantity int
}

// Aliases represents CNAMEs for a distribution.
type Aliases struct {
	Quantity int
	Items    []string
}

// CacheBehaviors represents cache behaviors.
type CacheBehaviors struct {
	Quantity int
	Items    []CacheBehavior
}

// CacheBehavior is an ordered cache behavior: the default behavior's settings
// applied to the requests whose path matches PathPattern.
type CacheBehavior struct {
	PathPattern string
	DefaultCacheBehavior
}

// ViewerCertificate represents viewer certificate configuration.
type ViewerCertificate struct {
	CloudFrontDefaultCertificate bool
	IAMCertificateID             string
	ACMCertificateArn            string
	SSLSupportMethod             string
	MinimumProtocolVersion       string
	Certificate                  string
	CertificateSource            string
}

// Invalidation represents a CloudFront invalidation.
type Invalidation struct {
	ID                string
	Status            string
	CreateTime        time.Time
	InvalidationBatch *InvalidationBatch
}

// InvalidationBatch represents an invalidation batch.
type InvalidationBatch struct {
	Paths           *Paths
	CallerReference string
}

// Paths represents paths to invalidate.
type Paths struct {
	Quantity int
	Items    []string
}

// XML Response Types

// CreateDistributionResult is the response for CreateDistribution.
type CreateDistributionResult struct {
	XMLName      xml.Name     `xml:"Distribution"`
	Xmlns        string       `xml:"xmlns,attr"`
	Distribution Distribution `xml:",innerxml"`
}

// DistributionXML represents a distribution in XML format.
type DistributionXML struct {
	XMLName                xml.Name                   `xml:"Distribution"`
	ID                     string                     `xml:"Id"`
	ARN                    string                     `xml:"ARN"`
	Status                 string                     `xml:"Status"`
	LastModifiedTime       string                     `xml:"LastModifiedTime"`
	DomainName             string                     `xml:"DomainName"`
	ActiveTrustedSigners   *ActiveTrustedSignersXML   `xml:"ActiveTrustedSigners"`
	ActiveTrustedKeyGroups *ActiveTrustedKeyGroupsXML `xml:"ActiveTrustedKeyGroups"`
	DistributionConfig     *DistributionConfigXML     `xml:"DistributionConfig"`
}

// ActiveTrustedSignersXML represents active trusted signers in XML.
type ActiveTrustedSignersXML struct {
	Enabled  bool `xml:"Enabled"`
	Quantity int  `xml:"Quantity"`
}

// ActiveTrustedKeyGroupsXML represents active trusted key groups in XML.
type ActiveTrustedKeyGroupsXML struct {
	Enabled  bool `xml:"Enabled"`
	Quantity int  `xml:"Quantity"`
}

// DistributionConfigXML represents distribution config in XML format.
type DistributionConfigXML struct {
	CallerReference      string                   `xml:"CallerReference"`
	Aliases              *AliasesXML              `xml:"Aliases,omitempty"`
	DefaultRootObject    string                   `xml:"DefaultRootObject,omitempty"`
	Origins              *OriginsXML              `xml:"Origins"`
	OriginGroups         *OriginGroupsXML         `xml:"OriginGroups,omitempty"`
	DefaultCacheBehavior *DefaultCacheBehaviorXML `xml:"DefaultCacheBehavior"`
	CacheBehaviors       *CacheBehaviorsXML       `xml:"CacheBehaviors,omitempty"`
	Comment              string                   `xml:"Comment"`
	Enabled              bool                     `xml:"Enabled"`
	PriceClass           string                   `xml:"PriceClass,omitempty"`
	ViewerCertificate    *ViewerCertificateXML    `xml:"ViewerCertificate,omitempty"`
	HTTPVersion          string                   `xml:"HttpVersion,omitempty"`
	IsIPV6Enabled        bool                     `xml:"IsIPV6Enabled,omitempty"`
}

// OriginGroupsXML represents origin groups in XML format. kumo does not
// implement origin groups, so only the empty form is emitted.
type OriginGroupsXML struct {
	Quantity int `xml:"Quantity"`
}

// AliasesXML represents aliases in XML format.
type AliasesXML struct {
	Quantity int       `xml:"Quantity"`
	Items    *ItemsXML `xml:"Items,omitempty"`
}

// ItemsXML is a generic items container for XML.
type ItemsXML struct {
	Items []string `xml:"CNAME,omitempty"`
}

// OriginsXML represents origins in XML format.
type OriginsXML struct {
	Quantity int         `xml:"Quantity"`
	Items    *OriginList `xml:"Items,omitempty"`
}

// OriginList is a list of origins.
type OriginList struct {
	Origin []OriginXML `xml:"Origin"`
}

// OriginXML represents an origin in XML format.
type OriginXML struct {
	ID                    string                 `xml:"Id"`
	DomainName            string                 `xml:"DomainName"`
	OriginPath            string                 `xml:"OriginPath,omitempty"`
	S3OriginConfig        *S3OriginConfigXML     `xml:"S3OriginConfig,omitempty"`
	CustomOriginConfig    *CustomOriginConfigXML `xml:"CustomOriginConfig,omitempty"`
	ConnectionAttempts    int                    `xml:"ConnectionAttempts,omitempty"`
	ConnectionTimeout     int                    `xml:"ConnectionTimeout,omitempty"`
	OriginAccessControlID string                 `xml:"OriginAccessControlId,omitempty"`
}

// S3OriginConfigXML represents S3 origin config in XML format.
type S3OriginConfigXML struct {
	OriginAccessIdentity string `xml:"OriginAccessIdentity"`
}

// CustomOriginConfigXML represents custom origin config in XML format.
type CustomOriginConfigXML struct {
	HTTPPort               int                    `xml:"HTTPPort"`
	HTTPSPort              int                    `xml:"HTTPSPort"`
	OriginProtocolPolicy   string                 `xml:"OriginProtocolPolicy"`
	OriginSSLProtocols     *OriginSSLProtocolsXML `xml:"OriginSslProtocols,omitempty"`
	OriginReadTimeout      int                    `xml:"OriginReadTimeout,omitempty"`
	OriginKeepaliveTimeout int                    `xml:"OriginKeepaliveTimeout,omitempty"`
}

// OriginSSLProtocolsXML represents origin SSL protocols in XML format.
type OriginSSLProtocolsXML struct {
	Quantity int      `xml:"Quantity"`
	Items    []string `xml:"Items>SslProtocol,omitempty"`
}

// DefaultCacheBehaviorXML represents default cache behavior in XML format.
type DefaultCacheBehaviorXML struct {
	TargetOriginID          string               `xml:"TargetOriginId"`
	ViewerProtocolPolicy    string               `xml:"ViewerProtocolPolicy"`
	AllowedMethods          *AllowedMethodsXML   `xml:"AllowedMethods,omitempty"`
	ForwardedValues         *ForwardedValuesXML  `xml:"ForwardedValues,omitempty"`
	MinTTL                  int64                `xml:"MinTTL,omitempty"`
	DefaultTTL              int64                `xml:"DefaultTTL,omitempty"`
	MaxTTL                  int64                `xml:"MaxTTL,omitempty"`
	Compress                bool                 `xml:"Compress,omitempty"`
	SmoothStreaming         bool                 `xml:"SmoothStreaming,omitempty"`
	CachePolicyID           string               `xml:"CachePolicyId,omitempty"`
	OriginRequestPolicyID   string               `xml:"OriginRequestPolicyId,omitempty"`
	ResponseHeadersPolicyID string               `xml:"ResponseHeadersPolicyId,omitempty"`
	FieldLevelEncryptionID  string               `xml:"FieldLevelEncryptionId,omitempty"`
	RealtimeLogConfigArn    string               `xml:"RealtimeLogConfigArn,omitempty"`
	TrustedSigners          *TrustedSignersXML   `xml:"TrustedSigners,omitempty"`
	TrustedKeyGroups        *TrustedKeyGroupsXML `xml:"TrustedKeyGroups,omitempty"`

	LambdaFunctionAssociations *LambdaFunctionAssociationsXML `xml:"LambdaFunctionAssociations,omitempty"`
	FunctionAssociations       *FunctionAssociationsXML       `xml:"FunctionAssociations,omitempty"`
}

// LambdaFunctionAssociationsXML represents Lambda@Edge associations in XML
// format. kumo does not implement them, so only the empty form is emitted.
type LambdaFunctionAssociationsXML struct {
	Quantity int `xml:"Quantity"`
}

// FunctionAssociationsXML represents CloudFront Function associations in XML
// format. kumo does not implement them, so only the empty form is emitted.
type FunctionAssociationsXML struct {
	Quantity int `xml:"Quantity"`
}

// AllowedMethodsXML represents allowed methods in XML format.
type AllowedMethodsXML struct {
	Quantity      int               `xml:"Quantity"`
	Items         []string          `xml:"Items>Method,omitempty"`
	CachedMethods *CachedMethodsXML `xml:"CachedMethods,omitempty"`
}

// CachedMethodsXML represents cached methods in XML format.
type CachedMethodsXML struct {
	Quantity int      `xml:"Quantity"`
	Items    []string `xml:"Items>Method,omitempty"`
}

// ForwardedValuesXML represents forwarded values in XML format.
type ForwardedValuesXML struct {
	QueryString          bool                     `xml:"QueryString"`
	Cookies              *CookiesXML              `xml:"Cookies"`
	Headers              *HeadersXML              `xml:"Headers,omitempty"`
	QueryStringCacheKeys *QueryStringCacheKeysXML `xml:"QueryStringCacheKeys,omitempty"`
}

// CookiesXML represents cookies configuration in XML format.
type CookiesXML struct {
	Forward          string          `xml:"Forward"`
	WhitelistedNames *CookieNamesXML `xml:"WhitelistedNames,omitempty"`
}

// CookieNamesXML lists whitelisted cookie names in XML format.
type CookieNamesXML struct {
	Quantity int      `xml:"Quantity"`
	Items    []string `xml:"Items>Name,omitempty"`
}

// QueryStringCacheKeysXML lists query string cache keys in XML format.
type QueryStringCacheKeysXML struct {
	Quantity int      `xml:"Quantity"`
	Items    []string `xml:"Items>Name,omitempty"`
}

// HeadersXML represents headers in XML format.
type HeadersXML struct {
	Quantity int      `xml:"Quantity"`
	Items    []string `xml:"Items>Name,omitempty"`
}

// TrustedSignersXML represents trusted signers in XML format.
type TrustedSignersXML struct {
	Enabled  bool     `xml:"Enabled"`
	Quantity int      `xml:"Quantity"`
	Items    []string `xml:"Items>AwsAccountNumber,omitempty"`
}

// TrustedKeyGroupsXML represents trusted key groups in XML format.
type TrustedKeyGroupsXML struct {
	Enabled  bool     `xml:"Enabled"`
	Quantity int      `xml:"Quantity"`
	Items    []string `xml:"Items>KeyGroup,omitempty"`
}

// CacheBehaviorsXML represents cache behaviors in XML format.
type CacheBehaviorsXML struct {
	Quantity int                `xml:"Quantity"`
	Items    []CacheBehaviorXML `xml:"Items>CacheBehavior,omitempty"`
}

// CacheBehaviorXML is an ordered cache behavior in XML format: PathPattern
// plus the default behavior's fields.
type CacheBehaviorXML struct {
	PathPattern string `xml:"PathPattern"`
	DefaultCacheBehaviorXML
}

// RestrictionsXML represents restrictions in XML format.
type RestrictionsXML struct {
	GeoRestriction *GeoRestrictionXML `xml:"GeoRestriction"`
}

// GeoRestrictionXML represents geo restriction in XML format.
type GeoRestrictionXML struct {
	RestrictionType string   `xml:"RestrictionType"`
	Quantity        int      `xml:"Quantity"`
	Items           []string `xml:"Items>Location,omitempty"`
}

// CustomErrorResponsesXML represents custom error responses in XML format.
type CustomErrorResponsesXML struct {
	Quantity int `xml:"Quantity"`
}

// ViewerCertificateXML represents viewer certificate in XML format.
type ViewerCertificateXML struct {
	CloudFrontDefaultCertificate bool   `xml:"CloudFrontDefaultCertificate,omitempty"`
	IAMCertificateID             string `xml:"IAMCertificateId,omitempty"`
	ACMCertificateArn            string `xml:"ACMCertificateArn,omitempty"`
	MinimumProtocolVersion       string `xml:"MinimumProtocolVersion,omitempty"`
	SSLSupportMethod             string `xml:"SSLSupportMethod,omitempty"`
}

// DistributionListXML represents a list of distributions in XML format.
type DistributionListXML struct {
	XMLName     xml.Name                 `xml:"DistributionList"`
	Xmlns       string                   `xml:"xmlns,attr"`
	Marker      string                   `xml:"Marker"`
	MaxItems    int                      `xml:"MaxItems"`
	IsTruncated bool                     `xml:"IsTruncated"`
	Quantity    int                      `xml:"Quantity"`
	Items       *DistributionSummaryList `xml:"Items,omitempty"`
	NextMarker  string                   `xml:"NextMarker,omitempty"`
}

// DistributionSummaryList is a list of distribution summaries.
type DistributionSummaryList struct {
	DistributionSummary []DistributionSummaryXML `xml:"DistributionSummary"`
}

// DistributionSummaryXML represents a distribution summary in XML format.
type DistributionSummaryXML struct {
	ID                   string                   `xml:"Id"`
	ARN                  string                   `xml:"ARN"`
	Status               string                   `xml:"Status"`
	LastModifiedTime     string                   `xml:"LastModifiedTime"`
	DomainName           string                   `xml:"DomainName"`
	Aliases              *AliasesXML              `xml:"Aliases"`
	Origins              *OriginsXML              `xml:"Origins"`
	DefaultCacheBehavior *DefaultCacheBehaviorXML `xml:"DefaultCacheBehavior"`
	CacheBehaviors       *CacheBehaviorsXML       `xml:"CacheBehaviors"`
	CustomErrorResponses *CustomErrorResponsesXML `xml:"CustomErrorResponses"`
	Comment              string                   `xml:"Comment"`
	PriceClass           string                   `xml:"PriceClass"`
	Enabled              bool                     `xml:"Enabled"`
	ViewerCertificate    *ViewerCertificateXML    `xml:"ViewerCertificate"`
	Restrictions         *RestrictionsXML         `xml:"Restrictions"`
	WebACLId             string                   `xml:"WebACLId"`
	HTTPVersion          string                   `xml:"HttpVersion"`
	IsIPV6Enabled        bool                     `xml:"IsIPV6Enabled"`
	Staging              bool                     `xml:"Staging"`
}

// GetDistributionResult is the response for GetDistribution.
type GetDistributionResult struct {
	XMLName                xml.Name                   `xml:"Distribution"`
	Xmlns                  string                     `xml:"xmlns,attr"`
	ID                     string                     `xml:"Id"`
	ARN                    string                     `xml:"ARN"`
	Status                 string                     `xml:"Status"`
	LastModifiedTime       string                     `xml:"LastModifiedTime"`
	DomainName             string                     `xml:"DomainName"`
	ActiveTrustedSigners   *ActiveTrustedSignersXML   `xml:"ActiveTrustedSigners"`
	ActiveTrustedKeyGroups *ActiveTrustedKeyGroupsXML `xml:"ActiveTrustedKeyGroups"`
	DistributionConfig     *DistributionConfigXML     `xml:"DistributionConfig"`
}

// CreateDistributionRequest is the request for CreateDistribution.
type CreateDistributionRequest struct {
	XMLName              xml.Name                 `xml:"DistributionConfig"`
	CallerReference      string                   `xml:"CallerReference"`
	Aliases              *AliasesXML              `xml:"Aliases,omitempty"`
	DefaultRootObject    string                   `xml:"DefaultRootObject,omitempty"`
	Origins              *OriginsXML              `xml:"Origins"`
	DefaultCacheBehavior *DefaultCacheBehaviorXML `xml:"DefaultCacheBehavior"`
	CacheBehaviors       *CacheBehaviorsXML       `xml:"CacheBehaviors,omitempty"`
	Comment              string                   `xml:"Comment"`
	Enabled              bool                     `xml:"Enabled"`
	PriceClass           string                   `xml:"PriceClass,omitempty"`
	ViewerCertificate    *ViewerCertificateXML    `xml:"ViewerCertificate,omitempty"`
	HTTPVersion          string                   `xml:"HttpVersion,omitempty"`
	IsIPV6Enabled        bool                     `xml:"IsIPV6Enabled,omitempty"`
}

// InvalidationXML represents an invalidation in XML format.
type InvalidationXML struct {
	XMLName           xml.Name              `xml:"Invalidation"`
	ID                string                `xml:"Id"`
	Status            string                `xml:"Status"`
	CreateTime        string                `xml:"CreateTime"`
	InvalidationBatch *InvalidationBatchXML `xml:"InvalidationBatch"`
}

// InvalidationBatchXML represents an invalidation batch in XML format.
type InvalidationBatchXML struct {
	Paths           *PathsXML `xml:"Paths"`
	CallerReference string    `xml:"CallerReference"`
}

// PathsXML represents paths in XML format.
type PathsXML struct {
	Quantity int      `xml:"Quantity"`
	Items    []string `xml:"Items>Path,omitempty"`
}

// CreateInvalidationRequest is the request for CreateInvalidation.
type CreateInvalidationRequest struct {
	XMLName         xml.Name  `xml:"InvalidationBatch"`
	Paths           *PathsXML `xml:"Paths"`
	CallerReference string    `xml:"CallerReference"`
}

// InvalidationListXML represents a list of invalidations in XML format.
type InvalidationListXML struct {
	XMLName     xml.Name                 `xml:"InvalidationList"`
	Xmlns       string                   `xml:"xmlns,attr"`
	Marker      string                   `xml:"Marker"`
	MaxItems    int                      `xml:"MaxItems"`
	IsTruncated bool                     `xml:"IsTruncated"`
	Quantity    int                      `xml:"Quantity"`
	Items       *InvalidationSummaryList `xml:"Items,omitempty"`
	NextMarker  string                   `xml:"NextMarker,omitempty"`
}

// InvalidationSummaryList is a list of invalidation summaries.
type InvalidationSummaryList struct {
	InvalidationSummary []InvalidationSummaryXML `xml:"InvalidationSummary"`
}

// InvalidationSummaryXML represents an invalidation summary in XML format.
type InvalidationSummaryXML struct {
	ID         string `xml:"Id"`
	CreateTime string `xml:"CreateTime"`
	Status     string `xml:"Status"`
}

// ErrorResponse represents a CloudFront error response.
type ErrorResponse struct {
	XMLName   xml.Name    `xml:"ErrorResponse"`
	Xmlns     string      `xml:"xmlns,attr"`
	Error     ErrorDetail `xml:"Error"`
	RequestID string      `xml:"RequestId"`
}

// ErrorDetail represents the error detail.
type ErrorDetail struct {
	Type    string `xml:"Type"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// HTTP methods CloudFront allows and caches when a behavior does not say.
const (
	methodGet  = "GET"
	methodHead = "HEAD"
)

// CloudFront error codes.
const (
	errDistributionNotFound      = "NoSuchDistribution"
	errDistributionAlreadyExists = "DistributionAlreadyExists"
	errInvalidArgument           = "InvalidArgument"
	errMissingBody               = "MissingBody"
	errAccessDenied              = "AccessDenied"
	errPreconditionFailed        = "PreconditionFailed"
	errInvalidIfMatchVersion     = "InvalidIfMatchVersion"
	errNoSuchInvalidation        = "NoSuchInvalidation"
	errNoSuchOrigin              = "NoSuchOrigin"
	errNoSuchResource            = "NoSuchResource"
)
