package cloudtrail

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/cloudtrailevents"
)

// Delivery constants.
const (
	defaultDeliverTick = 2 * time.Second
	eventVersion       = "1.09"
	eventType          = "AwsApiCall"
	hexIDLen           = 16
)

// S3Putter is the minimal S3 write surface the delivery loop needs. It is
// satisfied by an adapter installed via SetS3Putter in cross-service wiring,
// avoiding a direct dependency on the s3 package (and an import cycle).
type S3Putter interface {
	PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error
}

// SetS3Putter installs the S3 delivery target, reconciles the logging flag with
// any restored trails, and starts the background delivery loop (once).
func (m *MemoryStorage) SetS3Putter(p S3Putter) {
	m.mu.Lock()
	m.s3Putter = p
	m.refreshLoggingLocked()
	m.mu.Unlock()

	m.flushOnce.Do(func() {
		go m.deliveryLoop()
	})
}

// deliveryLoop drains captured events into S3 log files until flushCtx ends.
func (m *MemoryStorage) deliveryLoop() {
	ticker := time.NewTicker(deliverTick())
	defer ticker.Stop()

	for {
		select {
		case <-m.flushCtx.Done():
			return
		case <-ticker.C:
			m.deliverPending()
		}
	}
}

// deliveryJob is one logging trail's S3 destination for a delivery cycle.
type deliveryJob struct {
	bucket     string
	prefix     string
	needMarker bool
}

// deliverPending writes the 0-byte marker for newly-logging trails and, when
// events are buffered, a gzipped CloudTrail log file to each logging trail.
func (m *MemoryStorage) deliverPending() {
	jobs, putter := m.collectJobs()
	if putter == nil || len(jobs) == 0 {
		return
	}

	events := cloudtrailevents.Global.Drain()

	var object []byte
	if len(events) > 0 {
		object = buildLogObject(events, m.region)
	}

	for _, j := range jobs {
		if j.needMarker {
			_ = putter.PutObject(context.Background(), j.bucket, markerKey(j.prefix), nil, "application/octet-stream")
		}

		if object != nil {
			_ = putter.PutObject(context.Background(), j.bucket, objectKey(j.prefix, m.region), object, "application/x-gzip")
		}
	}
}

// collectJobs snapshots the logging trails and reserves their markers.
func (m *MemoryStorage) collectJobs() ([]deliveryJob, S3Putter) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.s3Putter == nil {
		return nil, nil
	}

	var jobs []deliveryJob

	for _, t := range m.Trails {
		if !t.IsLogging || t.S3BucketName == "" {
			continue
		}

		job := deliveryJob{bucket: t.S3BucketName, prefix: t.S3KeyPrefix}
		if !m.markerWritten[t.Name] {
			job.needMarker = true
			m.markerWritten[t.Name] = true
		}

		jobs = append(jobs, job)
	}

	return jobs, m.s3Putter
}

// logFile is the gzipped JSON envelope CloudTrail writes to S3.
//
//nolint:tagliatelle // AWS CloudTrail uses the PascalCase "Records" key.
type logFile struct {
	Records []logRecord `json:"Records"`
}

// logRecord is a single CloudTrail event record.
//
//nolint:tagliatelle // AWS CloudTrail uses these exact (mixed-case) field names.
type logRecord struct {
	EventVersion       string `json:"eventVersion"`
	EventTime          string `json:"eventTime"`
	EventSource        string `json:"eventSource"`
	EventName          string `json:"eventName"`
	AwsRegion          string `json:"awsRegion"`
	SourceIPAddress    string `json:"sourceIPAddress"`
	UserAgent          string `json:"userAgent"`
	RequestID          string `json:"requestID,omitempty"`
	EventID            string `json:"eventID"`
	EventType          string `json:"eventType"`
	RecipientAccountID string `json:"recipientAccountId"`
}

func buildLogObject(events []cloudtrailevents.Event, region string) []byte {
	records := make([]logRecord, 0, len(events))
	for _, e := range events {
		records = append(records, logRecord{
			EventVersion:       eventVersion,
			EventTime:          e.EventTime.UTC().Format(time.RFC3339),
			EventSource:        e.EventSource,
			EventName:          e.EventName,
			AwsRegion:          region,
			SourceIPAddress:    e.SourceIP,
			UserAgent:          e.UserAgent,
			RequestID:          e.RequestID,
			EventID:            uuid.New().String(),
			EventType:          eventType,
			RecipientAccountID: deliveryAccountID,
		})
	}

	body, err := json.Marshal(logFile{Records: records})
	if err != nil {
		return nil
	}

	return gzipBytes(body)
}

func gzipBytes(data []byte) []byte {
	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write(data)
	_ = zw.Close()

	return buf.Bytes()
}

// objectKey builds the CloudTrail S3 delivery key:
//
//	<prefix>/AWSLogs/<account>/CloudTrail/<region>/<YYYY>/<MM>/<DD>/<account>_CloudTrail_<region>_<YYYYMMDDTHHmmZ>_<16hex>.json.gz
func objectKey(prefix, region string) string {
	now := time.Now().UTC()
	hexID := strings.ReplaceAll(uuid.New().String(), "-", "")[:hexIDLen]

	return cloudTrailBase(prefix, region) +
		now.Format("2006/01/02") + "/" +
		deliveryAccountID + "_CloudTrail_" + region + "_" +
		now.Format("20060102T1504Z") + "_" + hexID + ".json.gz"
}

// markerKey is the 0-byte object CloudTrail writes when it begins delivering.
func markerKey(prefix string) string {
	return keyBase(prefix) + "AWSLogs/" + deliveryAccountID + "/CloudTrail/"
}

// cloudTrailBase is the key prefix up to and including the region segment.
func cloudTrailBase(prefix, region string) string {
	return markerKey(prefix) + region + "/"
}

// keyBase normalizes the trail's S3KeyPrefix into a path segment (empty, or
// "<prefix>/").
func keyBase(prefix string) string {
	if prefix == "" {
		return ""
	}

	return strings.TrimSuffix(prefix, "/") + "/"
}

// deliverTick is the delivery loop interval. KUMO_CLOUDTRAIL_DELIVER_INTERVAL_MS
// overrides it so tests need not wait seconds.
func deliverTick() time.Duration {
	v := os.Getenv("KUMO_CLOUDTRAIL_DELIVER_INTERVAL_MS")
	if v == "" {
		return defaultDeliverTick
	}

	ms, err := strconv.Atoi(v)
	if err != nil || ms <= 0 {
		return defaultDeliverTick
	}

	return time.Duration(ms) * time.Millisecond
}
