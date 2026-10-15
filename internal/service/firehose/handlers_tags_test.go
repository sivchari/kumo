package firehose

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

const tagTargetPrefix = "Firehose_20150804."

// Tag keys reused across the Firehose tag tests.
const (
	tagKeyApp  = "app"
	tagKeyEnv  = "env"
	tagKeyTeam = "team"
)

// newTagTestService builds a Firehose service backed by in-memory storage.
func newTagTestService(t *testing.T) *Service {
	t.Helper()

	return New(NewMemoryStorage())
}

// dispatchTag sends a JSON request to the given action and returns the recorder.
func dispatchTag(t *testing.T, svc *Service, action, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("X-Amz-Target", tagTargetPrefix+action)

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	return w
}

// createStreamWithTags creates a delivery stream, optionally with the given tags JSON array.
func createStreamWithTags(t *testing.T, svc *Service, name, tagsJSON string) {
	t.Helper()

	body := `{"DeliveryStreamName":"` + name + `","DeliveryStreamType":"DirectPut"`
	if tagsJSON != "" {
		body += `,"Tags":` + tagsJSON
	}

	body += `}`

	w := dispatchTag(t, svc, "CreateDeliveryStream", body)
	if w.Code != http.StatusOK {
		t.Fatalf("CreateDeliveryStream: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}

// listTags reads the tags of a delivery stream and returns them as a slice.
func listTags(t *testing.T, svc *Service, name string) []Tag {
	t.Helper()

	w := dispatchTag(t, svc, "ListTagsForDeliveryStream", `{"DeliveryStreamName":"`+name+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("ListTagsForDeliveryStream: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	var out ListTagsForDeliveryStreamOutput
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	return out.Tags
}

// tagsToMapForTest converts a tag slice to a key-value map for comparison.
func tagsToMapForTest(tags []Tag) map[string]string {
	m := make(map[string]string, len(tags))
	for _, tag := range tags {
		m[tag.Key] = tag.Value
	}

	return m
}

func TestFirehose_TagDeliveryStreamLifecycle(t *testing.T) {
	t.Parallel()

	svc := newTagTestService(t)

	const name = "audit-to-s3"

	// Tags supplied at creation are readable back, sorted by key.
	createStreamWithTags(t, svc, name, `[{"Key":"env","Value":"local"},{"Key":"app","Value":"idp"}]`)

	gotSorted := listTags(t, svc, name)
	wantSorted := []Tag{{Key: tagKeyApp, Value: "idp"}, {Key: tagKeyEnv, Value: "local"}}

	if !reflect.DeepEqual(gotSorted, wantSorted) {
		t.Fatalf("ListTags after create: got %v, want %v", gotSorted, wantSorted)
	}

	// TagDeliveryStream adds a new key and overwrites an existing one.
	w := dispatchTag(t, svc, "TagDeliveryStream",
		`{"DeliveryStreamName":"`+name+`","Tags":[{"Key":"env","Value":"prod"},{"Key":"team","Value":"sec"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("TagDeliveryStream: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	got := tagsToMapForTest(listTags(t, svc, name))
	want := map[string]string{tagKeyApp: "idp", tagKeyEnv: "prod", tagKeyTeam: "sec"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListTags after tag: got %v, want %v", got, want)
	}

	// UntagDeliveryStream removes only the specified keys.
	w = dispatchTag(t, svc, "UntagDeliveryStream",
		`{"DeliveryStreamName":"`+name+`","TagKeys":["app","team"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("UntagDeliveryStream: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	got = tagsToMapForTest(listTags(t, svc, name))
	want = map[string]string{tagKeyEnv: "prod"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListTags after untag: got %v, want %v", got, want)
	}
}

func TestFirehose_TagActionsResourceNotFound(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		action string
		body   string
	}{
		{"list", "ListTagsForDeliveryStream", `{"DeliveryStreamName":"missing"}`},
		{"tag", "TagDeliveryStream", `{"DeliveryStreamName":"missing","Tags":[{"Key":"k","Value":"v"}]}`},
		{"untag", "UntagDeliveryStream", `{"DeliveryStreamName":"missing","TagKeys":["k"]}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newTagTestService(t)

			w := dispatchTag(t, svc, tc.action, tc.body)
			if w.Code != http.StatusNotFound {
				t.Fatalf("%s: got %d, want 404 (body=%s)", tc.action, w.Code, w.Body.String())
			}

			var errResp ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("failed to unmarshal error: %v", err)
			}

			if errResp.Type != errResourceNotFound {
				t.Errorf("error type: got %q, want %q", errResp.Type, errResourceNotFound)
			}
		})
	}
}

// pagingCase is one row of the ListTagsForDeliveryStream paging table.
type pagingCase struct {
	name        string
	body        string
	wantKeys    []string
	wantHasMore bool
}

// listTagsPagingCases enumerates the paging table for a stream tagged with the
// keys app, env and team.
func listTagsPagingCases(streamName string) []pagingCase {
	allKeys := []string{tagKeyApp, tagKeyEnv, tagKeyTeam}
	req := func(extra string) string {
		return `{"DeliveryStreamName":"` + streamName + `"` + extra + `}`
	}

	return []pagingCase{
		{
			name:     "no limit returns every tag",
			body:     req(""),
			wantKeys: allKeys,
		},
		{
			name:        "limit caps the page and flags more",
			body:        req(`,"Limit":1`),
			wantKeys:    []string{tagKeyApp},
			wantHasMore: true,
		},
		{
			name:     "limit equal to the tag count has no more",
			body:     req(`,"Limit":3`),
			wantKeys: allKeys,
		},
		{
			name:        "exclusive start key is excluded",
			body:        req(`,"ExclusiveStartTagKey":"` + tagKeyApp + `","Limit":1`),
			wantKeys:    []string{tagKeyEnv},
			wantHasMore: true,
		},
		{
			name:     "last page reports no more",
			body:     req(`,"ExclusiveStartTagKey":"` + tagKeyEnv + `"`),
			wantKeys: []string{tagKeyTeam},
		},
		{
			name:     "limit above the maximum is clamped, not an error",
			body:     req(`,"Limit":9999`),
			wantKeys: allKeys,
		},
		{
			name:     "unknown exclusive start key restarts from the first tag",
			body:     req(`,"ExclusiveStartTagKey":"absent"`),
			wantKeys: allKeys,
		},
	}
}

// TestFirehose_ListTagsPagination covers the Limit / ExclusiveStartTagKey
// combinations the AWS SDK cannot express (an out-of-range Limit, a key that is
// not on the stream), alongside the ordinary paging cases.
func TestFirehose_ListTagsPagination(t *testing.T) {
	t.Parallel()

	const streamName = "paging-stream"

	svc := newTagTestService(t)
	createStreamWithTags(t, svc, streamName,
		`[{"Key":"app","Value":"idp"},{"Key":"env","Value":"local"},{"Key":"team","Value":"sec"}]`)

	for _, tc := range listTagsPagingCases(streamName) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w := dispatchTag(t, svc, "ListTagsForDeliveryStream", tc.body)
			if w.Code != http.StatusOK {
				t.Fatalf("got %d, want 200 (body=%s)", w.Code, w.Body.String())
			}

			var out ListTagsForDeliveryStreamOutput
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			gotKeys := make([]string, 0, len(out.Tags))
			for _, tag := range out.Tags {
				gotKeys = append(gotKeys, tag.Key)
			}

			if !reflect.DeepEqual(gotKeys, tc.wantKeys) {
				t.Errorf("tag keys: got %v, want %v", gotKeys, tc.wantKeys)
			}

			if out.HasMoreTags != tc.wantHasMore {
				t.Errorf("HasMoreTags: got %v, want %v", out.HasMoreTags, tc.wantHasMore)
			}
		})
	}
}

func TestFirehose_TagActionsValidation(t *testing.T) {
	t.Parallel()

	svc := newTagTestService(t)
	createStreamWithTags(t, svc, "validate-stream", "")

	cases := []struct {
		name   string
		action string
		body   string
	}{
		{"tag without tags", "TagDeliveryStream", `{"DeliveryStreamName":"validate-stream"}`},
		{"untag without keys", "UntagDeliveryStream", `{"DeliveryStreamName":"validate-stream"}`},
		{"list without name", "ListTagsForDeliveryStream", `{}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w := dispatchTag(t, svc, tc.action, tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("%s: got %d, want 400 (body=%s)", tc.action, w.Code, w.Body.String())
			}
		})
	}
}
