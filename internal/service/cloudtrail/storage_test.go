package cloudtrail

import (
	"strings"
	"testing"
)

func TestNormalizeTrailName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"short name unchanged", "authz-trail", "authz-trail"},
		{"full arn to short name", "arn:aws:cloudtrail:us-east-1:123456789012:trail/authz-trail", "authz-trail"},
		{"arn with different region and account", "arn:aws:cloudtrail:ap-northeast-1:000000000000:trail/my-trail", "my-trail"},
		{"unrelated string unchanged", "some-random-value", "some-random-value"},
		{"empty string unchanged", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizeTrailName(tc.input); got != tc.want {
				t.Fatalf("normalizeTrailName(%q): got %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestCreateTrail_ARNAlias verifies that a trail created with a short name can be
// referenced by its ARN and vice versa, matching real CloudTrail's behaviour where
// StartLogging/GetTrailStatus accept either identifier.
func TestCreateTrail_ARNAlias(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := t.Context()

	const trailName = "authz-trail"

	created, err := store.CreateTrail(ctx, &CreateTrailRequest{
		Name:         trailName,
		S3BucketName: "audit-bucket",
	})
	if err != nil {
		t.Fatalf("CreateTrail: unexpected error: %v", err)
	}

	// StartLogging by ARN must succeed; the migration target sends the ARN as Name.
	if err := store.StartLogging(ctx, created.TrailARN); err != nil {
		t.Fatalf("StartLogging by ARN: unexpected error: %v", err)
	}

	// GetTrailStatus by short name must observe the same trail.
	status, err := store.GetTrailStatus(ctx, trailName)
	if err != nil {
		t.Fatalf("GetTrailStatus by short name: unexpected error: %v", err)
	}

	if !status.IsLogging {
		t.Fatalf("IsLogging after StartLogging by ARN: got %v, want true", status.IsLogging)
	}

	// StopLogging by ARN flips it back, observable by short name.
	if err := store.StopLogging(ctx, created.TrailARN); err != nil {
		t.Fatalf("StopLogging by ARN: unexpected error: %v", err)
	}

	stopped, err := store.GetTrailStatus(ctx, trailName)
	if err != nil {
		t.Fatalf("GetTrailStatus after stop: unexpected error: %v", err)
	}

	if stopped.IsLogging {
		t.Fatalf("IsLogging after StopLogging by ARN: got %v, want false", stopped.IsLogging)
	}
}

// TestCreateTrail_NoDoubleARN verifies that passing an ARN to CreateTrail does not
// produce a nested "...:trail/arn:aws:cloudtrail:..." ARN.
func TestCreateTrail_NoDoubleARN(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := t.Context()

	const arn = "arn:aws:cloudtrail:us-east-1:123456789012:trail/authz-trail"

	created, err := store.CreateTrail(ctx, &CreateTrailRequest{
		Name:         arn,
		S3BucketName: "audit-bucket",
	})
	if err != nil {
		t.Fatalf("CreateTrail with ARN name: unexpected error: %v", err)
	}

	if created.Name != "authz-trail" {
		t.Fatalf("Trail.Name: got %q, want %q", created.Name, "authz-trail")
	}

	if strings.Count(created.TrailARN, ":trail/") != 1 {
		t.Fatalf("TrailARN must contain exactly one \":trail/\": got %q", created.TrailARN)
	}
}
