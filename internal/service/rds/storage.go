package rds

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
	defaultAccountID = "000000000000"
	defaultRegion    = "us-east-1"
)

// Storage defines the RDS storage interface.
type Storage interface {
	CreateDBInstance(ctx context.Context, input *CreateDBInstanceInput) (*DBInstance, error)
	DeleteDBInstance(ctx context.Context, identifier string, skipFinalSnapshot bool) (*DBInstance, error)
	DescribeDBInstances(ctx context.Context, identifier string) ([]DBInstance, error)
	ModifyDBInstance(ctx context.Context, input *ModifyDBInstanceInput) (*DBInstance, error)
	StartDBInstance(ctx context.Context, identifier string) (*DBInstance, error)
	StopDBInstance(ctx context.Context, identifier string) (*DBInstance, error)
	CreateDBCluster(ctx context.Context, input *CreateDBClusterInput) (*DBCluster, error)
	DeleteDBCluster(ctx context.Context, identifier string, skipFinalSnapshot bool) (*DBCluster, error)
	DescribeDBClusters(ctx context.Context, identifier string) ([]DBCluster, error)
	ModifyDBCluster(ctx context.Context, input *ModifyDBClusterInput) (*DBCluster, error)
	CreateDBSnapshot(ctx context.Context, input *CreateDBSnapshotInput) (*DBSnapshot, error)
	DeleteDBSnapshot(ctx context.Context, identifier string) (*DBSnapshot, error)
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
	Instances map[string]*DBInstance `json:"instances"`
	Clusters  map[string]*DBCluster  `json:"clusters"`
	Snapshots map[string]*DBSnapshot `json:"snapshots"`
	dataDir   string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Instances: make(map[string]*DBInstance),
		Clusters:  make(map[string]*DBCluster),
		Snapshots: make(map[string]*DBSnapshot),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "rds", s)
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

	if m.Instances == nil {
		m.Instances = make(map[string]*DBInstance)
	}

	if m.Clusters == nil {
		m.Clusters = make(map[string]*DBCluster)
	}

	if m.Snapshots == nil {
		m.Snapshots = make(map[string]*DBSnapshot)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "rds", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "rds", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateDBInstance creates a new DB instance.
func (m *MemoryStorage) CreateDBInstance(_ context.Context, input *CreateDBInstanceInput) (*DBInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Instances[input.DBInstanceIdentifier]; exists {
		return nil, &Error{
			Code:    errDBInstanceAlreadyExists,
			Message: fmt.Sprintf("DB instance already exists: %s", input.DBInstanceIdentifier),
		}
	}

	instance := m.buildDBInstance(input)
	m.Instances[input.DBInstanceIdentifier] = instance

	m.saveLocked()

	return instance, nil
}

func (m *MemoryStorage) buildDBInstance(input *CreateDBInstanceInput) *DBInstance {
	storageType := input.StorageType
	if storageType == "" {
		storageType = "gp2"
	}

	allocatedStorage := input.AllocatedStorage
	if allocatedStorage == 0 {
		allocatedStorage = 20
	}

	availabilityZone := input.AvailabilityZone
	if availabilityZone == "" {
		availabilityZone = defaultRegion + "a"
	}

	endpointAddr, endpointPort := endpointFor(input.Engine, input.DBInstanceIdentifier, m.getDefaultPort(input.Engine))

	instance := &DBInstance{
		DBInstanceIdentifier:       input.DBInstanceIdentifier,
		DBInstanceClass:            input.DBInstanceClass,
		Engine:                     input.Engine,
		EngineVersion:              input.EngineVersion,
		DBInstanceStatus:           DBInstanceStatusAvailable,
		MasterUsername:             input.MasterUsername,
		DBName:                     input.DBName,
		AllocatedStorage:           allocatedStorage,
		InstanceCreateTime:         time.Now(),
		DBInstanceArn:              m.dbInstanceArn(input.DBInstanceIdentifier),
		StorageType:                storageType,
		MultiAZ:                    input.MultiAZ,
		AvailabilityZone:           availabilityZone,
		BackupRetentionPeriod:      input.BackupRetentionPeriod,
		PreferredBackupWindow:      input.PreferredBackupWindow,
		PreferredMaintenanceWindow: input.PreferredMaintenanceWindow,
		PubliclyAccessible:         input.PubliclyAccessible,
		StorageEncrypted:           input.StorageEncrypted,
		DeletionProtection:         input.DeletionProtection,
		Tags:                       input.Tags,
		VpcSecurityGroups:          buildVpcSecurityGroups(input.VpcSecurityGroupIDs),
		Endpoint:                   &Endpoint{Address: endpointAddr, Port: endpointPort},
	}

	return instance
}

// DeleteDBInstance deletes a DB instance.
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

	delete(m.Instances, identifier)

	m.saveLocked()

	return instance, nil
}

// DescribeDBInstances describes DB instances.
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

// ModifyDBInstance modifies a DB instance.
func (m *MemoryStorage) ModifyDBInstance(_ context.Context, input *ModifyDBInstanceInput) (*DBInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.Instances[input.DBInstanceIdentifier]
	if !exists {
		return nil, &Error{
			Code:    errDBInstanceNotFound,
			Message: fmt.Sprintf("DB instance not found: %s", input.DBInstanceIdentifier),
		}
	}

	applyDBInstanceModifications(instance, input)

	m.saveLocked()

	return instance, nil
}

func applyDBInstanceModifications(instance *DBInstance, input *ModifyDBInstanceInput) {
	if input.DBInstanceClass != "" {
		instance.DBInstanceClass = input.DBInstanceClass
	}

	if input.AllocatedStorage > 0 {
		instance.AllocatedStorage = input.AllocatedStorage
	}

	if input.BackupRetentionPeriod != nil {
		instance.BackupRetentionPeriod = *input.BackupRetentionPeriod
	}

	if input.PreferredBackupWindow != "" {
		instance.PreferredBackupWindow = input.PreferredBackupWindow
	}

	if input.PreferredMaintenanceWindow != "" {
		instance.PreferredMaintenanceWindow = input.PreferredMaintenanceWindow
	}

	if input.MultiAZ != nil {
		instance.MultiAZ = *input.MultiAZ
	}

	if input.EngineVersion != "" {
		instance.EngineVersion = input.EngineVersion
	}

	if input.StorageType != "" {
		instance.StorageType = input.StorageType
	}

	if input.PubliclyAccessible != nil {
		instance.PubliclyAccessible = *input.PubliclyAccessible
	}

	if input.DeletionProtection != nil {
		instance.DeletionProtection = *input.DeletionProtection
	}

	if len(input.VpcSecurityGroupIDs) > 0 {
		instance.VpcSecurityGroups = buildVpcSecurityGroups(input.VpcSecurityGroupIDs)
	}
}

// StartDBInstance starts a stopped DB instance.
func (m *MemoryStorage) StartDBInstance(_ context.Context, identifier string) (*DBInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.Instances[identifier]
	if !exists {
		return nil, &Error{
			Code:    errDBInstanceNotFound,
			Message: fmt.Sprintf("DB instance not found: %s", identifier),
		}
	}

	if instance.DBInstanceStatus != DBInstanceStatusStopped {
		return nil, &Error{
			Code:    errInvalidDBInstanceState,
			Message: fmt.Sprintf("DB instance is not in stopped state: %s", identifier),
		}
	}

	instance.DBInstanceStatus = DBInstanceStatusAvailable

	m.saveLocked()

	return instance, nil
}

// StopDBInstance stops a running DB instance.
func (m *MemoryStorage) StopDBInstance(_ context.Context, identifier string) (*DBInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.Instances[identifier]
	if !exists {
		return nil, &Error{
			Code:    errDBInstanceNotFound,
			Message: fmt.Sprintf("DB instance not found: %s", identifier),
		}
	}

	if instance.DBInstanceStatus != DBInstanceStatusAvailable {
		return nil, &Error{
			Code:    errInvalidDBInstanceState,
			Message: fmt.Sprintf("DB instance is not in available state: %s", identifier),
		}
	}

	instance.DBInstanceStatus = DBInstanceStatusStopped

	m.saveLocked()

	return instance, nil
}

// CreateDBCluster creates a new DB cluster.
func (m *MemoryStorage) CreateDBCluster(_ context.Context, input *CreateDBClusterInput) (*DBCluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Clusters[input.DBClusterIdentifier]; exists {
		return nil, &Error{
			Code:    errDBClusterAlreadyExists,
			Message: fmt.Sprintf("DB cluster already exists: %s", input.DBClusterIdentifier),
		}
	}

	now := time.Now()

	port := input.Port
	if port == 0 {
		port = m.getDefaultPort(input.Engine)
	}

	clusterAddr, clusterPort := endpointFor(input.Engine, input.DBClusterIdentifier, port)
	readerAddr, _ := endpointFor(input.Engine, input.DBClusterIdentifier+"-ro", port)

	cluster := &DBCluster{
		DBClusterIdentifier: input.DBClusterIdentifier,
		DBClusterArn:        m.dbClusterArn(input.DBClusterIdentifier),
		Engine:              input.Engine,
		EngineVersion:       input.EngineVersion,
		Status:              DBClusterStatusAvailable,
		MasterUsername:      input.MasterUsername,
		DatabaseName:        input.DatabaseName,
		Endpoint:            clusterAddr,
		ReaderEndpoint:      readerAddr,
		Port:                clusterPort,
		AllocatedStorage:    input.AllocatedStorage,
		ClusterCreateTime:   now,
		AvailabilityZones:   input.AvailabilityZones,
		StorageEncrypted:    input.StorageEncrypted,
		DeletionProtection:  input.DeletionProtection,
		Tags:                input.Tags,
	}

	if len(input.AvailabilityZones) == 0 {
		cluster.AvailabilityZones = []string{defaultRegion + "a", defaultRegion + "b", defaultRegion + "c"}
	}

	if len(input.VpcSecurityGroupIDs) > 0 {
		for _, sgID := range input.VpcSecurityGroupIDs {
			cluster.VpcSecurityGroups = append(cluster.VpcSecurityGroups, VpcSecurityGroupMembership{
				VpcSecurityGroupID: sgID,
				Status:             "active",
			})
		}
	}

	m.Clusters[input.DBClusterIdentifier] = cluster

	m.saveLocked()

	return cluster, nil
}

// DeleteDBCluster deletes a DB cluster.
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

// DescribeDBClusters describes DB clusters.
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

// ModifyDBCluster modifies a DB cluster.
func (m *MemoryStorage) ModifyDBCluster(_ context.Context, input *ModifyDBClusterInput) (*DBCluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.Clusters[input.DBClusterIdentifier]
	if !exists {
		return nil, &Error{
			Code:    errDBClusterNotFound,
			Message: fmt.Sprintf("DB cluster not found: %s", input.DBClusterIdentifier),
		}
	}

	if input.EngineVersion != "" {
		cluster.EngineVersion = input.EngineVersion
	}

	if input.Port != nil {
		cluster.Port = *input.Port
	}

	if input.DeletionProtection != nil {
		cluster.DeletionProtection = *input.DeletionProtection
	}

	if len(input.VpcSecurityGroupIDs) > 0 {
		cluster.VpcSecurityGroups = buildVpcSecurityGroups(input.VpcSecurityGroupIDs)
	}

	m.saveLocked()

	return cluster, nil
}

// CreateDBSnapshot creates a DB snapshot.
func (m *MemoryStorage) CreateDBSnapshot(_ context.Context, input *CreateDBSnapshotInput) (*DBSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Snapshots[input.DBSnapshotIdentifier]; exists {
		return nil, &Error{
			Code:    errDBSnapshotAlreadyExists,
			Message: fmt.Sprintf("DB snapshot already exists: %s", input.DBSnapshotIdentifier),
		}
	}

	instance, exists := m.Instances[input.DBInstanceIdentifier]
	if !exists {
		return nil, &Error{
			Code:    errDBInstanceNotFound,
			Message: fmt.Sprintf("DB instance not found: %s", input.DBInstanceIdentifier),
		}
	}

	now := time.Now()
	snapshot := &DBSnapshot{
		DBSnapshotIdentifier: input.DBSnapshotIdentifier,
		DBSnapshotArn:        m.dbSnapshotArn(input.DBSnapshotIdentifier),
		DBInstanceIdentifier: input.DBInstanceIdentifier,
		Engine:               instance.Engine,
		EngineVersion:        instance.EngineVersion,
		Status:               DBSnapshotStatusAvailable,
		SnapshotType:         "manual",
		SnapshotCreateTime:   now,
		AllocatedStorage:     instance.AllocatedStorage,
		Port:                 instance.Endpoint.Port,
		AvailabilityZone:     instance.AvailabilityZone,
		MasterUsername:       instance.MasterUsername,
		StorageType:          instance.StorageType,
		Encrypted:            instance.StorageEncrypted,
		Tags:                 input.Tags,
	}

	m.Snapshots[input.DBSnapshotIdentifier] = snapshot

	m.saveLocked()

	return snapshot, nil
}

// DeleteDBSnapshot deletes a DB snapshot.
func (m *MemoryStorage) DeleteDBSnapshot(_ context.Context, identifier string) (*DBSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshot, exists := m.Snapshots[identifier]
	if !exists {
		return nil, &Error{
			Code:    errDBSnapshotNotFound,
			Message: fmt.Sprintf("DB snapshot not found: %s", identifier),
		}
	}

	delete(m.Snapshots, identifier)

	m.saveLocked()

	return snapshot, nil
}

// Helper functions.

func (m *MemoryStorage) dbInstanceArn(identifier string) string {
	return fmt.Sprintf("arn:aws:rds:%s:%s:db:%s", defaultRegion, defaultAccountID, identifier)
}

func (m *MemoryStorage) dbClusterArn(identifier string) string {
	return fmt.Sprintf("arn:aws:rds:%s:%s:cluster:%s", defaultRegion, defaultAccountID, identifier)
}

func (m *MemoryStorage) dbSnapshotArn(identifier string) string {
	return fmt.Sprintf("arn:aws:rds:%s:%s:snapshot:%s", defaultRegion, defaultAccountID, identifier)
}

func (m *MemoryStorage) getDefaultPort(engine string) int32 {
	switch engine {
	case "mysql", "mariadb", "aurora", "aurora-mysql":
		return 3306
	case "postgres", "aurora-postgresql":
		return 5432
	case "oracle-ee", "oracle-se", "oracle-se1", "oracle-se2":
		return 1521
	case "sqlserver-ee", "sqlserver-se", "sqlserver-ex", "sqlserver-web":
		return 1433
	default:
		return 3306
	}
}

func generateID() string {
	return uuid.New().String()[:8]
}

func buildVpcSecurityGroups(sgIDs []string) []VpcSecurityGroupMembership {
	if len(sgIDs) == 0 {
		return nil
	}

	groups := make([]VpcSecurityGroupMembership, 0, len(sgIDs))
	for _, sgID := range sgIDs {
		groups = append(groups, VpcSecurityGroupMembership{
			VpcSecurityGroupID: sgID,
			Status:             "active",
		})
	}

	return groups
}
