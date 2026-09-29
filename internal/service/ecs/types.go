package ecs

import (
	"encoding/json"
	"fmt"
	"time"
)

// Timestamp is a custom time type that marshals to Unix epoch seconds for AWS SDK compatibility.
type Timestamp struct {
	time.Time
}

// MarshalJSON marshals the timestamp as Unix epoch seconds (float).
func (t Timestamp) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}

	b, err := json.Marshal(float64(t.Unix()) + float64(t.Nanosecond())/1e9)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal timestamp: %w", err)
	}

	return b, nil
}

// UnmarshalJSON unmarshals a Unix epoch seconds value to a timestamp.
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	var v float64
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("failed to unmarshal timestamp: %w", err)
	}

	sec := int64(v)
	nsec := int64((v - float64(sec)) * 1e9)
	t.Time = time.Unix(sec, nsec)

	return nil
}

// Cluster represents an ECS cluster.
type Cluster struct {
	ClusterArn                        string    `json:"clusterArn"`
	ClusterName                       string    `json:"clusterName"`
	Status                            string    `json:"status"`
	RegisteredContainerInstancesCount int       `json:"registeredContainerInstancesCount"`
	RunningTasksCount                 int       `json:"runningTasksCount"`
	PendingTasksCount                 int       `json:"pendingTasksCount"`
	ActiveServicesCount               int       `json:"activeServicesCount"`
	Settings                          []Setting `json:"settings,omitempty"`
	Tags                              []Tag     `json:"tags,omitempty"`
}

// Setting represents an ECS cluster setting.
type Setting struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// TaskDefinition represents an ECS task definition.
type TaskDefinition struct {
	TaskDefinitionArn       string                `json:"taskDefinitionArn"`
	Family                  string                `json:"family"`
	Revision                int                   `json:"revision"`
	Status                  string                `json:"status"`
	ContainerDefinitions    []ContainerDefinition `json:"containerDefinitions"`
	CPU                     string                `json:"cpu,omitempty"`
	Memory                  string                `json:"memory,omitempty"`
	NetworkMode             string                `json:"networkMode,omitempty"`
	RequiresCompatibilities []string              `json:"requiresCompatibilities,omitempty"`
	ExecutionRoleArn        string                `json:"executionRoleArn,omitempty"`
	TaskRoleArn             string                `json:"taskRoleArn,omitempty"`
	EphemeralStorage        *EphemeralStorage     `json:"ephemeralStorage,omitempty"`
	Tags                    []Tag                 `json:"tags,omitempty"`
}

// ContainerDefinition represents a container in a task definition.
type ContainerDefinition struct {
	Name                   string            `json:"name"`
	Image                  string            `json:"image"`
	CPU                    int               `json:"cpu,omitempty"`
	Memory                 int               `json:"memory,omitempty"`
	Essential              bool              `json:"essential"`
	PortMappings           []PortMapping     `json:"portMappings,omitempty"`
	Environment            []KeyValuePair    `json:"environment,omitempty"`
	Command                []string          `json:"command,omitempty"`
	EntryPoint             []string          `json:"entryPoint,omitempty"`
	User                   string            `json:"user,omitempty"`
	ReadonlyRootFilesystem bool              `json:"readonlyRootFilesystem,omitempty"`
	MountPoints            []MountPoint      `json:"mountPoints"`
	LinuxParameters        *LinuxParameters  `json:"linuxParameters,omitempty"`
	LogConfiguration       *LogConfiguration `json:"logConfiguration,omitempty"`
}

// MountPoint represents a container volume mount.
type MountPoint struct {
	SourceVolume  string `json:"sourceVolume"`
	ContainerPath string `json:"containerPath"`
	ReadOnly      bool   `json:"readOnly"`
}

// LinuxParameters represents Linux-specific container settings.
type LinuxParameters struct {
	InitProcessEnabled bool                `json:"initProcessEnabled,omitempty"`
	Capabilities       *KernelCapabilities `json:"capabilities,omitempty"`
}

// KernelCapabilities represents Linux capability changes.
type KernelCapabilities struct {
	Drop []string `json:"drop"`
	Add  []string `json:"add"`
}

// LogConfiguration represents a container log driver configuration.
type LogConfiguration struct {
	LogDriver string            `json:"logDriver"`
	Options   map[string]string `json:"options,omitempty"`
}

// EphemeralStorage represents Fargate ephemeral storage.
type EphemeralStorage struct {
	SizeInGiB int `json:"sizeInGiB"`
}

// PortMapping represents a port mapping.
type PortMapping struct {
	ContainerPort int    `json:"containerPort"`
	HostPort      int    `json:"hostPort,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

// KeyValuePair represents a key-value pair.
type KeyValuePair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Task represents a running task.
type Task struct {
	TaskArn              string      `json:"taskArn"`
	ClusterArn           string      `json:"clusterArn"`
	TaskDefinitionArn    string      `json:"taskDefinitionArn"`
	ContainerInstanceArn string      `json:"containerInstanceArn,omitempty"`
	LastStatus           string      `json:"lastStatus"`
	DesiredStatus        string      `json:"desiredStatus"`
	CPU                  string      `json:"cpu,omitempty"`
	Memory               string      `json:"memory,omitempty"`
	Containers           []Container `json:"containers"`
	StartedAt            *Timestamp  `json:"startedAt,omitempty"`
	StoppedAt            *Timestamp  `json:"stoppedAt,omitempty"`
	StoppedReason        string      `json:"stoppedReason,omitempty"`
	Group                string      `json:"group,omitempty"`
	LaunchType           string      `json:"launchType,omitempty"`
	Tags                 []Tag       `json:"tags,omitempty"`
}

// Container represents a container in a task.
type Container struct {
	ContainerArn    string           `json:"containerArn"`
	Name            string           `json:"name"`
	Image           string           `json:"image,omitempty"`
	LastStatus      string           `json:"lastStatus"`
	ExitCode        *int             `json:"exitCode,omitempty"`
	Reason          string           `json:"reason,omitempty"`
	NetworkBindings []NetworkBinding `json:"networkBindings,omitempty"`
}

// NetworkBinding represents a network binding.
type NetworkBinding struct {
	BindIP        string `json:"bindIp,omitempty"`
	ContainerPort int    `json:"containerPort"`
	HostPort      int    `json:"hostPort,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

// ServiceResource represents an ECS service.
type ServiceResource struct {
	ServiceArn                  string                `json:"serviceArn"`
	ServiceName                 string                `json:"serviceName"`
	ClusterArn                  string                `json:"clusterArn"`
	TaskDefinition              string                `json:"taskDefinition"`
	DesiredCount                int                   `json:"desiredCount"`
	RunningCount                int                   `json:"runningCount"`
	PendingCount                int                   `json:"pendingCount"`
	LaunchType                  string                `json:"launchType,omitempty"`
	Status                      string                `json:"status"`
	SchedulingStrategy          string                `json:"schedulingStrategy,omitempty"`
	AvailabilityZoneRebalancing string                `json:"availabilityZoneRebalancing,omitempty"`
	LoadBalancers               []ServiceLoadBalancer `json:"loadBalancers,omitempty"`
	NetworkConfiguration        *NetworkConfiguration `json:"networkConfiguration,omitempty"`
	PlatformVersion             string                `json:"platformVersion,omitempty"`
	PropagateTags               string                `json:"propagateTags,omitempty"`
	EnableExecuteCommand        bool                  `json:"enableExecuteCommand"`
	Deployments                 []Deployment          `json:"deployments,omitempty"`
	Tags                        []Tag                 `json:"tags,omitempty"`
}

// ServiceLoadBalancer represents an ECS service load balancer attachment.
type ServiceLoadBalancer struct {
	TargetGroupArn   string `json:"targetGroupArn,omitempty"`
	LoadBalancerName string `json:"loadBalancerName,omitempty"`
	ContainerName    string `json:"containerName,omitempty"`
	ContainerPort    int    `json:"containerPort,omitempty"`
}

// NetworkConfiguration represents an ECS service network configuration.
type NetworkConfiguration struct {
	AwsvpcConfiguration *AwsVpcConfiguration `json:"awsvpcConfiguration,omitempty"`
}

// AwsVpcConfiguration represents awsvpc networking settings.
type AwsVpcConfiguration struct {
	Subnets        []string `json:"subnets,omitempty"`
	SecurityGroups []string `json:"securityGroups,omitempty"`
	AssignPublicIP string   `json:"assignPublicIp,omitempty"`
}

// Deployment represents a service deployment.
type Deployment struct {
	ID             string     `json:"id"`
	Status         string     `json:"status"`
	TaskDefinition string     `json:"taskDefinition"`
	DesiredCount   int        `json:"desiredCount"`
	RunningCount   int        `json:"runningCount"`
	PendingCount   int        `json:"pendingCount"`
	CreatedAt      *Timestamp `json:"createdAt,omitempty"`
	UpdatedAt      *Timestamp `json:"updatedAt,omitempty"`
}

// Tag represents a resource tag.
type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Request types.

// CreateClusterRequest represents a CreateCluster request.
type CreateClusterRequest struct {
	ClusterName string    `json:"clusterName"`
	Settings    []Setting `json:"settings,omitempty"`
	Tags        []Tag     `json:"tags,omitempty"`
}

// DeleteClusterRequest represents a DeleteCluster request.
type DeleteClusterRequest struct {
	Cluster string `json:"cluster"`
}

// DescribeClustersRequest represents a DescribeClusters request.
type DescribeClustersRequest struct {
	Clusters []string `json:"clusters,omitempty"`
}

// ListClustersRequest represents a ListClusters request.
type ListClustersRequest struct {
	MaxResults int    `json:"maxResults,omitempty"`
	NextToken  string `json:"nextToken,omitempty"`
}

// RegisterTaskDefinitionRequest represents a RegisterTaskDefinition request.
type RegisterTaskDefinitionRequest struct {
	Family                  string                `json:"family"`
	ContainerDefinitions    []ContainerDefinition `json:"containerDefinitions"`
	CPU                     string                `json:"cpu,omitempty"`
	Memory                  string                `json:"memory,omitempty"`
	NetworkMode             string                `json:"networkMode,omitempty"`
	RequiresCompatibilities []string              `json:"requiresCompatibilities,omitempty"`
	ExecutionRoleArn        string                `json:"executionRoleArn,omitempty"`
	TaskRoleArn             string                `json:"taskRoleArn,omitempty"`
	EphemeralStorage        *EphemeralStorage     `json:"ephemeralStorage,omitempty"`
	Tags                    []Tag                 `json:"tags,omitempty"`
}

// DeregisterTaskDefinitionRequest represents a DeregisterTaskDefinition request.
type DeregisterTaskDefinitionRequest struct {
	TaskDefinition string `json:"taskDefinition"`
}

// DescribeTaskDefinitionRequest represents a DescribeTaskDefinition request.
type DescribeTaskDefinitionRequest struct {
	TaskDefinition string `json:"taskDefinition"`
}

// RunTaskRequest represents a RunTask request.
type RunTaskRequest struct {
	Cluster        string `json:"cluster,omitempty"`
	TaskDefinition string `json:"taskDefinition"`
	Count          int    `json:"count,omitempty"`
	LaunchType     string `json:"launchType,omitempty"`
	Group          string `json:"group,omitempty"`
	Tags           []Tag  `json:"tags,omitempty"`
}

// StopTaskRequest represents a StopTask request.
type StopTaskRequest struct {
	Cluster string `json:"cluster,omitempty"`
	Task    string `json:"task"`
	Reason  string `json:"reason,omitempty"`
}

// DescribeTasksRequest represents a DescribeTasks request.
type DescribeTasksRequest struct {
	Cluster string   `json:"cluster,omitempty"`
	Tasks   []string `json:"tasks"`
}

// CreateServiceRequest represents a CreateService request.
type CreateServiceRequest struct {
	Cluster                     string                `json:"cluster,omitempty"`
	ServiceName                 string                `json:"serviceName"`
	TaskDefinition              string                `json:"taskDefinition"`
	DesiredCount                int                   `json:"desiredCount"`
	LaunchType                  string                `json:"launchType,omitempty"`
	SchedulingStrategy          string                `json:"schedulingStrategy,omitempty"`
	AvailabilityZoneRebalancing string                `json:"availabilityZoneRebalancing,omitempty"`
	LoadBalancers               []ServiceLoadBalancer `json:"loadBalancers,omitempty"`
	NetworkConfiguration        *NetworkConfiguration `json:"networkConfiguration,omitempty"`
	PlatformVersion             string                `json:"platformVersion,omitempty"`
	PropagateTags               string                `json:"propagateTags,omitempty"`
	EnableExecuteCommand        bool                  `json:"enableExecuteCommand,omitempty"`
	Tags                        []Tag                 `json:"tags,omitempty"`
}

// DescribeServicesRequest represents a DescribeServices request.
type DescribeServicesRequest struct {
	Cluster  string   `json:"cluster,omitempty"`
	Services []string `json:"services"`
}

// DeleteServiceRequest represents a DeleteService request.
type DeleteServiceRequest struct {
	Cluster string `json:"cluster,omitempty"`
	Service string `json:"service"`
	Force   bool   `json:"force,omitempty"`
}

// UpdateServiceRequest represents an UpdateService request.
type UpdateServiceRequest struct {
	Cluster        string `json:"cluster,omitempty"`
	Service        string `json:"service"`
	TaskDefinition string `json:"taskDefinition,omitempty"`
	DesiredCount   *int   `json:"desiredCount,omitempty"`
}

// Response types.

// CreateClusterResponse represents a CreateCluster response.
type CreateClusterResponse struct {
	Cluster *Cluster `json:"cluster"`
}

// DeleteClusterResponse represents a DeleteCluster response.
type DeleteClusterResponse struct {
	Cluster *Cluster `json:"cluster"`
}

// DescribeClustersResponse represents a DescribeClusters response.
type DescribeClustersResponse struct {
	Clusters []Cluster `json:"clusters"`
	Failures []Failure `json:"failures,omitempty"`
}

// ListClustersResponse represents a ListClusters response.
type ListClustersResponse struct {
	ClusterArns []string `json:"clusterArns"`
	NextToken   string   `json:"nextToken,omitempty"`
}

// RegisterTaskDefinitionResponse represents a RegisterTaskDefinition response.
type RegisterTaskDefinitionResponse struct {
	TaskDefinition *TaskDefinition `json:"taskDefinition"`
	Tags           []Tag           `json:"tags,omitempty"`
}

// DeregisterTaskDefinitionResponse represents a DeregisterTaskDefinition response.
type DeregisterTaskDefinitionResponse struct {
	TaskDefinition *TaskDefinition `json:"taskDefinition"`
}

// DescribeTaskDefinitionResponse represents a DescribeTaskDefinition response.
type DescribeTaskDefinitionResponse struct {
	TaskDefinition *TaskDefinition `json:"taskDefinition"`
	Tags           []Tag           `json:"tags,omitempty"`
}

// RunTaskResponse represents a RunTask response.
type RunTaskResponse struct {
	Tasks    []Task    `json:"tasks"`
	Failures []Failure `json:"failures,omitempty"`
}

// StopTaskResponse represents a StopTask response.
type StopTaskResponse struct {
	Task *Task `json:"task"`
}

// DescribeTasksResponse represents a DescribeTasks response.
type DescribeTasksResponse struct {
	Tasks    []Task    `json:"tasks"`
	Failures []Failure `json:"failures,omitempty"`
}

// CreateServiceResponse represents a CreateService response.
type CreateServiceResponse struct {
	Service *ServiceResource `json:"service"`
}

// DescribeServicesResponse represents a DescribeServices response.
type DescribeServicesResponse struct {
	Services []ServiceResource `json:"services"`
	Failures []Failure         `json:"failures,omitempty"`
}

// DeleteServiceResponse represents a DeleteService response.
type DeleteServiceResponse struct {
	Service *ServiceResource `json:"service"`
}

// UpdateServiceResponse represents an UpdateService response.
type UpdateServiceResponse struct {
	Service *ServiceResource `json:"service"`
}

// Failure represents a failure in a batch operation.
type Failure struct {
	Arn    string `json:"arn,omitempty"`
	Reason string `json:"reason"`
}

// Error represents an ECS error.
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}
