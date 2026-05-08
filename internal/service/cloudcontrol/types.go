package cloudcontrol

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// NotFoundError is returned by Handler.Read / Update / Delete when the
// resource doesn't exist. The dispatcher translates it into Cloud Control's
// "ResourceNotFoundException".
type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }

// IsNotFound reports whether err is (or wraps) a NotFoundError.
func IsNotFound(err error) bool {
	var nfe *NotFoundError

	return errors.As(err, &nfe)
}

// CreateResourceInput is the JSON body Cloud Control's CreateResource
// receives. Only the fields kumo actually uses are modeled.
type CreateResourceInput struct {
	TypeName     string `json:"TypeName"`
	DesiredState string `json:"DesiredState"`
	ClientToken  string `json:"ClientToken,omitempty"`
}

// GetResourceInput is the JSON body for GetResource.
type GetResourceInput struct {
	TypeName   string `json:"TypeName"`
	Identifier string `json:"Identifier"`
}

// UpdateResourceInput is the JSON body for UpdateResource. PatchDocument
// is an RFC 6902 JSON Patch as a string.
type UpdateResourceInput struct {
	TypeName      string `json:"TypeName"`
	Identifier    string `json:"Identifier"`
	PatchDocument string `json:"PatchDocument"`
	ClientToken   string `json:"ClientToken,omitempty"`
}

// DeleteResourceInput is the JSON body for DeleteResource.
type DeleteResourceInput struct {
	TypeName    string `json:"TypeName"`
	Identifier  string `json:"Identifier"`
	ClientToken string `json:"ClientToken,omitempty"`
}

// ListResourcesInput is the JSON body for ListResources.
type ListResourcesInput struct {
	TypeName   string `json:"TypeName"`
	MaxResults int    `json:"MaxResults,omitempty"`
	NextToken  string `json:"NextToken,omitempty"`
}

// GetResourceRequestStatusInput is the JSON body for status polling.
type GetResourceRequestStatusInput struct {
	RequestToken string `json:"RequestToken"`
}

// ProgressEvent is the wire shape Cloud Control returns from every
// asynchronous operation. kumo runs all operations synchronously, so we
// always return SUCCESS — the field is still populated for SDK
// compatibility.
type ProgressEvent struct {
	TypeName        string    `json:"TypeName,omitempty"`
	Identifier      string    `json:"Identifier,omitempty"`
	RequestToken    string    `json:"RequestToken,omitempty"`
	Operation       string    `json:"Operation,omitempty"`
	OperationStatus string    `json:"OperationStatus,omitempty"`
	EventTime       time.Time `json:"EventTime,omitempty"`
	ResourceModel   string    `json:"ResourceModel,omitempty"`
	StatusMessage   string    `json:"StatusMessage,omitempty"`
	ErrorCode       string    `json:"ErrorCode,omitempty"`
}

// CreateResourceOutput / UpdateResourceOutput / DeleteResourceOutput all
// share the same shape: a single ProgressEvent.
type ProgressEventOutput struct {
	ProgressEvent ProgressEvent `json:"ProgressEvent"`
}

// ResourceDescriptionWire is the wire shape for Get/List entries.
// Properties is a JSON document encoded as a string.
type ResourceDescriptionWire struct {
	Identifier string `json:"Identifier"`
	Properties string `json:"Properties"`
}

// GetResourceOutput is the response for GetResource.
type GetResourceOutput struct {
	TypeName            string                  `json:"TypeName"`
	ResourceDescription ResourceDescriptionWire `json:"ResourceDescription"`
}

// ListResourcesOutput is the response for ListResources.
type ListResourcesOutput struct {
	TypeName             string                    `json:"TypeName"`
	ResourceDescriptions []ResourceDescriptionWire `json:"ResourceDescriptions"`
	NextToken            string                    `json:"NextToken,omitempty"`
}

// writeJSON writes a JSON 1.0 response with the given body.
func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError writes an AWS JSON error response. code becomes __type and
// is what AWS SDK clients use to populate the error name.
func writeError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"__type":  code,
		"message": message,
	})
}

// readJSON decodes the request body into v. Returns an error string the
// caller can pass straight to writeError.
func readJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return err
	}

	return nil
}
