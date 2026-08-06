package rds

import (
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// DBInstance represents an RDS database instance.
type DBInstance struct {
	DBInstanceIdentifier       string
	DBInstanceClass            string
	Engine                     string
	EngineVersion              string
	DBInstanceStatus           string
	MasterUsername             string
	DBName                     string
	Endpoint                   *Endpoint
	AllocatedStorage           int32
	InstanceCreateTime         time.Time
	DBInstanceArn              string
	StorageType                string
	MultiAZ                    bool
	AvailabilityZone           string
	DBSubnetGroup              *DBSubnetGroup
	VpcSecurityGroups          []VpcSecurityGroupMembership
	DBParameterGroups          []DBParameterGroupStatus
	BackupRetentionPeriod      int32
	PreferredBackupWindow      string
	PreferredMaintenanceWindow string
	PubliclyAccessible         bool
	StorageEncrypted           bool
	DeletionProtection         bool
	Tags                       []Tag
}

// DBCluster represents an RDS database cluster.
type DBCluster struct {
	DBClusterIdentifier string
	DBClusterArn        string
	Engine              string
	EngineVersion       string
	Status              string
	MasterUsername      string
	DatabaseName        string
	Endpoint            string
	ReaderEndpoint      string
	Port                int32
	AllocatedStorage    int32
	ClusterCreateTime   time.Time
	MultiAZ             bool
	AvailabilityZones   []string
	DBClusterMembers    []DBClusterMember
	VpcSecurityGroups   []VpcSecurityGroupMembership
	StorageEncrypted    bool
	DeletionProtection  bool
	Tags                []Tag
}

// DBSnapshot represents an RDS database snapshot.
type DBSnapshot struct {
	DBSnapshotIdentifier string
	DBSnapshotArn        string
	DBInstanceIdentifier string
	Engine               string
	EngineVersion        string
	Status               string
	SnapshotType         string
	SnapshotCreateTime   time.Time
	AllocatedStorage     int32
	Port                 int32
	AvailabilityZone     string
	MasterUsername       string
	StorageType          string
	Encrypted            bool
	Tags                 []Tag
}

// Endpoint represents a database endpoint.
type Endpoint struct {
	Address      string `json:"Address,omitempty"`
	Port         int32  `json:"Port,omitempty"`
	HostedZoneID string `json:"HostedZoneId,omitempty"`
}

// DBSubnetGroup represents a DB subnet group.
type DBSubnetGroup struct {
	DBSubnetGroupName        string `json:"DBSubnetGroupName,omitempty"`
	DBSubnetGroupDescription string `json:"DBSubnetGroupDescription,omitempty"`
	VpcID                    string `json:"VpcId,omitempty"`
	SubnetGroupStatus        string `json:"SubnetGroupStatus,omitempty"`
}

// VpcSecurityGroupMembership represents a VPC security group membership.
type VpcSecurityGroupMembership struct {
	VpcSecurityGroupID string `json:"VpcSecurityGroupId,omitempty"`
	Status             string `json:"Status,omitempty"`
}

// DBParameterGroupStatus represents a DB parameter group status.
type DBParameterGroupStatus struct {
	DBParameterGroupName string `json:"DBParameterGroupName,omitempty"`
	ParameterApplyStatus string `json:"ParameterApplyStatus,omitempty"`
}

// DBClusterMember represents a member of a DB cluster.
type DBClusterMember struct {
	DBInstanceIdentifier          string `json:"DBInstanceIdentifier,omitempty"`
	IsClusterWriter               bool   `json:"IsClusterWriter,omitempty"`
	DBClusterParameterGroupStatus string `json:"DBClusterParameterGroupStatus,omitempty"`
}

// Tag represents a resource tag.
type Tag struct {
	Key   string `json:"Key,omitempty"`
	Value string `json:"Value,omitempty"`
}

// Request types.

// CreateDBInstanceInput represents the input for CreateDBInstance.
type CreateDBInstanceInput struct {
	DBInstanceIdentifier       string   `json:"DBInstanceIdentifier"`
	DBInstanceClass            string   `json:"DBInstanceClass"`
	Engine                     string   `json:"Engine"`
	EngineVersion              string   `json:"EngineVersion,omitempty"`
	MasterUsername             string   `json:"MasterUsername,omitempty"`
	MasterUserPassword         string   `json:"MasterUserPassword,omitempty"`
	DBName                     string   `json:"DBName,omitempty"`
	AllocatedStorage           int32    `json:"AllocatedStorage,omitempty"`
	StorageType                string   `json:"StorageType,omitempty"`
	MultiAZ                    bool     `json:"MultiAZ,omitempty"`
	AvailabilityZone           string   `json:"AvailabilityZone,omitempty"`
	DBSubnetGroupName          string   `json:"DBSubnetGroupName,omitempty"`
	VpcSecurityGroupIDs        []string `json:"VpcSecurityGroupIDs,omitempty"`
	BackupRetentionPeriod      int32    `json:"BackupRetentionPeriod,omitempty"`
	PreferredBackupWindow      string   `json:"PreferredBackupWindow,omitempty"`
	PreferredMaintenanceWindow string   `json:"PreferredMaintenanceWindow,omitempty"`
	PubliclyAccessible         bool     `json:"PubliclyAccessible,omitempty"`
	StorageEncrypted           bool     `json:"StorageEncrypted,omitempty"`
	DeletionProtection         bool     `json:"DeletionProtection,omitempty"`
	Tags                       []Tag    `json:"Tags,omitempty"`
}

// CreateDBInstanceOutput represents the output for CreateDBInstance.
type CreateDBInstanceOutput struct {
	DBInstance *DBInstance `json:"DBInstance,omitempty"`
}

// DeleteDBInstanceInput represents the input for DeleteDBInstance.
type DeleteDBInstanceInput struct {
	DBInstanceIdentifier      string `json:"DBInstanceIdentifier"`
	SkipFinalSnapshot         bool   `json:"SkipFinalSnapshot,omitempty"`
	FinalDBSnapshotIdentifier string `json:"FinalDBSnapshotIdentifier,omitempty"`
	DeleteAutomatedBackups    bool   `json:"DeleteAutomatedBackups,omitempty"`
}

// DeleteDBInstanceOutput represents the output for DeleteDBInstance.
type DeleteDBInstanceOutput struct {
	DBInstance *DBInstance `json:"DBInstance,omitempty"`
}

// DescribeDBInstancesInput represents the input for DescribeDBInstances.
type DescribeDBInstancesInput struct {
	DBInstanceIdentifier string `json:"DBInstanceIdentifier,omitempty"`
	MaxRecords           int32  `json:"MaxRecords,omitempty"`
	Marker               string `json:"Marker,omitempty"`
}

// DescribeDBInstancesOutput represents the output for DescribeDBInstances.
type DescribeDBInstancesOutput struct {
	DBInstances []DBInstance `json:"DBInstances,omitempty"`
	Marker      string       `json:"Marker,omitempty"`
}

// ModifyDBInstanceInput represents the input for ModifyDBInstance.
type ModifyDBInstanceInput struct {
	DBInstanceIdentifier       string   `json:"DBInstanceIdentifier"`
	DBInstanceClass            string   `json:"DBInstanceClass,omitempty"`
	AllocatedStorage           int32    `json:"AllocatedStorage,omitempty"`
	MasterUserPassword         string   `json:"MasterUserPassword,omitempty"`
	BackupRetentionPeriod      *int32   `json:"BackupRetentionPeriod,omitempty"`
	PreferredBackupWindow      string   `json:"PreferredBackupWindow,omitempty"`
	PreferredMaintenanceWindow string   `json:"PreferredMaintenanceWindow,omitempty"`
	MultiAZ                    *bool    `json:"MultiAZ,omitempty"`
	EngineVersion              string   `json:"EngineVersion,omitempty"`
	StorageType                string   `json:"StorageType,omitempty"`
	PubliclyAccessible         *bool    `json:"PubliclyAccessible,omitempty"`
	DeletionProtection         *bool    `json:"DeletionProtection,omitempty"`
	ApplyImmediately           bool     `json:"ApplyImmediately,omitempty"`
	VpcSecurityGroupIDs        []string `json:"VpcSecurityGroupIDs,omitempty"`
}

// ModifyDBInstanceOutput represents the output for ModifyDBInstance.
type ModifyDBInstanceOutput struct {
	DBInstance *DBInstance `json:"DBInstance,omitempty"`
}

// StartDBInstanceInput represents the input for StartDBInstance.
type StartDBInstanceInput struct {
	DBInstanceIdentifier string `json:"DBInstanceIdentifier"`
}

// StartDBInstanceOutput represents the output for StartDBInstance.
type StartDBInstanceOutput struct {
	DBInstance *DBInstance `json:"DBInstance,omitempty"`
}

// StopDBInstanceInput represents the input for StopDBInstance.
type StopDBInstanceInput struct {
	DBInstanceIdentifier string `json:"DBInstanceIdentifier"`
	DBSnapshotIdentifier string `json:"DBSnapshotIdentifier,omitempty"`
}

// StopDBInstanceOutput represents the output for StopDBInstance.
type StopDBInstanceOutput struct {
	DBInstance *DBInstance `json:"DBInstance,omitempty"`
}

// CreateDBClusterInput represents the input for CreateDBCluster.
type CreateDBClusterInput struct {
	DBClusterIdentifier string   `json:"DBClusterIdentifier"`
	Engine              string   `json:"Engine"`
	EngineVersion       string   `json:"EngineVersion,omitempty"`
	MasterUsername      string   `json:"MasterUsername,omitempty"`
	MasterUserPassword  string   `json:"MasterUserPassword,omitempty"`
	DatabaseName        string   `json:"DatabaseName,omitempty"`
	Port                int32    `json:"Port,omitempty"`
	AllocatedStorage    int32    `json:"AllocatedStorage,omitempty"`
	AvailabilityZones   []string `json:"AvailabilityZones,omitempty"`
	VpcSecurityGroupIDs []string `json:"VpcSecurityGroupIDs,omitempty"`
	StorageEncrypted    bool     `json:"StorageEncrypted,omitempty"`
	DeletionProtection  bool     `json:"DeletionProtection,omitempty"`
	Tags                []Tag    `json:"Tags,omitempty"`
}

// CreateDBClusterOutput represents the output for CreateDBCluster.
type CreateDBClusterOutput struct {
	DBCluster *DBCluster `json:"DBCluster,omitempty"`
}

// DeleteDBClusterInput represents the input for DeleteDBCluster.
type DeleteDBClusterInput struct {
	DBClusterIdentifier       string `json:"DBClusterIdentifier"`
	SkipFinalSnapshot         bool   `json:"SkipFinalSnapshot,omitempty"`
	FinalDBSnapshotIdentifier string `json:"FinalDBSnapshotIdentifier,omitempty"`
}

// DeleteDBClusterOutput represents the output for DeleteDBCluster.
type DeleteDBClusterOutput struct {
	DBCluster *DBCluster `json:"DBCluster,omitempty"`
}

// DescribeDBClustersInput represents the input for DescribeDBClusters.
type DescribeDBClustersInput struct {
	DBClusterIdentifier string `json:"DBClusterIdentifier,omitempty"`
	MaxRecords          int32  `json:"MaxRecords,omitempty"`
	Marker              string `json:"Marker,omitempty"`
}

// DescribeDBClustersOutput represents the output for DescribeDBClusters.
type DescribeDBClustersOutput struct {
	DBClusters []DBCluster `json:"DBClusters,omitempty"`
	Marker     string      `json:"Marker,omitempty"`
}

// ModifyDBClusterInput represents the input for ModifyDBCluster.
type ModifyDBClusterInput struct {
	DBClusterIdentifier string   `json:"DBClusterIdentifier"`
	EngineVersion       string   `json:"EngineVersion,omitempty"`
	MasterUserPassword  string   `json:"MasterUserPassword,omitempty"`
	Port                *int32   `json:"Port,omitempty"`
	DeletionProtection  *bool    `json:"DeletionProtection,omitempty"`
	VpcSecurityGroupIDs []string `json:"VpcSecurityGroupIDs,omitempty"`
	ApplyImmediately    bool     `json:"ApplyImmediately,omitempty"`
}

// CreateDBSnapshotInput represents the input for CreateDBSnapshot.
type CreateDBSnapshotInput struct {
	DBSnapshotIdentifier string `json:"DBSnapshotIdentifier"`
	DBInstanceIdentifier string `json:"DBInstanceIdentifier"`
	Tags                 []Tag  `json:"Tags,omitempty"`
}

// CreateDBSnapshotOutput represents the output for CreateDBSnapshot.
type CreateDBSnapshotOutput struct {
	DBSnapshot *DBSnapshot `json:"DBSnapshot,omitempty"`
}

// DeleteDBSnapshotInput represents the input for DeleteDBSnapshot.
type DeleteDBSnapshotInput struct {
	DBSnapshotIdentifier string `json:"DBSnapshotIdentifier"`
}

// DeleteDBSnapshotOutput represents the output for DeleteDBSnapshot.
type DeleteDBSnapshotOutput struct {
	DBSnapshot *DBSnapshot `json:"DBSnapshot,omitempty"`
}

// Error types.

// Error represents an RDS error.
type Error = service.CodedError

// ErrorResponse represents an RDS error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// Error codes.
const (
	errDBInstanceNotFound          = "DBInstanceNotFoundFault"
	errDBInstanceAlreadyExists     = "DBInstanceAlreadyExistsFault"
	errDBClusterNotFound           = "DBClusterNotFoundFault"
	errDBClusterAlreadyExists      = "DBClusterAlreadyExistsFault"
	errDBSnapshotNotFound          = "DBSnapshotNotFoundFault"
	errDBSnapshotAlreadyExists     = "DBSnapshotAlreadyExistsFault"
	errInvalidDBInstanceState      = "InvalidDBInstanceStateFault"
	errInvalidDBClusterState       = "InvalidDBClusterStateFault"
	errInvalidParameterValue       = "InvalidParameterValue"
	errInvalidParameterCombination = "InvalidParameterCombination"
)

// DB instance states.
const (
	DBInstanceStatusAvailable = "available"
	DBInstanceStatusCreating  = "creating"
	DBInstanceStatusDeleting  = "deleting"
	DBInstanceStatusStopped   = "stopped"
	DBInstanceStatusStopping  = "stopping"
	DBInstanceStatusStarting  = "starting"
	DBInstanceStatusModifying = "modifying"
)

// DB cluster states.
const (
	DBClusterStatusAvailable = "available"
	DBClusterStatusCreating  = "creating"
	DBClusterStatusDeleting  = "deleting"
)

// DB snapshot states.
const (
	DBSnapshotStatusAvailable = "available"
	DBSnapshotStatusCreating  = "creating"
)
