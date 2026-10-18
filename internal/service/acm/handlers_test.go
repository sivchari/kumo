package acm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func dispatch(t *testing.T, svc *Service, operation, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("X-Amz-Target", "CertificateManager."+operation)
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	return w
}

func TestDispatchAction_TagOperations(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	w := dispatch(t, svc, "RequestCertificate", `{"DomainName":"example.com"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("RequestCertificate: status %d body=%s", w.Code, w.Body.String())
	}

	var requested RequestCertificateOutput
	if err := json.Unmarshal(w.Body.Bytes(), &requested); err != nil {
		t.Fatalf("unmarshal RequestCertificate: %v", err)
	}

	arn := requested.CertificateArn

	// A certificate without tags lists none, and Tags is omitted.
	w = dispatch(t, svc, "ListTagsForCertificate", `{"CertificateArn":"`+arn+`"}`)
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "{}" {
		t.Fatalf("ListTagsForCertificate (empty): status %d body=%s", w.Code, w.Body.String())
	}

	w = dispatch(t, svc, "AddTagsToCertificate", `{"CertificateArn":"`+arn+`","Tags":[{"Key":"env","Value":"dev"},{"Key":"team"}]}`)
	if w.Code != http.StatusOK || w.Body.Len() != 0 {
		t.Fatalf("AddTagsToCertificate: status %d body=%s", w.Code, w.Body.String())
	}

	w = dispatch(t, svc, "RemoveTagsFromCertificate", `{"CertificateArn":"`+arn+`","Tags":[{"Key":"team"}]}`)
	if w.Code != http.StatusOK || w.Body.Len() != 0 {
		t.Fatalf("RemoveTagsFromCertificate: status %d body=%s", w.Code, w.Body.String())
	}

	w = dispatch(t, svc, "ListTagsForCertificate", `{"CertificateArn":"`+arn+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("ListTagsForCertificate: status %d body=%s", w.Code, w.Body.String())
	}

	var listed ListTagsForCertificateOutput
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal ListTagsForCertificate: %v", err)
	}

	if len(listed.Tags) != 1 || listed.Tags[0] != (Tag{Key: tagKeyEnv, Value: tagValDev}) {
		t.Fatalf("tags: got %v, want [env=dev]", listed.Tags)
	}
}

func TestDispatchAction_TagOperationErrors(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	cases := []struct {
		name      string
		operation string
		body      string
		wantCode  string
	}{
		{"list unknown arn", "ListTagsForCertificate", `{"CertificateArn":"` + absentCertificateARN + `"}`, errNotFound},
		{"add unknown arn", "AddTagsToCertificate", `{"CertificateArn":"` + absentCertificateARN + `","Tags":[{"Key":"k"}]}`, errNotFound},
		{"remove unknown arn", "RemoveTagsFromCertificate", `{"CertificateArn":"` + absentCertificateARN + `","Tags":[{"Key":"k"}]}`, errNotFound},
		{"list missing arn", "ListTagsForCertificate", `{}`, errInvalidParameter},
		{"add missing tags", "AddTagsToCertificate", `{"CertificateArn":"` + absentCertificateARN + `"}`, errInvalidParameter},
		{"remove missing tags", "RemoveTagsFromCertificate", `{"CertificateArn":"` + absentCertificateARN + `"}`, errInvalidParameter},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w := dispatch(t, svc, tc.operation, tc.body)
			if w.Code == http.StatusOK {
				t.Fatalf("status: got 200, want an error body=%s", w.Body.String())
			}

			var resp ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
			}

			if resp.Type != tc.wantCode {
				t.Errorf("__type: got %q, want %q", resp.Type, tc.wantCode)
			}
		})
	}
}
