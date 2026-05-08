package cloudcontrol

import (
	"encoding/json"
	"errors"
	"io"
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
// compatibility. EventTime is encoded as Unix-epoch seconds (a float)
// because the AWS JSON 1.0 protocol decodes timestamps as numbers; an
// RFC3339 string trips the SDK's `expected Timestamp to be a JSON Number`
// check.
type ProgressEvent struct {
	TypeName        string  `json:"TypeName,omitempty"`
	Identifier      string  `json:"Identifier,omitempty"`
	RequestToken    string  `json:"RequestToken,omitempty"`
	Operation       string  `json:"Operation,omitempty"`
	OperationStatus string  `json:"OperationStatus,omitempty"`
	EventTime       float64 `json:"EventTime,omitempty"`
	ResourceModel   string  `json:"ResourceModel,omitempty"`
	StatusMessage   string  `json:"StatusMessage,omitempty"`
	ErrorCode       string  `json:"ErrorCode,omitempty"`
}

// nowEpoch returns the current time encoded the way the AWS JSON 1.0
// protocol expects timestamps: Unix-epoch seconds with sub-second
// precision in the fractional part.
func nowEpoch() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}

// ProgressEventOutput is the response envelope shared by CreateResource,
// UpdateResource and DeleteResource. All three actions return a single
// ProgressEvent on success.
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
// is what AWS SDK clients use to populate the error name. Cloud Control
// always returns 400 for application errors; transport errors don't
// flow through here.
func writeError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"__type":  code,
		"message": message,
	})
}

// maxRequestBodyBytes caps the JSON request bodies we accept. Without
// this, an attacker streaming a multi-GB body would have it buffered
// in memory before the JSON decoder rejects it.
const maxRequestBodyBytes = 1 * 1024 * 1024

// readJSON decodes the request body into v. Returns the decoder error
// directly; callers wrap it for writeError. The body is capped at
// maxRequestBodyBytes so we don't read unbounded request streams.
func readJSON(r *http.Request, v any) error {
	limited := io.LimitReader(r.Body, maxRequestBodyBytes)

	return json.NewDecoder(limited).Decode(v)
}
