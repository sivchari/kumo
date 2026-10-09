package cloudwatchlogs

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	testGroupName    = "/ecs/kumo-local/kumo"
	missingGroupName = "/does/not/exist"
	tagKeyProject    = "Project"
	tagValueProject  = "kumo-local"
)

func dispatchAction(t *testing.T, svc *Service, action string, body any) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(string(payload)))
	req.Header.Set("X-Amz-Target", "Logs_20140328."+action)

	rec := httptest.NewRecorder()

	svc.DispatchAction(rec, req)

	return rec
}

func errorType(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error body %q: %v", rec.Body.String(), err)
	}

	got, _ := body["__type"].(string)

	return got
}

func listedTags(t *testing.T, svc *Service, action string, body any) map[string]string {
	t.Helper()

	rec := dispatchAction(t, svc, action, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: expected status 200, got %d: %s", action, rec.Code, rec.Body.String())
	}

	var resp TagsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("%s: failed to decode response %q: %v", action, rec.Body.String(), err)
	}

	return resp.Tags
}

func numberedTags(n int) map[string]string {
	tags := make(map[string]string, n)
	for i := range n {
		tags[fmt.Sprintf("key-%03d", i)] = "value"
	}

	return tags
}

func groupARN(s *MemoryStorage, name string) string {
	return fmt.Sprintf("arn:aws:logs:%s:%s:log-group:%s", s.region, defaultAccountID, name)
}

func TestLogGroupTagsRoundTrip(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage(defaultBaseURL)
	svc := New(s, defaultBaseURL)

	if err := s.CreateLogGroup(t.Context(), &CreateLogGroupRequest{
		LogGroupName: testGroupName,
		Tags: map[string]string{
			tagKeyProject: tagValueProject,
			"Component":   "kumo",
		},
	}); err != nil {
		t.Fatal(err)
	}

	rec := dispatchAction(t, svc, actionListTagsLogGroup, ListTagsLogGroupRequest{LogGroupName: testGroupName})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	for _, want := range []string{
		`"tags"`,
		`"Project":"kumo-local"`,
		`"Component":"kumo"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response to contain %q, got %s", want, body)
		}
	}
}

func TestTagActionsOnMissingLogGroup(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage(defaultBaseURL)
	svc := New(s, defaultBaseURL)

	missingARN := groupARN(s, missingGroupName)

	tests := []struct {
		action string
		body   any
	}{
		{actionListTagsLogGroup, ListTagsLogGroupRequest{LogGroupName: missingGroupName}},
		{actionTagLogGroup, TagLogGroupRequest{LogGroupName: missingGroupName, Tags: map[string]string{"k": "v"}}},
		{actionUntagLogGroup, UntagLogGroupRequest{LogGroupName: missingGroupName, Tags: []string{"k"}}},
		{actionListTagsForResource, ListTagsForResourceRequest{ResourceARN: missingARN}},
		{actionTagResource, TagResourceRequest{ResourceARN: missingARN, Tags: map[string]string{"k": "v"}}},
		{actionUntagResource, UntagResourceRequest{ResourceARN: missingARN, TagKeys: []string{"k"}}},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			t.Parallel()

			rec := dispatchAction(t, svc, tt.action, tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
			}

			if got := errorType(t, rec); got != errResourceNotFoundException {
				t.Fatalf("expected %s, got %s", errResourceNotFoundException, got)
			}
		})
	}
}

func TestTagResourceActionsByARN(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage(defaultBaseURL)
	svc := New(s, defaultBaseURL)

	if err := s.CreateLogGroup(t.Context(), &CreateLogGroupRequest{
		LogGroupName: testGroupName,
		Tags:         map[string]string{tagKeyProject: tagValueProject},
	}); err != nil {
		t.Fatal(err)
	}

	arn := groupARN(s, testGroupName)

	if got := listedTags(t, svc, actionListTagsForResource, ListTagsForResourceRequest{ResourceARN: arn}); got[tagKeyProject] != tagValueProject {
		t.Fatalf("ListTagsForResource by ARN: unexpected tags %v", got)
	}

	rec := dispatchAction(t, svc, actionTagResource, TagResourceRequest{ResourceARN: arn, Tags: map[string]string{"Env": "dev"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("TagResource: expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	want := map[string]string{tagKeyProject: tagValueProject, "Env": "dev"}
	if got := listedTags(t, svc, actionListTagsLogGroup, ListTagsLogGroupRequest{LogGroupName: testGroupName}); !maps.Equal(got, want) {
		t.Fatalf("tags after TagResource by ARN: got %v, want %v", got, want)
	}
}

func TestUntagRemovesKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		action string
		body   func(arn string) any
	}{
		{
			name:   actionUntagLogGroup,
			action: actionUntagLogGroup,
			body: func(string) any {
				return UntagLogGroupRequest{LogGroupName: testGroupName, Tags: []string{"A", "missing"}}
			},
		},
		{
			name:   actionUntagResource,
			action: actionUntagResource,
			body: func(arn string) any {
				return UntagResourceRequest{ResourceARN: arn, TagKeys: []string{"A", "missing"}}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := NewMemoryStorage(defaultBaseURL)
			svc := New(s, defaultBaseURL)

			if err := s.CreateLogGroup(t.Context(), &CreateLogGroupRequest{
				LogGroupName: testGroupName,
				Tags:         map[string]string{"A": "1", "B": "2"},
			}); err != nil {
				t.Fatal(err)
			}

			rec := dispatchAction(t, svc, tt.action, tt.body(groupARN(s, testGroupName)))
			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
			}

			want := map[string]string{"B": "2"}
			if got := listedTags(t, svc, actionListTagsLogGroup, ListTagsLogGroupRequest{LogGroupName: testGroupName}); !maps.Equal(got, want) {
				t.Fatalf("tags after untag: got %v, want %v", got, want)
			}
		})
	}
}

func assertResponse(t *testing.T, rec *httptest.ResponseRecorder, wantCode int, wantType string) {
	t.Helper()

	if rec.Code != wantCode {
		t.Fatalf("expected status %d, got %d: %s", wantCode, rec.Code, rec.Body.String())
	}

	if wantType == "" {
		return
	}

	if got := errorType(t, rec); got != wantType {
		t.Fatalf("expected %s, got %s", wantType, got)
	}
}

func TestTagLimit(t *testing.T) {
	t.Parallel()

	atLimit := numberedTags(maxTagsPerResource)
	overLimit := numberedTags(maxTagsPerResource + 1)

	tests := []struct {
		name     string
		existing int
		action   string
		tags     map[string]string
		wantCode int
		wantType string
		wantLen  int
	}{
		{"TagResource reaches exactly 50", 0, actionTagResource, atLimit, http.StatusOK, "", maxTagsPerResource},
		{"TagResource exceeds 50", 0, actionTagResource, overLimit, http.StatusBadRequest, errTooManyTagsException, 0},
		{"TagLogGroup reaches exactly 50", 0, actionTagLogGroup, atLimit, http.StatusOK, "", maxTagsPerResource},
		{"TagLogGroup exceeds 50", 0, actionTagLogGroup, overLimit, http.StatusBadRequest, errTooManyTagsException, 0},
		{"merge with existing tags exceeds 50", maxTagsPerResource, actionTagResource, map[string]string{"extra": "v"}, http.StatusBadRequest, errTooManyTagsException, maxTagsPerResource},
		{"overwriting existing keys at 50 is allowed", maxTagsPerResource, actionTagResource, map[string]string{"key-000": "new"}, http.StatusOK, "", maxTagsPerResource},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := NewMemoryStorage(defaultBaseURL)
			svc := New(s, defaultBaseURL)

			if err := s.CreateLogGroup(t.Context(), &CreateLogGroupRequest{
				LogGroupName: testGroupName,
				Tags:         numberedTags(tt.existing),
			}); err != nil {
				t.Fatal(err)
			}

			var body any = TagResourceRequest{ResourceARN: groupARN(s, testGroupName), Tags: tt.tags}
			if tt.action == actionTagLogGroup {
				body = TagLogGroupRequest{LogGroupName: testGroupName, Tags: tt.tags}
			}

			assertResponse(t, dispatchAction(t, svc, tt.action, body), tt.wantCode, tt.wantType)

			got := listedTags(t, svc, actionListTagsLogGroup, ListTagsLogGroupRequest{LogGroupName: testGroupName})
			if len(got) != tt.wantLen {
				t.Fatalf("expected %d tags, got %d", tt.wantLen, len(got))
			}
		})
	}
}

func TestCreateLogGroupTagLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tags     int
		wantCode int
		wantType string
	}{
		{"50 tags", maxTagsPerResource, http.StatusOK, ""},
		{"51 tags", maxTagsPerResource + 1, http.StatusBadRequest, errTooManyTagsException},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := New(NewMemoryStorage(defaultBaseURL), defaultBaseURL)

			rec := dispatchAction(t, svc, "CreateLogGroup", CreateLogGroupRequest{
				LogGroupName: testGroupName,
				Tags:         numberedTags(tt.tags),
			})
			assertResponse(t, rec, tt.wantCode, tt.wantType)
		})
	}
}

func TestTagMutationsPersistAcrossReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := t.Context()

	s1 := NewMemoryStorage(defaultBaseURL, WithDataDir(dir))

	if err := s1.CreateLogGroup(ctx, &CreateLogGroupRequest{
		LogGroupName: testGroupName,
		Tags:         map[string]string{"A": "1", "B": "2"},
	}); err != nil {
		t.Fatal(err)
	}

	// Flush the debounced snapshot after each mutation instead of calling
	// Close, which persists the final state on its own and would mask a
	// mutation that never scheduled a save.
	storage.FlushSnapshots()

	assertReloadedTags(t, dir, map[string]string{"A": "1", "B": "2"})

	if err := s1.Tag(ctx, testGroupName, map[string]string{"C": "3"}); err != nil {
		t.Fatal(err)
	}

	storage.FlushSnapshots()

	assertReloadedTags(t, dir, map[string]string{"A": "1", "B": "2", "C": "3"})

	if err := s1.Untag(ctx, groupARN(s1, testGroupName), []string{"A"}); err != nil {
		t.Fatal(err)
	}

	storage.FlushSnapshots()

	assertReloadedTags(t, dir, map[string]string{"B": "2", "C": "3"})
}

func assertReloadedTags(t *testing.T, dir string, want map[string]string) {
	t.Helper()

	got, err := NewMemoryStorage(defaultBaseURL, WithDataDir(dir)).ListTags(t.Context(), testGroupName)
	if err != nil {
		t.Fatal(err)
	}

	if !maps.Equal(got, want) {
		t.Fatalf("tags after reload: got %v, want %v", got, want)
	}
}
