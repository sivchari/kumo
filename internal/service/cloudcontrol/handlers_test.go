package cloudcontrol

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sivchari/kumo/internal/service/iam"
)

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

func TestIsIAMNotFoundUsesTypedIAMError(t *testing.T) {
	if !isIAMNotFound(&iam.Error{Code: "NoSuchEntity", Message: "missing"}) {
		t.Fatalf("NoSuchEntity iam.Error should be treated as not found")
	}

	if isIAMNotFound(errors.New("role not found")) {
		t.Fatalf("plain text errors should not be treated as IAM not found")
	}
}
