//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/sivchari/golden"
)

func newECSClient(t *testing.T) *ecs.Client {
	t.Helper()

	return ecs.NewFromConfig(awsConfig(t), func(o *ecs.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestECS_CreateAndDeleteCluster(t *testing.T) {
	client := newECSClient(t)
	ctx := t.Context()
	clusterName := "test-cluster-create-delete"

	// Create cluster.
	createOutput, err := client.CreateCluster(ctx, &ecs.CreateClusterInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ClusterArn", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Delete cluster.
	deleteOutput, err := client.DeleteCluster(ctx, &ecs.DeleteClusterInput{
		Cluster: aws.String(clusterName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ClusterArn", "ResultMetadata")).Assert(t.Name()+"_delete", deleteOutput)
}

func TestECS_ListClusters(t *testing.T) {
	client := newECSClient(t)
	ctx := t.Context()
	clusterName := "test-cluster-list"

	// Create cluster.
	_, err := client.CreateCluster(ctx, &ecs.CreateClusterInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(context.Background(), &ecs.DeleteClusterInput{
			Cluster: aws.String(clusterName),
		})
	})

	// List clusters - dynamic list, skip golden test.
	_, err = client.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestECS_DescribeClusters(t *testing.T) {
	client := newECSClient(t)
	ctx := t.Context()
	clusterName := "test-cluster-describe"

	// Create cluster.
	_, err := client.CreateCluster(ctx, &ecs.CreateClusterInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(context.Background(), &ecs.DeleteClusterInput{
			Cluster: aws.String(clusterName),
		})
	})

	// Describe clusters.
	descOutput, err := client.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{clusterName},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ClusterArn", "ResultMetadata")).Assert(t.Name(), descOutput)
}

func TestECS_RegisterAndDeregisterTaskDefinition(t *testing.T) {
	client := newECSClient(t)
	ctx := t.Context()
	family := "test-task-definition"

	// Register task definition.
	registerOutput, err := client.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family: aws.String(family),
		ContainerDefinitions: []types.ContainerDefinition{
			{
				Name:      aws.String("test-container"),
				Image:     aws.String("nginx:latest"),
				Essential: aws.Bool(true),
				Memory:    aws.Int32(512),
				PortMappings: []types.PortMapping{
					{
						ContainerPort: aws.Int32(80),
						HostPort:      aws.Int32(8080),
						Protocol:      types.TransportProtocolTcp,
					},
				},
			},
		},
		Cpu:         aws.String("256"),
		Memory:      aws.String("512"),
		NetworkMode: types.NetworkModeAwsvpc,
		RequiresCompatibilities: []types.Compatibility{
			types.CompatibilityFargate,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TaskDefinitionArn", "RegisteredAt", "ResultMetadata")).Assert(t.Name()+"_register", registerOutput)

	taskDefArn := *registerOutput.TaskDefinition.TaskDefinitionArn

	// Deregister task definition.
	deregisterOutput, err := client.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefArn),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TaskDefinitionArn", "RegisteredAt", "DeregisteredAt", "ResultMetadata")).Assert(t.Name()+"_deregister", deregisterOutput)
}

func TestECS_RunAndStopTask(t *testing.T) {
	client := newECSClient(t)
	ctx := t.Context()
	clusterName := "test-cluster-run-task"
	family := "test-task-run"

	// Create cluster.
	_, err := client.CreateCluster(ctx, &ecs.CreateClusterInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(context.Background(), &ecs.DeleteClusterInput{
			Cluster: aws.String(clusterName),
		})
	})

	// Register task definition.
	registerOutput, err := client.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family: aws.String(family),
		ContainerDefinitions: []types.ContainerDefinition{
			{
				Name:      aws.String("test-container"),
				Image:     aws.String("nginx:latest"),
				Essential: aws.Bool(true),
				Memory:    aws.Int32(512),
			},
		},
		Cpu:    aws.String("256"),
		Memory: aws.String("512"),
	})
	if err != nil {
		t.Fatalf("failed to register task definition: %v", err)
	}

	taskDefArn := *registerOutput.TaskDefinition.TaskDefinitionArn

	t.Cleanup(func() {
		_, _ = client.DeregisterTaskDefinition(context.Background(), &ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String(taskDefArn),
		})
	})

	// Run task.
	runOutput, err := client.RunTask(ctx, &ecs.RunTaskInput{
		Cluster:        aws.String(clusterName),
		TaskDefinition: aws.String(taskDefArn),
		Count:          aws.Int32(1),
		LaunchType:     types.LaunchTypeFargate,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TaskArn", "ClusterArn", "TaskDefinitionArn", "CreatedAt", "StartedAt", "PullStartedAt", "PullStoppedAt", "ContainerArn", "ResultMetadata")).Assert(t.Name()+"_run", runOutput)

	taskArn := *runOutput.Tasks[0].TaskArn

	// Stop task.
	stopOutput, err := client.StopTask(ctx, &ecs.StopTaskInput{
		Cluster: aws.String(clusterName),
		Task:    aws.String(taskArn),
		Reason:  aws.String("Test stop"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TaskArn", "ClusterArn", "TaskDefinitionArn", "ContainerArn", "CreatedAt", "StartedAt", "StoppedAt", "StoppingAt", "PullStartedAt", "PullStoppedAt", "ResultMetadata")).Assert(t.Name()+"_stop", stopOutput)
}

func TestECS_DescribeTasks(t *testing.T) {
	client := newECSClient(t)
	ctx := t.Context()
	clusterName := "test-cluster-describe-tasks"
	family := "test-task-describe"

	// Create cluster.
	_, err := client.CreateCluster(ctx, &ecs.CreateClusterInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(context.Background(), &ecs.DeleteClusterInput{
			Cluster: aws.String(clusterName),
		})
	})

	// Register task definition.
	registerOutput, err := client.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family: aws.String(family),
		ContainerDefinitions: []types.ContainerDefinition{
			{
				Name:      aws.String("test-container"),
				Image:     aws.String("nginx:latest"),
				Essential: aws.Bool(true),
				Memory:    aws.Int32(512),
			},
		},
		Cpu:    aws.String("256"),
		Memory: aws.String("512"),
	})
	if err != nil {
		t.Fatalf("failed to register task definition: %v", err)
	}

	taskDefArn := *registerOutput.TaskDefinition.TaskDefinitionArn

	t.Cleanup(func() {
		_, _ = client.DeregisterTaskDefinition(context.Background(), &ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String(taskDefArn),
		})
	})

	// Run task.
	runOutput, err := client.RunTask(ctx, &ecs.RunTaskInput{
		Cluster:        aws.String(clusterName),
		TaskDefinition: aws.String(taskDefArn),
		Count:          aws.Int32(1),
	})
	if err != nil {
		t.Fatalf("failed to run task: %v", err)
	}

	taskArn := *runOutput.Tasks[0].TaskArn

	t.Cleanup(func() {
		_, _ = client.StopTask(context.Background(), &ecs.StopTaskInput{
			Cluster: aws.String(clusterName),
			Task:    aws.String(taskArn),
		})
	})

	// Describe tasks.
	descOutput, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(clusterName),
		Tasks:   []string{taskArn},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TaskArn", "ClusterArn", "TaskDefinitionArn", "CreatedAt", "StartedAt", "PullStartedAt", "PullStoppedAt", "ContainerArn", "ResultMetadata")).Assert(t.Name(), descOutput)
}

func TestECS_CreateAndDeleteService(t *testing.T) {
	client := newECSClient(t)
	ctx := t.Context()
	clusterName := "test-cluster-service"
	serviceName := "test-service"
	family := "test-task-service"

	// Create cluster.
	_, err := client.CreateCluster(ctx, &ecs.CreateClusterInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(context.Background(), &ecs.DeleteClusterInput{
			Cluster: aws.String(clusterName),
		})
	})

	// Register task definition.
	registerOutput, err := client.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family: aws.String(family),
		ContainerDefinitions: []types.ContainerDefinition{
			{
				Name:      aws.String("test-container"),
				Image:     aws.String("nginx:latest"),
				Essential: aws.Bool(true),
				Memory:    aws.Int32(512),
			},
		},
		Cpu:    aws.String("256"),
		Memory: aws.String("512"),
	})
	if err != nil {
		t.Fatalf("failed to register task definition: %v", err)
	}

	taskDefArn := *registerOutput.TaskDefinition.TaskDefinitionArn

	t.Cleanup(func() {
		_, _ = client.DeregisterTaskDefinition(context.Background(), &ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String(taskDefArn),
		})
	})

	// Create service.
	createOutput, err := client.CreateService(ctx, &ecs.CreateServiceInput{
		Cluster:        aws.String(clusterName),
		ServiceName:    aws.String(serviceName),
		TaskDefinition: aws.String(taskDefArn),
		DesiredCount:   aws.Int32(2),
		LaunchType:     types.LaunchTypeFargate,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ServiceArn", "ClusterArn", "TaskDefinition", "CreatedAt", "UpdatedAt", "Id", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Delete service.
	deleteOutput, err := client.DeleteService(ctx, &ecs.DeleteServiceInput{
		Cluster: aws.String(clusterName),
		Service: aws.String(serviceName),
		Force:   aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ServiceArn", "ClusterArn", "TaskDefinition", "CreatedAt", "UpdatedAt", "Id", "ResultMetadata")).Assert(t.Name()+"_delete", deleteOutput)
}

func TestECS_UpdateService(t *testing.T) {
	client := newECSClient(t)
	ctx := t.Context()
	clusterName := "test-cluster-update-service"
	serviceName := "test-service-update"
	family := "test-task-update-service"

	// Create cluster.
	_, err := client.CreateCluster(ctx, &ecs.CreateClusterInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteCluster(context.Background(), &ecs.DeleteClusterInput{
			Cluster: aws.String(clusterName),
		})
	})

	// Register task definition.
	registerOutput, err := client.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family: aws.String(family),
		ContainerDefinitions: []types.ContainerDefinition{
			{
				Name:      aws.String("test-container"),
				Image:     aws.String("nginx:latest"),
				Essential: aws.Bool(true),
				Memory:    aws.Int32(512),
			},
		},
		Cpu:    aws.String("256"),
		Memory: aws.String("512"),
	})
	if err != nil {
		t.Fatalf("failed to register task definition: %v", err)
	}

	taskDefArn := *registerOutput.TaskDefinition.TaskDefinitionArn

	t.Cleanup(func() {
		_, _ = client.DeregisterTaskDefinition(context.Background(), &ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String(taskDefArn),
		})
	})

	// Create service.
	_, err = client.CreateService(ctx, &ecs.CreateServiceInput{
		Cluster:        aws.String(clusterName),
		ServiceName:    aws.String(serviceName),
		TaskDefinition: aws.String(taskDefArn),
		DesiredCount:   aws.Int32(1),
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteService(context.Background(), &ecs.DeleteServiceInput{
			Cluster: aws.String(clusterName),
			Service: aws.String(serviceName),
			Force:   aws.Bool(true),
		})
	})

	// Update service.
	updateOutput, err := client.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      aws.String(clusterName),
		Service:      aws.String(serviceName),
		DesiredCount: aws.Int32(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ServiceArn", "ClusterArn", "TaskDefinition", "CreatedAt", "UpdatedAt", "Id", "ResultMetadata")).Assert(t.Name(), updateOutput)
}

func TestECS_TaskDefinitionRevision(t *testing.T) {
	client := newECSClient(t)
	ctx := t.Context()
	family := "test-task-revision"

	var taskDefArns []string

	// Register first task definition.
	registerOutput1, err := client.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family: aws.String(family),
		ContainerDefinitions: []types.ContainerDefinition{
			{
				Name:      aws.String("container-v1"),
				Image:     aws.String("nginx:1.0"),
				Essential: aws.Bool(true),
				Memory:    aws.Int32(256),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TaskDefinitionArn", "RegisteredAt", "ResultMetadata")).Assert(t.Name()+"_register_v1", registerOutput1)

	taskDefArns = append(taskDefArns, *registerOutput1.TaskDefinition.TaskDefinitionArn)

	// Register second task definition with same family.
	registerOutput2, err := client.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family: aws.String(family),
		ContainerDefinitions: []types.ContainerDefinition{
			{
				Name:      aws.String("container-v2"),
				Image:     aws.String("nginx:2.0"),
				Essential: aws.Bool(true),
				Memory:    aws.Int32(512),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TaskDefinitionArn", "RegisteredAt", "ResultMetadata")).Assert(t.Name()+"_register_v2", registerOutput2)

	taskDefArns = append(taskDefArns, *registerOutput2.TaskDefinition.TaskDefinitionArn)

	// Cleanup.
	for _, arn := range taskDefArns {
		_, _ = client.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String(arn),
		})
	}
}
