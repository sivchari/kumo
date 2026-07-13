package cloudtrail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ctBody marshals v to a request body. The request structs never fail to
// marshal, so the error is ignored. It takes no *testing.T so arrange-style
// callers stay free of the thelper requirement.
func ctBody(v any) string {
	data, _ := json.Marshal(v)

	return string(data)
}

// dispatchCT drives an action through the JSON dispatcher exactly as the wire
// would, setting the X-Amz-Target header the server routes on.
func dispatchCT(t *testing.T, svc *Service, action, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("X-Amz-Target", "CloudTrail_20131101."+action)

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	return w
}

// dispatchCase is one row of the CloudTrail dispatch test table. arrange builds
// the request body (and any prerequisite state) from ctx but not *testing.T;
// verify takes *testing.T for assertions.
type dispatchCase struct {
	name       string
	action     string
	body       string
	arrange    func(ctx context.Context, store *MemoryStorage) string
	wantStatus int
	wantType   string
	verify     func(t *testing.T, store *MemoryStorage, w *httptest.ResponseRecorder)
}

// runDispatchCases dispatches each case on a fresh service and checks the
// status, optional error __type, and any extra verification.
func runDispatchCases(t *testing.T, cases []dispatchCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := NewMemoryStorage()
			svc := New(store)

			body := tc.body
			if tc.arrange != nil {
				body = tc.arrange(t.Context(), store)
			}

			w := dispatchCT(t, svc, tc.action, body)
			if w.Code != tc.wantStatus {
				t.Fatalf("status: got %d, want %d (body=%s)", w.Code, tc.wantStatus, w.Body.String())
			}

			if tc.wantType != "" {
				var resp ErrorResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode error response: %v", err)
				}

				if resp.Type != tc.wantType {
					t.Errorf("__type: got %q, want %q", resp.Type, tc.wantType)
				}
			}

			if tc.verify != nil {
				tc.verify(t, store, w)
			}
		})
	}
}

// createTrailFixture creates a trail named tagTrailName with the given tags. It
// takes ctx (not *testing.T) so arrange closures stay thelper-free.
func createTrailFixture(ctx context.Context, store *MemoryStorage, tags []Tag) *Trail {
	created, _ := store.CreateTrail(ctx, &CreateTrailRequest{
		Name:         tagTrailName,
		S3BucketName: tagBucket,
		TagsList:     tags,
	})

	return created
}

func verifyListTagsEnv(t *testing.T, _ *MemoryStorage, w *httptest.ResponseRecorder) {
	t.Helper()

	var resp ListTagsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.ResourceTagList) != 1 || len(resp.ResourceTagList[0].TagsList) != 1 {
		t.Fatalf("ResourceTagList: got %+v, want one resource with one tag", resp.ResourceTagList)
	}

	if tag := resp.ResourceTagList[0].TagsList[0]; tag.Key != tagKeyEnv || tag.Value != tagValLocal {
		t.Errorf("tag: got %+v, want {env local}", tag)
	}
}

func verifyListTagsEmpty(t *testing.T, _ *MemoryStorage, w *httptest.ResponseRecorder) {
	t.Helper()

	var resp ListTagsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.ResourceTagList) != 1 || len(resp.ResourceTagList[0].TagsList) != 0 {
		t.Errorf("ResourceTagList: got %+v, want one resource with no tags", resp.ResourceTagList)
	}
}

func verifyTagPresent(t *testing.T, store *MemoryStorage, _ *httptest.ResponseRecorder) {
	t.Helper()

	if tags := store.ListTrailTags(t.Context(), tagTrailName); len(tags) != 1 || tags[0].Key != tagKeyEnv {
		t.Errorf("tags: got %v, want [env]", tags)
	}
}

func verifyNoTags(t *testing.T, store *MemoryStorage, _ *httptest.ResponseRecorder) {
	t.Helper()

	if tags := store.ListTrailTags(t.Context(), tagTrailName); len(tags) != 0 {
		t.Errorf("tags: got %v, want empty", tags)
	}
}

// TestListTagsDispatch covers the read path: ListTags is a registered action
// (no UnknownOperationException), returns create-time tags, and yields an empty
// list for an unknown resource so the provider's read-time fetch stays stable.
func TestListTagsDispatch(t *testing.T) {
	t.Parallel()

	runDispatchCases(t, []dispatchCase{
		{
			name:   "returns create-time tags",
			action: "ListTags",
			arrange: func(ctx context.Context, store *MemoryStorage) string {
				created := createTrailFixture(ctx, store, []Tag{{Key: tagKeyEnv, Value: tagValLocal}})

				return ctBody(ListTagsRequest{ResourceIDList: []string{created.TrailARN}})
			},
			wantStatus: http.StatusOK,
			verify:     verifyListTagsEnv,
		},
		{
			name:       "unknown resource is empty",
			action:     "ListTags",
			body:       ctBody(ListTagsRequest{ResourceIDList: []string{absentTrailARN}}),
			wantStatus: http.StatusOK,
			verify:     verifyListTagsEmpty,
		},
	})
}

// TestTagMutationDispatch covers AddTags / RemoveTags routing, the create-tag
// effect, ResourceId validation, and the not-found path.
func TestTagMutationDispatch(t *testing.T) {
	t.Parallel()

	runDispatchCases(t, []dispatchCase{
		{
			name:   "AddTags adds a tag",
			action: "AddTags",
			arrange: func(ctx context.Context, store *MemoryStorage) string {
				created := createTrailFixture(ctx, store, nil)

				return ctBody(AddTagsRequest{ResourceID: created.TrailARN, TagsList: []Tag{{Key: tagKeyEnv, Value: tagValLocal}}})
			},
			wantStatus: http.StatusOK,
			verify:     verifyTagPresent,
		},
		{
			name:       "AddTags requires ResourceId",
			action:     "AddTags",
			body:       ctBody(AddTagsRequest{TagsList: []Tag{{Key: tagKeyEnv, Value: tagValLocal}}}),
			wantStatus: http.StatusBadRequest,
			wantType:   errValidationError,
		},
		{
			name:       "AddTags on missing trail is not found",
			action:     "AddTags",
			body:       ctBody(AddTagsRequest{ResourceID: absentTrailARN, TagsList: []Tag{{Key: tagKeyEnv, Value: tagValLocal}}}),
			wantStatus: http.StatusNotFound,
			wantType:   errTrailNotFound,
		},
		{
			name:   "RemoveTags removes a tag",
			action: "RemoveTags",
			arrange: func(ctx context.Context, store *MemoryStorage) string {
				created := createTrailFixture(ctx, store, []Tag{{Key: tagKeyEnv, Value: tagValLocal}})

				return ctBody(RemoveTagsRequest{ResourceID: created.TrailARN, TagsList: []Tag{{Key: tagKeyEnv}}})
			},
			wantStatus: http.StatusOK,
			verify:     verifyNoTags,
		},
	})
}

// TestUnknownActionDispatch verifies an unregistered action returns
// UnknownOperationException, guarding the dispatch fallthrough.
func TestUnknownActionDispatch(t *testing.T) {
	t.Parallel()

	runDispatchCases(t, []dispatchCase{
		{
			name:       "unknown action",
			action:     "Frobnicate",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantType:   "UnknownOperationException",
		},
	})
}
