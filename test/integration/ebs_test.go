//go:build integration

package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ebs"
	"github.com/aws/aws-sdk-go-v2/service/ebs/types"
	"github.com/sivchari/golden"
)

func newEBSClient(t *testing.T) *ebs.Client {
	t.Helper()

	return ebs.NewFromConfig(awsConfig(t), func(o *ebs.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestEBS_StartSnapshot(t *testing.T) {
	client := newEBSClient(t)
	ctx := t.Context()

	result, err := client.StartSnapshot(ctx, &ebs.StartSnapshotInput{
		VolumeSize:  aws.Int64(100),
		Description: aws.String("test snapshot"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("SnapshotId", "StartTime", "ResultMetadata")).Assert(t.Name(), result)
}

func TestEBS_CompleteSnapshot(t *testing.T) {
	client := newEBSClient(t)
	ctx := t.Context()

	startResult, err := client.StartSnapshot(ctx, &ebs.StartSnapshotInput{
		VolumeSize: aws.Int64(50),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.CompleteSnapshot(ctx, &ebs.CompleteSnapshotInput{
		SnapshotId:         startResult.SnapshotId,
		ChangedBlocksCount: aws.Int32(0),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), result)
}

func TestEBS_ListSnapshotBlocks_Empty(t *testing.T) {
	client := newEBSClient(t)
	ctx := t.Context()

	startResult, err := client.StartSnapshot(ctx, &ebs.StartSnapshotInput{
		VolumeSize: aws.Int64(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CompleteSnapshot(ctx, &ebs.CompleteSnapshotInput{
		SnapshotId:         startResult.SnapshotId,
		ChangedBlocksCount: aws.Int32(0),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListSnapshotBlocks(ctx, &ebs.ListSnapshotBlocksInput{
		SnapshotId: startResult.SnapshotId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), result)
}

func TestEBS_PutSnapshotBlock(t *testing.T) {
	client := newEBSClient(t)
	ctx := t.Context()

	startResult, err := client.StartSnapshot(ctx, &ebs.StartSnapshotInput{
		VolumeSize: aws.Int64(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	blockData := make([]byte, 524288)
	for i := range blockData {
		blockData[i] = byte(i % 256)
	}

	sum := sha256.Sum256(blockData)
	checksum := base64.StdEncoding.EncodeToString(sum[:])

	result, err := client.PutSnapshotBlock(ctx, &ebs.PutSnapshotBlockInput{
		SnapshotId:        startResult.SnapshotId,
		BlockIndex:        aws.Int32(0),
		BlockData:         bytes.NewReader(blockData),
		DataLength:        aws.Int32(524288),
		Checksum:          aws.String(checksum),
		ChecksumAlgorithm: types.ChecksumAlgorithmChecksumAlgorithmSha256,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Checksum", "ResultMetadata")).Assert(t.Name(), result)
}

func TestEBS_GetSnapshotBlock(t *testing.T) {
	client := newEBSClient(t)
	ctx := t.Context()

	startResult, err := client.StartSnapshot(ctx, &ebs.StartSnapshotInput{
		VolumeSize: aws.Int64(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	blockData := make([]byte, 524288)
	for i := range blockData {
		blockData[i] = byte(i % 256)
	}

	sum := sha256.Sum256(blockData)
	checksum := base64.StdEncoding.EncodeToString(sum[:])

	_, err = client.PutSnapshotBlock(ctx, &ebs.PutSnapshotBlockInput{
		SnapshotId:        startResult.SnapshotId,
		BlockIndex:        aws.Int32(0),
		BlockData:         bytes.NewReader(blockData),
		DataLength:        aws.Int32(524288),
		Checksum:          aws.String(checksum),
		ChecksumAlgorithm: types.ChecksumAlgorithmChecksumAlgorithmSha256,
	})
	if err != nil {
		t.Fatal(err)
	}

	// List blocks to get a block token.
	listResult, err := client.ListSnapshotBlocks(ctx, &ebs.ListSnapshotBlocksInput{
		SnapshotId: startResult.SnapshotId,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(listResult.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(listResult.Blocks))
	}

	// Get the block using the token.
	getResult, err := client.GetSnapshotBlock(ctx, &ebs.GetSnapshotBlockInput{
		SnapshotId: startResult.SnapshotId,
		BlockIndex: aws.Int32(0),
		BlockToken: listResult.Blocks[0].BlockToken,
	})
	if err != nil {
		t.Fatal(err)
	}

	gotData, err := io.ReadAll(getResult.BlockData)
	if err != nil {
		t.Fatalf("failed to read block data: %v", err)
	}

	if !bytes.Equal(gotData, blockData) {
		t.Error("block data mismatch")
	}

	if getResult.Checksum == nil || *getResult.Checksum != checksum {
		t.Errorf("expected checksum %s, got %v", checksum, getResult.Checksum)
	}
}

func TestEBS_ListSnapshotBlocks_WithBlocks(t *testing.T) {
	client := newEBSClient(t)
	ctx := t.Context()

	startResult, err := client.StartSnapshot(ctx, &ebs.StartSnapshotInput{
		VolumeSize: aws.Int64(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	blockData := make([]byte, 524288)
	sum := sha256.Sum256(blockData)
	checksum := base64.StdEncoding.EncodeToString(sum[:])

	_, err = client.PutSnapshotBlock(ctx, &ebs.PutSnapshotBlockInput{
		SnapshotId:        startResult.SnapshotId,
		BlockIndex:        aws.Int32(0),
		BlockData:         bytes.NewReader(blockData),
		DataLength:        aws.Int32(524288),
		Checksum:          aws.String(checksum),
		ChecksumAlgorithm: types.ChecksumAlgorithmChecksumAlgorithmSha256,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.PutSnapshotBlock(ctx, &ebs.PutSnapshotBlockInput{
		SnapshotId:        startResult.SnapshotId,
		BlockIndex:        aws.Int32(1),
		BlockData:         bytes.NewReader(blockData),
		DataLength:        aws.Int32(524288),
		Checksum:          aws.String(checksum),
		ChecksumAlgorithm: types.ChecksumAlgorithmChecksumAlgorithmSha256,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListSnapshotBlocks(ctx, &ebs.ListSnapshotBlocksInput{
		SnapshotId: startResult.SnapshotId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("BlockToken", "ResultMetadata")).Assert(t.Name(), result)
}

func TestEBS_ListChangedBlocks(t *testing.T) {
	client := newEBSClient(t)
	ctx := t.Context()

	// Create first snapshot with a block.
	first, err := client.StartSnapshot(ctx, &ebs.StartSnapshotInput{
		VolumeSize: aws.Int64(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	blockData := make([]byte, 524288)
	sum := sha256.Sum256(blockData)
	checksum := base64.StdEncoding.EncodeToString(sum[:])

	_, err = client.PutSnapshotBlock(ctx, &ebs.PutSnapshotBlockInput{
		SnapshotId:        first.SnapshotId,
		BlockIndex:        aws.Int32(0),
		BlockData:         bytes.NewReader(blockData),
		DataLength:        aws.Int32(524288),
		Checksum:          aws.String(checksum),
		ChecksumAlgorithm: types.ChecksumAlgorithmChecksumAlgorithmSha256,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CompleteSnapshot(ctx, &ebs.CompleteSnapshotInput{
		SnapshotId:         first.SnapshotId,
		ChangedBlocksCount: aws.Int32(1),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create second snapshot with a different block index.
	second, err := client.StartSnapshot(ctx, &ebs.StartSnapshotInput{
		VolumeSize: aws.Int64(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.PutSnapshotBlock(ctx, &ebs.PutSnapshotBlockInput{
		SnapshotId:        second.SnapshotId,
		BlockIndex:        aws.Int32(1),
		BlockData:         bytes.NewReader(blockData),
		DataLength:        aws.Int32(524288),
		Checksum:          aws.String(checksum),
		ChecksumAlgorithm: types.ChecksumAlgorithmChecksumAlgorithmSha256,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CompleteSnapshot(ctx, &ebs.CompleteSnapshotInput{
		SnapshotId:         second.SnapshotId,
		ChangedBlocksCount: aws.Int32(1),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List changed blocks between the two snapshots.
	result, err := client.ListChangedBlocks(ctx, &ebs.ListChangedBlocksInput{
		FirstSnapshotId:  first.SnapshotId,
		SecondSnapshotId: second.SnapshotId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("FirstBlockToken", "SecondBlockToken", "ResultMetadata")).Assert(t.Name(), result)
}

func TestEBS_SnapshotNotFound(t *testing.T) {
	client := newEBSClient(t)
	ctx := t.Context()

	_, err := client.ListSnapshotBlocks(ctx, &ebs.ListSnapshotBlocksInput{
		SnapshotId: aws.String("snap-nonexistent"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent snapshot")
	}
}
