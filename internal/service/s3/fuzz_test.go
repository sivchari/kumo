package s3

import "testing"

func FuzzParseByteRangeInvariants(f *testing.F) {
	f.Add("bytes=0-99", int64(1000))
	f.Add("bytes=-100", int64(1000))
	f.Add("bytes=500-", int64(1000))
	f.Add("bytes=200-100", int64(1000))
	f.Add("items=0-99", int64(1000))
	f.Add("bytes=-1", int64(0))

	f.Fuzz(func(t *testing.T, header string, totalSize int64) {
		start, end, ok := parseByteRange(header, totalSize)
		if !ok {
			return
		}

		if totalSize <= 0 {
			t.Fatalf("parseByteRange(%q, %d) returned ok for non-positive content length: start=%d end=%d",
				header, totalSize, start, end)
		}

		if start < 0 || end < start || end >= totalSize {
			t.Fatalf("parseByteRange(%q, %d) = (%d, %d, true), violates byte-range bounds",
				header, totalSize, start, end)
		}
	})
}

func FuzzParseCopySourceAndRangeNoPanic(f *testing.F) {
	f.Add("/bucket/key", "")
	f.Add("bucket%2Fkey", "bytes=0-99")
	f.Add("bucket/key%2Fpart", "bytes= 0 - 99 ")
	f.Add("%zz", "bytes=-99")
	f.Add("single-segment", "items=0-99")

	f.Fuzz(func(t *testing.T, source, copyRange string) {
		_, _, _ = parseCopySource(source)

		rng, err := parseCopySourceRange(copyRange)
		if err != nil || rng == nil {
			return
		}

		if rng.Start < 0 || rng.End < rng.Start {
			t.Fatalf("parseCopySourceRange(%q) = %+v, violates closed range bounds", copyRange, rng)
		}
	})
}
