package redshift

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultAccountID    = "000000000000"
	defaultRegion       = "us-east-1"
	defaultNodeType     = "dc2.large"
	defaultRedshiftPort = 5439
	defaultDBName       = "dev"
)

// Storage defines the Redshift storage interface.
type Storage interface {
	CreateCluster(ctx context.Context, input *CreateClusterInput) (*Cluster, error)
	DeleteCluster(ctx context.Context, input *DeleteClusterInput) (*Cluster, error)
	DescribeClusters(ctx context.Context, identifier string) ([]Cluster, error)
	ModifyCluster(ctx context.Context, input *ModifyClusterInput) (*Cluster, error)
	CreateClusterSnapshot(ctx context.Context, input *CreateClusterSnapshotInput) (*ClusterSnapshot, error)
	DeleteClusterSnapshot(ctx context.Context, identifier string) (*ClusterSnapshot, error)
	DescribeClusterSnapshots(ctx context.Context, clusterIdentifier, snapshotIdentifier string) ([]ClusterSnapshot, error)
}

// Option is a configuration option for MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables persistent storage in the specified directory.
func WithDataDir(dir string) Option {
	return func(s *MemoryStorage) {
		s.dataDir = dir
	}
}

// Compile-time interface checks.
var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MemoryStorage implements Storage with in-memory data.
type MemoryStorage struct {
	mu        sync.RWMutex                `json:"-"`
	Clusters  map[string]*Cluster         `json:"clusters"`
	Snapshots map[string]*ClusterSnapshot `json:"snapshots"`
	dataDir   string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Clusters:  make(map[string]*Cluster),
		Snapshots: make(map[string]*ClusterSnapshot),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "redshift", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (m *MemoryStorage) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(m)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (m *MemoryStorage) UnmarshalJSON(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(m)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if m.Clusters == nil {
		m.Clusters = make(map[string]*Cluster)
	}

	if m.Snapshots == nil {
		m.Snapshots = make(map[string]*ClusterSnapshot)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	type alias MemoryStorage

	data, err := json.Marshal(&struct{ *alias }{alias: (*alias)(m)})
	if err != nil {
		return
	}

	_ = storage.SaveBytes(m.dataDir, "redshift", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "redshift", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateCluster creates a new Redshift cluster.
func (m *MemoryStorage) CreateCluster(_ context.Context, input *CreateClusterInput) (*Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Clusters[input.ClusterIdentifier]; exists {
		return nil, &Error{
			Code:    errClusterAlreadyExists,
			Message: fmt.Sprintf("Cluster already exists: %s", input.ClusterIdentifier),
		}
	}

	nodeType := input.NodeType
	if nodeType == "" {
		nodeType = defaultNodeType
	}

	dbName := input.DBName
	if dbName == "" {
		dbName = defaultDBName
	}

	numberOfNodes := input.NumberOfNodes
	if numberOfNodes == 0 {
		numberOfNodes = 1
	}

	cluster := &Cluster{
		ClusterIdentifier:   input.ClusterIdentifier,
		ClusterNamespaceArn: m.clusterArn(input.ClusterIdentifier),
		NodeType:            nodeType,
		ClusterStatus:       clusterStatusAvailable,
		MasterUsername:      input.MasterUsername,
		DBName:              dbName,
		Endpoint: Endpoint{
			Address: fmt.Sprintf("%s.%s.%s.redshift.amazonaws.com", input.ClusterIdentifier, generateID(), defaultRegion),
			Port:    defaultRedshiftPort,
		},
		NumberOfNodes:     numberOfNodes,
		ClusterCreateTime: time.Now(),
		Tags:              input.Tags,
	}

	m.Clusters[input.ClusterIdentifier] = cluster

	m.saveLocked()

	return cluster, nil
}

// DeleteCluster deletes a Redshift cluster.
func (m *MemoryStorage) DeleteCluster(_ context.Context, input *DeleteClusterInput) (*Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.Clusters[input.ClusterIdentifier]
	if !exists {
		return nil, &Error{
			Code:    errClusterNotFound,
			Message: fmt.Sprintf("Cluster not found: %s", input.ClusterIdentifier),
		}
	}

	cluster.ClusterStatus = clusterStatusDeleting

	delete(m.Clusters, input.ClusterIdentifier)

	m.saveLocked()

	return cluster, nil
}

// DescribeClusters describes Redshift clusters.
func (m *MemoryStorage) DescribeClusters(_ context.Context, identifier string) ([]Cluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if identifier != "" {
		cluster, exists := m.Clusters[identifier]
		if !exists {
			return nil, &Error{
				Code:    errClusterNotFound,
				Message: fmt.Sprintf("Cluster not found: %s", identifier),
			}
		}

		return []Cluster{*cluster}, nil
	}

	clusters := make([]Cluster, 0, len(m.Clusters))

	for _, cluster := range m.Clusters {
		clusters = append(clusters, *cluster)
	}

	return clusters, nil
}

// ModifyCluster modifies a Redshift cluster.
func (m *MemoryStorage) ModifyCluster(_ context.Context, input *ModifyClusterInput) (*Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.Clusters[input.ClusterIdentifier]
	if !exists {
		return nil, &Error{
			Code:    errClusterNotFound,
			Message: fmt.Sprintf("Cluster not found: %s", input.ClusterIdentifier),
		}
	}

	if input.NodeType != "" {
		cluster.NodeType = input.NodeType
	}

	if input.NumberOfNodes > 0 {
		cluster.NumberOfNodes = input.NumberOfNodes
	}

	m.saveLocked()

	return cluster, nil
}

// CreateClusterSnapshot creates a new Redshift cluster snapshot.
func (m *MemoryStorage) CreateClusterSnapshot(_ context.Context, input *CreateClusterSnapshotInput) (*ClusterSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Snapshots[input.SnapshotIdentifier]; exists {
		return nil, &Error{
			Code:    errSnapshotAlreadyExists,
			Message: fmt.Sprintf("Cluster snapshot already exists: %s", input.SnapshotIdentifier),
		}
	}

	cluster, exists := m.Clusters[input.ClusterIdentifier]
	if !exists {
		return nil, &Error{
			Code:    errClusterNotFound,
			Message: fmt.Sprintf("Cluster not found: %s", input.ClusterIdentifier),
		}
	}

	snapshot := &ClusterSnapshot{
		SnapshotIdentifier: input.SnapshotIdentifier,
		ClusterIdentifier:  input.ClusterIdentifier,
		SnapshotCreateTime: time.Now(),
		Status:             snapshotStatusAvailable,
		Port:               cluster.Endpoint.Port,
		NumberOfNodes:      cluster.NumberOfNodes,
		DBName:             cluster.DBName,
		MasterUsername:     cluster.MasterUsername,
		Tags:               input.Tags,
	}

	m.Snapshots[input.SnapshotIdentifier] = snapshot

	m.saveLocked()

	return snapshot, nil
}

// DeleteClusterSnapshot deletes a Redshift cluster snapshot.
func (m *MemoryStorage) DeleteClusterSnapshot(_ context.Context, identifier string) (*ClusterSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshot, exists := m.Snapshots[identifier]
	if !exists {
		return nil, &Error{
			Code:    errSnapshotNotFound,
			Message: fmt.Sprintf("Cluster snapshot not found: %s", identifier),
		}
	}

	delete(m.Snapshots, identifier)

	m.saveLocked()

	return snapshot, nil
}

// DescribeClusterSnapshots describes Redshift cluster snapshots.
func (m *MemoryStorage) DescribeClusterSnapshots(_ context.Context, clusterIdentifier, snapshotIdentifier string) ([]ClusterSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if snapshotIdentifier != "" {
		snapshot, exists := m.Snapshots[snapshotIdentifier]
		if !exists {
			return nil, &Error{
				Code:    errSnapshotNotFound,
				Message: fmt.Sprintf("Cluster snapshot not found: %s", snapshotIdentifier),
			}
		}

		return []ClusterSnapshot{*snapshot}, nil
	}

	snapshots := make([]ClusterSnapshot, 0, len(m.Snapshots))

	for _, snapshot := range m.Snapshots {
		if clusterIdentifier != "" && snapshot.ClusterIdentifier != clusterIdentifier {
			continue
		}

		snapshots = append(snapshots, *snapshot)
	}

	return snapshots, nil
}

// Helper functions.

func (m *MemoryStorage) clusterArn(identifier string) string {
	return fmt.Sprintf("arn:aws:redshift:%s:%s:cluster:%s", defaultRegion, defaultAccountID, identifier)
}

func generateID() string {
	return uuid.New().String()[:8]
}
