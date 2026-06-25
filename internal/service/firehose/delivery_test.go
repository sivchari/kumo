package firehose

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testBucketARN = "arn:aws:s3:::audit-worm"
	testBucket    = "audit-worm"
	testPrefix    = "audit/"
	testStream    = "audit-to-s3"
)

// capturedPut records one PutObject call to the fake S3 layer.
type capturedPut struct {
	bucket      string
	key         string
	contentType string
	data        []byte
}

// fakePutter is an in-memory S3Putter that records calls.
type fakePutter struct {
	mu   sync.Mutex
	puts []capturedPut
}

func (f *fakePutter) PutObject(_ context.Context, bucket, key string, data []byte, contentType string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.puts = append(f.puts, capturedPut{bucket: bucket, key: key, contentType: contentType, data: append([]byte(nil), data...)})

	return nil
}

func (f *fakePutter) calls() []capturedPut {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]capturedPut(nil), f.puts...)
}

// newStreamWithS3 creates an ExtendedS3 delivery stream for delivery tests.
func newStreamWithS3(t *testing.T, s *MemoryStorage, compression string) {
	t.Helper()

	_, err := s.CreateDeliveryStream(t.Context(), &CreateDeliveryStreamInput{
		DeliveryStreamName: testStream,
		DeliveryStreamType: string(DeliveryStreamTypeDirectPut),
		ExtendedS3DestinationConfiguration: &ExtendedS3DestinationConfiguration{
			BucketARN:         testBucketARN,
			Prefix:            testPrefix,
			CompressionFormat: compression,
		},
	})
	if err != nil {
		t.Fatalf("CreateDeliveryStream: unexpected error: %v", err)
	}
}

func TestFlushAll_DeliversConcatenatedRecords(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage()
	newStreamWithS3(t, s, "")

	putter := &fakePutter{}
	s.s3Putter = putter

	for _, line := range []string{"line1\n", "line2\n"} {
		if _, err := s.PutRecord(t.Context(), testStream, Record{Data: []byte(line)}); err != nil {
			t.Fatalf("PutRecord: unexpected error: %v", err)
		}
	}

	s.flushAll(true)

	calls := putter.calls()
	if len(calls) != 1 {
		t.Fatalf("PutObject calls: got %d, want 1", len(calls))
	}

	got := calls[0]
	if got.bucket != testBucket {
		t.Errorf("bucket: got %q, want %q", got.bucket, testBucket)
	}

	if !strings.HasPrefix(got.key, testPrefix) {
		t.Errorf("key: got %q, want prefix %q", got.key, testPrefix)
	}

	if !strings.Contains(got.key, testStream) {
		t.Errorf("key: got %q, want it to contain stream name %q", got.key, testStream)
	}

	if want := "line1\nline2\n"; string(got.data) != want {
		t.Errorf("body: got %q, want %q", string(got.data), want)
	}

	if s.Streams[testStream].DeliveredCount != 2 {
		t.Errorf("DeliveredCount: got %d, want 2", s.Streams[testStream].DeliveredCount)
	}
}

func TestFlushAll_GzipCompression(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage()
	newStreamWithS3(t, s, "GZIP")

	putter := &fakePutter{}
	s.s3Putter = putter

	if _, err := s.PutRecord(t.Context(), testStream, Record{Data: []byte("hello\n")}); err != nil {
		t.Fatalf("PutRecord: unexpected error: %v", err)
	}

	s.flushAll(true)

	calls := putter.calls()
	if len(calls) != 1 {
		t.Fatalf("PutObject calls: got %d, want 1", len(calls))
	}

	if !strings.HasSuffix(calls[0].key, ".gz") {
		t.Errorf("key: got %q, want .gz suffix", calls[0].key)
	}

	zr, err := gzip.NewReader(bytes.NewReader(calls[0].data))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}

	decoded, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read gzip: %v", err)
	}

	if want := "hello\n"; string(decoded) != want {
		t.Errorf("decompressed body: got %q, want %q", string(decoded), want)
	}
}

func TestFlushAll_NotForcedHoldsBelowThreshold(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage()
	newStreamWithS3(t, s, "")

	putter := &fakePutter{}
	s.s3Putter = putter

	if _, err := s.PutRecord(t.Context(), testStream, Record{Data: []byte("x\n")}); err != nil {
		t.Fatalf("PutRecord: unexpected error: %v", err)
	}

	// Default buffering interval is 300s and size 5MB; a single fresh, tiny
	// record must not trigger an unforced flush.
	s.flushAll(false)

	if calls := putter.calls(); len(calls) != 0 {
		t.Fatalf("PutObject calls: got %d, want 0 (should stay buffered)", len(calls))
	}

	if s.Streams[testStream].DeliveredCount != 0 {
		t.Errorf("DeliveredCount: got %d, want 0", s.Streams[testStream].DeliveredCount)
	}
}

func TestFlushAll_NoDeliveryWithoutPutter(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage()
	newStreamWithS3(t, s, "")

	if _, err := s.PutRecord(t.Context(), testStream, Record{Data: []byte("x\n")}); err != nil {
		t.Fatalf("PutRecord: unexpected error: %v", err)
	}

	// No putter installed: flushAll must be a no-op and leave records buffered.
	s.flushAll(true)

	if s.Streams[testStream].DeliveredCount != 0 {
		t.Errorf("DeliveredCount: got %d, want 0", s.Streams[testStream].DeliveredCount)
	}
}

// errPutFailed is returned by errPutter to exercise the delivery-failure path.
var errPutFailed = errors.New("put failed")

// errPutter is an S3Putter whose writes always fail.
type errPutter struct{}

func (errPutter) PutObject(_ context.Context, _, _ string, _ []byte, _ string) error {
	return errPutFailed
}

// waitForPuts polls the fake putter until it has recorded want calls or the
// deadline passes (the background flush loop delivers from a goroutine).
func waitForPuts(t *testing.T, putter *fakePutter, want int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(putter.calls()) >= want {
			return
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Fatalf("waiting for %d puts: got %d", want, len(putter.calls()))
}

func TestFlushTick(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want time.Duration
	}{
		{"empty uses default", "", defaultFlushTick},
		{"valid override", "50", 50 * time.Millisecond},
		{"non-numeric falls back", "abc", defaultFlushTick},
		{"zero falls back", "0", defaultFlushTick},
		{"negative falls back", "-5", defaultFlushTick},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KUMO_FIREHOSE_FLUSH_INTERVAL_MS", tc.env)

			if got := flushTick(); got != tc.want {
				t.Fatalf("flushTick(): got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBufferInterval(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		hints *BufferingHints
		want  time.Duration
	}{
		{"nil uses default", nil, defaultBufferIntervalSeconds * time.Second},
		{"zero uses default", &BufferingHints{IntervalInSeconds: 0}, defaultBufferIntervalSeconds * time.Second},
		{"explicit interval", &BufferingHints{IntervalInSeconds: 60}, 60 * time.Second},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := bufferInterval(tc.hints); got != tc.want {
				t.Fatalf("bufferInterval: got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBufferSize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		hints *BufferingHints
		want  int
	}{
		{"nil uses default", nil, defaultBufferSizeMB * bytesPerMB},
		{"zero uses default", &BufferingHints{SizeInMBs: 0}, defaultBufferSizeMB * bytesPerMB},
		{"explicit size", &BufferingHints{SizeInMBs: 2}, 2 * bytesPerMB},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := bufferSize(tc.hints); got != tc.want {
				t.Fatalf("bufferSize: got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShouldFlush(t *testing.T) {
	t.Parallel()

	now := time.Now()

	cases := []struct {
		name    string
		pending []StoredRecord
		target  s3DeliveryTarget
		want    bool
	}{
		{
			"size threshold met",
			[]StoredRecord{{Data: []byte("0123456789"), Received: now}},
			s3DeliveryTarget{size: 5, interval: time.Hour},
			true,
		},
		{
			"interval elapsed",
			[]StoredRecord{{Data: []byte("x"), Received: now.Add(-time.Hour)}},
			s3DeliveryTarget{size: bytesPerMB, interval: time.Minute},
			true,
		},
		{
			"below both thresholds",
			[]StoredRecord{{Data: []byte("x"), Received: now}},
			s3DeliveryTarget{size: bytesPerMB, interval: time.Hour},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldFlush(tc.pending, tc.target); got != tc.want {
				t.Fatalf("shouldFlush: got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestS3Target(t *testing.T) {
	t.Parallel()

	extended := &DeliveryStream{Destinations: []DestinationDescription{{
		ExtendedS3DestinationDescription: &ExtendedS3DestinationDescription{BucketARN: testBucketARN},
	}}}
	plain := &DeliveryStream{Destinations: []DestinationDescription{{
		S3DestinationDescription: &S3DestinationDescription{BucketARN: testBucketARN},
	}}}
	none := &DeliveryStream{Destinations: []DestinationDescription{{DestinationID: "x"}}}

	cases := []struct {
		name       string
		stream     *DeliveryStream
		wantOK     bool
		wantBucket string
	}{
		{"extended s3", extended, true, testBucket},
		{"plain s3", plain, true, testBucket},
		{"no s3 destination", none, false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := s3Target(tc.stream)
			if ok != tc.wantOK {
				t.Fatalf("s3Target ok: got %v, want %v", ok, tc.wantOK)
			}

			if ok && got.bucket != tc.wantBucket {
				t.Errorf("bucket: got %q, want %q", got.bucket, tc.wantBucket)
			}
		})
	}
}

func TestFlushStream_ClampsDeliveredCount(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage()
	newStreamWithS3(t, s, "")

	putter := &fakePutter{}
	s.s3Putter = putter

	if _, err := s.PutRecord(t.Context(), testStream, Record{Data: []byte("x\n")}); err != nil {
		t.Fatalf("PutRecord: unexpected error: %v", err)
	}

	// Simulate a stale high-water mark above the record count (e.g. truncated
	// state restored from disk).
	s.Streams[testStream].DeliveredCount = 99

	s.flushAll(true)

	// flushStream clamps DeliveredCount to len(Records); pending is then empty so
	// nothing is delivered.
	if got := s.Streams[testStream].DeliveredCount; got != 1 {
		t.Errorf("DeliveredCount: got %d, want 1 (clamped)", got)
	}

	if calls := putter.calls(); len(calls) != 0 {
		t.Errorf("PutObject calls: got %d, want 0", len(calls))
	}
}

func TestFlushStream_PutObjectError(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage()
	newStreamWithS3(t, s, "")

	s.s3Putter = errPutter{}

	if _, err := s.PutRecord(t.Context(), testStream, Record{Data: []byte("x\n")}); err != nil {
		t.Fatalf("PutRecord: unexpected error: %v", err)
	}

	s.flushAll(true)

	// Delivery failed: records stay buffered for the next attempt.
	if got := s.Streams[testStream].DeliveredCount; got != 0 {
		t.Errorf("DeliveredCount: got %d, want 0 (delivery failed)", got)
	}
}

func TestFlushStream_NoS3Destination(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage()
	if _, err := s.CreateDeliveryStream(t.Context(), &CreateDeliveryStreamInput{
		DeliveryStreamName: "no-dest",
		DeliveryStreamType: string(DeliveryStreamTypeDirectPut),
	}); err != nil {
		t.Fatalf("CreateDeliveryStream: unexpected error: %v", err)
	}

	putter := &fakePutter{}
	s.s3Putter = putter

	if _, err := s.PutRecord(t.Context(), "no-dest", Record{Data: []byte("x\n")}); err != nil {
		t.Fatalf("PutRecord: unexpected error: %v", err)
	}

	s.flushAll(true)

	// A stream without an S3 destination is skipped: nothing is delivered.
	if calls := putter.calls(); len(calls) != 0 {
		t.Errorf("PutObject calls: got %d, want 0", len(calls))
	}
}

func TestSetS3Putter_BackgroundFlush(t *testing.T) {
	t.Setenv("KUMO_FIREHOSE_FLUSH_INTERVAL_MS", "5")

	s := NewMemoryStorage()

	t.Cleanup(func() { _ = s.Close() })

	newStreamWithS3(t, s, "")

	if _, err := s.PutRecord(t.Context(), testStream, Record{Data: []byte("buffered\n")}); err != nil {
		t.Fatalf("PutRecord: unexpected error: %v", err)
	}

	// SetS3Putter installs the target and starts the background flush loop, which
	// delivers the buffered record once the override interval elapses.
	putter := &fakePutter{}
	s.SetS3Putter(putter)

	waitForPuts(t, putter, 1)

	if got := string(putter.calls()[0].data); got != "buffered\n" {
		t.Errorf("body: got %q, want %q", got, "buffered\n")
	}
}

func TestBucketFromARN(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		arn  string
		want string
	}{
		{"standard arn", "arn:aws:s3:::audit-worm", "audit-worm"},
		{"plain name unchanged", "audit-worm", "audit-worm"},
		{"empty", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := bucketFromARN(tc.arn); got != tc.want {
				t.Fatalf("bucketFromARN(%q): got %q, want %q", tc.arn, got, tc.want)
			}
		})
	}
}
