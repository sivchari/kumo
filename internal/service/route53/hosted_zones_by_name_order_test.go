package route53

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
)

type byNamePage struct {
	IDs        []string `xml:"HostedZones>HostedZone>Id"`
	NextName   string   `xml:"NextDNSName"`
	NextZoneID string   `xml:"NextHostedZoneId"`
}

func listByName(t *testing.T, service *Service, query url.Values) byNamePage {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/2013-04-01/hostedzonesbyname?"+query.Encode(), http.NoBody)
	service.ListHostedZonesByName(recorder, request)

	var page byNamePage
	if err := xml.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}

	return page
}

func TestListHostedZonesByNameUsesReversedLabelOrder(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()

	for id, name := range map[string]string{"Z1": "example.net.", "Z2": "www.example.com.", "Z3": "dup.example.org.", "Z4": "dup.example.org."} {
		if err := storage.CreateHostedZone(&HostedZone{ID: "/hostedzone/" + id, Name: name, CallerReference: id}); err != nil {
			t.Fatal(err)
		}
	}

	service := New(storage)

	var got []string

	query := url.Values{"maxitems": {"1"}}

	for range 5 {
		page := listByName(t, service, query)
		got = append(got, page.IDs...)

		if page.NextName == "" {
			break
		}

		query = url.Values{"maxitems": {"1"}, "dnsname": {page.NextName}, "hostedzoneid": {page.NextZoneID}}
	}

	want := []string{"/hostedzone/Z2", "/hostedzone/Z1", "/hostedzone/Z3", "/hostedzone/Z4"}
	if !slices.Equal(got, want) {
		t.Fatalf("listed zones = %v, want %v", got, want)
	}
}
