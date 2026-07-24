package amp

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// CreateWorkspaceRequest is the JSON contract for POST /workspaces.
type CreateWorkspaceRequest struct {
	Alias string            `json:"alias"`
	Tags  map[string]string `json:"tags"`
}

// CreateWorkspaceResponse is the JSON contract for CreateWorkspace.
type CreateWorkspaceResponse struct {
	WorkspaceID string            `json:"workspaceId"`
	Arn         string            `json:"arn"`
	Status      Status            `json:"status"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// ListWorkspacesResponse is the JSON contract for ListWorkspaces.
type ListWorkspacesResponse struct {
	Workspaces []WorkspaceSummary `json:"workspaces"`
}

// DescribeWorkspaceResponse is the JSON contract for DescribeWorkspace.
type DescribeWorkspaceResponse struct {
	Workspace WorkspaceDescription `json:"workspace"`
}

// Workspace is one AMP workspace.
type Workspace struct {
	WorkspaceID        string            `json:"workspaceId"`
	Alias              string            `json:"alias,omitempty"`
	Arn                string            `json:"arn"`
	Status             Status            `json:"status"`
	PrometheusEndpoint string            `json:"prometheusEndpoint"`
	CreatedAt          time.Time         `json:"createdAt"`
	Tags               map[string]string `json:"tags,omitempty"`
}

// WorkspaceDescription is the wire shape returned by DescribeWorkspace.
type WorkspaceDescription struct {
	WorkspaceID        string            `json:"workspaceId"`
	Alias              string            `json:"alias,omitempty"`
	Arn                string            `json:"arn"`
	Status             Status            `json:"status"`
	PrometheusEndpoint string            `json:"prometheusEndpoint"`
	CreatedAt          AWSTimestamp      `json:"createdAt"`
	Tags               map[string]string `json:"tags,omitempty"`
}

// WorkspaceSummary is the smaller workspace shape returned by ListWorkspaces.
type WorkspaceSummary struct {
	WorkspaceID string            `json:"workspaceId"`
	Alias       string            `json:"alias,omitempty"`
	Arn         string            `json:"arn"`
	Status      Status            `json:"status"`
	CreatedAt   AWSTimestamp      `json:"createdAt"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// Status is the workspace lifecycle status.
type Status struct {
	StatusCode string `json:"statusCode"`
}

// ErrorResponse is the AWS-shaped error body.
type ErrorResponse struct {
	Message string `json:"message"`
}

// AWSTimestamp marshals Smithy timestamp values in the epoch-seconds
// JSON format expected by the AWS SDK for AMP.
type AWSTimestamp struct {
	time.Time
}

// MarshalJSON implements json.Marshaler.
func (t AWSTimestamp) MarshalJSON() ([]byte, error) {
	seconds := float64(t.Unix()) + float64(t.Nanosecond())/1e9

	return json.Marshal(seconds) //nolint:wrapcheck // MarshalJSON interface requirement
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *AWSTimestamp) UnmarshalJSON(data []byte) error {
	var seconds float64
	if err := json.Unmarshal(data, &seconds); err == nil {
		sec, frac := math.Modf(seconds)
		t.Time = time.Unix(int64(sec), int64(frac*1e9)).UTC()

		return nil
	}

	var formatted string
	if err := json.Unmarshal(data, &formatted); err != nil {
		return fmt.Errorf("unmarshal timestamp: %w", err)
	}

	parsed, err := time.Parse(time.RFC3339Nano, formatted)
	if err != nil {
		return fmt.Errorf("parse timestamp: %w", err)
	}

	t.Time = parsed

	return nil
}

// Error is the AWS-shaped storage error.
type Error struct {
	Code    string
	Message string
}

// Error implements error.
func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}
