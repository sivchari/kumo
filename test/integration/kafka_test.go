//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/sivchari/golden"
)

func newKafkaClient(t *testing.T) *kafka.Client {
	t.Helper()

	return kafka.NewFromConfig(awsConfig(t), func(o *kafka.Options) {
		o.BaseEndpoint = aws.String(testEndpoint() + "/kafka")
	})
}

func TestKafka_CreateAndDeleteCluster(t *testing.T) {
	client := newKafkaClient(t)
	ctx := t.Context()
	clusterName := "test-msk-cluster"

	createResult, err := client.CreateCluster(ctx, &kafka.CreateClusterInput{
		ClusterName:         aws.String(clusterName),
		KafkaVersion:        aws.String("3.6.0"),
		NumberOfBrokerNodes: aws.Int32(3),
		BrokerNodeGroupInfo: &types.BrokerNodeGroupInfo{
			ClientSubnets: []string{"subnet-12345678", "subnet-87654321"},
			InstanceType:  aws.String("kafka.m5.large"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "ClusterArn")).Assert(t.Name()+"_create", createResult)

	// Delete cluster
	deleteResult, err := client.DeleteCluster(ctx, &kafka.DeleteClusterInput{
		ClusterArn: createResult.ClusterArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "ClusterArn")).Assert(t.Name()+"_delete", deleteResult)
}

func TestKafka_DescribeCluster(t *testing.T) {
	client := newKafkaClient(t)
	ctx := t.Context()
	clusterName := "test-describe-msk-cluster"

	createResult, err := client.CreateCluster(ctx, &kafka.CreateClusterInput{
		ClusterName:         aws.String(clusterName),
		KafkaVersion:        aws.String("3.6.0"),
		NumberOfBrokerNodes: aws.Int32(3),
		BrokerNodeGroupInfo: &types.BrokerNodeGroupInfo{
			ClientSubnets: []string{"subnet-12345678"},
			InstanceType:  aws.String("kafka.m5.large"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(context.Background(), &kafka.DeleteClusterInput{
			ClusterArn: createResult.ClusterArn,
		})
	})

	describeResult, err := client.DescribeCluster(ctx, &kafka.DescribeClusterInput{
		ClusterArn: createResult.ClusterArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "ClusterArn", "CreationTime", "CurrentVersion")).Assert(t.Name()+"_describe", describeResult)
}

func TestKafka_ListClusters(t *testing.T) {
	client := newKafkaClient(t)
	ctx := t.Context()
	clusterName := "test-list-msk-cluster"

	createResult, err := client.CreateCluster(ctx, &kafka.CreateClusterInput{
		ClusterName:         aws.String(clusterName),
		KafkaVersion:        aws.String("3.6.0"),
		NumberOfBrokerNodes: aws.Int32(3),
		BrokerNodeGroupInfo: &types.BrokerNodeGroupInfo{
			ClientSubnets: []string{"subnet-12345678"},
			InstanceType:  aws.String("kafka.m5.large"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(context.Background(), &kafka.DeleteClusterInput{
			ClusterArn: createResult.ClusterArn,
		})
	})

	listResult, err := client.ListClusters(ctx, &kafka.ListClustersInput{})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, c := range listResult.ClusterInfoList {
		if *c.ClusterName == clusterName {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected to find cluster %s in list", clusterName)
	}
}

func TestKafka_GetBootstrapBrokers(t *testing.T) {
	client := newKafkaClient(t)
	ctx := t.Context()
	clusterName := "test-bootstrap-msk-cluster"

	createResult, err := client.CreateCluster(ctx, &kafka.CreateClusterInput{
		ClusterName:         aws.String(clusterName),
		KafkaVersion:        aws.String("3.6.0"),
		NumberOfBrokerNodes: aws.Int32(3),
		BrokerNodeGroupInfo: &types.BrokerNodeGroupInfo{
			ClientSubnets: []string{"subnet-12345678"},
			InstanceType:  aws.String("kafka.m5.large"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(context.Background(), &kafka.DeleteClusterInput{
			ClusterArn: createResult.ClusterArn,
		})
	})

	bootstrapResult, err := client.GetBootstrapBrokers(ctx, &kafka.GetBootstrapBrokersInput{
		ClusterArn: createResult.ClusterArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "BootstrapBrokerString", "BootstrapBrokerStringTls")).Assert(t.Name()+"_bootstrap", bootstrapResult)
}

func TestKafka_ClusterNotFound(t *testing.T) {
	client := newKafkaClient(t)
	ctx := t.Context()

	_, err := client.DescribeCluster(ctx, &kafka.DescribeClusterInput{
		ClusterArn: aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/non-existent/fake-uuid"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent cluster")
	}
}

func TestKafka_DuplicateCluster(t *testing.T) {
	client := newKafkaClient(t)
	ctx := t.Context()
	clusterName := "test-duplicate-msk-cluster"

	createResult, err := client.CreateCluster(ctx, &kafka.CreateClusterInput{
		ClusterName:         aws.String(clusterName),
		KafkaVersion:        aws.String("3.6.0"),
		NumberOfBrokerNodes: aws.Int32(3),
		BrokerNodeGroupInfo: &types.BrokerNodeGroupInfo{
			ClientSubnets: []string{"subnet-12345678"},
			InstanceType:  aws.String("kafka.m5.large"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(context.Background(), &kafka.DeleteClusterInput{
			ClusterArn: createResult.ClusterArn,
		})
	})

	_, err = client.CreateCluster(ctx, &kafka.CreateClusterInput{
		ClusterName:         aws.String(clusterName),
		KafkaVersion:        aws.String("3.6.0"),
		NumberOfBrokerNodes: aws.Int32(3),
		BrokerNodeGroupInfo: &types.BrokerNodeGroupInfo{
			ClientSubnets: []string{"subnet-12345678"},
			InstanceType:  aws.String("kafka.m5.large"),
		},
	})
	if err == nil {
		t.Fatal("expected error for duplicate cluster name")
	}
}
