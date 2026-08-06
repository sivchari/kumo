// Package batch provides AWS Batch service emulation for kumo.
package batch

import (
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// Compute environment states.
const (
	CEStateEnabled  = "ENABLED"
	CEStateDisabled = "DISABLED"
)

// Compute environment statuses.
const (
	CEStatusCreating = "CREATING"
	CEStatusUpdating = "UPDATING"
	CEStatusDeleting = "DELETING"
	CEStatusDeleted  = "DELETED"
	CEStatusValid    = "VALID"
	CEStatusInvalid  = "INVALID"
)

// Compute environment types.
const (
	CETypeManaged   = "MANAGED"
	CETypeUnmanaged = "UNMANAGED"
)

// Job queue states.
const (
	JQStateEnabled  = "ENABLED"
	JQStateDisabled = "DISABLED"
)

// Job queue statuses.
const (
	JQStatusCreating = "CREATING"
	JQStatusUpdating = "UPDATING"
	JQStatusDeleting = "DELETING"
	JQStatusDeleted  = "DELETED"
	JQStatusValid    = "VALID"
	JQStatusInvalid  = "INVALID"
)

// Job statuses.
const (
	JobStatusSubmitted = "SUBMITTED"
	JobStatusPending   = "PENDING"
	JobStatusRunnable  = "RUNNABLE"
	JobStatusStarting  = "STARTING"
	JobStatusRunning   = "RUNNING"
	JobStatusSucceeded = "SUCCEEDED"
	JobStatusFailed    = "FAILED"
)

// Job definition types.
const (
	JobDefTypeContainer = "container"
	JobDefTypeMultinode = "multinode"
)

// ComputeEnvironment represents a Batch compute environment.
type ComputeEnvironment struct {
	ComputeEnvironmentARN  string            `json:"computeEnvironmentArn,omitempty"`
	ComputeEnvironmentName string            `json:"computeEnvironmentName,omitempty"`
	ComputeResources       *ComputeResource  `json:"computeResources,omitempty"`
	EksConfiguration       *EksConfiguration `json:"eksConfiguration,omitempty"`
	ServiceRole            string            `json:"serviceRole,omitempty"`
	State                  string            `json:"state,omitempty"`
	Status                 string            `json:"status,omitempty"`
	StatusReason           string            `json:"statusReason,omitempty"`
	Type                   string            `json:"type,omitempty"`
	Tags                   map[string]string `json:"tags,omitempty"`
	UpdatePolicy           *UpdatePolicy     `json:"updatePolicy,omitempty"`
	UUID                   string            `json:"uuid,omitempty"`
}

// ComputeResource represents compute resources for a compute environment.
type ComputeResource struct {
	AllocationStrategy string             `json:"allocationStrategy,omitempty"`
	BidPercentage      int32              `json:"bidPercentage,omitempty"`
	DesiredvCpus       int32              `json:"desiredvCpus,omitempty"`
	Ec2Configuration   []Ec2Configuration `json:"ec2Configuration,omitempty"`
	Ec2KeyPair         string             `json:"ec2KeyPair,omitempty"`
	ImageID            string             `json:"imageId,omitempty"`
	InstanceRole       string             `json:"instanceRole,omitempty"`
	InstanceTypes      []string           `json:"instanceTypes,omitempty"`
	LaunchTemplate     *LaunchTemplate    `json:"launchTemplate,omitempty"`
	MaxvCpus           int32              `json:"maxvCpus,omitempty"`
	MinvCpus           int32              `json:"minvCpus,omitempty"`
	PlacementGroup     string             `json:"placementGroup,omitempty"`
	SecurityGroupIDs   []string           `json:"securityGroupIds,omitempty"`
	SpotIamFleetRole   string             `json:"spotIamFleetRole,omitempty"`
	Subnets            []string           `json:"subnets,omitempty"`
	Tags               map[string]string  `json:"tags,omitempty"`
	Type               string             `json:"type,omitempty"`
}

// Ec2Configuration represents EC2 configuration.
type Ec2Configuration struct {
	ImageIDOverride        string `json:"imageIdOverride,omitempty"`
	ImageKubernetesVersion string `json:"imageKubernetesVersion,omitempty"`
	ImageType              string `json:"imageType,omitempty"`
}

// LaunchTemplate represents a launch template.
type LaunchTemplate struct {
	LaunchTemplateID   string `json:"launchTemplateId,omitempty"`
	LaunchTemplateName string `json:"launchTemplateName,omitempty"`
	Version            string `json:"version,omitempty"`
}

// EksConfiguration represents EKS configuration.
type EksConfiguration struct {
	EksClusterARN       string `json:"eksClusterArn,omitempty"`
	KubernetesNamespace string `json:"kubernetesNamespace,omitempty"`
}

// UpdatePolicy represents an update policy.
type UpdatePolicy struct {
	JobExecutionTimeoutMinutes int64 `json:"jobExecutionTimeoutMinutes,omitempty"`
	TerminateJobsOnUpdate      bool  `json:"terminateJobsOnUpdate,omitempty"`
}

// JobQueue represents a Batch job queue.
type JobQueue struct {
	ComputeEnvironmentOrder  []ComputeEnvironmentOrder `json:"computeEnvironmentOrder,omitempty"`
	JobQueueARN              string                    `json:"jobQueueArn,omitempty"`
	JobQueueName             string                    `json:"jobQueueName,omitempty"`
	JobStateTimeLimitActions []JobStateTimeLimitAction `json:"jobStateTimeLimitActions,omitempty"`
	Priority                 int32                     `json:"priority,omitempty"`
	SchedulingPolicyARN      string                    `json:"schedulingPolicyArn,omitempty"`
	State                    string                    `json:"state,omitempty"`
	Status                   string                    `json:"status,omitempty"`
	StatusReason             string                    `json:"statusReason,omitempty"`
	Tags                     map[string]string         `json:"tags,omitempty"`
}

// ComputeEnvironmentOrder represents the order of compute environments.
type ComputeEnvironmentOrder struct {
	ComputeEnvironment string `json:"computeEnvironment,omitempty"`
	Order              int32  `json:"order,omitempty"`
}

// JobStateTimeLimitAction represents a job state time limit action.
type JobStateTimeLimitAction struct {
	Action         string `json:"action,omitempty"`
	MaxTimeSeconds int32  `json:"maxTimeSeconds,omitempty"`
	Reason         string `json:"reason,omitempty"`
	State          string `json:"state,omitempty"`
}

// JobDefinition represents a Batch job definition.
type JobDefinition struct {
	ContainerProperties  *ContainerProperties `json:"containerProperties,omitempty"`
	EksProperties        *EksProperties       `json:"eksProperties,omitempty"`
	JobDefinitionARN     string               `json:"jobDefinitionArn,omitempty"`
	JobDefinitionName    string               `json:"jobDefinitionName,omitempty"`
	NodeProperties       *NodeProperties      `json:"nodeProperties,omitempty"`
	Parameters           map[string]string    `json:"parameters,omitempty"`
	PlatformCapabilities []string             `json:"platformCapabilities,omitempty"`
	PropagateTags        bool                 `json:"propagateTags,omitempty"`
	RetryStrategy        *RetryStrategy       `json:"retryStrategy,omitempty"`
	Revision             int32                `json:"revision,omitempty"`
	SchedulingPriority   int32                `json:"schedulingPriority,omitempty"`
	Status               string               `json:"status,omitempty"`
	Tags                 map[string]string    `json:"tags,omitempty"`
	Timeout              *JobTimeout          `json:"timeout,omitempty"`
	Type                 string               `json:"type,omitempty"`
}

// ContainerProperties represents container properties.
type ContainerProperties struct {
	Command                      []string                      `json:"command,omitempty"`
	Environment                  []KeyValuePair                `json:"environment,omitempty"`
	ExecutionRoleARN             string                        `json:"executionRoleArn,omitempty"`
	FargatePlatformConfiguration *FargatePlatformConfiguration `json:"fargatePlatformConfiguration,omitempty"`
	Image                        string                        `json:"image,omitempty"`
	InstanceType                 string                        `json:"instanceType,omitempty"`
	JobRoleARN                   string                        `json:"jobRoleArn,omitempty"`
	LinuxParameters              *LinuxParameters              `json:"linuxParameters,omitempty"`
	LogConfiguration             *LogConfiguration             `json:"logConfiguration,omitempty"`
	Memory                       int32                         `json:"memory,omitempty"`
	MountPoints                  []MountPoint                  `json:"mountPoints,omitempty"`
	NetworkConfiguration         *NetworkConfiguration         `json:"networkConfiguration,omitempty"`
	Privileged                   bool                          `json:"privileged,omitempty"`
	ReadonlyRootFilesystem       bool                          `json:"readonlyRootFilesystem,omitempty"`
	ResourceRequirements         []ResourceRequirement         `json:"resourceRequirements,omitempty"`
	RuntimePlatform              *RuntimePlatform              `json:"runtimePlatform,omitempty"`
	Secrets                      []Secret                      `json:"secrets,omitempty"`
	Ulimits                      []Ulimit                      `json:"ulimits,omitempty"`
	User                         string                        `json:"user,omitempty"`
	Vcpus                        int32                         `json:"vcpus,omitempty"`
	Volumes                      []Volume                      `json:"volumes,omitempty"`
}

// KeyValuePair represents a key-value pair.
type KeyValuePair struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// FargatePlatformConfiguration represents Fargate platform configuration.
type FargatePlatformConfiguration struct {
	PlatformVersion string `json:"platformVersion,omitempty"`
}

// LinuxParameters represents Linux parameters.
type LinuxParameters struct {
	Devices            []Device `json:"devices,omitempty"`
	InitProcessEnabled bool     `json:"initProcessEnabled,omitempty"`
	MaxSwap            int32    `json:"maxSwap,omitempty"`
	SharedMemorySize   int32    `json:"sharedMemorySize,omitempty"`
	Swappiness         int32    `json:"swappiness,omitempty"`
	Tmpfs              []Tmpfs  `json:"tmpfs,omitempty"`
}

// Device represents a device.
type Device struct {
	ContainerPath string   `json:"containerPath,omitempty"`
	HostPath      string   `json:"hostPath,omitempty"`
	Permissions   []string `json:"permissions,omitempty"`
}

// Tmpfs represents a tmpfs mount.
type Tmpfs struct {
	ContainerPath string   `json:"containerPath,omitempty"`
	MountOptions  []string `json:"mountOptions,omitempty"`
	Size          int32    `json:"size,omitempty"`
}

// LogConfiguration represents log configuration.
type LogConfiguration struct {
	LogDriver     string            `json:"logDriver,omitempty"`
	Options       map[string]string `json:"options,omitempty"`
	SecretOptions []Secret          `json:"secretOptions,omitempty"`
}

// MountPoint represents a mount point.
type MountPoint struct {
	ContainerPath string `json:"containerPath,omitempty"`
	ReadOnly      bool   `json:"readOnly,omitempty"`
	SourceVolume  string `json:"sourceVolume,omitempty"`
}

// NetworkConfiguration represents network configuration.
type NetworkConfiguration struct {
	AssignPublicIP string `json:"assignPublicIp,omitempty"`
}

// ResourceRequirement represents a resource requirement.
type ResourceRequirement struct {
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

// RuntimePlatform represents a runtime platform.
type RuntimePlatform struct {
	CPUArchitecture       string `json:"cpuArchitecture,omitempty"`
	OperatingSystemFamily string `json:"operatingSystemFamily,omitempty"`
}

// Secret represents a secret.
type Secret struct {
	Name      string `json:"name,omitempty"`
	ValueFrom string `json:"valueFrom,omitempty"`
}

// Ulimit represents a ulimit.
type Ulimit struct {
	HardLimit int32  `json:"hardLimit,omitempty"`
	Name      string `json:"name,omitempty"`
	SoftLimit int32  `json:"softLimit,omitempty"`
}

// Volume represents a volume.
type Volume struct {
	EfsVolumeConfiguration *EfsVolumeConfiguration `json:"efsVolumeConfiguration,omitempty"`
	Host                   *Host                   `json:"host,omitempty"`
	Name                   string                  `json:"name,omitempty"`
}

// EfsVolumeConfiguration represents EFS volume configuration.
type EfsVolumeConfiguration struct {
	AuthorizationConfig   *AuthorizationConfig `json:"authorizationConfig,omitempty"`
	FileSystemID          string               `json:"fileSystemId,omitempty"`
	RootDirectory         string               `json:"rootDirectory,omitempty"`
	TransitEncryption     string               `json:"transitEncryption,omitempty"`
	TransitEncryptionPort int32                `json:"transitEncryptionPort,omitempty"`
}

// AuthorizationConfig represents authorization config for EFS.
type AuthorizationConfig struct {
	AccessPointID string `json:"accessPointId,omitempty"`
	Iam           string `json:"iam,omitempty"`
}

// Host represents a host volume.
type Host struct {
	SourcePath string `json:"sourcePath,omitempty"`
}

// EksProperties represents EKS properties.
type EksProperties struct {
	PodProperties *EksPodProperties `json:"podProperties,omitempty"`
}

// EksPodProperties represents EKS pod properties.
type EksPodProperties struct {
	Containers         []EksContainer `json:"containers,omitempty"`
	DNSPolicy          string         `json:"dnsPolicy,omitempty"`
	HostNetwork        bool           `json:"hostNetwork,omitempty"`
	Metadata           *EksMetadata   `json:"metadata,omitempty"`
	ServiceAccountName string         `json:"serviceAccountName,omitempty"`
	Volumes            []EksVolume    `json:"volumes,omitempty"`
}

// EksContainer represents an EKS container.
type EksContainer struct {
	Args            []string                     `json:"args,omitempty"`
	Command         []string                     `json:"command,omitempty"`
	Env             []EksContainerEnvVar         `json:"env,omitempty"`
	Image           string                       `json:"image,omitempty"`
	ImagePullPolicy string                       `json:"imagePullPolicy,omitempty"`
	Name            string                       `json:"name,omitempty"`
	Resources       *EksContainerResources       `json:"resources,omitempty"`
	SecurityContext *EksContainerSecurityContext `json:"securityContext,omitempty"`
	VolumeMounts    []EksContainerVolumeMount    `json:"volumeMounts,omitempty"`
}

// EksContainerEnvVar represents an EKS container environment variable.
type EksContainerEnvVar struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// EksContainerResources represents EKS container resources.
type EksContainerResources struct {
	Limits   map[string]string `json:"limits,omitempty"`
	Requests map[string]string `json:"requests,omitempty"`
}

// EksContainerSecurityContext represents EKS container security context.
type EksContainerSecurityContext struct {
	AllowPrivilegeEscalation bool  `json:"allowPrivilegeEscalation,omitempty"`
	Privileged               bool  `json:"privileged,omitempty"`
	ReadOnlyRootFilesystem   bool  `json:"readOnlyRootFilesystem,omitempty"`
	RunAsGroup               int64 `json:"runAsGroup,omitempty"`
	RunAsNonRoot             bool  `json:"runAsNonRoot,omitempty"`
	RunAsUser                int64 `json:"runAsUser,omitempty"`
}

// EksContainerVolumeMount represents an EKS container volume mount.
type EksContainerVolumeMount struct {
	MountPath string `json:"mountPath,omitempty"`
	Name      string `json:"name,omitempty"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

// EksMetadata represents EKS metadata.
type EksMetadata struct {
	Labels map[string]string `json:"labels,omitempty"`
}

// EksVolume represents an EKS volume.
type EksVolume struct {
	EmptyDir *EksEmptyDir `json:"emptyDir,omitempty"`
	HostPath *EksHostPath `json:"hostPath,omitempty"`
	Name     string       `json:"name,omitempty"`
	Secret   *EksSecret   `json:"secret,omitempty"`
}

// EksEmptyDir represents an EKS empty dir volume.
type EksEmptyDir struct {
	Medium    string `json:"medium,omitempty"`
	SizeLimit string `json:"sizeLimit,omitempty"`
}

// EksHostPath represents an EKS host path volume.
type EksHostPath struct {
	Path string `json:"path,omitempty"`
}

// EksSecret represents an EKS secret volume.
type EksSecret struct {
	Optional   bool   `json:"optional,omitempty"`
	SecretName string `json:"secretName,omitempty"`
}

// NodeProperties represents node properties for multi-node parallel jobs.
type NodeProperties struct {
	MainNode            int32               `json:"mainNode,omitempty"`
	NodeRangeProperties []NodeRangeProperty `json:"nodeRangeProperties,omitempty"`
	NumNodes            int32               `json:"numNodes,omitempty"`
}

// NodeRangeProperty represents a node range property.
type NodeRangeProperty struct {
	Container   *ContainerProperties `json:"container,omitempty"`
	TargetNodes string               `json:"targetNodes,omitempty"`
}

// RetryStrategy represents a retry strategy.
type RetryStrategy struct {
	Attempts       int32            `json:"attempts,omitempty"`
	EvaluateOnExit []EvaluateOnExit `json:"evaluateOnExit,omitempty"`
}

// EvaluateOnExit represents an evaluate on exit action.
type EvaluateOnExit struct {
	Action         string `json:"action,omitempty"`
	OnExitCode     string `json:"onExitCode,omitempty"`
	OnReason       string `json:"onReason,omitempty"`
	OnStatusReason string `json:"onStatusReason,omitempty"`
}

// JobTimeout represents a job timeout.
type JobTimeout struct {
	AttemptDurationSeconds int32 `json:"attemptDurationSeconds,omitempty"`
}

// Job represents a Batch job.
type Job struct {
	ArrayProperties      *ArrayPropertiesDetail `json:"arrayProperties,omitempty"`
	Attempts             []AttemptDetail        `json:"attempts,omitempty"`
	Container            *ContainerDetail       `json:"container,omitempty"`
	CreatedAt            int64                  `json:"createdAt,omitempty"`
	DependsOn            []JobDependency        `json:"dependsOn,omitempty"`
	EksAttempts          []EksAttemptDetail     `json:"eksAttempts,omitempty"`
	EksProperties        *EksPropertiesDetail   `json:"eksProperties,omitempty"`
	IsCancelled          bool                   `json:"isCancelled,omitempty"`
	IsTerminated         bool                   `json:"isTerminated,omitempty"`
	JobARN               string                 `json:"jobArn,omitempty"`
	JobDefinition        string                 `json:"jobDefinition,omitempty"`
	JobID                string                 `json:"jobId,omitempty"`
	JobName              string                 `json:"jobName,omitempty"`
	JobQueue             string                 `json:"jobQueue,omitempty"`
	NodeDetails          *NodeDetails           `json:"nodeDetails,omitempty"`
	NodeProperties       *NodeProperties        `json:"nodeProperties,omitempty"`
	Parameters           map[string]string      `json:"parameters,omitempty"`
	PlatformCapabilities []string               `json:"platformCapabilities,omitempty"`
	PropagateTags        bool                   `json:"propagateTags,omitempty"`
	RetryStrategy        *RetryStrategy         `json:"retryStrategy,omitempty"`
	SchedulingPriority   int32                  `json:"schedulingPriority,omitempty"`
	ShareIdentifier      string                 `json:"shareIdentifier,omitempty"`
	StartedAt            int64                  `json:"startedAt,omitempty"`
	Status               string                 `json:"status,omitempty"`
	StatusReason         string                 `json:"statusReason,omitempty"`
	StoppedAt            int64                  `json:"stoppedAt,omitempty"`
	Tags                 map[string]string      `json:"tags,omitempty"`
	Timeout              *JobTimeout            `json:"timeout,omitempty"`
}

// ArrayPropertiesDetail represents array properties detail.
type ArrayPropertiesDetail struct {
	Index         int32            `json:"index,omitempty"`
	Size          int32            `json:"size,omitempty"`
	StatusSummary map[string]int32 `json:"statusSummary,omitempty"`
}

// AttemptDetail represents an attempt detail.
type AttemptDetail struct {
	Container    *AttemptContainerDetail `json:"container,omitempty"`
	StartedAt    int64                   `json:"startedAt,omitempty"`
	StatusReason string                  `json:"statusReason,omitempty"`
	StoppedAt    int64                   `json:"stoppedAt,omitempty"`
}

// AttemptContainerDetail represents an attempt container detail.
type AttemptContainerDetail struct {
	ContainerInstanceARN string             `json:"containerInstanceArn,omitempty"`
	ExitCode             int32              `json:"exitCode,omitempty"`
	LogStreamName        string             `json:"logStreamName,omitempty"`
	NetworkInterfaces    []NetworkInterface `json:"networkInterfaces,omitempty"`
	Reason               string             `json:"reason,omitempty"`
	TaskARN              string             `json:"taskArn,omitempty"`
}

// NetworkInterface represents a network interface.
type NetworkInterface struct {
	AttachmentID       string `json:"attachmentId,omitempty"`
	Ipv6Address        string `json:"ipv6Address,omitempty"`
	PrivateIpv4Address string `json:"privateIpv4Address,omitempty"`
}

// ContainerDetail represents container detail.
type ContainerDetail struct {
	Command                      []string                      `json:"command,omitempty"`
	ContainerInstanceARN         string                        `json:"containerInstanceArn,omitempty"`
	Environment                  []KeyValuePair                `json:"environment,omitempty"`
	ExecutionRoleARN             string                        `json:"executionRoleArn,omitempty"`
	ExitCode                     int32                         `json:"exitCode,omitempty"`
	FargatePlatformConfiguration *FargatePlatformConfiguration `json:"fargatePlatformConfiguration,omitempty"`
	Image                        string                        `json:"image,omitempty"`
	InstanceType                 string                        `json:"instanceType,omitempty"`
	JobRoleARN                   string                        `json:"jobRoleArn,omitempty"`
	LinuxParameters              *LinuxParameters              `json:"linuxParameters,omitempty"`
	LogConfiguration             *LogConfiguration             `json:"logConfiguration,omitempty"`
	LogStreamName                string                        `json:"logStreamName,omitempty"`
	Memory                       int32                         `json:"memory,omitempty"`
	MountPoints                  []MountPoint                  `json:"mountPoints,omitempty"`
	NetworkConfiguration         *NetworkConfiguration         `json:"networkConfiguration,omitempty"`
	NetworkInterfaces            []NetworkInterface            `json:"networkInterfaces,omitempty"`
	Privileged                   bool                          `json:"privileged,omitempty"`
	ReadonlyRootFilesystem       bool                          `json:"readonlyRootFilesystem,omitempty"`
	Reason                       string                        `json:"reason,omitempty"`
	ResourceRequirements         []ResourceRequirement         `json:"resourceRequirements,omitempty"`
	RuntimePlatform              *RuntimePlatform              `json:"runtimePlatform,omitempty"`
	Secrets                      []Secret                      `json:"secrets,omitempty"`
	TaskARN                      string                        `json:"taskArn,omitempty"`
	Ulimits                      []Ulimit                      `json:"ulimits,omitempty"`
	User                         string                        `json:"user,omitempty"`
	Vcpus                        int32                         `json:"vcpus,omitempty"`
	Volumes                      []Volume                      `json:"volumes,omitempty"`
}

// JobDependency represents a job dependency.
type JobDependency struct {
	JobID string `json:"jobId,omitempty"`
	Type  string `json:"type,omitempty"`
}

// EksAttemptDetail represents an EKS attempt detail.
type EksAttemptDetail struct {
	Containers   []EksAttemptContainerDetail `json:"containers,omitempty"`
	PodName      string                      `json:"podName,omitempty"`
	StartedAt    int64                       `json:"startedAt,omitempty"`
	StatusReason string                      `json:"statusReason,omitempty"`
	StoppedAt    int64                       `json:"stoppedAt,omitempty"`
}

// EksAttemptContainerDetail represents an EKS attempt container detail.
type EksAttemptContainerDetail struct {
	ExitCode int32  `json:"exitCode,omitempty"`
	Name     string `json:"name,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// EksPropertiesDetail represents EKS properties detail.
type EksPropertiesDetail struct {
	PodProperties *EksPodPropertiesDetail `json:"podProperties,omitempty"`
}

// EksPodPropertiesDetail represents EKS pod properties detail.
type EksPodPropertiesDetail struct {
	Containers         []EksContainerDetail `json:"containers,omitempty"`
	DNSPolicy          string               `json:"dnsPolicy,omitempty"`
	HostNetwork        bool                 `json:"hostNetwork,omitempty"`
	Metadata           *EksMetadata         `json:"metadata,omitempty"`
	NodeName           string               `json:"nodeName,omitempty"`
	PodName            string               `json:"podName,omitempty"`
	ServiceAccountName string               `json:"serviceAccountName,omitempty"`
	Volumes            []EksVolume          `json:"volumes,omitempty"`
}

// EksContainerDetail represents an EKS container detail.
type EksContainerDetail struct {
	Args            []string                     `json:"args,omitempty"`
	Command         []string                     `json:"command,omitempty"`
	Env             []EksContainerEnvVar         `json:"env,omitempty"`
	ExitCode        int32                        `json:"exitCode,omitempty"`
	Image           string                       `json:"image,omitempty"`
	ImagePullPolicy string                       `json:"imagePullPolicy,omitempty"`
	Name            string                       `json:"name,omitempty"`
	Reason          string                       `json:"reason,omitempty"`
	Resources       *EksContainerResources       `json:"resources,omitempty"`
	SecurityContext *EksContainerSecurityContext `json:"securityContext,omitempty"`
	VolumeMounts    []EksContainerVolumeMount    `json:"volumeMounts,omitempty"`
}

// NodeDetails represents node details.
type NodeDetails struct {
	IsMainNode bool  `json:"isMainNode,omitempty"`
	NodeIndex  int32 `json:"nodeIndex,omitempty"`
}

// CreateComputeEnvironmentInput is the request for CreateComputeEnvironment.
type CreateComputeEnvironmentInput struct {
	ComputeEnvironmentName string            `json:"computeEnvironmentName"`
	ComputeResources       *ComputeResource  `json:"computeResources,omitempty"`
	EksConfiguration       *EksConfiguration `json:"eksConfiguration,omitempty"`
	ServiceRole            string            `json:"serviceRole,omitempty"`
	State                  string            `json:"state,omitempty"`
	Tags                   map[string]string `json:"tags,omitempty"`
	Type                   string            `json:"type"`
	UnmanagedvCpus         int32             `json:"unmanagedvCpus,omitempty"`
}

// CreateComputeEnvironmentOutput is the response for CreateComputeEnvironment.
type CreateComputeEnvironmentOutput struct {
	ComputeEnvironmentARN  string `json:"computeEnvironmentArn,omitempty"`
	ComputeEnvironmentName string `json:"computeEnvironmentName,omitempty"`
}

// DeleteComputeEnvironmentInput is the request for DeleteComputeEnvironment.
type DeleteComputeEnvironmentInput struct {
	ComputeEnvironment string `json:"computeEnvironment"`
}

// DescribeComputeEnvironmentsInput is the request for DescribeComputeEnvironments.
type DescribeComputeEnvironmentsInput struct {
	ComputeEnvironments []string `json:"computeEnvironments,omitempty"`
	MaxResults          int32    `json:"maxResults,omitempty"`
	NextToken           string   `json:"nextToken,omitempty"`
}

// DescribeComputeEnvironmentsOutput is the response for DescribeComputeEnvironments.
type DescribeComputeEnvironmentsOutput struct {
	ComputeEnvironments []ComputeEnvironment `json:"computeEnvironments,omitempty"`
	NextToken           string               `json:"nextToken,omitempty"`
}

// CreateJobQueueInput is the request for CreateJobQueue.
type CreateJobQueueInput struct {
	ComputeEnvironmentOrder  []ComputeEnvironmentOrder `json:"computeEnvironmentOrder"`
	JobQueueName             string                    `json:"jobQueueName"`
	JobStateTimeLimitActions []JobStateTimeLimitAction `json:"jobStateTimeLimitActions,omitempty"`
	Priority                 int32                     `json:"priority"`
	SchedulingPolicyARN      string                    `json:"schedulingPolicyArn,omitempty"`
	State                    string                    `json:"state,omitempty"`
	Tags                     map[string]string         `json:"tags,omitempty"`
}

// CreateJobQueueOutput is the response for CreateJobQueue.
type CreateJobQueueOutput struct {
	JobQueueARN  string `json:"jobQueueArn,omitempty"`
	JobQueueName string `json:"jobQueueName,omitempty"`
}

// DeleteJobQueueInput is the request for DeleteJobQueue.
type DeleteJobQueueInput struct {
	JobQueue string `json:"jobQueue"`
}

// DescribeJobQueuesInput is the request for DescribeJobQueues.
type DescribeJobQueuesInput struct {
	JobQueues  []string `json:"jobQueues,omitempty"`
	MaxResults int32    `json:"maxResults,omitempty"`
	NextToken  string   `json:"nextToken,omitempty"`
}

// DescribeJobQueuesOutput is the response for DescribeJobQueues.
type DescribeJobQueuesOutput struct {
	JobQueues []JobQueue `json:"jobQueues,omitempty"`
	NextToken string     `json:"nextToken,omitempty"`
}

// RegisterJobDefinitionInput is the request for RegisterJobDefinition.
type RegisterJobDefinitionInput struct {
	ContainerProperties  *ContainerProperties `json:"containerProperties,omitempty"`
	EksProperties        *EksProperties       `json:"eksProperties,omitempty"`
	JobDefinitionName    string               `json:"jobDefinitionName"`
	NodeProperties       *NodeProperties      `json:"nodeProperties,omitempty"`
	Parameters           map[string]string    `json:"parameters,omitempty"`
	PlatformCapabilities []string             `json:"platformCapabilities,omitempty"`
	PropagateTags        bool                 `json:"propagateTags,omitempty"`
	RetryStrategy        *RetryStrategy       `json:"retryStrategy,omitempty"`
	SchedulingPriority   int32                `json:"schedulingPriority,omitempty"`
	Tags                 map[string]string    `json:"tags,omitempty"`
	Timeout              *JobTimeout          `json:"timeout,omitempty"`
	Type                 string               `json:"type"`
}

// RegisterJobDefinitionOutput is the response for RegisterJobDefinition.
type RegisterJobDefinitionOutput struct {
	JobDefinitionARN  string `json:"jobDefinitionArn,omitempty"`
	JobDefinitionName string `json:"jobDefinitionName,omitempty"`
	Revision          int32  `json:"revision,omitempty"`
}

// SubmitJobInput is the request for SubmitJob.
type SubmitJobInput struct {
	ArrayProperties            *ArrayProperties       `json:"arrayProperties,omitempty"`
	ContainerOverrides         *ContainerOverrides    `json:"containerOverrides,omitempty"`
	DependsOn                  []JobDependency        `json:"dependsOn,omitempty"`
	EksPropertiesOverride      *EksPropertiesOverride `json:"eksPropertiesOverride,omitempty"`
	JobDefinition              string                 `json:"jobDefinition"`
	JobName                    string                 `json:"jobName"`
	JobQueue                   string                 `json:"jobQueue"`
	NodeOverrides              *NodeOverrides         `json:"nodeOverrides,omitempty"`
	Parameters                 map[string]string      `json:"parameters,omitempty"`
	PropagateTags              bool                   `json:"propagateTags,omitempty"`
	RetryStrategy              *RetryStrategy         `json:"retryStrategy,omitempty"`
	SchedulingPriorityOverride int32                  `json:"schedulingPriorityOverride,omitempty"`
	ShareIdentifier            string                 `json:"shareIdentifier,omitempty"`
	Tags                       map[string]string      `json:"tags,omitempty"`
	Timeout                    *JobTimeout            `json:"timeout,omitempty"`
}

// ArrayProperties represents array properties.
type ArrayProperties struct {
	Size int32 `json:"size,omitempty"`
}

// ContainerOverrides represents container overrides.
type ContainerOverrides struct {
	Command              []string              `json:"command,omitempty"`
	Environment          []KeyValuePair        `json:"environment,omitempty"`
	InstanceType         string                `json:"instanceType,omitempty"`
	Memory               int32                 `json:"memory,omitempty"`
	ResourceRequirements []ResourceRequirement `json:"resourceRequirements,omitempty"`
	Vcpus                int32                 `json:"vcpus,omitempty"`
}

// EksPropertiesOverride represents EKS properties override.
type EksPropertiesOverride struct {
	PodProperties *EksPodPropertiesOverride `json:"podProperties,omitempty"`
}

// EksPodPropertiesOverride represents EKS pod properties override.
type EksPodPropertiesOverride struct {
	Containers []EksContainerOverride `json:"containers,omitempty"`
}

// EksContainerOverride represents an EKS container override.
type EksContainerOverride struct {
	Args      []string               `json:"args,omitempty"`
	Command   []string               `json:"command,omitempty"`
	Env       []EksContainerEnvVar   `json:"env,omitempty"`
	Image     string                 `json:"image,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Resources *EksContainerResources `json:"resources,omitempty"`
}

// NodeOverrides represents node overrides.
type NodeOverrides struct {
	NodePropertyOverrides []NodePropertyOverride `json:"nodePropertyOverrides,omitempty"`
	NumNodes              int32                  `json:"numNodes,omitempty"`
}

// NodePropertyOverride represents a node property override.
type NodePropertyOverride struct {
	ContainerOverrides *ContainerOverrides `json:"containerOverrides,omitempty"`
	TargetNodes        string              `json:"targetNodes,omitempty"`
}

// SubmitJobOutput is the response for SubmitJob.
type SubmitJobOutput struct {
	JobARN  string `json:"jobArn,omitempty"`
	JobID   string `json:"jobId,omitempty"`
	JobName string `json:"jobName,omitempty"`
}

// DescribeJobsInput is the request for DescribeJobs.
type DescribeJobsInput struct {
	Jobs []string `json:"jobs"`
}

// DescribeJobsOutput is the response for DescribeJobs.
type DescribeJobsOutput struct {
	Jobs []Job `json:"jobs,omitempty"`
}

// TerminateJobInput is the request for TerminateJob.
type TerminateJobInput struct {
	JobID  string `json:"jobId"`
	Reason string `json:"reason"`
}

// ErrorResponse represents a Batch error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// Error represents a Batch error.
type Error = service.CodedError

// Timestamp helper for job creation.
func nowMillis() int64 {
	return time.Now().UnixMilli()
}
