package cloudfront

import (
	"encoding/xml"
	"time"
)

// CloudFront signing-related error codes.
const (
	errNoSuchPublicKey         = "NoSuchPublicKey"
	errNoSuchKeyGroup          = "NoSuchResource"
	errPublicKeyAlreadyExists  = "PublicKeyAlreadyExists"
	errKeyGroupAlreadyExists   = "KeyGroupAlreadyExists"
	errPublicKeyInUse          = "PublicKeyInUse"
	errKeyGroupReferencedError = "ResourceInUse"
)

// PublicKey is a CloudFront-managed RSA public key. Used as a building
// block for signed URL / signed cookie verification: a KeyGroup
// references one or more PublicKeys, and a Distribution references
// KeyGroups via TrustedKeyGroups on a CacheBehavior.
type PublicKey struct {
	ID              string
	CreatedTime     time.Time
	ETag            string
	PublicKeyConfig *PublicKeyConfig
}

// PublicKeyConfig is the user-supplied configuration of a PublicKey.
type PublicKeyConfig struct {
	CallerReference string
	Name            string
	EncodedKey      string // PEM-encoded RSA public key
	Comment         string
}

// KeyGroup groups one or more PublicKey IDs that a Distribution can
// reference via TrustedKeyGroups.
type KeyGroup struct {
	ID             string
	LastModified   time.Time
	ETag           string
	KeyGroupConfig *KeyGroupConfig
}

// KeyGroupConfig is the user-supplied configuration of a KeyGroup.
type KeyGroupConfig struct {
	Name    string
	Items   []string // PublicKey IDs
	Comment string
}

// Wire (XML) types for the CloudFront 2020-05-31 API.

// PublicKeyConfigXML is the XML view of PublicKeyConfig — also used as
// the request body for CreatePublicKey.
type PublicKeyConfigXML struct {
	XMLName         xml.Name `xml:"PublicKeyConfig"`
	Xmlns           string   `xml:"xmlns,attr,omitempty"`
	CallerReference string   `xml:"CallerReference"`
	Name            string   `xml:"Name"`
	EncodedKey      string   `xml:"EncodedKey"`
	Comment         string   `xml:"Comment,omitempty"`
}

// PublicKeyResultXML is the XML body of GetPublicKey / CreatePublicKey
// responses.
type PublicKeyResultXML struct {
	XMLName         xml.Name            `xml:"PublicKey"`
	Xmlns           string              `xml:"xmlns,attr"`
	ID              string              `xml:"Id"`
	CreatedTime     string              `xml:"CreatedTime"`
	PublicKeyConfig *PublicKeyConfigXML `xml:"PublicKeyConfig"`
}

// PublicKeyListXML is the body of ListPublicKeys.
type PublicKeyListXML struct {
	XMLName  xml.Name              `xml:"PublicKeyList"`
	Xmlns    string                `xml:"xmlns,attr"`
	MaxItems int                   `xml:"MaxItems"`
	Quantity int                   `xml:"Quantity"`
	Items    []PublicKeySummaryXML `xml:"Items>PublicKeySummary,omitempty"`
}

// PublicKeySummaryXML is one item in PublicKeyList.
type PublicKeySummaryXML struct {
	ID          string `xml:"Id"`
	Name        string `xml:"Name"`
	CreatedTime string `xml:"CreatedTime"`
	EncodedKey  string `xml:"EncodedKey"`
	Comment     string `xml:"Comment,omitempty"`
}

// KeyGroupConfigXML is the XML view of KeyGroupConfig.
type KeyGroupConfigXML struct {
	XMLName xml.Name `xml:"KeyGroupConfig"`
	Xmlns   string   `xml:"xmlns,attr,omitempty"`
	Name    string   `xml:"Name"`
	Items   []string `xml:"Items>PublicKey"`
	Comment string   `xml:"Comment,omitempty"`
}

// KeyGroupResultXML is the XML body of GetKeyGroup / CreateKeyGroup
// responses.
type KeyGroupResultXML struct {
	XMLName        xml.Name           `xml:"KeyGroup"`
	Xmlns          string             `xml:"xmlns,attr"`
	ID             string             `xml:"Id"`
	LastModified   string             `xml:"LastModifiedTime"`
	KeyGroupConfig *KeyGroupConfigXML `xml:"KeyGroupConfig"`
}

// KeyGroupListXML is the body of ListKeyGroups.
type KeyGroupListXML struct {
	XMLName  xml.Name             `xml:"KeyGroupList"`
	Xmlns    string               `xml:"xmlns,attr"`
	MaxItems int                  `xml:"MaxItems"`
	Quantity int                  `xml:"Quantity"`
	Items    []KeyGroupSummaryXML `xml:"Items>KeyGroupSummary,omitempty"`
}

// KeyGroupSummaryXML is one item in KeyGroupList.
type KeyGroupSummaryXML struct {
	KeyGroup KeyGroupResultXML `xml:"KeyGroup"`
}
