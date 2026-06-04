package neptune

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
	defaultAccountID   = "000000000000"
	defaultRegion      = "us-east-1"
	defaultEngine      = "neptune"
	defaultEngineVer   = "1.3.0.0"
	defaultNeptunePort = 8182
)

// Storage defines the Neptune storage interface.
type Storage interface {
	CreateDBCluster(ctx context.Context, input *CreateDBClusterInput) (*DBCluster, error)
	DeleteDBCluster(ctx context.Context, identifier string, skipFinalSnapshot bool) (*DBCluster, error)
	DescribeDBClusters(ctx context.Context, identifier string) ([]DBCluster, error)
	CreateDBInstance(ctx context.Context, input *CreateDBInstanceInput) (*DBInstance, error)
	DeleteDBInstance(ctx context.Context, identifier string, skipFinalSnapshot bool) (*DBInstance, error)
	DescribeDBInstances(ctx context.Context, identifier string) ([]DBInstance, error)
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
	mu        sync.RWMutex           `json:"-"`
	Clusters  map[string]*DBCluster  `json:"clusters"`
	Instances map[string]*DBInstance `json:"instances"`
	dataDir   string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Clusters:  make(map[string]*DBCluster),
		Instances: make(map[string]*DBInstance),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "neptune", s)
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
		m.Clusters = make(map[string]*DBCluster)
	}

	if m.Instances == nil {
		m.Instances = make(map[string]*DBInstance)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "neptune", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "neptune", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateDBCluster creates a new Neptune DB cluster.
func (m *MemoryStorage) CreateDBCluster(_ context.Context, input *CreateDBClusterInput) (*DBCluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Clusters[input.DBClusterIdentifier]; exists {
		return nil, &Error{
			Code:    errDBClusterAlreadyExists,
			Message: fmt.Sprintf("DB cluster already exists: %s", input.DBClusterIdentifier),
		}
	}

	engine := input.Engine
	if engine == "" {
		engine = defaultEngine
	}

	engineVersion := input.EngineVersion
	if engineVersion == "" {
		engineVersion = defaultEngineVer
	}

	port := input.Port
	if port == 0 {
		port = defaultNeptunePort
	}

	cluster := &DBCluster{
		DBClusterIdentifier: input.DBClusterIdentifier,
		DBClusterArn:        m.dbClusterArn(input.DBClusterIdentifier),
		Engine:              engine,
		EngineVersion:       engineVersion,
		Status:              DBClusterStatusAvailable,
		Endpoint:            fmt.Sprintf("%s.cluster-%s.%s.neptune.amazonaws.com", input.DBClusterIdentifier, generateID(), defaultRegion),
		ReaderEndpoint:      fmt.Sprintf("%s.cluster-ro-%s.%s.neptune.amazonaws.com", input.DBClusterIdentifier, generateID(), defaultRegion),
		Port:                port,
		ClusterCreateTime:   time.Now(),
		Tags:                input.Tags,
	}

	m.Clusters[input.DBClusterIdentifier] = cluster

	m.saveLocked()

	return cluster, nil
}

// DeleteDBCluster deletes a Neptune DB cluster.
func (m *MemoryStorage) DeleteDBCluster(_ context.Context, identifier string, _ bool) (*DBCluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.Clusters[identifier]
	if !exists {
		return nil, &Error{
			Code:    errDBClusterNotFound,
			Message: fmt.Sprintf("DB cluster not found: %s", identifier),
		}
	}

	cluster.Status = DBClusterStatusDeleting

	delete(m.Clusters, identifier)

	m.saveLocked()

	return cluster, nil
}

// DescribeDBClusters describes Neptune DB clusters.
func (m *MemoryStorage) DescribeDBClusters(_ context.Context, identifier string) ([]DBCluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if identifier != "" {
		cluster, exists := m.Clusters[identifier]
		if !exists {
			return nil, &Error{
				Code:    errDBClusterNotFound,
				Message: fmt.Sprintf("DB cluster not found: %s", identifier),
			}
		}

		return []DBCluster{*cluster}, nil
	}

	clusters := make([]DBCluster, 0, len(m.Clusters))
	for _, cluster := range m.Clusters {
		clusters = append(clusters, *cluster)
	}

	return clusters, nil
}

// CreateDBInstance creates a new Neptune DB instance.
func (m *MemoryStorage) CreateDBInstance(_ context.Context, input *CreateDBInstanceInput) (*DBInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Instances[input.DBInstanceIdentifier]; exists {
		return nil, &Error{
			Code:    errDBInstanceAlreadyExists,
			Message: fmt.Sprintf("DB instance already exists: %s", input.DBInstanceIdentifier),
		}
	}

	engine := input.Engine
	if engine == "" {
		engine = defaultEngine
	}

	instance := &DBInstance{
		DBInstanceIdentifier: input.DBInstanceIdentifier,
		DBInstanceArn:        m.dbInstanceArn(input.DBInstanceIdentifier),
		DBInstanceClass:      input.DBInstanceClass,
		Engine:               engine,
		EngineVersion:        defaultEngineVer,
		DBInstanceStatus:     DBInstanceStatusAvailable,
		DBClusterIdentifier:  input.DBClusterIdentifier,
		InstanceCreateTime:   time.Now(),
		Tags:                 input.Tags,
		Endpoint: &Endpoint{
			Address: fmt.Sprintf("%s.%s.%s.neptune.amazonaws.com", input.DBInstanceIdentifier, generateID(), defaultRegion),
			Port:    defaultNeptunePort,
		},
	}

	m.Instances[input.DBInstanceIdentifier] = instance

	// Add instance to cluster's DBClusterMembers if associated with a cluster.
	if input.DBClusterIdentifier != "" {
		if cluster, exists := m.Clusters[input.DBClusterIdentifier]; exists {
			isWriter := len(cluster.DBClusterMembers) == 0
			cluster.DBClusterMembers = append(cluster.DBClusterMembers, DBClusterMember{
				DBInstanceIdentifier:          input.DBInstanceIdentifier,
				IsClusterWriter:               isWriter,
				DBClusterParameterGroupStatus: "in-sync",
			})
		}
	}

	m.saveLocked()

	return instance, nil
}

// DeleteDBInstance deletes a Neptune DB instance.
func (m *MemoryStorage) DeleteDBInstance(_ context.Context, identifier string, _ bool) (*DBInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.Instances[identifier]
	if !exists {
		return nil, &Error{
			Code:    errDBInstanceNotFound,
			Message: fmt.Sprintf("DB instance not found: %s", identifier),
		}
	}

	instance.DBInstanceStatus = DBInstanceStatusDeleting

	// Remove instance from cluster's DBClusterMembers.
	if instance.DBClusterIdentifier != "" {
		if cluster, clusterExists := m.Clusters[instance.DBClusterIdentifier]; clusterExists {
			members := make([]DBClusterMember, 0, len(cluster.DBClusterMembers))

			for _, member := range cluster.DBClusterMembers {
				if member.DBInstanceIdentifier != identifier {
					members = append(members, member)
				}
			}

			cluster.DBClusterMembers = members
		}
	}

	delete(m.Instances, identifier)

	m.saveLocked()

	return instance, nil
}

// DescribeDBInstances describes Neptune DB instances.
func (m *MemoryStorage) DescribeDBInstances(_ context.Context, identifier string) ([]DBInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if identifier != "" {
		instance, exists := m.Instances[identifier]
		if !exists {
			return nil, &Error{
				Code:    errDBInstanceNotFound,
				Message: fmt.Sprintf("DB instance not found: %s", identifier),
			}
		}

		return []DBInstance{*instance}, nil
	}

	instances := make([]DBInstance, 0, len(m.Instances))
	for _, instance := range m.Instances {
		instances = append(instances, *instance)
	}

	return instances, nil
}

// Helper functions.

func (m *MemoryStorage) dbClusterArn(identifier string) string {
	return fmt.Sprintf("arn:aws:neptune:%s:%s:cluster:%s", defaultRegion, defaultAccountID, identifier)
}

func (m *MemoryStorage) dbInstanceArn(identifier string) string {
	return fmt.Sprintf("arn:aws:neptune:%s:%s:db:%s", defaultRegion, defaultAccountID, identifier)
}

func generateID() string {
	return uuid.New().String()[:8]
}
