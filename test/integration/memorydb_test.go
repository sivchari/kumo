//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/memorydb"
	"github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	"github.com/sivchari/golden"
)

func newMemoryDBClient(t *testing.T) *memorydb.Client {
	t.Helper()

	return memorydb.NewFromConfig(awsConfig(t), func(o *memorydb.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestMemoryDB_CreateAndDeleteCluster(t *testing.T) {
	client := newMemoryDBClient(t)
	ctx := t.Context()
	clusterName := "test-cluster"

	createResult, err := client.CreateCluster(ctx, &memorydb.CreateClusterInput{
		ClusterName: aws.String(clusterName),
		NodeType:    aws.String("db.r6g.large"),
		ACLName:     aws.String("open-access"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "ARN")).Assert(t.Name()+"_create", createResult)

	// Delete
	deleteResult, err := client.DeleteCluster(ctx, &memorydb.DeleteClusterInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "ARN")).Assert(t.Name()+"_delete", deleteResult)
}

func TestMemoryDB_DescribeClusters(t *testing.T) {
	client := newMemoryDBClient(t)
	ctx := t.Context()
	clusterName := "test-describe-cluster"

	_, err := client.CreateCluster(ctx, &memorydb.CreateClusterInput{
		ClusterName: aws.String(clusterName),
		NodeType:    aws.String("db.r6g.large"),
		ACLName:     aws.String("open-access"),
		Description: aws.String("test description"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(t.Context(), &memorydb.DeleteClusterInput{
			ClusterName: aws.String(clusterName),
		})
	})

	describeResult, err := client.DescribeClusters(ctx, &memorydb.DescribeClustersInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "ARN")).Assert(t.Name()+"_describe", describeResult)
}

func TestMemoryDB_UpdateCluster(t *testing.T) {
	client := newMemoryDBClient(t)
	ctx := t.Context()
	clusterName := "test-update-cluster"

	_, err := client.CreateCluster(ctx, &memorydb.CreateClusterInput{
		ClusterName: aws.String(clusterName),
		NodeType:    aws.String("db.r6g.large"),
		ACLName:     aws.String("open-access"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(t.Context(), &memorydb.DeleteClusterInput{
			ClusterName: aws.String(clusterName),
		})
	})

	updateResult, err := client.UpdateCluster(ctx, &memorydb.UpdateClusterInput{
		ClusterName: aws.String(clusterName),
		Description: aws.String("updated description"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "ARN")).Assert(t.Name()+"_update", updateResult)
}

func TestMemoryDB_CreateAndDeleteUser(t *testing.T) {
	client := newMemoryDBClient(t)
	ctx := t.Context()
	userName := "test-user"

	createResult, err := client.CreateUser(ctx, &memorydb.CreateUserInput{
		UserName:     aws.String(userName),
		AccessString: aws.String("on ~* &* +@all"),
		AuthenticationMode: &types.AuthenticationMode{
			Type:      types.InputAuthenticationTypePassword,
			Passwords: []string{"testpassword123"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "ARN")).Assert(t.Name()+"_create", createResult)

	// Delete
	deleteResult, err := client.DeleteUser(ctx, &memorydb.DeleteUserInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "ARN")).Assert(t.Name()+"_delete", deleteResult)
}

func TestMemoryDB_DescribeUsers(t *testing.T) {
	client := newMemoryDBClient(t)
	ctx := t.Context()
	userName := "test-describe-user"

	_, err := client.CreateUser(ctx, &memorydb.CreateUserInput{
		UserName:     aws.String(userName),
		AccessString: aws.String("on ~* &* +@all"),
		AuthenticationMode: &types.AuthenticationMode{
			Type:      types.InputAuthenticationTypePassword,
			Passwords: []string{"testpassword123"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteUser(t.Context(), &memorydb.DeleteUserInput{
			UserName: aws.String(userName),
		})
	})

	describeResult, err := client.DescribeUsers(ctx, &memorydb.DescribeUsersInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "ARN")).Assert(t.Name()+"_describe", describeResult)
}

func TestMemoryDB_CreateAndDeleteACL(t *testing.T) {
	client := newMemoryDBClient(t)
	ctx := t.Context()
	aclName := "test-acl"

	createResult, err := client.CreateACL(ctx, &memorydb.CreateACLInput{
		ACLName: aws.String(aclName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "ARN")).Assert(t.Name()+"_create", createResult)

	// Delete
	deleteResult, err := client.DeleteACL(ctx, &memorydb.DeleteACLInput{
		ACLName: aws.String(aclName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "ARN")).Assert(t.Name()+"_delete", deleteResult)
}

func TestMemoryDB_DescribeACLs(t *testing.T) {
	client := newMemoryDBClient(t)
	ctx := t.Context()
	aclName := "test-describe-acl"

	_, err := client.CreateACL(ctx, &memorydb.CreateACLInput{
		ACLName:   aws.String(aclName),
		UserNames: []string{"default"},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteACL(t.Context(), &memorydb.DeleteACLInput{
			ACLName: aws.String(aclName),
		})
	})

	describeResult, err := client.DescribeACLs(ctx, &memorydb.DescribeACLsInput{
		ACLName: aws.String(aclName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "ARN")).Assert(t.Name()+"_describe", describeResult)
}

func TestMemoryDB_ClusterNotFound(t *testing.T) {
	client := newMemoryDBClient(t)
	ctx := t.Context()

	_, err := client.DescribeClusters(ctx, &memorydb.DescribeClustersInput{
		ClusterName: aws.String("non-existent-cluster"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent cluster")
	}
}

func TestMemoryDB_DuplicateCluster(t *testing.T) {
	client := newMemoryDBClient(t)
	ctx := t.Context()
	clusterName := "test-dup-cluster"

	_, err := client.CreateCluster(ctx, &memorydb.CreateClusterInput{
		ClusterName: aws.String(clusterName),
		NodeType:    aws.String("db.r6g.large"),
		ACLName:     aws.String("open-access"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(t.Context(), &memorydb.DeleteClusterInput{
			ClusterName: aws.String(clusterName),
		})
	})

	_, err = client.CreateCluster(ctx, &memorydb.CreateClusterInput{
		ClusterName: aws.String(clusterName),
		NodeType:    aws.String("db.r6g.large"),
		ACLName:     aws.String("open-access"),
	})
	if err == nil {
		t.Fatal("expected error for duplicate cluster")
	}
}
