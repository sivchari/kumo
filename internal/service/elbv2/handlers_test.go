package elbv2

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTagNoOpResponsesUseMatchingActionRoot(t *testing.T) {
	t.Parallel()

	service := New(NewMemoryStorage())

	for _, tc := range []struct {
		action string
		root   string
	}{
		{action: "AddTags", root: "<AddTagsResponse"},
		{action: "RemoveTags", root: "<RemoveTagsResponse"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
			rec := httptest.NewRecorder()

			req.Header.Set("X-Amz-Target", "ElasticLoadBalancing."+tc.action)
			service.DispatchAction(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}

			if !strings.Contains(rec.Body.String(), tc.root) {
				t.Fatalf("response body = %q, want root %s", rec.Body.String(), tc.root)
			}
		})
	}
}
