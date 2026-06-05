package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

// SQSPublisher is the interface the S3 service uses to deliver event
// notification messages to SQS queues. The server wiring layer
// provides a concrete implementation backed by the SQS storage.
type SQSPublisher interface {
	// PublishToSQS sends a single message to the SQS queue identified
	// by queueARN. The body is the full JSON event notification.
	PublishToSQS(ctx context.Context, queueARN, body string) error
}

const defaultBaseURL = "http://localhost:4566"

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	baseURL := defaultBaseURL

	if port := os.Getenv("KUMO_PORT"); port != "" {
		baseURL = fmt.Sprintf("http://localhost:%s", port)
	}

	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...), baseURL))
}

// Service implements the S3 service.
type Service struct {
	storage      Storage
	baseURL      string
	logger       *slog.Logger
	sqsPublisher SQSPublisher
}

// New creates a new S3 service.
func New(storage Storage, baseURL string) *Service {
	return &Service{
		storage: storage,
		baseURL: baseURL,
		logger:  slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "s3"
}

// Storage exposes the underlying storage so other services that need to
// operate on the same bucket store (notably the cloudcontrol service,
// which proxies AWS::S3::Bucket through the existing S3 storage) can
// read and mutate it without going back through HTTP.
func (s *Service) Storage() Storage {
	return s.storage
}

// RegisterRoutes registers the S3 routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// Bucket operations
	r.Handle("GET", "/", s.ListBuckets)
	r.Handle("PUT", "/{bucket}", s.handleBucketPut)
	r.Handle("DELETE", "/{bucket}", s.handleBucketDelete)
	r.Handle("HEAD", "/{bucket}", s.HeadBucket)

	// Bucket-level GET handles ListObjects, ListMultipartUploads, versioning queries
	r.Handle("GET", "/{bucket}", s.handleBucketGet)
	r.Handle("POST", "/{bucket}", s.handleBucketPost)

	// Object operations with multipart upload support
	r.Handle("PUT", "/{bucket}/{key...}", s.handleObjectPut)
	r.Handle("GET", "/{bucket}/{key...}", s.handleObjectGet)
	r.Handle("DELETE", "/{bucket}/{key...}", s.handleObjectDelete)
	r.Handle("HEAD", "/{bucket}/{key...}", s.HeadObject)
	r.Handle("POST", "/{bucket}/{key...}", s.handleObjectPost)

	// CORS preflight
	r.Handle("OPTIONS", "/{bucket}/{key...}", s.HandleCORSPreflight)
}

// Close saves the storage state if persistence is enabled.
func (s *Service) Close() error {
	if c, ok := s.storage.(io.Closer); ok {
		if err := c.Close(); err != nil {
			return fmt.Errorf("failed to close storage: %w", err)
		}
	}

	return nil
}

// SetSQSPublisher installs the adapter that delivers S3 event
// notification messages to SQS queues. Called by the server wiring
// layer after all services have been registered.
func (s *Service) SetSQSPublisher(p SQSPublisher) {
	s.sqsPublisher = p
}

// emitSQSNotifications delivers S3 event notification messages to
// every SQS queue configured in the bucket's notification configuration
// whose event filter matches the given eventName.
func (s *Service) emitSQSNotifications(ctx context.Context, bucket, key, eventName string, size int64, etag string) {
	if s.sqsPublisher == nil {
		return
	}

	configs := s.storage.GetQueueConfigurations(ctx, bucket)
	if len(configs) == 0 {
		return
	}

	for _, cfg := range configs {
		if !matchesEventFilter(cfg.Events, eventName) {
			continue
		}

		record := EventNotification{
			Records: []EventRecord{{
				EventVersion: "2.1",
				EventSource:  "aws:s3",
				AWSRegion:    "us-east-1",
				EventTime:    time.Now().UTC().Format(time.RFC3339),
				EventName:    eventName,
				UserIdentity: map[string]string{"principalId": "EXAMPLE"},
				S3: EventRecordS3Detail{
					SchemaVersion:   "1.0",
					ConfigurationID: cfg.ID,
					Bucket: EventRecordBucket{
						Name: bucket,
						Arn:  "arn:aws:s3:::" + bucket,
					},
					Object: EventRecordObject{
						Key:  url.QueryEscape(key),
						Size: size,
						ETag: strings.Trim(etag, "\""),
					},
				},
			}},
		}

		body, err := json.Marshal(record)
		if err != nil {
			s.logger.Error("failed to marshal S3 event notification", "error", err)

			continue
		}

		if err := s.sqsPublisher.PublishToSQS(ctx, cfg.QueueArn, string(body)); err != nil {
			s.logger.Error("failed to deliver S3 notification to SQS",
				"bucket", bucket, "key", key, "queueArn", cfg.QueueArn, "error", err)
		}
	}
}

// matchesEventFilter checks whether an eventName (e.g. "s3:ObjectCreated:Put")
// matches any of the configured event filter strings (e.g. "s3:ObjectCreated:*").
func matchesEventFilter(filters []string, eventName string) bool {
	for _, f := range filters {
		if f == eventName {
			return true
		}

		// Handle wildcard: "s3:ObjectCreated:*" matches "s3:ObjectCreated:Put"
		if strings.HasSuffix(f, "*") {
			prefix := strings.TrimSuffix(f, "*")
			if strings.HasPrefix(eventName, prefix) {
				return true
			}
		}
	}

	return false
}

// emitObjectCreatedEvent sends an S3 Object Created event to EventBridge.
func (s *Service) emitObjectCreatedEvent(ctx context.Context, bucket, key string, size int64, etag string) {
	if !s.storage.IsEventBridgeEnabled(ctx, bucket) {
		return
	}

	detail := map[string]any{
		"version":    "0",
		"bucket":     map[string]string{"name": bucket},
		"object":     map[string]any{"key": key, "size": size, "etag": etag},
		"request-id": uuid.New().String(),
	}

	detailJSON, err := json.Marshal(detail)
	if err != nil {
		s.logger.Error("failed to marshal S3 event detail", "error", err)

		return
	}

	body, _ := json.Marshal(map[string]any{
		"Entries": []map[string]any{{
			"Source": "aws.s3", "DetailType": "Object Created", "Detail": string(detailJSON),
		}},
	})

	s.putEvents(ctx, body, bucket, key)
}

// putEvents sends a PutEvents request to the internal EventBridge endpoint.
func (s *Service) putEvents(ctx context.Context, body []byte, bucket, key string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/", bytes.NewReader(body))
	if err != nil {
		s.logger.Error("failed to create EventBridge request", "error", err)

		return
	}

	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AWSEvents.PutEvents")

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		s.logger.Error("failed to emit S3 event to EventBridge", "error", err)

		return
	}

	defer func() { _ = resp.Body.Close() }()

	s.logger.Info("emitted S3 Object Created event", "bucket", bucket, "key", key, "status", resp.StatusCode)
}

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "S3",
		Category:    "Storage",
		Description: "Object storage",
	}
}
