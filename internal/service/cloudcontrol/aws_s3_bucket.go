package cloudcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sivchari/kumo/internal/service/s3"
)

// awsS3Bucket adapts the AWS::S3::Bucket Cloud Control resource type to
// kumo's existing S3 storage. The Properties payload is full-schema:
// every CloudFormation-modelled property is emitted (with null / empty
// defaults when kumo doesn't model it yet) so terraform-provider-awscc's
// "unknown after apply" plan resolves cleanly.
type awsS3Bucket struct{}

func init() {
	registerDefaultHandler(&awsS3Bucket{})
}

func (*awsS3Bucket) TypeName() string { return "AWS::S3::Bucket" }

func (*awsS3Bucket) s3Storage() (s3.Storage, error) {
	return lookupStorage[s3.Storage]("s3")
}

func (h *awsS3Bucket) Create(ctx context.Context, desiredState []byte) (string, []byte, error) {
	var props struct {
		BucketName string `json:"BucketName"`
	}

	if err := json.Unmarshal(desiredState, &props); err != nil {
		return "", nil, fmt.Errorf("invalid AWS::S3::Bucket properties: %w", err)
	}

	if props.BucketName == "" {
		return "", nil, errors.New("BucketName is required")
	}

	storage, err := h.s3Storage()
	if err != nil {
		return "", nil, err
	}

	if err := storage.CreateBucket(ctx, props.BucketName); err != nil {
		return "", nil, err
	}

	return props.BucketName, s3BucketStateJSON(props.BucketName), nil
}

func (h *awsS3Bucket) Read(ctx context.Context, identifier string) ([]byte, error) {
	storage, err := h.s3Storage()
	if err != nil {
		return nil, err
	}

	exists, err := storage.BucketExists(ctx, identifier)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, &NotFoundError{Message: "bucket " + identifier + " does not exist"}
	}

	return s3BucketStateJSON(identifier), nil
}

func (h *awsS3Bucket) Update(ctx context.Context, identifier string, _ []byte) ([]byte, error) {
	return h.Read(ctx, identifier)
}

func (h *awsS3Bucket) Delete(ctx context.Context, identifier string) error {
	storage, err := h.s3Storage()
	if err != nil {
		return err
	}

	exists, err := storage.BucketExists(ctx, identifier)
	if err != nil {
		return err
	}

	if !exists {
		return &NotFoundError{Message: "bucket " + identifier + " does not exist"}
	}

	return storage.DeleteBucket(ctx, identifier)
}

func (h *awsS3Bucket) List(ctx context.Context) ([]ResourceDescription, error) {
	storage, err := h.s3Storage()
	if err != nil {
		return nil, err
	}

	buckets, err := storage.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]ResourceDescription, 0, len(buckets))

	for _, b := range buckets {
		out = append(out, ResourceDescription{Identifier: b.Name, Properties: s3BucketStateJSON(b.Name)})
	}

	return out, nil
}

// s3BucketStateJSON emits the full AWS::S3::Bucket CloudFormation schema
// for the named bucket. Sub-resources kumo doesn't model (encryption,
// lifecycle, replication, …) come back as JSON null so the awscc
// provider's "(known after apply)" plan resolves without a placeholder
// being left behind.
func s3BucketStateJSON(name string) []byte {
	state := map[string]any{
		"BucketName":                       name,
		"Arn":                              "arn:aws:s3:::" + name,
		"DomainName":                       name + ".s3.amazonaws.com",
		"DualStackDomainName":              name + ".s3.dualstack.us-east-1.amazonaws.com",
		"RegionalDomainName":               name + ".s3.us-east-1.amazonaws.com",
		"WebsiteURL":                       "http://" + name + ".s3-website-us-east-1.amazonaws.com",
		"AccelerateConfiguration":          nil,
		"AccessControl":                    nil,
		"AnalyticsConfigurations":          nil,
		"BucketEncryption":                 nil,
		"CorsConfiguration":                nil,
		"IntelligentTieringConfigurations": nil,
		"InventoryConfigurations":          nil,
		"LifecycleConfiguration":           nil,
		"LoggingConfiguration":             nil,
		"MetadataTableConfiguration":       nil,
		"MetricsConfigurations":            nil,
		"NotificationConfiguration":        nil,
		"ObjectLockConfiguration":          nil,
		"ObjectLockEnabled":                false,
		"OwnershipControls":                nil,
		"PublicAccessBlockConfiguration":   nil,
		"ReplicationConfiguration":         nil,
		"Tags":                             []any{},
		"VersioningConfiguration":          nil,
		"WebsiteConfiguration":             nil,
	}

	out, _ := json.Marshal(state)

	return out
}
