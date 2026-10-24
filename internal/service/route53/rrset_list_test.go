package route53

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"testing"
)

const (
	rrsetListTestZoneID = "/hostedzone/ZLISTTEST"
	zoneApex            = "example.com."
	weightedName        = "w.example.com."
	bName               = "b.example.com."
	typeTXT             = "TXT"
	qName               = "name"
	qType               = "type"
	qMaxItems           = "maxitems"
	qIdentifier         = "identifier"
	escapedStarName     = `\052.example.com.`
	escapedUpperZName   = `\132.example.com.`
	starName            = "*.example.com."
	zName               = "z.example.com."
	zKey                = "com.example.z."
	yName               = "y.example.com."
	aName               = "a.example.com."
	aNameNoDot          = "a.example.com"
	idTwo               = "two"
	idOne               = "one"
	escaped201Name      = `\201.example.com.`
	escapedDotName      = `a\056b.example.com.`
	dottedName          = "b.a.example.com."
	wwwName             = "www.example.com."
)

// newRRSetListService returns a service whose test zone holds records in the
// given (deliberately unsorted) order.
func newRRSetListService(t *testing.T, records []ResourceRecordSet) *Service {
	t.Helper()

	st := NewMemoryStorage()
	if err := st.CreateHostedZone(&HostedZone{ID: rrsetListTestZoneID, Name: zoneApex, CallerReference: "ref"}); err != nil {
		t.Fatalf("create zone: %v", err)
	}

	// Seed storage directly so records sharing a name and type but differing
	// in SetIdentifier can coexist.
	st.RecordSets[rrsetListTestZoneID] = slices.Clone(records)

	return New(st)
}

// listRRSets calls ListResourceRecordSets with the given query parameters.
func listRRSets(t *testing.T, svc *Service, query url.Values) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/2013-04-01/hostedzone/ZLISTTEST/rrset?"+query.Encode(), nil)
	req.SetPathValue("id", "ZLISTTEST")

	w := httptest.NewRecorder()
	svc.ListResourceRecordSets(w, req)

	return w
}

func decodeRRSetList(t *testing.T, w *httptest.ResponseRecorder) ListResourceRecordSetsResponse {
	t.Helper()

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 body=%s", w.Code, w.Body.String())
	}

	var resp ListResourceRecordSetsResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}

	return resp
}

// recordKeys renders records as "name type [setIdentifier]" for comparison.
func recordKeys(records []ResourceRecordSet) []string {
	keys := make([]string, 0, len(records))

	for i := range records {
		k := records[i].Name + " " + records[i].Type
		if records[i].SetIdentifier != "" {
			k += " " + records[i].SetIdentifier
		}

		keys = append(keys, k)
	}

	return keys
}

// rrsetListFixture is stored out of order; rrsetListSorted is the order the
// AWS API reference describes: labels reversed, then by type, with the
// trailing dot taking part in the comparison.
var (
	rrsetListFixture = []ResourceRecordSet{
		{Name: wwwName, Type: "A"},
		{Name: zoneApex, Type: "SOA"},
		{Name: bName, Type: typeTXT},
		{Name: "www-1.example.com.", Type: "A"},
		{Name: "a.b.example.com.", Type: "A"},
		{Name: zoneApex, Type: "NS"},
		{Name: bName, Type: "A"},
		{Name: "api.example.com.", Type: "A"},
	}
	rrsetListSorted = []string{
		"example.com. NS",
		"example.com. SOA",
		"api.example.com. A",
		"b.example.com. A",
		"b.example.com. TXT",
		"a.b.example.com. A",
		"www-1.example.com. A",
		"www.example.com. A",
	}
)

func TestListResourceRecordSets_SortOrder(t *testing.T) {
	t.Parallel()

	svc := newRRSetListService(t, rrsetListFixture)

	resp := decodeRRSetList(t, listRRSets(t, svc, url.Values{}))

	if got := recordKeys(resp.ResourceRecordSets); !slices.Equal(got, rrsetListSorted) {
		t.Errorf("order:\n got  %q\n want %q", got, rrsetListSorted)
	}

	if resp.IsTruncated {
		t.Error("IsTruncated: got true, want false")
	}

	if resp.MaxItems != "300" {
		t.Errorf("MaxItems: got %q, want default 300", resp.MaxItems)
	}
}

// The reference notes that the trailing dot "can change the sort order when
// the record name contains characters that appear before . (decimal 46) in
// the ASCII table. These characters include the following: ! " # $ % & ' ( )
// * + , -". With the dots taking part in the comparison, com.example.www!1.
// sorts before com.example.www. for each of them, spelled as is or as the
// \ddd escape that Route 53 returns.
func TestListResourceRecordSets_TrailingDotChangesSortOrder(t *testing.T) {
	t.Parallel()

	for _, c := range `!"#$%&'()*+,-` {
		for _, name := range []string{
			"www" + string(c) + "1.example.com.",
			fmt.Sprintf(`www\%03o1.example.com.`, c),
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				svc := newRRSetListService(t, []ResourceRecordSet{
					{Name: wwwName, Type: "A"},
					{Name: name, Type: "A"},
				})

				resp := decodeRRSetList(t, listRRSets(t, svc, url.Values{}))

				want := []string{name + " A", wwwName + " A"}
				if got := recordKeys(resp.ResourceRecordSets); !slices.Equal(got, want) {
					t.Errorf("order:\n got  %q\n want %q", got, want)
				}
			})
		}
	}
}

func TestListResourceRecordSets_StartPosition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query url.Values
		want  []string
	}{
		{
			name:  "existing name starts at that name",
			query: url.Values{qName: {bName}},
			want:  rrsetListSorted[3:],
		},
		{
			name:  "missing name starts at the next greater name",
			query: url.Values{qName: {"c.example.com."}},
			want:  rrsetListSorted[6:],
		},
		{
			name:  "name and type start at the first type greater than or equal",
			query: url.Values{qName: {bName}, qType: {"MX"}},
			want:  rrsetListSorted[4:],
		},
		{
			name:  "name without trailing dot and in upper case",
			query: url.Values{qName: {"B.Example.COM"}, qType: {typeTXT}},
			want:  rrsetListSorted[4:],
		},
		{
			name:  "name past the last record returns nothing",
			query: url.Values{qName: {"zzz.example.com."}},
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := newRRSetListService(t, rrsetListFixture)

			resp := decodeRRSetList(t, listRRSets(t, svc, tt.query))

			if got := recordKeys(resp.ResourceRecordSets); !slices.Equal(got, tt.want) {
				t.Errorf("records:\n got  %q\n want %q", got, tt.want)
			}
		})
	}
}

// assertInvalidInput fails the test unless w is a 400 InvalidInput error.
func assertInvalidInput(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400 body=%s", w.Code, w.Body.String())
	}

	var resp ErrorResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}

	if resp.Error.Code != "InvalidInput" {
		t.Errorf("code: got %q, want InvalidInput", resp.Error.Code)
	}
}

func TestListResourceRecordSets_TypeWithoutNameIsInvalidInput(t *testing.T) {
	t.Parallel()

	svc := newRRSetListService(t, rrsetListFixture)

	assertInvalidInput(t, listRRSets(t, svc, url.Values{qType: {"A"}}))
}

// The reference lists the valid values of type; any other value, including a
// valid one in lower case, is rejected rather than positioned.
func TestListResourceRecordSets_UnlistedTypeIsInvalidInput(t *testing.T) {
	t.Parallel()

	svc := newRRSetListService(t, rrsetListFixture)

	for _, recordType := range []string{"FOO", "a", "Txt"} {
		t.Run(recordType, func(t *testing.T) {
			t.Parallel()

			assertInvalidInput(t, listRRSets(t, svc, url.Values{qName: {bName}, qType: {recordType}}))
		})
	}
}

func TestListResourceRecordSets_EveryListedTypeIsAccepted(t *testing.T) {
	t.Parallel()

	svc := newRRSetListService(t, rrsetListFixture)

	for _, recordType := range []string{
		"SOA", "A", typeTXT, "NS", "CNAME", "MX", "NAPTR", "PTR", "SRV",
		"SPF", "AAAA", "CAA", "DS", "TLSA", "SSHFP", "SVCB", "HTTPS",
	} {
		decodeRRSetList(t, listRRSets(t, svc, url.Values{qName: {bName}, qType: {recordType}}))
	}
}

// A record set stored with an unlisted type must not strand a listing: the
// NextRecordType returned for it continues the listing, since it matches a
// stored type exactly.
func TestListResourceRecordSets_ContinuesFromStoredUnlistedType(t *testing.T) {
	t.Parallel()

	for _, recordType := range []string{"FOO", "a"} {
		t.Run(recordType, func(t *testing.T) {
			t.Parallel()

			records := []ResourceRecordSet{{Name: aName, Type: "A"}, {Name: bName, Type: recordType}}
			svc := newRRSetListService(t, records)

			want := []string{aName + " A", bName + " " + recordType}
			if got := pageAllRecordSets(t, svc, len(records), 1); !slices.Equal(got, want) {
				t.Errorf("paged one at a time:\n got  %q\n want %q", got, want)
			}
		})
	}
}

func TestListResourceRecordSets_MaxItemsTruncates(t *testing.T) {
	t.Parallel()

	svc := newRRSetListService(t, rrsetListFixture)

	resp := decodeRRSetList(t, listRRSets(t, svc, url.Values{qMaxItems: {"3"}}))

	if got := recordKeys(resp.ResourceRecordSets); !slices.Equal(got, rrsetListSorted[:3]) {
		t.Errorf("records:\n got  %q\n want %q", got, rrsetListSorted[:3])
	}

	if !resp.IsTruncated {
		t.Error("IsTruncated: got false, want true")
	}

	if resp.MaxItems != "3" {
		t.Errorf("MaxItems: got %q, want 3", resp.MaxItems)
	}

	if resp.NextRecordName != bName || resp.NextRecordType != "A" {
		t.Errorf("next: got (%q, %q), want (b.example.com., A)", resp.NextRecordName, resp.NextRecordType)
	}

	if resp.NextRecordIdentifier != "" {
		t.Errorf("NextRecordIdentifier: got %q, want empty", resp.NextRecordIdentifier)
	}
}

func TestListResourceRecordSets_MaxItemsCappedAt300(t *testing.T) {
	t.Parallel()

	records := make([]ResourceRecordSet, 0, 301)
	for i := range 301 {
		records = append(records, ResourceRecordSet{Name: "r" + strconv.Itoa(i) + ".example.com.", Type: "A"})
	}

	svc := newRRSetListService(t, records)

	tests := []struct {
		maxItems   string
		wantEchoed string
	}{
		{maxItems: "", wantEchoed: "300"},
		{maxItems: "1000", wantEchoed: "1000"},
		{maxItems: "abc", wantEchoed: "300"},
		{maxItems: "0", wantEchoed: "300"},
		{maxItems: "-1", wantEchoed: "300"},
	}

	for _, tt := range tests {
		resp := decodeRRSetList(t, listRRSets(t, svc, url.Values{qMaxItems: {tt.maxItems}}))

		if len(resp.ResourceRecordSets) != 300 || !resp.IsTruncated {
			t.Errorf("maxitems=%q: got %d records truncated=%v, want 300 truncated=true",
				tt.maxItems, len(resp.ResourceRecordSets), resp.IsTruncated)
		}

		if resp.MaxItems != tt.wantEchoed {
			t.Errorf("maxitems=%q: MaxItems got %q, want %q", tt.maxItems, resp.MaxItems, tt.wantEchoed)
		}
	}
}

// weightedFixture includes records sharing a name and type that differ only
// in SetIdentifier, so a page boundary can fall between them.
var weightedFixture = []ResourceRecordSet{
	{Name: weightedName, Type: "A", SetIdentifier: idTwo},
	{Name: zoneApex, Type: "NS"},
	{Name: weightedName, Type: "A", SetIdentifier: idOne},
	{Name: weightedName, Type: "A", SetIdentifier: "three"},
	{Name: weightedName, Type: typeTXT},
}

func TestListResourceRecordSets_NextRecordIdentifier(t *testing.T) {
	t.Parallel()

	svc := newRRSetListService(t, weightedFixture)

	resp := decodeRRSetList(t, listRRSets(t, svc, url.Values{qMaxItems: {"2"}}))

	want := []string{"example.com. NS", "w.example.com. A one"}
	if got := recordKeys(resp.ResourceRecordSets); !slices.Equal(got, want) {
		t.Errorf("records:\n got  %q\n want %q", got, want)
	}

	if resp.NextRecordName != weightedName || resp.NextRecordType != "A" || resp.NextRecordIdentifier != "three" {
		t.Errorf("next: got (%q, %q, %q), want (w.example.com., A, three)",
			resp.NextRecordName, resp.NextRecordType, resp.NextRecordIdentifier)
	}

	resp = decodeRRSetList(t, listRRSets(t, svc, url.Values{
		qMaxItems: {"2"}, qName: {resp.NextRecordName}, qType: {resp.NextRecordType}, qIdentifier: {resp.NextRecordIdentifier},
	}))

	want = []string{"w.example.com. A three", "w.example.com. A two"}
	if got := recordKeys(resp.ResourceRecordSets); !slices.Equal(got, want) {
		t.Errorf("second page:\n got  %q\n want %q", got, want)
	}
}

func TestListResourceRecordSets_IdentifierWithoutTypeIsIgnored(t *testing.T) {
	t.Parallel()

	svc := newRRSetListService(t, weightedFixture)

	resp := decodeRRSetList(t, listRRSets(t, svc, url.Values{qName: {weightedName}, qIdentifier: {idTwo}}))

	want := []string{"w.example.com. A one", "w.example.com. A three", "w.example.com. A two", "w.example.com. TXT"}
	if got := recordKeys(resp.ResourceRecordSets); !slices.Equal(got, want) {
		t.Errorf("records:\n got  %q\n want %q", got, want)
	}
}

func TestListResourceRecordSets_ContinuationIntoWeightedRecord(t *testing.T) {
	t.Parallel()

	svc := newRRSetListService(t, []ResourceRecordSet{
		{Name: yName, Type: "A", SetIdentifier: idTwo},
		{Name: "x.example.com.", Type: typeTXT},
		{Name: yName, Type: "A", SetIdentifier: idOne},
	})

	resp := decodeRRSetList(t, listRRSets(t, svc, url.Values{qMaxItems: {"1"}}))

	if resp.NextRecordName != yName || resp.NextRecordType != "A" || resp.NextRecordIdentifier != idOne {
		t.Fatalf("next: got (%q, %q, %q), want (y.example.com., A, one)",
			resp.NextRecordName, resp.NextRecordType, resp.NextRecordIdentifier)
	}

	resp = decodeRRSetList(t, listRRSets(t, svc, url.Values{
		qMaxItems: {"1"}, qName: {resp.NextRecordName}, qType: {resp.NextRecordType}, qIdentifier: {resp.NextRecordIdentifier},
	}))

	want := []string{"y.example.com. A one"}
	if got := recordKeys(resp.ResourceRecordSets); !slices.Equal(got, want) {
		t.Errorf("second page:\n got  %q\n want %q", got, want)
	}
}

// pageAllRecordSets follows Next* markers with the given page size and
// returns every listed record. It fails the test instead of looping forever
// when continuation does not make progress.
func pageAllRecordSets(t *testing.T, svc *Service, stored, pageSize int) []string {
	t.Helper()

	var got []string

	query := url.Values{qMaxItems: {strconv.Itoa(pageSize)}}

	for range stored + 2 {
		resp := decodeRRSetList(t, listRRSets(t, svc, query))
		got = append(got, recordKeys(resp.ResourceRecordSets)...)

		if !resp.IsTruncated {
			return got
		}

		query = url.Values{
			qMaxItems: {strconv.Itoa(pageSize)},
			qName:     {resp.NextRecordName},
			qType:     {resp.NextRecordType},
		}
		if resp.NextRecordIdentifier != "" {
			query.Set(qIdentifier, resp.NextRecordIdentifier)
		}
	}

	t.Fatalf("maxitems=%d: still truncated after %d pages; listed so far %q", pageSize, stored+2, got)

	return nil
}

func TestListResourceRecordSets_PagingReturnsEveryRecordOnce(t *testing.T) {
	t.Parallel()

	fixtures := map[string][]ResourceRecordSet{
		"simple":   rrsetListFixture,
		"weighted": weightedFixture,
		// Spellings that normalize to the same position must still each be
		// listed exactly once when a page boundary falls between them.
		"trailing dot": {
			{Name: aNameNoDot, Type: "A"},
			{Name: aName, Type: "A"},
			{Name: bName, Type: "A"},
		},
		"trailing dot reversed": {
			{Name: aName, Type: "A"},
			{Name: aNameNoDot, Type: "A"},
			{Name: bName, Type: "A"},
		},
		"case": {
			{Name: "A.example.com.", Type: "A"},
			{Name: aName, Type: "A"},
			{Name: "a.EXAMPLE.com", Type: "A"},
		},
		"escape": {
			{Name: escapedStarName, Type: "A"},
			{Name: starName, Type: "A"},
			{Name: escapedUpperZName, Type: "A"},
			{Name: zName, Type: "A"},
		},
	}

	for name, records := range fixtures {
		for pageSize := 1; pageSize <= len(records)+1; pageSize++ {
			t.Run(name+"/maxitems="+strconv.Itoa(pageSize), func(t *testing.T) {
				t.Parallel()

				svc := newRRSetListService(t, records)
				want := recordKeys(decodeRRSetList(t, listRRSets(t, svc, url.Values{})).ResourceRecordSets)

				got := pageAllRecordSets(t, svc, len(records), pageSize)

				if len(want) != len(records) || !slices.Equal(got, want) {
					t.Errorf("paged records:\n got  %q\n want %q (%d stored)", got, want, len(records))
				}

				if stored := recordKeys(records); !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(stored))) {
					t.Errorf("each stored record must be listed exactly once:\n got  %q\n stored %q", got, stored)
				}
			})
		}
	}
}

// Spellings that share a position are listed in the ASCII order of their
// names, so the order, and continuing from Next*, does not depend on the
// order they were stored in.
func TestListResourceRecordSets_EqualPositionsOrderedByName(t *testing.T) {
	t.Parallel()

	want := []string{"A.EXAMPLE.COM. A", `\141.example.com. A`, aNameNoDot + " A", aName + " A"}

	stored := []ResourceRecordSet{
		{Name: aName, Type: "A"},
		{Name: `\141.example.com.`, Type: "A"},
		{Name: aNameNoDot, Type: "A"},
		{Name: "A.EXAMPLE.COM.", Type: "A"},
	}
	reversed := slices.Clone(stored)
	slices.Reverse(reversed)

	for name, records := range map[string][]ResourceRecordSet{"stored": stored, "reversed": reversed} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc := newRRSetListService(t, records)

			resp := decodeRRSetList(t, listRRSets(t, svc, url.Values{}))
			if got := recordKeys(resp.ResourceRecordSets); !slices.Equal(got, want) {
				t.Errorf("order:\n got  %q\n want %q", got, want)
			}

			if got := pageAllRecordSets(t, svc, len(records), 1); !slices.Equal(got, want) {
				t.Errorf("paged one at a time:\n got  %q\n want %q", got, want)
			}
		})
	}
}

func TestListResourceRecordSets_EscapedDotStaysInsideLabel(t *testing.T) {
	t.Parallel()

	svc := newRRSetListService(t, []ResourceRecordSet{
		{Name: escapedDotName, Type: typeTXT},
		{Name: dottedName, Type: typeTXT},
		{Name: escapedDotName, Type: "A"},
		{Name: dottedName, Type: "A"},
	})

	sorted := []string{
		dottedName + " A",
		dottedName + " TXT",
		escapedDotName + " A",
		escapedDotName + " TXT",
	}

	tests := []struct {
		name  string
		query url.Values
		want  []string
	}{
		{name: "no start", query: url.Values{}, want: sorted},
		{name: "start at escaped name", query: url.Values{qName: {escapedDotName}, qType: {"A"}}, want: sorted[2:]},
		{name: "start at dotted name TXT", query: url.Values{qName: {dottedName}, qType: {typeTXT}}, want: sorted[1:]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp := decodeRRSetList(t, listRRSets(t, svc, tt.query))

			if got := recordKeys(resp.ResourceRecordSets); !slices.Equal(got, tt.want) {
				t.Errorf("records:\n got  %q\n want %q", got, tt.want)
			}
		})
	}
}

func TestListResourceRecordSets_HighByteEscapesStayDistinct(t *testing.T) {
	t.Parallel()

	svc := newRRSetListService(t, []ResourceRecordSet{
		{Name: escaped201Name, Type: "A"},
		{Name: `\200.example.com.`, Type: "A"},
	})

	resp := decodeRRSetList(t, listRRSets(t, svc, url.Values{}))

	want := []string{`\200.example.com. A`, `\201.example.com. A`}
	if got := recordKeys(resp.ResourceRecordSets); !slices.Equal(got, want) {
		t.Errorf("order:\n got  %q\n want %q", got, want)
	}

	resp = decodeRRSetList(t, listRRSets(t, svc, url.Values{qName: {escaped201Name}, qType: {"A"}}))

	want = []string{`\201.example.com. A`}
	if got := recordKeys(resp.ResourceRecordSets); !slices.Equal(got, want) {
		t.Errorf("start at \\201:\n got  %q\n want %q", got, want)
	}
}

func TestListResourceRecordSets_StartNameMatchesOtherSpellings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stored string
		start  string
	}{
		{name: "stored upper case, queried lower case", stored: "A.example.com.", start: aName},
		{name: "stored lower case, queried upper case", stored: aName, start: "A.EXAMPLE.COM."},
		{name: "stored without trailing dot", stored: aNameNoDot, start: aName},
		{name: "stored escaped upper case, queried lower case", stored: escapedUpperZName, start: zName},
		{name: "stored lower case, queried escaped upper case", stored: zName, start: escapedUpperZName},
		{name: "stored escaped asterisk, queried literal", stored: escapedStarName, start: starName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := newRRSetListService(t, []ResourceRecordSet{
				{Name: zoneApex, Type: "NS"},
				{Name: tt.stored, Type: "A"},
			})

			resp := decodeRRSetList(t, listRRSets(t, svc, url.Values{qName: {tt.start}, qType: {"A"}}))

			want := []string{tt.stored + " A"}
			if got := recordKeys(resp.ResourceRecordSets); !slices.Equal(got, want) {
				t.Errorf("records:\n got  %q\n want %q", got, want)
			}
		})
	}
}

func TestReversedLabelsKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{name: wwwName, want: "com.example.www."},
		{name: "WWW.Example.com", want: "com.example.www."},
		{name: escapedStarName, want: "com.example.*."},
		{name: starName, want: "com.example.*."},
		{name: escapedUpperZName, want: zKey},
		{name: "Z.example.com.", want: zKey},
		{name: `\200.example.com.`, want: "com.example.\x80."},
		{name: escaped201Name, want: "com.example.\x81."},
		{name: `\101\200.example.com.`, want: "com.example.a\x80."},
		{name: zName, want: zKey},
		// A period or backslash inside a label stays escaped so the key
		// cannot be confused with a label boundary or another escape.
		{name: escapedDotName, want: `com.example.a\056b.`},
		{name: "b.a.example.com.", want: "com.example.a.b."},
		{name: `A\056B.example.com.`, want: `com.example.a\056b.`},
		{name: `a\134056b.example.com.`, want: `com.example.a\134056b.`},
		{name: `a\134x.example.com.`, want: `com.example.a\134x.`},
		{name: `a\x.example.com.`, want: `com.example.a\134x.`},
		{name: `\400.example.com.`, want: `com.example.\134400.`},
		{name: ".", want: ""},
		{name: "", want: ""},
	}

	for _, tt := range tests {
		if got := reversedLabelsKey(tt.name); got != tt.want {
			t.Errorf("reversedLabelsKey(%q): got %q, want %q", tt.name, got, tt.want)
		}
	}
}
