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

// xmlCreateFunctionResult is the body wrapper returned from
// CreateFunction. Update / Publish use the same shape but with their
// own outer element names.
type xmlCreateFunctionResult struct {
	XMLName         xml.Name           `xml:"CreateFunctionResult"`
	Xmlns           string             `xml:"xmlns,attr"`
	FunctionSummary xmlFunctionSummary `xml:"FunctionSummary"`
}

type xmlUpdateFunctionResult struct {
	XMLName         xml.Name           `xml:"UpdateFunctionResult"`
	Xmlns           string             `xml:"xmlns,attr"`
	FunctionSummary xmlFunctionSummary `xml:"FunctionSummary"`
}

type xmlPublishFunctionResult struct {
	XMLName         xml.Name           `xml:"PublishFunctionResult"`
	Xmlns           string             `xml:"xmlns,attr"`
	FunctionSummary xmlFunctionSummary `xml:"FunctionSummary"`
}

type xmlDescribeFunctionResult struct {
	XMLName         xml.Name           `xml:"DescribeFunctionResult"`
	Xmlns           string             `xml:"xmlns,attr"`
	FunctionSummary xmlFunctionSummary `xml:"FunctionSummary"`
}

// xmlListFunctionsResult is the wrapper for ListFunctions. AWS uses an
// inner FunctionList element with a list of FunctionSummary children.
type xmlListFunctionsResult struct {
	XMLName      xml.Name        `xml:"ListFunctionsResult"`
	Xmlns        string          `xml:"xmlns,attr"`
	FunctionList xmlFunctionList `xml:"FunctionList"`
}

type xmlFunctionList struct {
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

// xmlTestFunctionResult mirrors the AWS shape: a TestResult element
// with the function's stdout-equivalent (FunctionExecutionLogList),
// the output JSON, runtime, compute usage, and any error message.
type xmlTestFunctionResult struct {
	XMLName    xml.Name      `xml:"TestFunctionResult"`
	Xmlns      string        `xml:"xmlns,attr"`
	TestResult xmlTestResult `xml:"TestResult"`
}

type xmlTestResult struct {
	FunctionSummary          xmlFunctionSummary `xml:"FunctionSummary"`
	ComputeUtilization       string             `xml:"ComputeUtilization"`
	FunctionExecutionLogList xmlFunctionLogList `xml:"FunctionExecutionLogList"`
	FunctionErrorMessage     string             `xml:"FunctionErrorMessage,omitempty"`
	FunctionOutput           string             `xml:"FunctionOutput,omitempty"`
}

type xmlFunctionLogList struct {
	Items []string `xml:"Items>member"`
}
