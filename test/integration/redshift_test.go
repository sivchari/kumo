//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/sivchari/golden"
)

func newRedshiftClient(t *testing.T) *redshift.Client {
	t.Helper()

	return redshift.NewFromConfig(awsConfig(t), func(o *redshift.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestRedshift_CreateAndDeleteCluster(t *testing.T) {
	client := newRedshiftClient(t)
	ctx := t.Context()

	clusterID := "test-redshift-cluster"

	createResult, err := client.CreateCluster(ctx, &redshift.CreateClusterInput{
		ClusterIdentifier:  aws.String(clusterID),
		NodeType:           aws.String("dc2.large"),
		MasterUsername:     aws.String("admin"),
		MasterUserPassword: aws.String("Password123"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(context.Background(), &redshift.DeleteClusterInput{
			ClusterIdentifier:        aws.String(clusterID),
			SkipFinalClusterSnapshot: aws.Bool(true),
		})
	})

	g := golden.New(t, golden.WithIgnoreFields(
		"ClusterNamespaceArn", "ClusterCreateTime", "Endpoint",
		"ClusterRevisionNumber", "AutomatedSnapshotRetentionPeriod",
		"NumberOfNodes", "PubliclyAccessible", "Encrypted",
		"AllowVersionUpgrade", "MaintenanceTrackName",
		"ResultMetadata",
	))
	g.Assert(t.Name()+"_create", createResult)

	// Describe cluster.
	descResult, err := client.DescribeClusters(ctx, &redshift.DescribeClustersInput{
		ClusterIdentifier: aws.String(clusterID),
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"_describe", descResult)

	// Delete cluster.
	_, err = client.DeleteCluster(ctx, &redshift.DeleteClusterInput{
		ClusterIdentifier:        aws.String(clusterID),
		SkipFinalClusterSnapshot: aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify cluster is deleted.
	descResult, err = client.DescribeClusters(ctx, &redshift.DescribeClustersInput{
		ClusterIdentifier: aws.String(clusterID),
	})
	if err == nil && len(descResult.Clusters) > 0 {
		t.Error("expected cluster to be deleted")
	}
}

func TestRedshift_CreateAndDeleteClusterSnapshot(t *testing.T) {
	client := newRedshiftClient(t)
	ctx := t.Context()

	clusterID := "test-redshift-snapshot-cluster"
	snapshotID := "test-redshift-snapshot"

	// Create cluster first.
	_, err := client.CreateCluster(ctx, &redshift.CreateClusterInput{
		ClusterIdentifier:  aws.String(clusterID),
		NodeType:           aws.String("dc2.large"),
		MasterUsername:     aws.String("admin"),
		MasterUserPassword: aws.String("Password123"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteClusterSnapshot(context.Background(), &redshift.DeleteClusterSnapshotInput{
			SnapshotIdentifier: aws.String(snapshotID),
		})
		_, _ = client.DeleteCluster(context.Background(), &redshift.DeleteClusterInput{
			ClusterIdentifier:        aws.String(clusterID),
			SkipFinalClusterSnapshot: aws.Bool(true),
		})
	})

	// Create snapshot.
	createResult, err := client.CreateClusterSnapshot(ctx, &redshift.CreateClusterSnapshotInput{
		SnapshotIdentifier: aws.String(snapshotID),
		ClusterIdentifier:  aws.String(clusterID),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t, golden.WithIgnoreFields(
		"SnapshotCreateTime", "Port", "NumberOfNodes",
		"ResultMetadata",
	))
	g.Assert(t.Name()+"_create", createResult)

	// Describe snapshots.
	descResult, err := client.DescribeClusterSnapshots(ctx, &redshift.DescribeClusterSnapshotsInput{
		SnapshotIdentifier: aws.String(snapshotID),
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"_describe", descResult)
}

func TestRedshift_ClusterNotFound(t *testing.T) {
	client := newRedshiftClient(t)
	ctx := t.Context()

	_, err := client.DeleteCluster(ctx, &redshift.DeleteClusterInput{
		ClusterIdentifier:        aws.String("non-existent"),
		SkipFinalClusterSnapshot: aws.Bool(true),
	})
	if err == nil {
		t.Error("expected error")
	}
}
