package cloudtrail

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// Literals reused across the CloudTrail tag tests.
const (
	tagKeyEnv      = "env"
	tagValLocal    = "local"
	tagKeyTeam     = "team"
	tagTrailName   = "authz-trail"
	tagBucket      = "audit-bucket"
	absentTrailARN = "arn:aws:cloudtrail:us-east-1:123456789012:trail/absent"
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

// trailTagCase is one row of the trail-tag table: starting tags, an optional
// add/remove mutation, and the tag list expected on read-back.
type trailTagCase struct {
	name       string
	createTags []Tag
	add        []Tag
	remove     []Tag
	want       []Tag
}

// runTrailTagCases creates a trail with the case's tags, applies its mutation,
// and asserts the read-back (by short name, an alias of the ARN) on a fresh
// store per case.
func runTrailTagCases(t *testing.T, cases []trailTagCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := NewMemoryStorage()
			ctx := t.Context()

			created, err := store.CreateTrail(ctx, &CreateTrailRequest{
				Name:         tagTrailName,
				S3BucketName: tagBucket,
				TagsList:     tc.createTags,
			})
			if err != nil {
				t.Fatalf("CreateTrail: unexpected error: %v", err)
			}

			if len(tc.add) > 0 {
				if err := store.AddTrailTags(ctx, created.TrailARN, tc.add); err != nil {
					t.Fatalf("AddTrailTags: unexpected error: %v", err)
				}
			}

			if len(tc.remove) > 0 {
				if err := store.RemoveTrailTags(ctx, created.TrailARN, tc.remove); err != nil {
					t.Fatalf("RemoveTrailTags: unexpected error: %v", err)
				}
			}

			if got := store.ListTrailTags(ctx, tagTrailName); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ListTrailTags: got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestTrailTags verifies tags read back after create / add / remove, including
// that an untagged trail reads back empty.
func TestTrailTags(t *testing.T) {
	t.Parallel()

	runTrailTagCases(t, []trailTagCase{
		{
			name:       "create-time tags read back",
			createTags: []Tag{{Key: tagKeyEnv, Value: tagValLocal}},
			want:       []Tag{{Key: tagKeyEnv, Value: tagValLocal}},
		},
		{name: "untagged trail reads empty", want: []Tag{}},
		{
			name:       "add merges and sorts by key",
			createTags: []Tag{{Key: tagKeyEnv, Value: tagValLocal}},
			add:        []Tag{{Key: tagKeyTeam, Value: "idp"}},
			want:       []Tag{{Key: tagKeyEnv, Value: tagValLocal}, {Key: tagKeyTeam, Value: "idp"}},
		},
		{
			name:       "remove deletes by key",
			createTags: []Tag{{Key: tagKeyEnv, Value: tagValLocal}, {Key: tagKeyTeam, Value: "idp"}},
			remove:     []Tag{{Key: tagKeyEnv}},
			want:       []Tag{{Key: tagKeyTeam, Value: "idp"}},
		},
	})
}

// TestTrailTags_MissingTrail verifies the mutating tag ops report not-found for
// an unknown trail (ListTrailTags-on-unknown is covered at the dispatch layer).
func TestTrailTags_MissingTrail(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		op   func(ctx context.Context, s *MemoryStorage) error
	}{
		{
			name: "AddTrailTags reports not found",
			op: func(ctx context.Context, s *MemoryStorage) error {
				return s.AddTrailTags(ctx, absentTrailARN, []Tag{{Key: "k", Value: "v"}})
			},
		},
		{
			name: "RemoveTrailTags reports not found",
			op: func(ctx context.Context, s *MemoryStorage) error {
				return s.RemoveTrailTags(ctx, absentTrailARN, []Tag{{Key: "k"}})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.op(t.Context(), NewMemoryStorage()); err == nil {
				t.Fatalf("%s: expected error for a missing trail, got nil", tc.name)
			}
		})
	}
}
