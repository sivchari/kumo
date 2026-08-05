// Package awsapi contains normalized request metadata shared by kumo runtime features.
package awsapi

// RequestInfo is the normalized identity of an emulated AWS API request.
type RequestInfo struct {
	Service  string `json:"service,omitempty"`
	Action   string `json:"action,omitempty"`
	Method   string `json:"method,omitempty"`
	Path     string `json:"path,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Resource string `json:"resource,omitempty"`
}
