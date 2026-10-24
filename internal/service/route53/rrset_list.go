package route53

import (
	"slices"
	"strconv"
	"strings"
)

// maxListRecordSets is the largest page size of ListResourceRecordSets: the
// API reference says it "returns up to 300 resource record sets at a time".
// The reference does not state a default; kumo uses the same value.
const maxListRecordSets = 300

// listRecordTypes are the valid values the API reference lists for the type
// parameter of ListResourceRecordSets.
var listRecordTypes = []string{
	"SOA", "A", "TXT", "NS", "CNAME", "MX", "NAPTR", "PTR", "SRV",
	"SPF", "AAAA", "CAA", "DS", "TLSA", "SSHFP", "SVCB", "HTTPS",
}

// isListRecordType reports whether recordType is one of listRecordTypes,
// spelled exactly as the reference lists it.
func isListRecordType(recordType string) bool {
	return slices.Contains(listRecordTypes, recordType)
}

// recordSetPosition is a record set's place in the ListResourceRecordSets
// order. The API reference documents the name order (labels reversed, ASCII)
// and that records with the same name are sorted by type. Comparing types in
// ASCII order follows the reference's "in ASCII order" prose; its example
// response lists SOA before NS, which ASCII order does not. Types are compared
// as spelled, since the reference lists each valid value in one spelling.
// Ordering by set identifier after type is a kumo choice so that weighted and
// other routed record sets have a stable order.
type recordSetPosition struct {
	name          string
	recordType    string
	setIdentifier string
}

// positionOf returns the sort position of a stored record set.
func positionOf(rrset *ResourceRecordSet) recordSetPosition {
	return recordSetPosition{
		name:          reversedLabelsKey(rrset.Name),
		recordType:    rrset.Type,
		setIdentifier: rrset.SetIdentifier,
	}
}

// compare orders positions by name, then type, then set identifier, each in
// ASCII order.
func (p recordSetPosition) compare(o recordSetPosition) int {
	if c := strings.Compare(p.name, o.name); c != 0 {
		return c
	}

	if c := strings.Compare(p.recordType, o.recordType); c != 0 {
		return c
	}

	return strings.Compare(p.setIdentifier, o.setIdentifier)
}

// listStart is where a ListResourceRecordSets page begins: the normalized
// position plus the name, type and identifier exactly as the request spelled
// them.
type listStart struct {
	position      recordSetPosition
	name          string
	recordType    string
	setIdentifier string
}

// newListStart builds the start of a listing from the name, type and
// identifier query values. The reference describes identifier only as a
// continuation within a name and type and documents no error for it alone,
// so kumo ignores it unless type is given rather than rejecting the request.
func newListStart(name, recordType, identifier string) listStart {
	if recordType == "" {
		identifier = ""
	}

	return listStart{
		position: recordSetPosition{
			name:          reversedLabelsKey(name),
			recordType:    recordType,
			setIdentifier: identifier,
		},
		name:          name,
		recordType:    recordType,
		setIdentifier: identifier,
	}
}

// reversedLabelsKey returns name with its labels reversed and a trailing dot,
// e.g. www.example.com -> com.example.www., the form the API reference says
// ListResourceRecordSets sorts on. Each label has its \ddd octal escapes
// decoded and is then passed through labelKey, which folds ASCII letters to
// lowercase, since Route 53 stores letters as lowercase whether given as
// upper case, lower case or escape codes. A name with and without the
// trailing dot gives the same key.
func reversedLabelsKey(name string) string {
	name = strings.TrimSuffix(name, ".")
	if name == "" {
		return ""
	}

	labels := strings.Split(name, ".")
	slices.Reverse(labels)

	var b strings.Builder

	for _, label := range labels {
		b.WriteString(labelKey(decodeOctalEscapes(label)))
		b.WriteByte('.')
	}

	return b.String()
}

// labelKey turns a decoded label into its part of the sort key: A-Z are
// folded to a-z, a period inside the label is written back as \056 so it
// cannot be mistaken for a label boundary, and a backslash as \134 so that a
// literal backslash followed by digits cannot be mistaken for such an escape.
// Every other byte is kept as is. It works on bytes rather than runes because
// decoded escapes such as \200 are not valid UTF-8 and must stay distinct.
func labelKey(label string) string {
	var b strings.Builder

	b.Grow(len(label))

	for i := range len(label) {
		switch c := label[i]; {
		case 'A' <= c && c <= 'Z':
			b.WriteByte(c + ('a' - 'A'))
		case c == '.':
			b.WriteString(`\056`)
		case c == '\\':
			b.WriteString(`\134`)
		default:
			b.WriteByte(c)
		}
	}

	return b.String()
}

// decodeOctalEscapes replaces each \ddd octal escape in a label with the byte
// it stands for, leaving anything else unchanged.
func decodeOctalEscapes(label string) string {
	if !strings.Contains(label, `\`) {
		return label
	}

	var b strings.Builder

	for i := 0; i < len(label); i++ {
		if label[i] == '\\' && i+4 <= len(label) && isOctalDigits(label[i+1:i+4]) {
			v, _ := strconv.ParseUint(label[i+1:i+4], 8, 8)
			b.WriteByte(byte(v))

			i += 3

			continue
		}

		b.WriteByte(label[i])
	}

	return b.String()
}

// isOctalDigits reports whether s is three octal digits forming a byte value.
func isOctalDigits(s string) bool {
	if len(s) != 3 || s[0] < '0' || s[0] > '3' {
		return false
	}

	for i := range len(s) {
		if s[i] < '0' || s[i] > '7' {
			return false
		}
	}

	return true
}

// parseRecordSetsMaxItems reads the maxitems query value. A positive integer
// is echoed back as MaxItems and caps the page at maxListRecordSets. Anything
// else falls back to maxListRecordSets instead of being rejected, a kumo
// choice the reference does not describe.
func parseRecordSetsMaxItems(raw string) (pageSize int, echoed string) {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return maxListRecordSets, strconv.Itoa(maxListRecordSets)
	}

	return min(n, maxListRecordSets), strconv.Itoa(n)
}

// positionedRecordSet is a record set together with its sort position, which
// is computed once per listing rather than in every comparison.
type positionedRecordSet struct {
	position recordSetPosition
	rrset    *ResourceRecordSet
}

// sortRecordSets returns records in ListResourceRecordSets order, each paired
// with its position. The returned entries point into records. Record sets
// that share a position are ordered by name as stored, in ASCII order, so the
// order does not depend on the order they were stored in.
func sortRecordSets(records []ResourceRecordSet) []positionedRecordSet {
	sorted := make([]positionedRecordSet, len(records))
	for i := range records {
		sorted[i] = positionedRecordSet{position: positionOf(&records[i]), rrset: &records[i]}
	}

	slices.SortFunc(sorted, func(a, b positionedRecordSet) int {
		if c := a.position.compare(b.position); c != 0 {
			return c
		}

		return strings.Compare(a.rrset.Name, b.rrset.Name)
	})

	return sorted
}

// pageRecordSets sorts records into ListResourceRecordSets order and returns
// the page that begins at start, the record set that would begin the next
// page, and whether there is one.
//
// The page begins at the first record set at or after start.position. Stored
// spellings that normalize to the same position (with and without the
// trailing dot, differing case or escapes) sort next to each other, ordered
// by name; when one of them matches the requested name, type and identifier
// exactly, the page begins there instead, so that a NextRecordName echoed
// from storage resumes on that record rather than repeating the earlier ones.
//
// If that record set is deleted before the next call, no spelling matches
// exactly and the page begins at the first record set of that position,
// listing again those of its spellings that the previous page returned. The
// reference asks callers not to submit a ChangeResourceRecordSets request
// "while you're paging through the results of a ListResourceRecordSets
// request".
func pageRecordSets(records []ResourceRecordSet, start *listStart, pageSize int) ([]ResourceRecordSet, *ResourceRecordSet, bool) {
	sorted := sortRecordSets(slices.Clone(records))

	first, _ := slices.BinarySearchFunc(sorted, start.position, func(entry positionedRecordSet, target recordSetPosition) int {
		return entry.position.compare(target)
	})

	if start.name != "" {
		for i := first; i < len(sorted) && sorted[i].position.compare(start.position) == 0; i++ {
			if rrset := sorted[i].rrset; rrset.Name == start.name && rrset.Type == start.recordType && rrset.SetIdentifier == start.setIdentifier {
				first = i

				break
			}
		}
	}

	rest := sorted[first:]

	page := make([]ResourceRecordSet, min(len(rest), pageSize))
	for i := range page {
		page[i] = *rest[i].rrset
	}

	if len(rest) <= pageSize {
		return page, nil, false
	}

	return page, rest[pageSize].rrset, true
}
