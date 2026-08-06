//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/directoryservice"
	"github.com/aws/aws-sdk-go-v2/service/directoryservice/types"
	"github.com/sivchari/golden"
)

func newDSClient(t *testing.T) *directoryservice.Client {
	t.Helper()

	return directoryservice.NewFromConfig(awsConfig(t), func(o *directoryservice.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestDS_CreateAndDeleteDirectory(t *testing.T) {
	client := newDSClient(t)
	ctx := t.Context()
	directoryName := "test.example.com"

	// Create directory.
	createOutput, err := client.CreateDirectory(ctx, &directoryservice.CreateDirectoryInput{
		Name:     aws.String(directoryName),
		Password: aws.String("Test1234!"),
		Size:     types.DirectorySizeSmall,
		VpcSettings: &types.DirectoryVpcSettings{
			VpcId:     aws.String("vpc-12345678"),
			SubnetIds: []string{"subnet-11111111", "subnet-22222222"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	directoryID := createOutput.DirectoryId

	t.Cleanup(func() {
		_, _ = client.DeleteDirectory(context.Background(), &directoryservice.DeleteDirectoryInput{
			DirectoryId: directoryID,
		})
	})

	golden.New(t, golden.WithIgnoreFields("DirectoryId", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Delete directory.
	deleteOutput, err := client.DeleteDirectory(ctx, &directoryservice.DeleteDirectoryInput{
		DirectoryId: directoryID,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("DirectoryId", "ResultMetadata")).Assert(t.Name()+"_delete", deleteOutput)
}

func TestDS_DescribeDirectories(t *testing.T) {
	client := newDSClient(t)
	ctx := t.Context()
	directoryName := "describe-test.example.com"

	// Create directory.
	createOutput, err := client.CreateDirectory(ctx, &directoryservice.CreateDirectoryInput{
		Name:        aws.String(directoryName),
		Password:    aws.String("Test1234!"),
		Size:        types.DirectorySizeSmall,
		Description: aws.String("Test directory for describe"),
		VpcSettings: &types.DirectoryVpcSettings{
			VpcId:     aws.String("vpc-12345678"),
			SubnetIds: []string{"subnet-11111111", "subnet-22222222"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	directoryID := createOutput.DirectoryId

	t.Cleanup(func() {
		_, _ = client.DeleteDirectory(context.Background(), &directoryservice.DeleteDirectoryInput{
			DirectoryId: directoryID,
		})
	})

	// Describe directories.
	describeOutput, err := client.DescribeDirectories(ctx, &directoryservice.DescribeDirectoriesInput{
		DirectoryIds: []string{*directoryID},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(describeOutput.DirectoryDescriptions) == 0 {
		t.Fatal("expected at least one directory")
	}

	golden.New(t, golden.WithIgnoreFields(
		"DirectoryId", "LaunchTime", "StageLastUpdatedDateTime", "DnsIpAddrs",
		"VpcSettings", "ResultMetadata",
	)).Assert(t.Name(), describeOutput)
}

func TestDS_CreateAndDeleteSnapshot(t *testing.T) {
	client := newDSClient(t)
	ctx := t.Context()
	directoryName := "snapshot-test.example.com"

	// Create directory first.
	createDirOutput, err := client.CreateDirectory(ctx, &directoryservice.CreateDirectoryInput{
		Name:     aws.String(directoryName),
		Password: aws.String("Test1234!"),
		Size:     types.DirectorySizeSmall,
		VpcSettings: &types.DirectoryVpcSettings{
			VpcId:     aws.String("vpc-12345678"),
			SubnetIds: []string{"subnet-11111111", "subnet-22222222"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	directoryID := createDirOutput.DirectoryId

	t.Cleanup(func() {
		_, _ = client.DeleteDirectory(context.Background(), &directoryservice.DeleteDirectoryInput{
			DirectoryId: directoryID,
		})
	})

	// Create snapshot.
	createSnapOutput, err := client.CreateSnapshot(ctx, &directoryservice.CreateSnapshotInput{
		DirectoryId: directoryID,
		Name:        aws.String("test-snapshot"),
	})
	if err != nil {
		t.Fatal(err)
	}

	snapshotID := createSnapOutput.SnapshotId

	golden.New(t, golden.WithIgnoreFields("SnapshotId", "ResultMetadata")).Assert(t.Name()+"_create", createSnapOutput)

	// Describe snapshots.
	describeSnapOutput, err := client.DescribeSnapshots(ctx, &directoryservice.DescribeSnapshotsInput{
		SnapshotIds: []string{*snapshotID},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(describeSnapOutput.Snapshots) == 0 {
		t.Fatal("expected at least one snapshot")
	}

	golden.New(t, golden.WithIgnoreFields(
		"SnapshotId", "DirectoryId", "StartTime", "ResultMetadata",
	)).Assert(t.Name()+"_describe", describeSnapOutput)

	// Delete snapshot.
	deleteSnapOutput, err := client.DeleteSnapshot(ctx, &directoryservice.DeleteSnapshotInput{
		SnapshotId: snapshotID,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("SnapshotId", "ResultMetadata")).Assert(t.Name()+"_delete", deleteSnapOutput)
}

func TestDS_DirectoryNotFound(t *testing.T) {
	client := newDSClient(t)
	ctx := t.Context()

	_, err := client.DeleteDirectory(ctx, &directoryservice.DeleteDirectoryInput{
		DirectoryId: aws.String("d-nonexistent"),
	})
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}
