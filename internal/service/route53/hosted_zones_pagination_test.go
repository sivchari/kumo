package route53

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
)

func TestListHostedZonesReturnsEveryZoneAcrossPages(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()
	want := []string{"/hostedzone/Z1", "/hostedzone/Z2", "/hostedzone/Z3"}

	for i, id := range want {
		name := string(rune('a'+i)) + ".example.com."
		if err := storage.CreateHostedZone(&HostedZone{ID: id, Name: name, CallerReference: id}); err != nil {
			t.Fatal(err)
		}
	}

	service := New(storage)

	var got []string

	marker := ""

	for range len(want) + 1 {
		query := url.Values{"maxitems": {"1"}}
		if marker != "" {
			query.Set("marker", marker)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/2013-04-01/hostedzone?"+query.Encode(), http.NoBody)
		service.ListHostedZones(recorder, request)

		var page struct {
			IDs         []string `xml:"HostedZones>HostedZone>Id"`
			IsTruncated bool     `xml:"IsTruncated"`
			NextMarker  string   `xml:"NextMarker"`
		}

		if err := xml.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}

		got = append(got, page.IDs...)

		if !page.IsTruncated {
			break
		}

		marker = page.NextMarker
	}

	if !slices.Equal(got, want) {
		t.Fatalf("listed zones = %v, want %v", got, want)
	}
}
