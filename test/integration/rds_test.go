//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/sivchari/golden"
)

func newRDSClient(t *testing.T) *rds.Client {
	t.Helper()

	return rds.NewFromConfig(awsConfig(t), func(o *rds.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestRDS_CreateAndDescribeDBInstance(t *testing.T) {
	client := newRDSClient(t)
	ctx := t.Context()

	instanceID := "test-db-instance"

	// Create DB instance
	createResult, err := client.CreateDBInstance(ctx, &rds.CreateDBInstanceInput{
		DBInstanceIdentifier: aws.String(instanceID),
		DBInstanceClass:      aws.String("db.t3.micro"),
		Engine:               aws.String("mysql"),
		MasterUsername:       aws.String("admin"),
		MasterUserPassword:   aws.String("password123"),
		AllocatedStorage:     aws.Int32(20),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DBInstanceArn", "DbiResourceId", "InstanceCreateTime", "Address", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	t.Cleanup(func() {
		_, _ = client.DeleteDBInstance(context.Background(), &rds.DeleteDBInstanceInput{
			DBInstanceIdentifier: aws.String(instanceID),
			SkipFinalSnapshot:    aws.Bool(true),
		})
	})

	// Describe DB instances
	descResult, err := client.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(instanceID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DBInstanceArn", "DbiResourceId", "InstanceCreateTime", "Address", "ResultMetadata")).Assert(t.Name()+"_describe", descResult)
}

func TestRDS_ModifyDBInstance(t *testing.T) {
	client := newRDSClient(t)
	ctx := t.Context()

	instanceID := "test-modify-db-instance"

	// Create DB instance
	_, err := client.CreateDBInstance(ctx, &rds.CreateDBInstanceInput{
		DBInstanceIdentifier: aws.String(instanceID),
		DBInstanceClass:      aws.String("db.t3.micro"),
		Engine:               aws.String("postgres"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDBInstance(context.Background(), &rds.DeleteDBInstanceInput{
			DBInstanceIdentifier: aws.String(instanceID),
			SkipFinalSnapshot:    aws.Bool(true),
		})
	})

	// Modify DB instance
	modifyResult, err := client.ModifyDBInstance(ctx, &rds.ModifyDBInstanceInput{
		DBInstanceIdentifier: aws.String(instanceID),
		DBInstanceClass:      aws.String("db.t3.small"),
		ApplyImmediately:     aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DBInstanceArn", "DbiResourceId", "InstanceCreateTime", "Address", "ResultMetadata")).Assert(t.Name(), modifyResult)
}

func TestRDS_StartAndStopDBInstance(t *testing.T) {
	client := newRDSClient(t)
	ctx := t.Context()

	instanceID := "test-start-stop-db-instance"

	// Create DB instance
	_, err := client.CreateDBInstance(ctx, &rds.CreateDBInstanceInput{
		DBInstanceIdentifier: aws.String(instanceID),
		DBInstanceClass:      aws.String("db.t3.micro"),
		Engine:               aws.String("mysql"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDBInstance(context.Background(), &rds.DeleteDBInstanceInput{
			DBInstanceIdentifier: aws.String(instanceID),
			SkipFinalSnapshot:    aws.Bool(true),
		})
	})

	// Stop DB instance
	stopResult, err := client.StopDBInstance(ctx, &rds.StopDBInstanceInput{
		DBInstanceIdentifier: aws.String(instanceID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DBInstanceArn", "DbiResourceId", "InstanceCreateTime", "Address", "ResultMetadata")).Assert(t.Name()+"_stop", stopResult)

	// Start DB instance
	startResult, err := client.StartDBInstance(ctx, &rds.StartDBInstanceInput{
		DBInstanceIdentifier: aws.String(instanceID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DBInstanceArn", "DbiResourceId", "InstanceCreateTime", "Address", "ResultMetadata")).Assert(t.Name()+"_start", startResult)
}

func TestRDS_DeleteDBInstance(t *testing.T) {
	client := newRDSClient(t)
	ctx := t.Context()

	instanceID := "test-delete-db-instance"

	// Create DB instance
	_, err := client.CreateDBInstance(ctx, &rds.CreateDBInstanceInput{
		DBInstanceIdentifier: aws.String(instanceID),
		DBInstanceClass:      aws.String("db.t3.micro"),
		Engine:               aws.String("mysql"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete DB instance
	deleteResult, err := client.DeleteDBInstance(context.Background(), &rds.DeleteDBInstanceInput{
		DBInstanceIdentifier: aws.String(instanceID),
		SkipFinalSnapshot:    aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DBInstanceArn", "DbiResourceId", "InstanceCreateTime", "Address", "ResultMetadata")).Assert(t.Name()+"_delete", deleteResult)

	// Verify instance is deleted
	_, err = client.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(instanceID),
	})
	if err == nil {
		t.Error("expected error when describing deleted instance, got nil")
	}
}

func TestRDS_CreateAndDescribeDBCluster(t *testing.T) {
	client := newRDSClient(t)
	ctx := t.Context()

	clusterID := "test-db-cluster"

	// Create DB cluster
	createResult, err := client.CreateDBCluster(ctx, &rds.CreateDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		Engine:              aws.String("aurora-mysql"),
		MasterUsername:      aws.String("admin"),
		MasterUserPassword:  aws.String("password123"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DBClusterArn", "DbClusterResourceId", "ClusterCreateTime", "Endpoint", "ReaderEndpoint", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	t.Cleanup(func() {
		_, _ = client.DeleteDBCluster(context.Background(), &rds.DeleteDBClusterInput{
			DBClusterIdentifier: aws.String(clusterID),
			SkipFinalSnapshot:   aws.Bool(true),
		})
	})

	// Describe DB clusters
	descResult, err := client.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(clusterID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DBClusterArn", "DbClusterResourceId", "ClusterCreateTime", "Endpoint", "ReaderEndpoint", "ResultMetadata")).Assert(t.Name()+"_describe", descResult)
}

func TestRDS_DeleteDBCluster(t *testing.T) {
	client := newRDSClient(t)
	ctx := t.Context()

	clusterID := "test-delete-db-cluster"

	// Create DB cluster
	_, err := client.CreateDBCluster(ctx, &rds.CreateDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		Engine:              aws.String("aurora-postgresql"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete DB cluster
	deleteResult, err := client.DeleteDBCluster(context.Background(), &rds.DeleteDBClusterInput{
		DBClusterIdentifier: aws.String(clusterID),
		SkipFinalSnapshot:   aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DBClusterArn", "DbClusterResourceId", "ClusterCreateTime", "Endpoint", "ReaderEndpoint", "ResultMetadata")).Assert(t.Name()+"_delete", deleteResult)

	// Verify cluster is deleted
	_, err = client.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(clusterID),
	})
	if err == nil {
		t.Error("expected error when describing deleted cluster, got nil")
	}
}

func TestRDS_CreateAndDeleteDBSnapshot(t *testing.T) {
	client := newRDSClient(t)
	ctx := t.Context()

	instanceID := "test-snapshot-db-instance"
	snapshotID := "test-db-snapshot"

	// Create DB instance first
	_, err := client.CreateDBInstance(ctx, &rds.CreateDBInstanceInput{
		DBInstanceIdentifier: aws.String(instanceID),
		DBInstanceClass:      aws.String("db.t3.micro"),
		Engine:               aws.String("mysql"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDBSnapshot(context.Background(), &rds.DeleteDBSnapshotInput{
			DBSnapshotIdentifier: aws.String(snapshotID),
		})
		_, _ = client.DeleteDBInstance(context.Background(), &rds.DeleteDBInstanceInput{
			DBInstanceIdentifier: aws.String(instanceID),
			SkipFinalSnapshot:    aws.Bool(true),
		})
	})

	// Create DB snapshot
	createResult, err := client.CreateDBSnapshot(ctx, &rds.CreateDBSnapshotInput{
		DBSnapshotIdentifier: aws.String(snapshotID),
		DBInstanceIdentifier: aws.String(instanceID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DBSnapshotArn", "DbiResourceId", "SnapshotCreateTime", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	// Delete DB snapshot
	deleteResult, err := client.DeleteDBSnapshot(context.Background(), &rds.DeleteDBSnapshotInput{
		DBSnapshotIdentifier: aws.String(snapshotID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DBSnapshotArn", "DbiResourceId", "SnapshotCreateTime", "ResultMetadata")).Assert(t.Name()+"_delete", deleteResult)
}

func TestRDS_DescribeDBInstances_All(t *testing.T) {
	client := newRDSClient(t)
	ctx := t.Context()

	// Create multiple DB instances
	instanceIDs := []string{"test-db-instance-1", "test-db-instance-2"}
	for _, id := range instanceIDs {
		_, err := client.CreateDBInstance(ctx, &rds.CreateDBInstanceInput{
			DBInstanceIdentifier: aws.String(id),
			DBInstanceClass:      aws.String("db.t3.micro"),
			Engine:               aws.String("mysql"),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(func() {
		for _, id := range instanceIDs {
			_, _ = client.DeleteDBInstance(context.Background(), &rds.DeleteDBInstanceInput{
				DBInstanceIdentifier: aws.String(id),
				SkipFinalSnapshot:    aws.Bool(true),
			})
		}
	})

	// Describe all DB instances
	_, err := client.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		t.Fatal(err)
	}
}
