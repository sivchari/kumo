//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/sivchari/golden"
)

func newEKSClient(t *testing.T) *eks.Client {
	t.Helper()

	return eks.NewFromConfig(awsConfig(t), func(o *eks.Options) {
		o.BaseEndpoint = aws.String(testEndpoint() + "/eks")
	})
}

func TestEKS_CreateAndDeleteCluster(t *testing.T) {
	client := newEKSClient(t)
	ctx := t.Context()
	clusterName := "test-cluster"

	// Create cluster
	createResult, err := client.CreateCluster(ctx, &eks.CreateClusterInput{
		Name:    aws.String(clusterName),
		RoleArn: aws.String("arn:aws:iam::123456789012:role/eks-cluster-role"),
		ResourcesVpcConfig: &types.VpcConfigRequest{
			SubnetIds: []string{"subnet-12345678", "subnet-87654321"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "Endpoint", "CreatedAt", "ClusterSecurityGroupId", "Issuer", "VpcId", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	// Delete cluster
	deleteResult, err := client.DeleteCluster(context.Background(), &eks.DeleteClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "Endpoint", "CreatedAt", "ClusterSecurityGroupId", "Issuer", "VpcId", "ResultMetadata")).Assert(t.Name()+"_delete", deleteResult)
}

func TestEKS_DescribeCluster(t *testing.T) {
	client := newEKSClient(t)
	ctx := t.Context()
	clusterName := "test-describe-cluster"

	// Create cluster
	_, err := client.CreateCluster(ctx, &eks.CreateClusterInput{
		Name:    aws.String(clusterName),
		RoleArn: aws.String("arn:aws:iam::123456789012:role/eks-cluster-role"),
		ResourcesVpcConfig: &types.VpcConfigRequest{
			SubnetIds: []string{"subnet-12345678"},
		},
	})
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(context.Background(), &eks.DeleteClusterInput{
			Name: aws.String(clusterName),
		})
	})

	// Describe cluster
	describeResult, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "Endpoint", "CreatedAt", "ClusterSecurityGroupId", "Issuer", "VpcId", "ResultMetadata")).Assert(t.Name(), describeResult)
}

func TestEKS_ListClusters(t *testing.T) {
	client := newEKSClient(t)
	ctx := t.Context()
	clusterName := "test-list-cluster"

	// Create cluster
	_, err := client.CreateCluster(ctx, &eks.CreateClusterInput{
		Name:    aws.String(clusterName),
		RoleArn: aws.String("arn:aws:iam::123456789012:role/eks-cluster-role"),
		ResourcesVpcConfig: &types.VpcConfigRequest{
			SubnetIds: []string{"subnet-12345678"},
		},
	})
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(context.Background(), &eks.DeleteClusterInput{
			Name: aws.String(clusterName),
		})
	})

	// List clusters - dynamic list, skip golden test.
	_, err = client.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEKS_CreateAndDeleteNodegroup(t *testing.T) {
	client := newEKSClient(t)
	ctx := t.Context()
	clusterName := "test-nodegroup-cluster"
	nodegroupName := "test-nodegroup"

	// Create cluster first
	_, err := client.CreateCluster(ctx, &eks.CreateClusterInput{
		Name:    aws.String(clusterName),
		RoleArn: aws.String("arn:aws:iam::123456789012:role/eks-cluster-role"),
		ResourcesVpcConfig: &types.VpcConfigRequest{
			SubnetIds: []string{"subnet-12345678", "subnet-87654321"},
		},
	})
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	t.Cleanup(func() {
		// Delete nodegroup first, then cluster
		_, _ = client.DeleteNodegroup(context.Background(), &eks.DeleteNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodegroupName: aws.String(nodegroupName),
		})
		_, _ = client.DeleteCluster(context.Background(), &eks.DeleteClusterInput{
			Name: aws.String(clusterName),
		})
	})

	// Create nodegroup
	createResult, err := client.CreateNodegroup(ctx, &eks.CreateNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodegroupName),
		NodeRole:      aws.String("arn:aws:iam::123456789012:role/eks-nodegroup-role"),
		Subnets:       []string{"subnet-12345678", "subnet-87654321"},
		ScalingConfig: &types.NodegroupScalingConfig{
			MinSize:     aws.Int32(1),
			MaxSize:     aws.Int32(3),
			DesiredSize: aws.Int32(2),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("NodegroupArn", "ClusterName", "CreatedAt", "ModifiedAt", "ReleaseVersion", "Name", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	// Delete nodegroup
	deleteResult, err := client.DeleteNodegroup(context.Background(), &eks.DeleteNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodegroupName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("NodegroupArn", "ClusterName", "CreatedAt", "ModifiedAt", "ReleaseVersion", "Name", "ResultMetadata")).Assert(t.Name()+"_delete", deleteResult)
}

func TestEKS_DescribeNodegroup(t *testing.T) {
	client := newEKSClient(t)
	ctx := t.Context()
	clusterName := "test-describe-nodegroup-cluster"
	nodegroupName := "test-describe-nodegroup"

	// Create cluster first
	_, err := client.CreateCluster(ctx, &eks.CreateClusterInput{
		Name:    aws.String(clusterName),
		RoleArn: aws.String("arn:aws:iam::123456789012:role/eks-cluster-role"),
		ResourcesVpcConfig: &types.VpcConfigRequest{
			SubnetIds: []string{"subnet-12345678"},
		},
	})
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteNodegroup(context.Background(), &eks.DeleteNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodegroupName: aws.String(nodegroupName),
		})
		_, _ = client.DeleteCluster(context.Background(), &eks.DeleteClusterInput{
			Name: aws.String(clusterName),
		})
	})

	// Create nodegroup
	_, err = client.CreateNodegroup(ctx, &eks.CreateNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodegroupName),
		NodeRole:      aws.String("arn:aws:iam::123456789012:role/eks-nodegroup-role"),
		Subnets:       []string{"subnet-12345678"},
	})
	if err != nil {
		t.Fatalf("failed to create nodegroup: %v", err)
	}

	// Describe nodegroup
	describeResult, err := client.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodegroupName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("NodegroupArn", "CreatedAt", "ModifiedAt", "ReleaseVersion", "Name", "ResultMetadata")).Assert(t.Name(), describeResult)
}

func TestEKS_ListNodegroups(t *testing.T) {
	client := newEKSClient(t)
	ctx := t.Context()
	clusterName := "test-list-nodegroups-cluster"
	nodegroupName := "test-list-nodegroup"

	// Create cluster first
	_, err := client.CreateCluster(ctx, &eks.CreateClusterInput{
		Name:    aws.String(clusterName),
		RoleArn: aws.String("arn:aws:iam::123456789012:role/eks-cluster-role"),
		ResourcesVpcConfig: &types.VpcConfigRequest{
			SubnetIds: []string{"subnet-12345678"},
		},
	})
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteNodegroup(context.Background(), &eks.DeleteNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodegroupName: aws.String(nodegroupName),
		})
		_, _ = client.DeleteCluster(context.Background(), &eks.DeleteClusterInput{
			Name: aws.String(clusterName),
		})
	})

	// Create nodegroup
	_, err = client.CreateNodegroup(ctx, &eks.CreateNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodegroupName),
		NodeRole:      aws.String("arn:aws:iam::123456789012:role/eks-nodegroup-role"),
		Subnets:       []string{"subnet-12345678"},
	})
	if err != nil {
		t.Fatalf("failed to create nodegroup: %v", err)
	}

	// List nodegroups - dynamic list, skip golden test.
	_, err = client.ListNodegroups(ctx, &eks.ListNodegroupsInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEKS_ClusterNotFound(t *testing.T) {
	client := newEKSClient(t)
	ctx := t.Context()

	// Try to describe non-existent cluster - error case, skip golden test.
	_, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String("non-existent-cluster"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent cluster")
	}
}

func TestEKS_NodegroupNotFound(t *testing.T) {
	client := newEKSClient(t)
	ctx := t.Context()
	clusterName := "test-nodegroup-not-found-cluster"

	// Create cluster first
	_, err := client.CreateCluster(ctx, &eks.CreateClusterInput{
		Name:    aws.String(clusterName),
		RoleArn: aws.String("arn:aws:iam::123456789012:role/eks-cluster-role"),
		ResourcesVpcConfig: &types.VpcConfigRequest{
			SubnetIds: []string{"subnet-12345678"},
		},
	})
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(context.Background(), &eks.DeleteClusterInput{
			Name: aws.String(clusterName),
		})
	})

	// Try to describe non-existent nodegroup - error case, skip golden test.
	_, err = client.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String("non-existent-nodegroup"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent nodegroup")
	}
}
