package elasticache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultAccountID = "000000000000"
	defaultRegion    = "us-east-1"
)

// Storage defines the ElastiCache storage interface.
type Storage interface {
	CreateCacheCluster(ctx context.Context, input *CreateCacheClusterInput) (*CacheCluster, error)
	DeleteCacheCluster(ctx context.Context, clusterID string) (*CacheCluster, error)
	DescribeCacheClusters(ctx context.Context, clusterID string, showNodeInfo bool) ([]CacheCluster, error)
	ModifyCacheCluster(ctx context.Context, input *ModifyCacheClusterInput) (*CacheCluster, error)
	CreateReplicationGroup(ctx context.Context, input *CreateReplicationGroupInput) (*ReplicationGroup, error)
	DeleteReplicationGroup(ctx context.Context, groupID string) (*ReplicationGroup, error)
	DescribeReplicationGroups(ctx context.Context, groupID string) ([]ReplicationGroup, error)
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
	mu                sync.RWMutex                 `json:"-"`
	CacheClusters     map[string]*CacheCluster     `json:"cacheClusters"`
	ReplicationGroups map[string]*ReplicationGroup `json:"replicationGroups"`
	region            string
	dataDir           string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		CacheClusters:     make(map[string]*CacheCluster),
		ReplicationGroups: make(map[string]*ReplicationGroup),
		region:            region,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "elasticache", s)
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

	if m.CacheClusters == nil {
		m.CacheClusters = make(map[string]*CacheCluster)
	}

	if m.ReplicationGroups == nil {
		m.ReplicationGroups = make(map[string]*ReplicationGroup)
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

	_ = storage.SaveBytes(m.dataDir, "elasticache", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "elasticache", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateCacheCluster creates a new cache cluster.
func (m *MemoryStorage) CreateCacheCluster(_ context.Context, input *CreateCacheClusterInput) (*CacheCluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.CacheClusters[input.CacheClusterID]; exists {
		return nil, &Error{
			Code:    errCacheClusterAlreadyExists,
			Message: fmt.Sprintf("Cache cluster already exists: %s", input.CacheClusterID),
		}
	}

	cluster := m.buildCacheCluster(input)
	m.CacheClusters[input.CacheClusterID] = cluster

	m.saveLocked()

	return cluster, nil
}

func (m *MemoryStorage) buildCacheCluster(input *CreateCacheClusterInput) *CacheCluster {
	numNodes := input.NumCacheNodes
	if numNodes == 0 {
		numNodes = 1
	}

	az := input.PreferredAvailabilityZone
	if az == "" {
		az = m.region + "a"
	}

	port := input.Port
	if port == 0 {
		port = m.getDefaultPort(input.Engine)
	}

	now := time.Now()

	cluster := &CacheCluster{
		CacheClusterID:             input.CacheClusterID,
		CacheClusterStatus:         CacheClusterStatusAvailable,
		CacheNodeType:              input.CacheNodeType,
		Engine:                     input.Engine,
		EngineVersion:              input.EngineVersion,
		NumCacheNodes:              numNodes,
		PreferredAvailabilityZone:  az,
		CacheClusterCreateTime:     now,
		PreferredMaintenanceWindow: input.PreferredMaintenanceWindow,
		CacheSubnetGroupName:       input.CacheSubnetGroupName,
		AutoMinorVersionUpgrade:    input.AutoMinorVersionUpgrade,
		SnapshotRetentionLimit:     input.SnapshotRetentionLimit,
		SnapshotWindow:             input.SnapshotWindow,
		ARN:                        m.cacheClusterArn(input.CacheClusterID),
		CacheNodes:                 m.buildCacheNodes(numNodes, az, port, now),
		SecurityGroups:             buildSecurityGroups(input.SecurityGroupIDs),
		ConfigurationEndpoint: &Endpoint{
			Address: fmt.Sprintf("%s.%s.cfg.%s.cache.amazonaws.com", input.CacheClusterID, generateID(), m.region),
			Port:    port,
		},
	}

	return cluster
}

func (m *MemoryStorage) buildCacheNodes(numNodes int32, az string, port int32, createTime time.Time) []CacheNode {
	nodes := make([]CacheNode, 0, numNodes)

	for i := range numNodes {
		nodeID := fmt.Sprintf("%04d", i+1)
		nodes = append(nodes, CacheNode{
			CacheNodeID:              nodeID,
			CacheNodeStatus:          CacheClusterStatusAvailable,
			CacheNodeCreateTime:      createTime,
			CustomerAvailabilityZone: az,
			ParameterGroupStatus:     "in-sync",
			Endpoint: &Endpoint{
				Address: fmt.Sprintf("%s.%s.%s.cache.amazonaws.com", nodeID, generateID(), m.region),
				Port:    port,
			},
		})
	}

	return nodes
}

// DeleteCacheCluster deletes a cache cluster.
func (m *MemoryStorage) DeleteCacheCluster(_ context.Context, clusterID string) (*CacheCluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.CacheClusters[clusterID]
	if !exists {
		return nil, &Error{
			Code:    errCacheClusterNotFound,
			Message: fmt.Sprintf("Cache cluster not found: %s", clusterID),
		}
	}

	cluster.CacheClusterStatus = CacheClusterStatusDeleting

	delete(m.CacheClusters, clusterID)

	m.saveLocked()

	return cluster, nil
}

// DescribeCacheClusters describes cache clusters.
func (m *MemoryStorage) DescribeCacheClusters(_ context.Context, clusterID string, showNodeInfo bool) ([]CacheCluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if clusterID != "" {
		cluster, exists := m.CacheClusters[clusterID]
		if !exists {
			return nil, &Error{
				Code:    errCacheClusterNotFound,
				Message: fmt.Sprintf("Cache cluster not found: %s", clusterID),
			}
		}

		result := *cluster
		if !showNodeInfo {
			result.CacheNodes = nil
		}

		return []CacheCluster{result}, nil
	}

	clusters := make([]CacheCluster, 0, len(m.CacheClusters))

	for _, cluster := range m.CacheClusters {
		result := *cluster
		if !showNodeInfo {
			result.CacheNodes = nil
		}

		clusters = append(clusters, result)
	}

	return clusters, nil
}

// ModifyCacheCluster modifies a cache cluster.
func (m *MemoryStorage) ModifyCacheCluster(_ context.Context, input *ModifyCacheClusterInput) (*CacheCluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.CacheClusters[input.CacheClusterID]
	if !exists {
		return nil, &Error{
			Code:    errCacheClusterNotFound,
			Message: fmt.Sprintf("Cache cluster not found: %s", input.CacheClusterID),
		}
	}

	applyCacheClusterModifications(cluster, input)

	m.saveLocked()

	return cluster, nil
}

func applyCacheClusterModifications(cluster *CacheCluster, input *ModifyCacheClusterInput) {
	if input.CacheNodeType != "" {
		cluster.CacheNodeType = input.CacheNodeType
	}

	if input.EngineVersion != "" {
		cluster.EngineVersion = input.EngineVersion
	}

	if input.NumCacheNodes != nil {
		cluster.NumCacheNodes = *input.NumCacheNodes
	}

	if input.PreferredMaintenanceWindow != "" {
		cluster.PreferredMaintenanceWindow = input.PreferredMaintenanceWindow
	}

	if input.AutoMinorVersionUpgrade != nil {
		cluster.AutoMinorVersionUpgrade = *input.AutoMinorVersionUpgrade
	}

	if input.SnapshotRetentionLimit != nil {
		cluster.SnapshotRetentionLimit = *input.SnapshotRetentionLimit
	}

	if input.SnapshotWindow != "" {
		cluster.SnapshotWindow = input.SnapshotWindow
	}

	if len(input.SecurityGroupIDs) > 0 {
		cluster.SecurityGroups = buildSecurityGroups(input.SecurityGroupIDs)
	}
}

// CreateReplicationGroup creates a new replication group.
func (m *MemoryStorage) CreateReplicationGroup(_ context.Context, input *CreateReplicationGroupInput) (*ReplicationGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.ReplicationGroups[input.ReplicationGroupID]; exists {
		return nil, &Error{
			Code:    errReplicationGroupAlreadyExists,
			Message: fmt.Sprintf("Replication group already exists: %s", input.ReplicationGroupID),
		}
	}

	group := m.buildReplicationGroup(input)
	m.ReplicationGroups[input.ReplicationGroupID] = group

	m.saveLocked()

	return group, nil
}

func (m *MemoryStorage) buildReplicationGroup(input *CreateReplicationGroupInput) *ReplicationGroup {
	port := input.Port
	if port == 0 {
		port = m.getDefaultPort(input.Engine)
	}

	now := time.Now()

	group := &ReplicationGroup{
		ReplicationGroupID:         input.ReplicationGroupID,
		Description:                input.ReplicationGroupDescription,
		Status:                     ReplicationGroupStatusAvailable,
		CacheNodeType:              input.CacheNodeType,
		AutomaticFailover:          automaticFailoverStatus(input.AutomaticFailoverEnabled),
		MultiAZ:                    multiAZStatus(input.MultiAZEnabled),
		SnapshotRetentionLimit:     input.SnapshotRetentionLimit,
		SnapshotWindow:             input.SnapshotWindow,
		ClusterEnabled:             input.NumNodeGroups > 0,
		TransitEncryptionEnabled:   input.TransitEncryptionEnabled,
		AtRestEncryptionEnabled:    input.AtRestEncryptionEnabled,
		ARN:                        m.replicationGroupArn(input.ReplicationGroupID),
		ReplicationGroupCreateTime: now,
		AutoMinorVersionUpgrade:    input.AutoMinorVersionUpgrade,
		PreferredMaintenanceWindow: input.PreferredMaintenanceWindow,
		ConfigurationEndpoint: &Endpoint{
			Address: fmt.Sprintf("%s.%s.clustercfg.%s.cache.amazonaws.com", input.ReplicationGroupID, generateID(), m.region),
			Port:    port,
		},
		NodeGroups: m.buildNodeGroups(input, port),
	}

	return group
}

func (m *MemoryStorage) buildNodeGroups(input *CreateReplicationGroupInput, port int32) []NodeGroup {
	numGroups := input.NumNodeGroups
	if numGroups == 0 {
		numGroups = 1
	}

	replicas := input.ReplicasPerNodeGroup

	groups := make([]NodeGroup, 0, numGroups)

	for i := range numGroups {
		groupID := fmt.Sprintf("%04d", i+1)
		nodeGroup := NodeGroup{
			NodeGroupID: groupID,
			Status:      ReplicationGroupStatusAvailable,
			PrimaryEndpoint: &Endpoint{
				Address: fmt.Sprintf("%s-%s.%s.%s.cache.amazonaws.com", input.ReplicationGroupID, groupID, generateID(), m.region),
				Port:    port,
			},
			ReaderEndpoint: &Endpoint{
				Address: fmt.Sprintf("%s-%s-ro.%s.%s.cache.amazonaws.com", input.ReplicationGroupID, groupID, generateID(), m.region),
				Port:    port,
			},
			NodeGroupMembers: m.buildNodeGroupMembers(input.ReplicationGroupID, groupID, replicas, port),
		}
		groups = append(groups, nodeGroup)
	}

	return groups
}

func (m *MemoryStorage) buildNodeGroupMembers(rgID, ngID string, replicas, port int32) []NodeGroupMember {
	// Primary + replicas
	totalNodes := 1 + replicas
	members := make([]NodeGroupMember, 0, totalNodes)

	for i := range totalNodes {
		role := "replica"
		if i == 0 {
			role = "primary"
		}

		nodeID := fmt.Sprintf("%04d", i+1)
		clusterID := fmt.Sprintf("%s-%s-%s", rgID, ngID, nodeID)

		members = append(members, NodeGroupMember{
			CacheClusterID:            clusterID,
			CacheNodeID:               nodeID,
			PreferredAvailabilityZone: m.region + "a",
			CurrentRole:               role,
			ReadEndpoint: &Endpoint{
				Address: fmt.Sprintf("%s.%s.%s.cache.amazonaws.com", clusterID, generateID(), m.region),
				Port:    port,
			},
		})
	}

	return members
}

// DeleteReplicationGroup deletes a replication group.
func (m *MemoryStorage) DeleteReplicationGroup(_ context.Context, groupID string) (*ReplicationGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, exists := m.ReplicationGroups[groupID]
	if !exists {
		return nil, &Error{
			Code:    errReplicationGroupNotFound,
			Message: fmt.Sprintf("Replication group not found: %s", groupID),
		}
	}

	group.Status = ReplicationGroupStatusDeleting

	delete(m.ReplicationGroups, groupID)

	m.saveLocked()

	return group, nil
}

// DescribeReplicationGroups describes replication groups.
func (m *MemoryStorage) DescribeReplicationGroups(_ context.Context, groupID string) ([]ReplicationGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if groupID != "" {
		group, exists := m.ReplicationGroups[groupID]
		if !exists {
			return nil, &Error{
				Code:    errReplicationGroupNotFound,
				Message: fmt.Sprintf("Replication group not found: %s", groupID),
			}
		}

		return []ReplicationGroup{*group}, nil
	}

	groups := make([]ReplicationGroup, 0, len(m.ReplicationGroups))
	for _, group := range m.ReplicationGroups {
		groups = append(groups, *group)
	}

	return groups, nil
}

// Helper functions.

func (m *MemoryStorage) cacheClusterArn(clusterID string) string {
	return fmt.Sprintf("arn:aws:elasticache:%s:%s:cluster:%s", m.region, defaultAccountID, clusterID)
}

func (m *MemoryStorage) replicationGroupArn(groupID string) string {
	return fmt.Sprintf("arn:aws:elasticache:%s:%s:replicationgroup:%s", m.region, defaultAccountID, groupID)
}

func (m *MemoryStorage) getDefaultPort(engine string) int32 {
	switch engine {
	case "redis", "valkey":
		return 6379
	case "memcached":
		return 11211
	default:
		return 6379
	}
}

func generateID() string {
	return uuid.New().String()[:8]
}

func buildSecurityGroups(sgIDs []string) []SecurityGroupMembership {
	if len(sgIDs) == 0 {
		return nil
	}

	groups := make([]SecurityGroupMembership, 0, len(sgIDs))
	for _, sgID := range sgIDs {
		groups = append(groups, SecurityGroupMembership{
			SecurityGroupID: sgID,
			Status:          "active",
		})
	}

	return groups
}

func automaticFailoverStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}

	return "disabled"
}

func multiAZStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}

	return "disabled"
}
