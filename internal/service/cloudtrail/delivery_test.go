package cloudtrail

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sivchari/kumo/internal/cloudtrailevents"
)

const (
	deliverBucket = "audit-worm"
	deliverPrefix = "cloudtrail"
	deliverTrail  = "authz-trail"
)

type capturedPut struct {
	bucket string
	key    string
	data   []byte
}

type fakePutter struct {
	mu   sync.Mutex
	puts []capturedPut
}

func (f *fakePutter) PutObject(_ context.Context, bucket, key string, data []byte, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.puts = append(f.puts, capturedPut{bucket: bucket, key: key, data: append([]byte(nil), data...)})

	return nil
}

func (f *fakePutter) calls() []capturedPut {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]capturedPut(nil), f.puts...)
}

func TestObjectKey(t *testing.T) {
	t.Parallel()

	key := objectKey(deliverPrefix, "us-east-1")

	wantPrefix := "cloudtrail/AWSLogs/000000000000/CloudTrail/us-east-1/"
	if !strings.HasPrefix(key, wantPrefix) {
		t.Errorf("key: got %q, want prefix %q", key, wantPrefix)
	}

	if !strings.HasSuffix(key, ".json.gz") {
		t.Errorf("key: got %q, want .json.gz suffix", key)
	}

	if !strings.Contains(key, "000000000000_CloudTrail_us-east-1_") {
		t.Errorf("key: got %q, want it to contain the file-name stem", key)
	}
}

func TestMarkerKey(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		prefix string
		want   string
	}{
		{"with prefix", "cloudtrail", "cloudtrail/AWSLogs/000000000000/CloudTrail/"},
		{"prefix trailing slash", "cloudtrail/", "cloudtrail/AWSLogs/000000000000/CloudTrail/"},
		{"empty prefix", "", "AWSLogs/000000000000/CloudTrail/"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := markerKey(tc.prefix); got != tc.want {
				t.Fatalf("markerKey(%q): got %q, want %q", tc.prefix, got, tc.want)
			}
		})
	}
}

func TestBuildLogObject(t *testing.T) {
	t.Parallel()

	events := []cloudtrailevents.Event{
		{EventTime: time.Unix(0, 0).UTC(), EventSource: "dynamodb.amazonaws.com", EventName: "PutItem"},
		{EventTime: time.Unix(0, 0).UTC(), EventSource: "cognito-idp.amazonaws.com", EventName: "AdminSetUserPassword"},
	}

	body := buildLogObject(events, "us-east-1")

	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}

	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read gzip: %v", err)
	}

	var got logFile
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal log file: %v (raw=%s)", err, raw)
	}

	if len(got.Records) != 2 {
		t.Fatalf("records: got %d, want 2", len(got.Records))
	}

	if got.Records[0].EventName != "PutItem" {
		t.Errorf("record[0] eventName: got %q, want PutItem", got.Records[0].EventName)
	}

	if got.Records[0].RecipientAccountID != deliveryAccountID {
		t.Errorf("record[0] recipientAccountId: got %q, want %q", got.Records[0].RecipientAccountID, deliveryAccountID)
	}
}

// TestDeliverPending exercises the full drain-to-S3 path. It touches the shared
// cloudtrailevents.Global sink, so it does not run in parallel.
func TestDeliverPending(t *testing.T) {
	cloudtrailevents.Global.Drain()
	cloudtrailevents.Global.SetLogging(false)

	t.Cleanup(func() {
		cloudtrailevents.Global.Drain()
		cloudtrailevents.Global.SetLogging(false)
	})

	s := NewMemoryStorage()
	ctx := t.Context()

	if _, err := s.CreateTrail(ctx, &CreateTrailRequest{Name: deliverTrail, S3BucketName: deliverBucket, S3KeyPrefix: deliverPrefix}); err != nil {
		t.Fatalf("CreateTrail: %v", err)
	}

	if err := s.StartLogging(ctx, deliverTrail); err != nil {
		t.Fatalf("StartLogging: %v", err)
	}

	if !cloudtrailevents.Global.Logging() {
		t.Fatalf("StartLogging should enable the global sink")
	}

	putter := &fakePutter{}
	s.s3Putter = putter

	cloudtrailevents.Global.Record(&cloudtrailevents.Event{EventName: "PutItem", EventSource: "dynamodb.amazonaws.com"})
	cloudtrailevents.Global.Record(&cloudtrailevents.Event{EventName: "Invoke", EventSource: "lambda.amazonaws.com"})

	s.deliverPending()

	calls := putter.calls()
	if len(calls) != 2 {
		t.Fatalf("PutObject calls: got %d, want 2 (marker + log object)", len(calls))
	}

	assertMarkerAndLog(t, calls)

	// A second cycle with no new events writes nothing further.
	s.deliverPending()

	if got := len(putter.calls()); got != 2 {
		t.Fatalf("PutObject calls after idle cycle: got %d, want 2", got)
	}
}

func assertMarkerAndLog(t *testing.T, calls []capturedPut) {
	t.Helper()

	var sawMarker, sawLog bool

	for _, c := range calls {
		if c.bucket != deliverBucket {
			t.Errorf("bucket: got %q, want %q", c.bucket, deliverBucket)
		}

		switch {
		case c.key == "cloudtrail/AWSLogs/000000000000/CloudTrail/":
			sawMarker = true

			if len(c.data) != 0 {
				t.Errorf("marker object should be empty, got %d bytes", len(c.data))
			}
		case strings.HasSuffix(c.key, ".json.gz"):
			sawLog = true
		}
	}

	if !sawMarker {
		t.Error("did not see the 0-byte CloudTrail marker object")
	}

	if !sawLog {
		t.Error("did not see a gzipped CloudTrail log object")
	}
}

// waitForPuts polls the fake putter until it has recorded want calls or the
// deadline passes (the background delivery loop writes from a goroutine).
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

func TestDeliverTick(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want time.Duration
	}{
		{"empty uses default", "", defaultDeliverTick},
		{"valid override", "50", 50 * time.Millisecond},
		{"non-numeric falls back", "abc", defaultDeliverTick},
		{"zero falls back", "0", defaultDeliverTick},
		{"negative falls back", "-5", defaultDeliverTick},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KUMO_CLOUDTRAIL_DELIVER_INTERVAL_MS", tc.env)

			if got := deliverTick(); got != tc.want {
				t.Fatalf("deliverTick(): got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCollectJobs exercises job collection in isolation. It flips IsLogging
// directly instead of via StartLogging so it never touches the shared sink and
// can run in parallel.
func TestCollectJobs(t *testing.T) {
	t.Parallel()

	m := NewMemoryStorage()

	// No putter installed yet.
	if jobs, p := m.collectJobs(); jobs != nil || p != nil {
		t.Fatalf("no putter: got (%v, %v), want (nil, nil)", jobs, p)
	}

	m.s3Putter = &fakePutter{}

	if _, err := m.CreateTrail(t.Context(), &CreateTrailRequest{Name: deliverTrail, S3BucketName: deliverBucket}); err != nil {
		t.Fatalf("CreateTrail: unexpected error: %v", err)
	}

	// A trail that is not logging is skipped.
	if jobs, _ := m.collectJobs(); len(jobs) != 0 {
		t.Fatalf("non-logging trail: got %d jobs, want 0", len(jobs))
	}

	m.Trails[deliverTrail].IsLogging = true

	// First pass reserves the marker.
	jobs, _ := m.collectJobs()
	if len(jobs) != 1 || !jobs[0].needMarker {
		t.Fatalf("logging trail: got %+v, want one job needing a marker", jobs)
	}

	// Second pass: the marker is already written.
	again, _ := m.collectJobs()
	if len(again) != 1 || again[0].needMarker {
		t.Fatalf("second pass: got %+v, want one job without a marker", again)
	}
}

func TestDeliverPending_NoJobs(t *testing.T) {
	t.Parallel()

	m := NewMemoryStorage()

	// No putter installed: deliverPending returns before draining events.
	m.deliverPending()

	// Putter installed but no logging trail: nothing to deliver.
	putter := &fakePutter{}
	m.s3Putter = putter

	m.deliverPending()

	if got := len(putter.calls()); got != 0 {
		t.Fatalf("PutObject calls: got %d, want 0", got)
	}
}

// TestSetS3Putter_BackgroundDelivery drives the background loop end to end. It
// touches the shared cloudtrailevents.Global sink, so it does not run parallel.
func TestSetS3Putter_BackgroundDelivery(t *testing.T) {
	t.Setenv("KUMO_CLOUDTRAIL_DELIVER_INTERVAL_MS", "5")

	cloudtrailevents.Global.Drain()
	cloudtrailevents.Global.SetLogging(false)

	t.Cleanup(func() {
		cloudtrailevents.Global.Drain()
		cloudtrailevents.Global.SetLogging(false)
	})

	s := NewMemoryStorage()

	t.Cleanup(func() { _ = s.Close() })

	ctx := t.Context()
	if _, err := s.CreateTrail(ctx, &CreateTrailRequest{Name: deliverTrail, S3BucketName: deliverBucket, S3KeyPrefix: deliverPrefix}); err != nil {
		t.Fatalf("CreateTrail: unexpected error: %v", err)
	}

	if err := s.StartLogging(ctx, deliverTrail); err != nil {
		t.Fatalf("StartLogging: unexpected error: %v", err)
	}

	cloudtrailevents.Global.Record(&cloudtrailevents.Event{EventName: "PutItem", EventSource: "dynamodb.amazonaws.com"})

	// SetS3Putter installs the target and starts the delivery loop, which writes
	// the marker plus a log object on its next tick.
	putter := &fakePutter{}
	s.SetS3Putter(putter)

	waitForPuts(t, putter, 2)
}
