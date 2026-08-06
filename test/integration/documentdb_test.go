//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	"github.com/sivchari/golden"
)

func newDocDBClient(t *testing.T) *docdb.Client {
	t.Helper()

	return docdb.NewFromConfig(awsConfig(t), func(o *docdb.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestDocDB_CreateAndDeleteCluster(t *testing.T) {
	t.Parallel()

	client := newDocDBClient(t)
	ctx := t.Context()

	clusterID := "test-docdb-cluster"

	createResult, err := client.CreateDBCluster(ctx, &docdb.CreateDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		Engine:              aws.String("docdb"),
		MasterUsername:      aws.String("admin"),
		MasterUserPassword:  aws.String("password123"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDBCluster(context.Background(), &docdb.DeleteDBClusterInput{
			DBClusterIdentifier: aws.String(clusterID),
			SkipFinalSnapshot:   aws.Bool(true),
		})
	})

	g := golden.New(t, golden.WithIgnoreFields("DBClusterArn", "DbClusterResourceId", "ClusterCreateTime", "Endpoint", "ReaderEndpoint", "AllocatedStorage", "AvailabilityZones", "BackupRetentionPeriod", "Port", "ResultMetadata"))
	g.Assert(t.Name()+"_create", createResult)

	// Describe cluster.
	descResult, err := client.DescribeDBClusters(ctx, &docdb.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(clusterID),
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"_describe", descResult)

	// Delete cluster.
	_, err = client.DeleteDBCluster(ctx, &docdb.DeleteDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		SkipFinalSnapshot:   aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify cluster is deleted.
	_, err = client.DescribeDBClusters(ctx, &docdb.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(clusterID),
	})
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestDocDB_CreateAndDeleteInstance(t *testing.T) {
	t.Parallel()

	client := newDocDBClient(t)
	ctx := t.Context()

	clusterID := "test-docdb-cluster-for-instance"
	instanceID := "test-docdb-instance"

	// Create cluster first.
	_, err := client.CreateDBCluster(ctx, &docdb.CreateDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		Engine:              aws.String("docdb"),
		MasterUsername:      aws.String("admin"),
		MasterUserPassword:  aws.String("password123"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDBInstance(context.Background(), &docdb.DeleteDBInstanceInput{
			DBInstanceIdentifier: aws.String(instanceID),
		})
		_, _ = client.DeleteDBCluster(context.Background(), &docdb.DeleteDBClusterInput{
			DBClusterIdentifier: aws.String(clusterID),
			SkipFinalSnapshot:   aws.Bool(true),
		})
	})

	// Create instance in cluster.
	createResult, err := client.CreateDBInstance(ctx, &docdb.CreateDBInstanceInput{
		DBInstanceIdentifier: aws.String(instanceID),
		DBInstanceClass:      aws.String("db.r5.large"),
		Engine:               aws.String("docdb"),
		DBClusterIdentifier:  aws.String(clusterID),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t, golden.WithIgnoreFields("DBInstanceArn", "DbiResourceId", "InstanceCreateTime", "Address", "AllocatedStorage", "BackupRetentionPeriod", "AvailabilityZone", "Port", "ResultMetadata"))
	g.Assert(t.Name()+"_create", createResult)

	// Describe instance.
	descResult, err := client.DescribeDBInstances(ctx, &docdb.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(instanceID),
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"_describe", descResult)
}

func TestDocDB_DescribeClusters(t *testing.T) {
	t.Parallel()

	client := newDocDBClient(t)
	ctx := t.Context()

	clusterID := "test-docdb-describe-cluster"

	_, err := client.CreateDBCluster(ctx, &docdb.CreateDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		Engine:              aws.String("docdb"),
		MasterUsername:      aws.String("admin"),
		MasterUserPassword:  aws.String("password123"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDBCluster(context.Background(), &docdb.DeleteDBClusterInput{
			DBClusterIdentifier: aws.String(clusterID),
			SkipFinalSnapshot:   aws.Bool(true),
		})
	})

	descResult, err := client.DescribeDBClusters(ctx, &docdb.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(clusterID),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t, golden.WithIgnoreFields("DBClusterArn", "DbClusterResourceId", "ClusterCreateTime", "Endpoint", "ReaderEndpoint", "Port", "ResultMetadata"))
	g.Assert(t.Name(), descResult)
}

func TestDocDB_ModifyCluster(t *testing.T) {
	t.Parallel()

	client := newDocDBClient(t)
	ctx := t.Context()

	clusterID := "test-docdb-modify-cluster"

	_, err := client.CreateDBCluster(ctx, &docdb.CreateDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		Engine:              aws.String("docdb"),
		MasterUsername:      aws.String("admin"),
		MasterUserPassword:  aws.String("password123"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDBCluster(context.Background(), &docdb.DeleteDBClusterInput{
			DBClusterIdentifier: aws.String(clusterID),
			SkipFinalSnapshot:   aws.Bool(true),
		})
	})

	modifyResult, err := client.ModifyDBCluster(ctx, &docdb.ModifyDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		DeletionProtection:  aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t, golden.WithIgnoreFields("DBClusterArn", "DbClusterResourceId", "ClusterCreateTime", "Endpoint", "ReaderEndpoint", "Port", "ResultMetadata"))
	g.Assert(t.Name(), modifyResult)
}

func TestDocDB_DescribeInstances(t *testing.T) {
	t.Parallel()

	client := newDocDBClient(t)
	ctx := t.Context()

	clusterID := "test-docdb-cluster-for-desc-inst"
	instanceID := "test-docdb-desc-instance"

	// Create cluster first.
	_, err := client.CreateDBCluster(ctx, &docdb.CreateDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		Engine:              aws.String("docdb"),
		MasterUsername:      aws.String("admin"),
		MasterUserPassword:  aws.String("password123"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDBInstance(context.Background(), &docdb.DeleteDBInstanceInput{
			DBInstanceIdentifier: aws.String(instanceID),
		})
		_, _ = client.DeleteDBCluster(context.Background(), &docdb.DeleteDBClusterInput{
			DBClusterIdentifier: aws.String(clusterID),
			SkipFinalSnapshot:   aws.Bool(true),
		})
	})

	_, err = client.CreateDBInstance(ctx, &docdb.CreateDBInstanceInput{
		DBInstanceIdentifier: aws.String(instanceID),
		DBInstanceClass:      aws.String("db.r5.large"),
		Engine:               aws.String("docdb"),
		DBClusterIdentifier:  aws.String(clusterID),
	})
	if err != nil {
		t.Fatal(err)
	}

	descResult, err := client.DescribeDBInstances(ctx, &docdb.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(instanceID),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t, golden.WithIgnoreFields("DBInstanceArn", "DbiResourceId", "InstanceCreateTime", "Address", "Port", "ResultMetadata"))
	g.Assert(t.Name(), descResult)
}

func TestDocDB_ClusterNotFound(t *testing.T) {
	t.Parallel()

	client := newDocDBClient(t)
	ctx := t.Context()

	_, err := client.DeleteDBCluster(ctx, &docdb.DeleteDBClusterInput{
		DBClusterIdentifier: aws.String("non-existent-cluster"),
		SkipFinalSnapshot:   aws.Bool(true),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestDocDB_DuplicateCluster(t *testing.T) {
	t.Parallel()

	client := newDocDBClient(t)
	ctx := t.Context()

	clusterID := "test-docdb-duplicate-cluster"

	_, err := client.CreateDBCluster(ctx, &docdb.CreateDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		Engine:              aws.String("docdb"),
		MasterUsername:      aws.String("admin"),
		MasterUserPassword:  aws.String("password123"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDBCluster(context.Background(), &docdb.DeleteDBClusterInput{
			DBClusterIdentifier: aws.String(clusterID),
			SkipFinalSnapshot:   aws.Bool(true),
		})
	})

	// Attempt to create duplicate cluster.
	_, err = client.CreateDBCluster(ctx, &docdb.CreateDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		Engine:              aws.String("docdb"),
		MasterUsername:      aws.String("admin"),
		MasterUserPassword:  aws.String("password123"),
	})
	if err == nil {
		t.Error("expected error for duplicate cluster")
	}
}
