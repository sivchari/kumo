package route53

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestMux() *http.ServeMux {
	svc := New(NewMemoryStorage())
	mux := http.NewServeMux()
	mux.HandleFunc("POST /2013-04-01/hostedzone", svc.CreateHostedZone)
	mux.HandleFunc("GET /2013-04-01/hostedzone", svc.ListHostedZones)
	mux.HandleFunc("GET /2013-04-01/hostedzonesbyname", svc.ListHostedZonesByName)
	mux.HandleFunc("GET /2013-04-01/hostedzone/{id}", svc.GetHostedZone)

	return mux
}

func doRequest(t *testing.T, mux *http.ServeMux, method, path, body string, out any) {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code >= http.StatusBadRequest {
		t.Fatalf("%s %s: status %d: %s", method, path, rec.Code, rec.Body.String())
	}

	if err := xml.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatalf("%s %s: unmarshal: %v: %s", method, path, err, rec.Body.String())
	}
}

// Route 53 stores alphabetic characters of domain names as lowercase
// letters, regardless of how they are specified.
func TestCreateHostedZone_LowercasesName(t *testing.T) {
	mux := newTestMux()

	var created CreateHostedZoneResponse

	doRequest(t, mux, http.MethodPost, "/2013-04-01/hostedzone",
		`<CreateHostedZoneRequest xmlns="`+xmlns+`"><Name>Example.COM</Name><CallerReference>ref-1</CallerReference></CreateHostedZoneRequest>`,
		&created)

	const want = "example.com."
	if created.HostedZone.Name != want {
		t.Errorf("CreateHostedZone Name = %q, want %q", created.HostedZone.Name, want)
	}

	id := strings.TrimPrefix(created.HostedZone.ID, "/hostedzone/")

	var got GetHostedZoneResponse

	doRequest(t, mux, http.MethodGet, "/2013-04-01/hostedzone/"+id, "", &got)

	if got.HostedZone.Name != want {
		t.Errorf("GetHostedZone Name = %q, want %q", got.HostedZone.Name, want)
	}

	var list ListHostedZonesResponse

	doRequest(t, mux, http.MethodGet, "/2013-04-01/hostedzone", "", &list)

	if len(list.HostedZones) != 1 || list.HostedZones[0].Name != want {
		t.Errorf("ListHostedZones = %+v, want one zone named %q", list.HostedZones, want)
	}

	var byName ListHostedZonesByNameResponse

	doRequest(t, mux, http.MethodGet, "/2013-04-01/hostedzonesbyname?dnsname=example.com", "", &byName)

	if len(byName.HostedZones) != 1 || byName.HostedZones[0].Name != want {
		t.Errorf("ListHostedZonesByName = %+v, want one zone named %q", byName.HostedZones, want)
	}
}

// TestNormalizeDNSName pins the canonical hosted zone name: a trailing dot,
// ASCII A-Z folded to a-z, and every other byte kept as sent.
func TestNormalizeDNSName(t *testing.T) {
	t.Parallel()

	const canonical = "example.com."

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"mixed case without trailing dot", "Example.COM", canonical},
		{"mixed case with trailing dot", "Example.COM.", canonical},
		{"already canonical", canonical, canonical},
		{"empty", "", ""},
		{"bytes next to A-Z are kept", "@AZ[.Example.COM", "@az[.example.com."},
		{"non-ASCII letters are kept", "EX\u00c4MPLE.com", "ex\u00c4mple.com."},
		{"invalid UTF-8 is kept", "Ex\xe4mple.COM", "ex\xe4mple.com."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizeDNSName(tt.in); got != tt.want {
				t.Errorf("normalizeDNSName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
