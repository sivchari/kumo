//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/sivchari/golden"
)

func newElastiCacheClient(t *testing.T) *elasticache.Client {
	t.Helper()

	return elasticache.NewFromConfig(awsConfig(t), func(o *elasticache.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestElastiCache_CreateAndDescribeCacheCluster(t *testing.T) {
	client := newElastiCacheClient(t)
	ctx := t.Context()

	clusterID := "test-cache-cluster"

	// Create cache cluster
	createResult, err := client.CreateCacheCluster(ctx, &elasticache.CreateCacheClusterInput{
		CacheClusterId: aws.String(clusterID),
		CacheNodeType:  aws.String("cache.t3.micro"),
		Engine:         aws.String("redis"),
		NumCacheNodes:  aws.Int32(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Address", "CacheClusterCreateTime", "CacheNodeCreateTime", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	t.Cleanup(func() {
		_, _ = client.DeleteCacheCluster(context.Background(), &elasticache.DeleteCacheClusterInput{
			CacheClusterId: aws.String(clusterID),
		})
	})

	// Describe cache clusters
	descResult, err := client.DescribeCacheClusters(ctx, &elasticache.DescribeCacheClustersInput{
		CacheClusterId: aws.String(clusterID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Address", "CacheClusterCreateTime", "CacheNodeCreateTime", "ResultMetadata")).Assert(t.Name()+"_describe", descResult)
}

func TestElastiCache_ModifyCacheCluster(t *testing.T) {
	client := newElastiCacheClient(t)
	ctx := t.Context()

	clusterID := "test-modify-cache-cluster"

	// Create cache cluster
	_, err := client.CreateCacheCluster(ctx, &elasticache.CreateCacheClusterInput{
		CacheClusterId: aws.String(clusterID),
		CacheNodeType:  aws.String("cache.t3.micro"),
		Engine:         aws.String("redis"),
		NumCacheNodes:  aws.Int32(1),
	})
	if err != nil {
		t.Fatalf("failed to create cache cluster: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCacheCluster(context.Background(), &elasticache.DeleteCacheClusterInput{
			CacheClusterId: aws.String(clusterID),
		})
	})

	// Modify cache cluster
	modifyResult, err := client.ModifyCacheCluster(ctx, &elasticache.ModifyCacheClusterInput{
		CacheClusterId:   aws.String(clusterID),
		CacheNodeType:    aws.String("cache.t3.small"),
		ApplyImmediately: aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Address", "CacheClusterCreateTime", "CacheNodeCreateTime", "ResultMetadata")).Assert(t.Name(), modifyResult)
}

func TestElastiCache_DeleteCacheCluster(t *testing.T) {
	client := newElastiCacheClient(t)
	ctx := t.Context()

	clusterID := "test-delete-cache-cluster"

	// Create cache cluster
	_, err := client.CreateCacheCluster(ctx, &elasticache.CreateCacheClusterInput{
		CacheClusterId: aws.String(clusterID),
		CacheNodeType:  aws.String("cache.t3.micro"),
		Engine:         aws.String("redis"),
		NumCacheNodes:  aws.Int32(1),
	})
	if err != nil {
		t.Fatalf("failed to create cache cluster: %v", err)
	}

	// Delete cache cluster
	deleteResult, err := client.DeleteCacheCluster(ctx, &elasticache.DeleteCacheClusterInput{
		CacheClusterId: aws.String(clusterID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Address", "CacheClusterCreateTime", "CacheNodeCreateTime", "ResultMetadata")).Assert(t.Name()+"_delete", deleteResult)

	// Verify cluster is deleted - error case, skip golden test.
	_, err = client.DescribeCacheClusters(ctx, &elasticache.DescribeCacheClustersInput{
		CacheClusterId: aws.String(clusterID),
	})
	if err == nil {
		t.Error("expected error when describing deleted cluster, got nil")
	}
}

func TestElastiCache_CreateAndDescribeReplicationGroup(t *testing.T) {
	client := newElastiCacheClient(t)
	ctx := t.Context()

	groupID := "test-replication-group"

	// Create replication group
	createResult, err := client.CreateReplicationGroup(ctx, &elasticache.CreateReplicationGroupInput{
		ReplicationGroupId:          aws.String(groupID),
		ReplicationGroupDescription: aws.String("Test replication group"),
		CacheNodeType:               aws.String("cache.t3.micro"),
		Engine:                      aws.String("redis"),
		NumCacheClusters:            aws.Int32(2),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Address", "ReplicationGroupCreateTime", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	t.Cleanup(func() {
		_, _ = client.DeleteReplicationGroup(context.Background(), &elasticache.DeleteReplicationGroupInput{
			ReplicationGroupId: aws.String(groupID),
		})
	})

	// Describe replication groups
	descResult, err := client.DescribeReplicationGroups(ctx, &elasticache.DescribeReplicationGroupsInput{
		ReplicationGroupId: aws.String(groupID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Address", "ReplicationGroupCreateTime", "ResultMetadata")).Assert(t.Name()+"_describe", descResult)
}

func TestElastiCache_DeleteReplicationGroup(t *testing.T) {
	client := newElastiCacheClient(t)
	ctx := t.Context()

	groupID := "test-delete-replication-group"

	// Create replication group
	_, err := client.CreateReplicationGroup(ctx, &elasticache.CreateReplicationGroupInput{
		ReplicationGroupId:          aws.String(groupID),
		ReplicationGroupDescription: aws.String("Test replication group for deletion"),
		CacheNodeType:               aws.String("cache.t3.micro"),
		Engine:                      aws.String("redis"),
	})
	if err != nil {
		t.Fatalf("failed to create replication group: %v", err)
	}

	// Delete replication group
	deleteResult, err := client.DeleteReplicationGroup(ctx, &elasticache.DeleteReplicationGroupInput{
		ReplicationGroupId: aws.String(groupID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Address", "ReplicationGroupCreateTime", "ResultMetadata")).Assert(t.Name()+"_delete", deleteResult)

	// Verify group is deleted - error case, skip golden test.
	_, err = client.DescribeReplicationGroups(ctx, &elasticache.DescribeReplicationGroupsInput{
		ReplicationGroupId: aws.String(groupID),
	})
	if err == nil {
		t.Error("expected error when describing deleted group, got nil")
	}
}

func TestElastiCache_DescribeCacheClusters_All(t *testing.T) {
	client := newElastiCacheClient(t)
	ctx := t.Context()

	// Create multiple cache clusters
	clusterIDs := []string{"test-cache-cluster-1", "test-cache-cluster-2"}
	for _, id := range clusterIDs {
		_, err := client.CreateCacheCluster(ctx, &elasticache.CreateCacheClusterInput{
			CacheClusterId: aws.String(id),
			CacheNodeType:  aws.String("cache.t3.micro"),
			Engine:         aws.String("redis"),
			NumCacheNodes:  aws.Int32(1),
		})
		if err != nil {
			t.Fatalf("failed to create cache cluster %s: %v", id, err)
		}
	}

	t.Cleanup(func() {
		for _, id := range clusterIDs {
			_, _ = client.DeleteCacheCluster(context.Background(), &elasticache.DeleteCacheClusterInput{
				CacheClusterId: aws.String(id),
			})
		}
	})

	// Describe all cache clusters - dynamic list, skip golden test.
	_, err := client.DescribeCacheClusters(ctx, &elasticache.DescribeCacheClustersInput{})
	if err != nil {
		t.Fatal(err)
	}
}
