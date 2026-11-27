package s3

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// Keys and common prefixes are paged together, so each prefix shows up once.
func TestListObjectsV2_DelimiterPagesPrefixesOnce(t *testing.T) {
	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	_ = store.CreateBucket(ctx, "lb")
	for _, key := range []string{"a.txt", "b/1", "c.txt", "d/1", "e.txt"} {
		_, _ = store.PutObject(ctx, "lb", key, strings.NewReader("x"), nil)
	}

	var got []string

	token := ""

	for page := 0; page < 10; page++ {
		query := "/lb?list-type=2&delimiter=/&max-keys=2"
		if token != "" {
			query += "&continuation-token=" + url.QueryEscape(token)
		}

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, query, http.NoBody)
		req.SetPathValue("bucket", "lb")

		w := httptest.NewRecorder()
		svc.ListObjects(w, req)

		var result ListBucketResult
		if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("page %d: %v (body=%s)", page, err, w.Body.String())
		}

		for _, c := range result.Contents {
			got = append(got, c.Key)
		}

		for _, p := range result.CommonPrefixes {
			got = append(got, p.Prefix)
		}

		if !result.IsTruncated {
			break
		}

		token = result.NextContinuationToken
	}

	slices.Sort(got)

	want := []string{"a.txt", "b/", "c.txt", "d/", "e.txt"}
	if !slices.Equal(got, want) {
		t.Fatalf("entries across pages = %v, want each of %v once", got, want)
	}
}
