package cloudcontrol

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubHandler is a Handler implementation backed by an in-memory map. It
// keeps the cloudcontrol package's test self-contained without pulling in
// the s3 service.
type stubHandler struct {
	state map[string][]byte
}

func newStubHandler() *stubHandler { return &stubHandler{state: make(map[string][]byte)} }

func (*stubHandler) TypeName() string { return "Kumo::Test::Resource" }

func (h *stubHandler) Create(_ context.Context, desired []byte) (string, []byte, error) {
	var props struct {
		Name string `json:"Name"`
	}

	if err := json.Unmarshal(desired, &props); err != nil {
		return "", nil, err
	}

	h.state[props.Name] = desired

	return props.Name, desired, nil
}

func (h *stubHandler) Read(_ context.Context, id string) ([]byte, error) {
	state, ok := h.state[id]
	if !ok {
		return nil, &NotFoundError{Message: id + " not found"}
	}

	return state, nil
}

func (h *stubHandler) Update(_ context.Context, id string, _ []byte) ([]byte, error) {
	state, ok := h.state[id]
	if !ok {
		return nil, &NotFoundError{Message: id + " not found"}
	}

	return state, nil
}

func (h *stubHandler) Delete(_ context.Context, id string) error {
	if _, ok := h.state[id]; !ok {
		return &NotFoundError{Message: id + " not found"}
	}

	delete(h.state, id)

	return nil
}

func (h *stubHandler) List(_ context.Context) ([]ResourceDescription, error) {
	out := make([]ResourceDescription, 0, len(h.state))
	for id, props := range h.state {
		out = append(out, ResourceDescription{Identifier: id, Properties: props})
	}

	return out, nil
}

// post simulates an SDK request: the X-Amz-Target header sets the
// dispatch action, the body is the JSON request envelope.
func post(t *testing.T, svc *Service, target, body string) (int, string) {
	t.Helper()

	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", target)

	rec := httptest.NewRecorder()
	svc.DispatchAction(rec, req)

	return rec.Code, rec.Body.String()
}

func TestCloudControl_LifecycleViaStubHandler(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newStubHandler())
	svc := New(reg)

	code, body := post(t, svc,
		"CloudApiService.CreateResource",
		`{"TypeName":"Kumo::Test::Resource","DesiredState":"{\"Name\":\"alpha\"}"}`,
	)
	if code != 200 || !strings.Contains(body, `"Identifier":"alpha"`) || !strings.Contains(body, `"OperationStatus":"SUCCESS"`) {
		t.Fatalf("Create: code=%d body=%s", code, body)
	}

	code, body = post(t, svc,
		"CloudApiService.GetResource",
		`{"TypeName":"Kumo::Test::Resource","Identifier":"alpha"}`,
	)
	if code != 200 || !strings.Contains(body, `"Identifier":"alpha"`) {
		t.Fatalf("Get: code=%d body=%s", code, body)
	}

	code, body = post(t, svc,
		"CloudApiService.ListResources",
		`{"TypeName":"Kumo::Test::Resource"}`,
	)
	if code != 200 || !strings.Contains(body, `"alpha"`) {
		t.Fatalf("List: code=%d body=%s", code, body)
	}

	code, body = post(t, svc,
		"CloudApiService.DeleteResource",
		`{"TypeName":"Kumo::Test::Resource","Identifier":"alpha"}`,
	)
	if code != 200 || !strings.Contains(body, `"Operation":"DELETE"`) {
		t.Fatalf("Delete: code=%d body=%s", code, body)
	}

	code, body = post(t, svc,
		"CloudApiService.GetResource",
		`{"TypeName":"Kumo::Test::Resource","Identifier":"alpha"}`,
	)
	if code != 400 || !strings.Contains(body, "ResourceNotFoundException") {
		t.Fatalf("Get-after-delete: code=%d body=%s", code, body)
	}
}

func TestCloudControl_TypeNotRegistered(t *testing.T) {
	svc := New(NewRegistry())

	code, body := post(t, svc,
		"CloudApiService.CreateResource",
		`{"TypeName":"AWS::Imaginary::Type","DesiredState":"{}"}`,
	)
	if code != 400 || !strings.Contains(body, "TypeNotFoundException") {
		t.Fatalf("expected TypeNotFoundException, got code=%d body=%s", code, body)
	}
}

func TestCloudControl_DefaultRegistryRegistersBuiltinTypes(t *testing.T) {
	reg := defaultRegistry()

	for _, want := range []string{
		"AWS::S3::Bucket",
		"AWS::EC2::VPC",
		"AWS::EC2::Subnet",
		"AWS::IAM::Role",
	} {
		if _, ok := reg.Get(want); !ok {
			t.Errorf("default registry missing handler for %s", want)
		}
	}
}

func TestCloudControl_UnknownAction(t *testing.T) {
	svc := New(NewRegistry())

	code, body := post(t, svc,
		"CloudApiService.NotARealAction",
		`{}`,
	)
	if code != 400 || !strings.Contains(body, "InvalidAction") {
		t.Fatalf("expected InvalidAction, got code=%d body=%s", code, body)
	}
}

func TestCloudControl_GetResourceRequestStatus_AlwaysSuccess(t *testing.T) {
	svc := New(NewRegistry())

	code, body := post(t, svc,
		"CloudApiService.GetResourceRequestStatus",
		`{"RequestToken":"any-token"}`,
	)
	if code != 200 || !strings.Contains(body, `"OperationStatus":"SUCCESS"`) || !strings.Contains(body, `"any-token"`) {
		t.Fatalf("status: code=%d body=%s", code, body)
	}
}
