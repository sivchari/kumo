package redshift

import (
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// Cluster represents a Redshift cluster.
type Cluster struct {
	ClusterIdentifier   string
	ClusterNamespaceArn string
	NodeType            string
	ClusterStatus       string
	MasterUsername      string
	DBName              string
	Endpoint            Endpoint
	NumberOfNodes       int32
	ClusterCreateTime   time.Time
	Tags                []Tag
}

// ClusterSnapshot represents a Redshift cluster snapshot.
type ClusterSnapshot struct {
	SnapshotIdentifier string
	ClusterIdentifier  string
	SnapshotCreateTime time.Time
	Status             string
	Port               int32
	NumberOfNodes      int32
	DBName             string
	MasterUsername     string
	Tags               []Tag
}

// Endpoint represents a Redshift cluster endpoint.
type Endpoint struct {
	Address string
	Port    int32
}

// Tag represents a resource tag.
type Tag struct {
	Key   string `json:"Key,omitempty"`
	Value string `json:"Value,omitempty"`
}

// Request types.

// CreateClusterInput represents the input for CreateCluster.
type CreateClusterInput struct {
	ClusterIdentifier  string `json:"ClusterIdentifier"`
	NodeType           string `json:"NodeType,omitempty"`
	MasterUsername     string `json:"MasterUsername,omitempty"`
	MasterUserPassword string `json:"MasterUserPassword,omitempty"`
	DBName             string `json:"DBName,omitempty"`
	NumberOfNodes      int32  `json:"NumberOfNodes,omitempty"`
	Tags               []Tag  `json:"Tags,omitempty"`
}

// DeleteClusterInput represents the input for DeleteCluster.
type DeleteClusterInput struct {
	ClusterIdentifier              string `json:"ClusterIdentifier"`
	SkipFinalClusterSnapshot       bool   `json:"SkipFinalClusterSnapshot,omitempty"`
	FinalClusterSnapshotIdentifier string `json:"FinalClusterSnapshotIdentifier,omitempty"`
}

// DescribeClustersInput represents the input for DescribeClusters.
type DescribeClustersInput struct {
	ClusterIdentifier string `json:"ClusterIdentifier,omitempty"`
}

// ModifyClusterInput represents the input for ModifyCluster.
type ModifyClusterInput struct {
	ClusterIdentifier string `json:"ClusterIdentifier"`
	NodeType          string `json:"NodeType,omitempty"`
	NumberOfNodes     int32  `json:"NumberOfNodes,omitempty"`
	ClusterType       string `json:"ClusterType,omitempty"`
}

// CreateClusterSnapshotInput represents the input for CreateClusterSnapshot.
type CreateClusterSnapshotInput struct {
	SnapshotIdentifier string `json:"SnapshotIdentifier"`
	ClusterIdentifier  string `json:"ClusterIdentifier"`
	Tags               []Tag  `json:"Tags,omitempty"`
}

// DeleteClusterSnapshotInput represents the input for DeleteClusterSnapshot.
type DeleteClusterSnapshotInput struct {
	SnapshotIdentifier string `json:"SnapshotIdentifier"`
}

// DescribeClusterSnapshotsInput represents the input for DescribeClusterSnapshots.
type DescribeClusterSnapshotsInput struct {
	ClusterIdentifier  string `json:"ClusterIdentifier,omitempty"`
	SnapshotIdentifier string `json:"SnapshotIdentifier,omitempty"`
}

// Error types.

// Error represents a Redshift error.
type Error = service.CodedError

// ErrorResponse represents a Redshift error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// Error codes.
const (
	errClusterNotFound       = "ClusterNotFound"
	errClusterAlreadyExists  = "ClusterAlreadyExists"
	errSnapshotNotFound      = "ClusterSnapshotNotFound"
	errSnapshotAlreadyExists = "ClusterSnapshotAlreadyExists"
	errInvalidParameterValue = "InvalidParameterValue"
)

// Cluster states.
const (
	clusterStatusAvailable = "available"
	clusterStatusDeleting  = "deleting"
	clusterStatusModifying = "modifying"
)

// Snapshot states.
const (
	snapshotStatusAvailable = "available"
)
