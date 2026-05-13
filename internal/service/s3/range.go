package s3

import (
	"strconv"
	"strings"
)

// parseByteRange interprets a single-range "Range: bytes=START-END"
// header against a known content length. Returns (start, end, true)
// where end is inclusive (the wire form of Content-Range).
//
// Spec covered (RFC 9110 §14):
//
//   - Suffix range: `bytes=-100`  → last 100 bytes
//   - Open-ended:    `bytes=100-`  → byte 100 to end
//   - Closed:        `bytes=0-99`  → bytes 0 through 99 inclusive
//
// Multi-range requests (`bytes=0-99,200-299`), syntactically invalid
// specs, and unsatisfiable ranges (start past end-of-content,
// inverted range, wrong unit) all return ok=false. The caller should
// fall through to a full 200 in those cases — multi-part 206 is left
// for a follow-up since the typical S3 consumer (multipart download)
// only sends single-range requests.
func parseByteRange(header string, totalSize int64) (start, end int64, ok bool) {
	startRaw, endRaw, valid := splitByteRangeSpec(header)
	if !valid {
		return 0, 0, false
	}

	switch {
	case startRaw == "" && endRaw == "":
		return 0, 0, false
	case startRaw == "":
		return parseSuffixRange(endRaw, totalSize)
	case endRaw == "":
		return parseOpenRange(startRaw, totalSize)
	default:
		return parseClosedRange(startRaw, endRaw, totalSize)
	}
}

// splitByteRangeSpec strips the "bytes=" prefix and splits on the
// dash. Returns ok=false on multi-range or wrong unit.
func splitByteRangeSpec(header string) (string, string, bool) {
	spec, ok := strings.CutPrefix(header, "bytes=")
	if !ok {
		return "", "", false
	}

	if strings.Contains(spec, ",") {
		return "", "", false
	}

	dash := strings.IndexByte(spec, '-')
	if dash < 0 {
		return "", "", false
	}

	return strings.TrimSpace(spec[:dash]), strings.TrimSpace(spec[dash+1:]), true
}

// parseSuffixRange resolves `bytes=-N` (the last N bytes). N is
// clamped to totalSize so a request for "the last 9999 bytes" of a
// 100-byte object returns the whole object.
func parseSuffixRange(endRaw string, totalSize int64) (int64, int64, bool) {
	n, err := strconv.ParseInt(endRaw, 10, 64)
	if err != nil || n <= 0 || totalSize <= 0 {
		return 0, 0, false
	}

	if n > totalSize {
		n = totalSize
	}

	return totalSize - n, totalSize - 1, true
}

// parseOpenRange resolves `bytes=START-` (from START to end of
// content).
func parseOpenRange(startRaw string, totalSize int64) (int64, int64, bool) {
	start, err := strconv.ParseInt(startRaw, 10, 64)
	if err != nil || start < 0 || start >= totalSize {
		return 0, 0, false
	}

	return start, totalSize - 1, true
}

// parseClosedRange resolves `bytes=START-END`. END is clamped to the
// last byte of content; start past end → unsatisfiable.
func parseClosedRange(startRaw, endRaw string, totalSize int64) (int64, int64, bool) {
	start, err1 := strconv.ParseInt(startRaw, 10, 64)
	end, err2 := strconv.ParseInt(endRaw, 10, 64)

	if err1 != nil || err2 != nil || start < 0 || end < start || start >= totalSize {
		return 0, 0, false
	}

	if end >= totalSize {
		end = totalSize - 1
	}

	return start, end, true
}
