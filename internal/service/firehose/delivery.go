package firehose

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Delivery defaults mirror AWS Firehose's buffering behaviour.
const (
	defaultBufferIntervalSeconds = 300
	defaultBufferSizeMB          = 5
	defaultFlushTick             = time.Second
	bytesPerMB                   = 1024 * 1024
	s3ARNPrefix                  = "arn:aws:s3:::"
)

// SetS3Putter installs the S3 delivery target and starts the background flush
// loop (once). Cross-service wiring calls this after services are registered.
// Buffered records are delivered to each stream's S3/ExtendedS3 destination on
// the configured buffering interval.
func (s *MemoryStorage) SetS3Putter(p S3Putter) {
	s.mu.Lock()
	s.s3Putter = p
	s.mu.Unlock()

	s.flushOnce.Do(func() {
		go s.flushLoop()
	})
}

// flushLoop flushes buffered records on a ticker until flushCtx is cancelled.
func (s *MemoryStorage) flushLoop() {
	ticker := time.NewTicker(flushTick())
	defer ticker.Stop()

	for {
		select {
		case <-s.flushCtx.Done():
			return
		case <-ticker.C:
			s.flushAll(false)
		}
	}
}

// flushAll attempts delivery for every stream. When force is true the
// interval/size thresholds are ignored and all undelivered records are written.
func (s *MemoryStorage) flushAll(force bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.s3Putter == nil {
		return
	}

	for name, data := range s.Streams {
		s.flushStream(name, data, force)
	}
}

// flushStream delivers one stream's undelivered records when thresholds are met.
// Must be called while holding s.mu.
func (s *MemoryStorage) flushStream(name string, data *StreamData, force bool) {
	target, ok := s3Target(data.Stream)
	if !ok {
		return
	}

	if data.DeliveredCount > len(data.Records) {
		data.DeliveredCount = len(data.Records)
	}

	pending := data.Records[data.DeliveredCount:]
	if len(pending) == 0 {
		return
	}

	if !force && !shouldFlush(pending, target) {
		return
	}

	body := concatRecords(pending)
	if target.compress {
		body = gzipBytes(body)
	}

	version := data.DeliveryVersion + 1

	key := objectKey(target.prefix, name, version, target.compress)
	if err := s.s3Putter.PutObject(context.Background(), target.bucket, key, body, objectContentType(target.compress)); err != nil {
		return // leave records buffered; retry on the next tick
	}

	data.DeliveredCount = len(data.Records)
	data.DeliveryVersion = version

	s.saveLocked()
}

// s3DeliveryTarget is the resolved S3 destination of a delivery stream.
type s3DeliveryTarget struct {
	bucket   string
	prefix   string
	compress bool
	interval time.Duration
	size     int
}

// s3Target resolves the first S3 or ExtendedS3 destination of a stream.
func s3Target(stream *DeliveryStream) (s3DeliveryTarget, bool) {
	for _, d := range stream.Destinations {
		if d.ExtendedS3DestinationDescription != nil {
			ed := d.ExtendedS3DestinationDescription

			return newS3DeliveryTarget(ed.BucketARN, ed.Prefix, ed.CompressionFormat, ed.BufferingHints), true
		}

		if d.S3DestinationDescription != nil {
			sd := d.S3DestinationDescription

			return newS3DeliveryTarget(sd.BucketARN, sd.Prefix, sd.CompressionFormat, sd.BufferingHints), true
		}
	}

	return s3DeliveryTarget{}, false
}

func newS3DeliveryTarget(bucketARN, prefix, compression string, hints *BufferingHints) s3DeliveryTarget {
	return s3DeliveryTarget{
		bucket:   bucketFromARN(bucketARN),
		prefix:   prefix,
		compress: strings.EqualFold(compression, "GZIP"),
		interval: bufferInterval(hints),
		size:     bufferSize(hints),
	}
}

// shouldFlush reports whether the buffered records meet a delivery threshold.
func shouldFlush(pending []StoredRecord, target s3DeliveryTarget) bool {
	if override, ok := flushIntervalOverride(); ok {
		return time.Since(pending[0].Received) >= override
	}

	total := 0
	for _, r := range pending {
		total += len(r.Data)
	}

	if total >= target.size {
		return true
	}

	return time.Since(pending[0].Received) >= target.interval
}

func bufferInterval(hints *BufferingHints) time.Duration {
	secs := defaultBufferIntervalSeconds
	if hints != nil && hints.IntervalInSeconds > 0 {
		secs = int(hints.IntervalInSeconds)
	}

	return time.Duration(secs) * time.Second
}

func bufferSize(hints *BufferingHints) int {
	mb := defaultBufferSizeMB
	if hints != nil && hints.SizeInMBs > 0 {
		mb = int(hints.SizeInMBs)
	}

	return mb * bytesPerMB
}

// flushTick is the loop interval. KUMO_FIREHOSE_FLUSH_INTERVAL_MS overrides it
// (and the buffering threshold) so tests can flush without waiting minutes.
func flushTick() time.Duration {
	if override, ok := flushIntervalOverride(); ok {
		return override
	}

	return defaultFlushTick
}

func flushIntervalOverride() (time.Duration, bool) {
	v := os.Getenv("KUMO_FIREHOSE_FLUSH_INTERVAL_MS")
	if v == "" {
		return 0, false
	}

	ms, err := strconv.Atoi(v)
	if err != nil || ms <= 0 {
		return 0, false
	}

	return time.Duration(ms) * time.Millisecond, true
}

func concatRecords(pending []StoredRecord) []byte {
	var buf bytes.Buffer
	for _, r := range pending {
		buf.Write(r.Data)
	}

	return buf.Bytes()
}

func gzipBytes(data []byte) []byte {
	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write(data)
	_ = zw.Close()

	return buf.Bytes()
}

func objectContentType(compress bool) string {
	if compress {
		return "application/gzip"
	}

	return "application/octet-stream"
}

// objectKey builds the Firehose S3 delivery key:
//
//	<prefix><YYYY>/<MM>/<DD>/<HH>/<stream>-<version>-<YYYY-MM-DD-HH-MM-SS>-<uuid>[.gz]
func objectKey(prefix, stream string, version int, compress bool) string {
	now := time.Now().UTC()
	datePath := now.Format("2006/01/02/15/")
	ts := now.Format("2006-01-02-15-04-05")

	key := prefix + datePath + stream + "-" + strconv.Itoa(version) + "-" + ts + "-" + uuid.New().String()
	if compress {
		key += ".gz"
	}

	return key
}

func bucketFromARN(arn string) string {
	return strings.TrimPrefix(arn, s3ARNPrefix)
}
