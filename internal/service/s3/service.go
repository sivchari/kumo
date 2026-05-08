package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

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
