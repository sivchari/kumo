//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/neptune"
	"github.com/sivchari/golden"
)

func newNeptuneClient(t *testing.T) *neptune.Client {
	t.Helper()

	return neptune.NewFromConfig(awsConfig(t), func(o *neptune.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestNeptune_CreateAndDeleteDBCluster(t *testing.T) {
	client := newNeptuneClient(t)
	ctx := t.Context()

	clusterID := "test-neptune-cluster"

	createResult, err := client.CreateDBCluster(ctx, &neptune.CreateDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		Engine:              aws.String("neptune"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDBCluster(context.Background(), &neptune.DeleteDBClusterInput{
			DBClusterIdentifier: aws.String(clusterID),
			SkipFinalSnapshot:   aws.Bool(true),
		})
	})

	g := golden.New(t, golden.WithIgnoreFields("DBClusterArn", "DbClusterResourceId", "ClusterCreateTime", "Endpoint", "ReaderEndpoint", "AllocatedStorage", "AvailabilityZones", "BackupRetentionPeriod", "Port", "ResultMetadata"))
	g.Assert(t.Name()+"_create", createResult)

	// Describe cluster.
	descResult, err := client.DescribeDBClusters(ctx, &neptune.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(clusterID),
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"_describe", descResult)

	// Delete cluster.
	_, err = client.DeleteDBCluster(ctx, &neptune.DeleteDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		SkipFinalSnapshot:   aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify cluster is deleted.
	_, err = client.DescribeDBClusters(ctx, &neptune.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(clusterID),
	})
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestNeptune_CreateAndDeleteDBInstance(t *testing.T) {
	client := newNeptuneClient(t)
	ctx := t.Context()

	clusterID := "test-neptune-cluster-for-instance"
	instanceID := "test-neptune-instance"

	// Create cluster first.
	_, err := client.CreateDBCluster(ctx, &neptune.CreateDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		Engine:              aws.String("neptune"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDBInstance(context.Background(), &neptune.DeleteDBInstanceInput{
			DBInstanceIdentifier: aws.String(instanceID),
			SkipFinalSnapshot:    aws.Bool(true),
		})
		_, _ = client.DeleteDBCluster(context.Background(), &neptune.DeleteDBClusterInput{
			DBClusterIdentifier: aws.String(clusterID),
			SkipFinalSnapshot:   aws.Bool(true),
		})
	})

	// Create instance in cluster.
	createResult, err := client.CreateDBInstance(ctx, &neptune.CreateDBInstanceInput{
		DBInstanceIdentifier: aws.String(instanceID),
		DBInstanceClass:      aws.String("db.r5.large"),
		Engine:               aws.String("neptune"),
		DBClusterIdentifier:  aws.String(clusterID),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t, golden.WithIgnoreFields("DBInstanceArn", "DbiResourceId", "InstanceCreateTime", "Address", "AllocatedStorage", "BackupRetentionPeriod", "AvailabilityZone", "Port", "ResultMetadata"))
	g.Assert(t.Name()+"_create", createResult)

	// Describe instance.
	descResult, err := client.DescribeDBInstances(ctx, &neptune.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(instanceID),
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"_describe", descResult)
}

func TestNeptune_ClusterNotFound(t *testing.T) {
	client := newNeptuneClient(t)
	ctx := t.Context()

	_, err := client.DeleteDBCluster(ctx, &neptune.DeleteDBClusterInput{
		DBClusterIdentifier: aws.String("non-existent-cluster"),
		SkipFinalSnapshot:   aws.Bool(true),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestNeptune_InstanceNotFound(t *testing.T) {
	client := newNeptuneClient(t)
	ctx := t.Context()

	_, err := client.DeleteDBInstance(ctx, &neptune.DeleteDBInstanceInput{
		DBInstanceIdentifier: aws.String("non-existent-instance"),
		SkipFinalSnapshot:    aws.Bool(true),
	})
	if err == nil {
		t.Error("expected error")
	}
}
