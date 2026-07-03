package cloudfront

import (
	"encoding/xml"
	"time"
)

// Function is the in-memory representation of a CloudFront Function. A
// function exists in two parallel stages — DEVELOPMENT (the most recent
// edit) and LIVE (the published copy that real CloudFront would route
// traffic through). PublishFunction promotes DEVELOPMENT to LIVE.
type Function struct {
	Name             string
	Runtime          string // cloudfront-js-1.0 | cloudfront-js-2.0
	Comment          string
	CreatedTime      time.Time
	LastModifiedTime time.Time

	// Code holds the function source per stage. The DEVELOPMENT entry is
	// always present after Create / Update; the LIVE entry only after
	// PublishFunction. Real CloudFront returns the bytes verbatim
	// (decoded from base64); kumo stores the decoded form.
	Code map[string][]byte

	// ETag is the opaque version identifier the API surfaces in
	// response headers and requires on Update / Delete / Publish via
	// If-Match. kumo regenerates it on every mutation.
	ETag string
}

// FunctionStageDevelopment / FunctionStageLive are the two stages real
// CloudFront exposes. ListFunctions / GetFunction / DescribeFunction /
// TestFunction all accept Stage as a query parameter.
const (
	FunctionStageDevelopment = "DEVELOPMENT"
	FunctionStageLive        = "LIVE"
)

// xmlCreateFunctionRequest is the request body shape for both
// CreateFunction (POST /function) and UpdateFunction (PUT /function/{Name}).
type xmlCreateFunctionRequest struct {
	XMLName        xml.Name          `xml:"CreateFunctionRequest"`
	Name           string            `xml:"Name"`
	FunctionConfig xmlFunctionConfig `xml:"FunctionConfig"`
	FunctionCode   string            `xml:"FunctionCode"`
}

// xmlUpdateFunctionRequest is shape-equal but the wrapping element
// differs ("UpdateFunctionRequest"); a separate type keeps the XML
// element name correct for unmarshalling.
type xmlUpdateFunctionRequest struct {
	XMLName        xml.Name          `xml:"UpdateFunctionRequest"`
	Name           string            `xml:"Name"`
	FunctionConfig xmlFunctionConfig `xml:"FunctionConfig"`
	FunctionCode   string            `xml:"FunctionCode"`
}

type xmlFunctionConfig struct {
	Comment string `xml:"Comment"`
	Runtime string `xml:"Runtime"`
}

// xmlFunctionSummary is the AWS wire shape for a function. It is
// returned from Create / Describe / List / Update / Publish — the four
// endpoints all share this shape.
type xmlFunctionSummary struct {
	XMLName          xml.Name            `xml:"FunctionSummary"`
	Xmlns            string              `xml:"xmlns,attr,omitempty"`
	Name             string              `xml:"Name"`
	Status           string              `xml:"Status"`
	FunctionConfig   xmlFunctionConfig   `xml:"FunctionConfig"`
	FunctionMetadata xmlFunctionMetadata `xml:"FunctionMetadata"`
}

type xmlFunctionMetadata struct {
	FunctionARN      string `xml:"FunctionARN"`
	Stage            string `xml:"Stage"`
	CreatedTime      string `xml:"CreatedTime"`
	LastModifiedTime string `xml:"LastModifiedTime"`
}

// xmlListFunctionsResult is the root body for ListFunctions. AWS uses a
// FunctionList element with a list of FunctionSummary children.
type xmlListFunctionsResult struct {
	XMLName    xml.Name             `xml:"FunctionList"`
	Xmlns      string               `xml:"xmlns,attr,omitempty"`
	NextMarker string               `xml:"NextMarker,omitempty"`
	MaxItems   int                  `xml:"MaxItems"`
	Quantity   int                  `xml:"Quantity"`
	Items      []xmlFunctionSummary `xml:"Items>FunctionSummary"`
}

// xmlTestFunctionRequest is the body shape for TestFunction. EventObject
// is a base64-encoded JSON event (the same JSON CloudFront passes to
// the function at the edge); Stage is the stage to run against.
type xmlTestFunctionRequest struct {
	XMLName     xml.Name `xml:"TestFunctionRequest"`
	IfMatch     string   `xml:"-"`
	Stage       string   `xml:"Stage"`
	EventObject string   `xml:"EventObject"`
}

type xmlTestResult struct {
	XMLName               xml.Name           `xml:"TestResult"`
	Xmlns                 string             `xml:"xmlns,attr,omitempty"`
	FunctionSummary       xmlFunctionSummary `xml:"FunctionSummary"`
	ComputeUtilization    string             `xml:"ComputeUtilization"`
	FunctionExecutionLogs xmlFunctionLogList `xml:"FunctionExecutionLogs"`
	FunctionErrorMessage  string             `xml:"FunctionErrorMessage,omitempty"`
	FunctionOutput        string             `xml:"FunctionOutput,omitempty"`
}

type xmlFunctionLogList struct {
	Items []string `xml:"Items>member"`
}
