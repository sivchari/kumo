package elasticache

import (
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// CacheCluster represents an ElastiCache cluster.
type CacheCluster struct {
	CacheClusterID             string
	CacheClusterStatus         string
	CacheNodeType              string
	Engine                     string
	EngineVersion              string
	NumCacheNodes              int32
	PreferredAvailabilityZone  string
	CacheClusterCreateTime     time.Time
	PreferredMaintenanceWindow string
	CacheSubnetGroupName       string
	AutoMinorVersionUpgrade    bool
	SnapshotRetentionLimit     int32
	SnapshotWindow             string
	ARN                        string
	CacheNodes                 []CacheNode
	CacheSecurityGroups        []CacheSecurityGroupMembership
	SecurityGroups             []SecurityGroupMembership
	CacheParameterGroup        *CacheParameterGroupStatus
	ConfigurationEndpoint      *Endpoint
}

// CacheNode represents a node in a cache cluster.
type CacheNode struct {
	CacheNodeID              string
	CacheNodeStatus          string
	CacheNodeCreateTime      time.Time
	Endpoint                 *Endpoint
	ParameterGroupStatus     string
	CustomerAvailabilityZone string
}

// ReplicationGroup represents an ElastiCache replication group.
type ReplicationGroup struct {
	ReplicationGroupID         string
	Description                string
	Status                     string
	MemberClusters             []string
	NodeGroups                 []NodeGroup
	AutomaticFailover          string
	MultiAZ                    string
	SnapshotRetentionLimit     int32
	SnapshotWindow             string
	ClusterEnabled             bool
	CacheNodeType              string
	AuthTokenEnabled           bool
	TransitEncryptionEnabled   bool
	AtRestEncryptionEnabled    bool
	ARN                        string
	ConfigurationEndpoint      *Endpoint
	ReplicationGroupCreateTime time.Time
	AutoMinorVersionUpgrade    bool
	PreferredMaintenanceWindow string
}

// NodeGroup represents a node group in a replication group.
type NodeGroup struct {
	NodeGroupID      string
	Status           string
	PrimaryEndpoint  *Endpoint
	ReaderEndpoint   *Endpoint
	NodeGroupMembers []NodeGroupMember
}

// NodeGroupMember represents a member of a node group.
type NodeGroupMember struct {
	CacheClusterID            string
	CacheNodeID               string
	ReadEndpoint              *Endpoint
	PreferredAvailabilityZone string
	CurrentRole               string
}

// Endpoint represents an endpoint.
type Endpoint struct {
	Address string
	Port    int32
}

// CacheSecurityGroupMembership represents a cache security group membership.
type CacheSecurityGroupMembership struct {
	CacheSecurityGroupName string
	Status                 string
}

// SecurityGroupMembership represents a security group membership.
type SecurityGroupMembership struct {
	SecurityGroupID string
	Status          string
}

// CacheParameterGroupStatus represents a cache parameter group status.
type CacheParameterGroupStatus struct {
	CacheParameterGroupName string
	ParameterApplyStatus    string
	CacheNodeIDsToReboot    []string
}

// Request types.

// CreateCacheClusterInput represents the input for CreateCacheCluster.
type CreateCacheClusterInput struct {
	CacheClusterID             string   `json:"CacheClusterId"`
	CacheNodeType              string   `json:"CacheNodeType"`
	Engine                     string   `json:"Engine"`
	EngineVersion              string   `json:"EngineVersion,omitempty"`
	NumCacheNodes              int32    `json:"NumCacheNodes,omitempty"`
	PreferredAvailabilityZone  string   `json:"PreferredAvailabilityZone,omitempty"`
	PreferredMaintenanceWindow string   `json:"PreferredMaintenanceWindow,omitempty"`
	CacheSubnetGroupName       string   `json:"CacheSubnetGroupName,omitempty"`
	SecurityGroupIDs           []string `json:"SecurityGroupIds,omitempty"`
	AutoMinorVersionUpgrade    bool     `json:"AutoMinorVersionUpgrade,omitempty"`
	SnapshotRetentionLimit     int32    `json:"SnapshotRetentionLimit,omitempty"`
	SnapshotWindow             string   `json:"SnapshotWindow,omitempty"`
	ReplicationGroupID         string   `json:"ReplicationGroupId,omitempty"`
	Port                       int32    `json:"Port,omitempty"`
}

// CreateCacheClusterOutput represents the output for CreateCacheCluster.
type CreateCacheClusterOutput struct {
	CacheCluster *CacheCluster `json:"CacheCluster,omitempty"`
}

// DeleteCacheClusterInput represents the input for DeleteCacheCluster.
type DeleteCacheClusterInput struct {
	CacheClusterID          string `json:"CacheClusterId"`
	FinalSnapshotIdentifier string `json:"FinalSnapshotIdentifier,omitempty"`
}

// DeleteCacheClusterOutput represents the output for DeleteCacheCluster.
type DeleteCacheClusterOutput struct {
	CacheCluster *CacheCluster `json:"CacheCluster,omitempty"`
}

// DescribeCacheClustersInput represents the input for DescribeCacheClusters.
type DescribeCacheClustersInput struct {
	CacheClusterID    string `json:"CacheClusterId,omitempty"`
	MaxRecords        int32  `json:"MaxRecords,omitempty"`
	Marker            string `json:"Marker,omitempty"`
	ShowCacheNodeInfo bool   `json:"ShowCacheNodeInfo,omitempty"`
}

// DescribeCacheClustersOutput represents the output for DescribeCacheClusters.
type DescribeCacheClustersOutput struct {
	CacheClusters []CacheCluster `json:"CacheClusters,omitempty"`
	Marker        string         `json:"Marker,omitempty"`
}

// ModifyCacheClusterInput represents the input for ModifyCacheCluster.
type ModifyCacheClusterInput struct {
	CacheClusterID             string   `json:"CacheClusterId"`
	NumCacheNodes              *int32   `json:"NumCacheNodes,omitempty"`
	CacheNodeIDsToRemove       []string `json:"CacheNodeIdsToRemove,omitempty"`
	EngineVersion              string   `json:"EngineVersion,omitempty"`
	CacheNodeType              string   `json:"CacheNodeType,omitempty"`
	PreferredMaintenanceWindow string   `json:"PreferredMaintenanceWindow,omitempty"`
	AutoMinorVersionUpgrade    *bool    `json:"AutoMinorVersionUpgrade,omitempty"`
	SnapshotRetentionLimit     *int32   `json:"SnapshotRetentionLimit,omitempty"`
	SnapshotWindow             string   `json:"SnapshotWindow,omitempty"`
	SecurityGroupIDs           []string `json:"SecurityGroupIds,omitempty"`
	ApplyImmediately           bool     `json:"ApplyImmediately,omitempty"`
}

// ModifyCacheClusterOutput represents the output for ModifyCacheCluster.
type ModifyCacheClusterOutput struct {
	CacheCluster *CacheCluster `json:"CacheCluster,omitempty"`
}

// CreateReplicationGroupInput represents the input for CreateReplicationGroup.
type CreateReplicationGroupInput struct {
	ReplicationGroupID          string   `json:"ReplicationGroupId"`
	ReplicationGroupDescription string   `json:"ReplicationGroupDescription"`
	PrimaryClusterID            string   `json:"PrimaryClusterId,omitempty"`
	AutomaticFailoverEnabled    bool     `json:"AutomaticFailoverEnabled,omitempty"`
	MultiAZEnabled              bool     `json:"MultiAZEnabled,omitempty"`
	NumCacheClusters            int32    `json:"NumCacheClusters,omitempty"`
	NumNodeGroups               int32    `json:"NumNodeGroups,omitempty"`
	ReplicasPerNodeGroup        int32    `json:"ReplicasPerNodeGroup,omitempty"`
	CacheNodeType               string   `json:"CacheNodeType,omitempty"`
	Engine                      string   `json:"Engine,omitempty"`
	EngineVersion               string   `json:"EngineVersion,omitempty"`
	CacheSubnetGroupName        string   `json:"CacheSubnetGroupName,omitempty"`
	SecurityGroupIDs            []string `json:"SecurityGroupIds,omitempty"`
	PreferredMaintenanceWindow  string   `json:"PreferredMaintenanceWindow,omitempty"`
	SnapshotRetentionLimit      int32    `json:"SnapshotRetentionLimit,omitempty"`
	SnapshotWindow              string   `json:"SnapshotWindow,omitempty"`
	AutoMinorVersionUpgrade     bool     `json:"AutoMinorVersionUpgrade,omitempty"`
	TransitEncryptionEnabled    bool     `json:"TransitEncryptionEnabled,omitempty"`
	AtRestEncryptionEnabled     bool     `json:"AtRestEncryptionEnabled,omitempty"`
	Port                        int32    `json:"Port,omitempty"`
}

// CreateReplicationGroupOutput represents the output for CreateReplicationGroup.
type CreateReplicationGroupOutput struct {
	ReplicationGroup *ReplicationGroup `json:"ReplicationGroup,omitempty"`
}

// DeleteReplicationGroupInput represents the input for DeleteReplicationGroup.
type DeleteReplicationGroupInput struct {
	ReplicationGroupID      string `json:"ReplicationGroupId"`
	RetainPrimaryCluster    bool   `json:"RetainPrimaryCluster,omitempty"`
	FinalSnapshotIdentifier string `json:"FinalSnapshotIdentifier,omitempty"`
}

// DeleteReplicationGroupOutput represents the output for DeleteReplicationGroup.
type DeleteReplicationGroupOutput struct {
	ReplicationGroup *ReplicationGroup `json:"ReplicationGroup,omitempty"`
}

// DescribeReplicationGroupsInput represents the input for DescribeReplicationGroups.
type DescribeReplicationGroupsInput struct {
	ReplicationGroupID string `json:"ReplicationGroupId,omitempty"`
	MaxRecords         int32  `json:"MaxRecords,omitempty"`
	Marker             string `json:"Marker,omitempty"`
}

// DescribeReplicationGroupsOutput represents the output for DescribeReplicationGroups.
type DescribeReplicationGroupsOutput struct {
	ReplicationGroups []ReplicationGroup `json:"ReplicationGroups,omitempty"`
	Marker            string             `json:"Marker,omitempty"`
}

// Error types.

// Error represents an ElastiCache error.
type Error = service.CodedError

// Error codes.
const (
	errCacheClusterNotFound          = "CacheClusterNotFoundFault"
	errCacheClusterAlreadyExists     = "CacheClusterAlreadyExistsFault"
	errReplicationGroupNotFound      = "ReplicationGroupNotFoundFault"
	errReplicationGroupAlreadyExists = "ReplicationGroupAlreadyExistsFault"
	errInvalidCacheClusterState      = "InvalidCacheClusterStateFault"
	errInvalidReplicationGroupState  = "InvalidReplicationGroupStateFault"
	errInvalidParameterValue         = "InvalidParameterValue"
	errInvalidParameterCombination   = "InvalidParameterCombination"
)

// Cache cluster states.
const (
	CacheClusterStatusAvailable = "available"
	CacheClusterStatusCreating  = "creating"
	CacheClusterStatusDeleting  = "deleting"
	CacheClusterStatusModifying = "modifying"
)

// Replication group states.
const (
	ReplicationGroupStatusAvailable = "available"
	ReplicationGroupStatusCreating  = "creating"
	ReplicationGroupStatusDeleting  = "deleting"
	ReplicationGroupStatusModifying = "modifying"
)
