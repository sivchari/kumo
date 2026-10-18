package route53

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	legacyID      = "0b20e967-b61f-4b98-bdb0-ea6c9e3e5a6c"
	legacyZoneKey = "/hostedzone/" + legacyID
)

// legacySnapshot is a route53.json written while hosted zone IDs were UUIDs.
const legacySnapshot = `{
  "hostedZones": {
    "` + legacyZoneKey + `": {"ID": "` + legacyZoneKey + `", "Name": "legacy.example.com.", "CallerReference": "legacy-ref", "ResourceRecordSetCount": 1}
  },
  "recordSets": {
    "` + legacyZoneKey + `": [{"Name": "www.legacy.example.com.", "Type": "A", "TTL": 300, "ResourceRecords": [{"Value": "192.0.2.1"}]}]
  },
  "changes": {},
  "tags": {"hostedzone/` + legacyID + `": [{"Key": "env", "Value": "legacy"}]}
}`

// muxRouter adapts http.ServeMux to service.Router so tests exercise the
// registered routes and their path values.
type muxRouter struct{ *http.ServeMux }

func (m muxRouter) Handle(method, pattern string, handler http.HandlerFunc) {
	m.ServeMux.HandleFunc(method+" "+pattern, handler)
}

func (m muxRouter) HandleFunc(method, pattern string, handler http.HandlerFunc) {
	m.Handle(method, pattern, handler)
}

// newServiceFromSnapshot loads snapshot through the KUMO_DATA_DIR
// persistence path and returns the service's routes.
func newServiceFromSnapshot(t *testing.T, snapshot string) http.Handler {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "route53.json"), []byte(snapshot), 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewMemoryStorage(WithDataDir(dir))
	// Flush synchronously so no debounced save outlives the temp dir.
	t.Cleanup(func() { _ = store.Close() })

	mux := muxRouter{http.NewServeMux()}
	New(store).RegisterRoutes(mux)

	return mux
}

// serve sends the request to h and fails the test unless it answers with want.
func serve(t *testing.T, h http.Handler, want int, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != want {
		t.Fatalf("%s %s = %d, want %d: %s", method, path, rec.Code, want, rec.Body)
	}

	return rec
}

// Zones persisted with UUID IDs must keep working under their existing IDs.
func TestLegacyUUIDHostedZoneFromSnapshot(t *testing.T) {
	t.Parallel()

	mux := newServiceFromSnapshot(t, legacySnapshot)

	rec := serve(t, mux, http.StatusOK, http.MethodGet, "/2013-04-01/hostedzone/"+legacyID, "")

	var got GetHostedZoneResponse
	if err := xml.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	if got.HostedZone.ID != legacyZoneKey {
		t.Fatalf("GetHostedZone Id = %q, want %q", got.HostedZone.ID, legacyZoneKey)
	}

	rec = serve(t, mux, http.StatusOK, http.MethodGet, "/2013-04-01/hostedzone/"+legacyID+"/rrset", "")
	if !strings.Contains(rec.Body.String(), "www.legacy.example.com.") {
		t.Fatalf("ListResourceRecordSets(%s) is missing the record: %s", legacyID, rec.Body)
	}

	rec = serve(t, mux, http.StatusOK, http.MethodGet, "/2013-04-01/tags/hostedzone/"+legacyID, "")
	if !strings.Contains(rec.Body.String(), "<Value>legacy</Value>") {
		t.Fatalf("ListTagsForResource(%s) is missing the tag: %s", legacyID, rec.Body)
	}

	deleteRecord := `<ChangeResourceRecordSetsRequest xmlns="` + xmlns + `"><ChangeBatch><Changes><Change>
<Action>DELETE</Action>
<ResourceRecordSet><Name>www.legacy.example.com.</Name><Type>A</Type><TTL>300</TTL>
<ResourceRecords><ResourceRecord><Value>192.0.2.1</Value></ResourceRecord></ResourceRecords></ResourceRecordSet>
</Change></Changes></ChangeBatch></ChangeResourceRecordSetsRequest>`

	serve(t, mux, http.StatusOK, http.MethodPost, "/2013-04-01/hostedzone/"+legacyID+"/rrset", deleteRecord)
	serve(t, mux, http.StatusOK, http.MethodDelete, "/2013-04-01/hostedzone/"+legacyID, "")
}

// A zone created next to a legacy UUID zone gets the new ID format.
func TestNewHostedZoneNextToLegacyZone(t *testing.T) {
	t.Parallel()

	mux := newServiceFromSnapshot(t, legacySnapshot)

	createZone := `<CreateHostedZoneRequest xmlns="` + xmlns + `"><Name>new.example.com</Name>` +
		`<CallerReference>new-ref</CallerReference></CreateHostedZoneRequest>`
	rec := serve(t, mux, http.StatusCreated, http.MethodPost, "/2013-04-01/hostedzone", createZone)

	var created CreateHostedZoneResponse
	if err := xml.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	newID := strings.TrimPrefix(created.HostedZone.ID, "/hostedzone/")
	if len(newID) > 32 || !strings.HasPrefix(newID, "Z") {
		t.Fatalf("new hosted zone Id = %q, want Z-prefixed and at most 32 characters", created.HostedZone.ID)
	}

	serve(t, mux, http.StatusOK, http.MethodGet, "/2013-04-01/hostedzone/"+legacyID, "")
	serve(t, mux, http.StatusOK, http.MethodGet, "/2013-04-01/hostedzone/"+newID, "")
}
