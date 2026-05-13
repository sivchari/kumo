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

const defaultBaseURL = "http://localhost:4566"

const (
	objectCreatedPutEvent                     = "s3:ObjectCreated:Put"
	objectCreatedCopyEvent                    = "s3:ObjectCreated:Copy"
	objectCreatedCompleteMultipartUploadEvent = "s3:ObjectCreated:CompleteMultipartUpload"
)

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
	storage Storage
	baseURL string
	logger  *slog.Logger
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

// emitObjectCreatedEvent sends an S3 Object Created event to configured bucket notifications.
func (s *Service) emitObjectCreatedEvent(ctx context.Context, bucket, key string, size int64, etag, event string) {
	if s.storage.IsEventBridgeEnabled(ctx, bucket) {
		s.emitObjectCreatedEventBridgeEvent(ctx, bucket, key, size, etag)
	}

	s.emitObjectCreatedQueueNotifications(ctx, bucket, key, size, etag, event)
	s.emitObjectCreatedLambdaNotifications(ctx, bucket, key, size, etag, event)
}

func (s *Service) emitObjectCreatedEventBridgeEvent(ctx context.Context, bucket, key string, size int64, etag string) {
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

func (s *Service) emitObjectCreatedQueueNotifications(ctx context.Context, bucket, key string, size int64, etag, event string) {
	cfg, err := s.storage.GetBucketNotificationConfiguration(ctx, bucket)
	if err != nil || cfg == nil {
		return
	}

	for _, queueConfig := range cfg.QueueConfigurations {
		if !notificationEventsMatch(queueConfig.Events, event) {
			continue
		}

		if !notificationFilterMatches(queueConfig.Filter, key) {
			continue
		}

		payload, err := objectCreatedNotificationPayload(bucket, key, size, etag, event)
		if err != nil {
			s.logger.Error("failed to marshal S3 queue notification", "error", err)

			continue
		}

		s.sendSQSNotification(ctx, queueConfig.Queue, payload)
	}
}

func (s *Service) emitObjectCreatedLambdaNotifications(ctx context.Context, bucket, key string, size int64, etag, event string) {
	cfg, err := s.storage.GetBucketNotificationConfiguration(ctx, bucket)
	if err != nil || cfg == nil {
		return
	}

	for _, lambdaConfig := range cfg.LambdaFunctionConfigurations {
		if !notificationEventsMatch(lambdaConfig.Events, event) {
			continue
		}

		if !notificationFilterMatches(lambdaConfig.Filter, key) {
			continue
		}

		payload, err := objectCreatedNotificationPayload(bucket, key, size, etag, event)
		if err != nil {
			s.logger.Error("failed to marshal S3 Lambda notification", "error", err)

			continue
		}

		s.sendLambdaNotification(ctx, lambdaConfig.CloudFunction, payload)
	}
}

func objectCreatedNotificationPayload(bucket, key string, size int64, etag, event string) ([]byte, error) {
	body, err := json.Marshal(map[string]any{
		"Records": []map[string]any{{
			"eventVersion": "2.1",
			"eventSource":  "aws:s3",
			"awsRegion":    "us-east-1",
			"eventTime":    time.Now().UTC().Format(time.RFC3339),
			"eventName":    strings.TrimPrefix(event, "s3:"),
			"s3": map[string]any{
				"bucket": map[string]string{
					"name": bucket,
					"arn":  "arn:aws:s3:::" + bucket,
				},
				"object": map[string]any{
					"key":  key,
					"size": size,
					"eTag": strings.Trim(etag, `"`),
				},
			},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal object created notification: %w", err)
	}

	return body, nil
}

func notificationEventsMatch(events []string, event string) bool {
	for _, configured := range events {
		if configured == event || configured == "s3:ObjectCreated:*" || configured == "s3:*" {
			return true
		}
	}

	return false
}

func notificationFilterMatches(filter *NotificationFilter, key string) bool {
	if filter == nil || filter.S3Key == nil {
		return true
	}

	for _, rule := range filter.S3Key.Rules {
		switch strings.ToLower(rule.Name) {
		case "prefix":
			if !strings.HasPrefix(key, rule.Value) {
				return false
			}
		case "suffix":
			if !strings.HasSuffix(key, rule.Value) {
				return false
			}
		}
	}

	return true
}

func (s *Service) sendSQSNotification(ctx context.Context, queueARN string, payload []byte) {
	queueURL := s.sqsQueueURL(queueARN)
	if queueURL == "" {
		return
	}

	body, err := json.Marshal(map[string]string{
		"QueueUrl":    queueURL,
		"MessageBody": string(payload),
	})
	if err != nil {
		s.logger.Error("failed to marshal SQS SendMessage request", "error", err)

		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/", bytes.NewReader(body))
	if err != nil {
		s.logger.Error("failed to create SQS SendMessage request", "error", err)

		return
	}

	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		s.logger.Error("failed to deliver S3 notification to SQS", "error", err, "queue", queueARN)

		return
	}

	_ = resp.Body.Close()
}

func (s *Service) sendLambdaNotification(ctx context.Context, functionARN string, payload []byte) {
	functionName := lambdaFunctionName(functionARN)
	if functionName == "" {
		return
	}

	endpoint := s.baseURL + "/lambda/2015-03-31/functions/" + url.PathEscape(functionName) + "/invocations"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		s.logger.Error("failed to create Lambda invoke request", "error", err, "function", functionName)

		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Amz-Invocation-Type", "Event")

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		s.logger.Error("failed to deliver S3 notification to Lambda", "error", err, "function", functionName)

		return
	}

	_ = resp.Body.Close()
}

func lambdaFunctionName(functionARN string) string {
	if !strings.HasPrefix(functionARN, "arn:") {
		return functionARN
	}

	parts := strings.Split(functionARN, ":")
	if len(parts) < 7 || parts[5] != "function" {
		return ""
	}

	return parts[6]
}

func (s *Service) sqsQueueURL(queueARN string) string {
	if !strings.HasPrefix(queueARN, "arn:") {
		return queueARN
	}

	parts := strings.Split(queueARN, ":")
	if len(parts) < 6 {
		return ""
	}

	return s.baseURL + "/" + parts[4] + "/" + parts[5]
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
