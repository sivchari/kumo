package neptune

import (
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// DBCluster represents a Neptune database cluster.
type DBCluster struct {
	DBClusterIdentifier string
	DBClusterArn        string
	Engine              string
	EngineVersion       string
	Status              string
	Endpoint            string
	ReaderEndpoint      string
	Port                int32
	ClusterCreateTime   time.Time
	DBClusterMembers    []DBClusterMember
	Tags                []Tag
}

// DBClusterMember represents a member of a Neptune DB cluster.
type DBClusterMember struct {
	DBInstanceIdentifier          string `json:"DBInstanceIdentifier,omitempty"`
	IsClusterWriter               bool   `json:"IsClusterWriter,omitempty"`
	DBClusterParameterGroupStatus string `json:"DBClusterParameterGroupStatus,omitempty"`
}

// DBInstance represents a Neptune database instance.
type DBInstance struct {
	DBInstanceIdentifier string
	DBInstanceArn        string
	DBInstanceClass      string
	Engine               string
	EngineVersion        string
	DBInstanceStatus     string
	Endpoint             *Endpoint
	DBClusterIdentifier  string
	InstanceCreateTime   time.Time
	Tags                 []Tag
}

// Endpoint represents a database endpoint.
type Endpoint struct {
	Address string `json:"Address,omitempty"`
	Port    int32  `json:"Port,omitempty"`
}

// Tag represents a resource tag.
type Tag struct {
	Key   string `json:"Key,omitempty"`
	Value string `json:"Value,omitempty"`
}

// Request types.

// CreateDBClusterInput represents the input for CreateDBCluster.
type CreateDBClusterInput struct {
	DBClusterIdentifier string `json:"DBClusterIdentifier"`
	Engine              string `json:"Engine,omitempty"`
	EngineVersion       string `json:"EngineVersion,omitempty"`
	Port                int32  `json:"Port,omitempty"`
	Tags                []Tag  `json:"Tags,omitempty"`
}

// DeleteDBClusterInput represents the input for DeleteDBCluster.
type DeleteDBClusterInput struct {
	DBClusterIdentifier string `json:"DBClusterIdentifier"`
	SkipFinalSnapshot   bool   `json:"SkipFinalSnapshot,omitempty"`
}

// DescribeDBClustersInput represents the input for DescribeDBClusters.
type DescribeDBClustersInput struct {
	DBClusterIdentifier string `json:"DBClusterIdentifier,omitempty"`
}

// CreateDBInstanceInput represents the input for CreateDBInstance.
type CreateDBInstanceInput struct {
	DBInstanceIdentifier string `json:"DBInstanceIdentifier"`
	DBInstanceClass      string `json:"DBInstanceClass"`
	Engine               string `json:"Engine,omitempty"`
	DBClusterIdentifier  string `json:"DBClusterIdentifier,omitempty"`
	Tags                 []Tag  `json:"Tags,omitempty"`
}

// DeleteDBInstanceInput represents the input for DeleteDBInstance.
type DeleteDBInstanceInput struct {
	DBInstanceIdentifier string `json:"DBInstanceIdentifier"`
	SkipFinalSnapshot    bool   `json:"SkipFinalSnapshot,omitempty"`
}

// DescribeDBInstancesInput represents the input for DescribeDBInstances.
type DescribeDBInstancesInput struct {
	DBInstanceIdentifier string `json:"DBInstanceIdentifier,omitempty"`
}

// Error types.

// Error represents a Neptune error.
type Error = service.CodedError

// ErrorResponse represents a Neptune error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// Error codes.
const (
	errDBClusterNotFound       = "DBClusterNotFoundFault"
	errDBClusterAlreadyExists  = "DBClusterAlreadyExistsFault"
	errDBInstanceNotFound      = "DBInstanceNotFoundFault"
	errDBInstanceAlreadyExists = "DBInstanceAlreadyExistsFault"
	errInvalidParameterValue   = "InvalidParameterValue"
)

// DB cluster states.
const (
	DBClusterStatusAvailable = "available"
	DBClusterStatusDeleting  = "deleting"
)

// DB instance states.
const (
	DBInstanceStatusAvailable = "available"
	DBInstanceStatusDeleting  = "deleting"
)
