package cloudcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sivchari/kumo/internal/service/s3"
)

// awsS3Bucket adapts the AWS::S3::Bucket Cloud Control resource type to
// kumo's existing S3 storage. Only the BucketName property is honoured —
// the other AWS::S3::Bucket properties (versioning, tags, public access
// block, encryption …) are stored as opaque sub-resources by the S3
// service today, and Cloud Control wiring for those can be added later
// without changing the Create/Read/Delete contract.
type awsS3Bucket struct{}

func init() {
	registerDefaultHandler(&awsS3Bucket{})
}

// TypeName is the canonical CloudFormation type identifier.
func (*awsS3Bucket) TypeName() string { return "AWS::S3::Bucket" }

// s3Storage resolves the live S3 storage at request time so we always
// share the same bucket store as the s3 service. Doing it lazily avoids
// init-order coupling between the cloudcontrol and s3 packages.
func (*awsS3Bucket) s3Storage() (s3.Storage, error) {
	return lookupStorage[s3.Storage]("s3")
}

// Create extracts BucketName from the desired-state JSON and creates the
// bucket via the existing S3 storage. The identifier Cloud Control uses
// from this point on is the bucket name itself (S3 buckets have no
// separate ARN-based identifier in the CloudFormation model).
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

	state, err := json.Marshal(map[string]any{"BucketName": props.BucketName})
	if err != nil {
		return "", nil, err
	}

	return props.BucketName, state, nil
}

// Read confirms the bucket exists and returns a minimal state document.
// The wider set of AWS::S3::Bucket properties is unmodelled — surfacing
// just BucketName matches what the awscc provider treats as required.
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

	return json.Marshal(map[string]any{"BucketName": identifier})
}

// Update is a no-op today. AWS::S3::Bucket has only one updatable scalar
// (BucketName, which is actually replace-only) and the rest of the
// settings live in sub-resource APIs that this PR does not yet expose.
func (h *awsS3Bucket) Update(ctx context.Context, identifier string, _ []byte) ([]byte, error) {
	return h.Read(ctx, identifier)
}

// Delete removes the bucket. NotFound is mapped to NotFoundError so the
// dispatcher returns ResourceNotFoundException — matching real Cloud
// Control which rejects deletes against absent resources.
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

// List returns one ResourceDescription per existing bucket.
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
		props, err := json.Marshal(map[string]any{"BucketName": b.Name})
		if err != nil {
			return nil, err
		}

		out = append(out, ResourceDescription{Identifier: b.Name, Properties: props})
	}

	return out, nil
}
