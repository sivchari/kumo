package memorydb

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "000000000000"
)

// ServiceError represents a MemoryDB service error.
type ServiceError struct {
	Code    string
	Message string
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

const (
	errClusterNotFound = "ClusterNotFoundFault"
	errClusterExists   = "ClusterAlreadyExistsFault"
	errUserNotFound    = "UserNotFoundFault"
	errUserExists      = "UserAlreadyExistsFault"
	errACLNotFound     = "ACLNotFoundFault"
	errACLExists       = "ACLAlreadyExistsFault"

	statusDeleting = "deleting"
)

// Storage defines the MemoryDB storage interface.
type Storage interface {
	CreateCluster(ctx context.Context, req *CreateClusterRequest) (*Cluster, error)
	DescribeClusters(ctx context.Context, clusterName string) ([]Cluster, error)
	UpdateCluster(ctx context.Context, req *UpdateClusterRequest) (*Cluster, error)
	DeleteCluster(ctx context.Context, clusterName string) (*Cluster, error)

	CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error)
	DescribeUsers(ctx context.Context, userName string) ([]User, error)
	DeleteUser(ctx context.Context, userName string) (*User, error)

	CreateACL(ctx context.Context, req *CreateACLRequest) (*ACL, error)
	DescribeACLs(ctx context.Context, aclName string) ([]ACL, error)
	DeleteACL(ctx context.Context, aclName string) (*ACL, error)
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
	mu       sync.RWMutex        `json:"-"`
	Clusters map[string]*Cluster `json:"clusters"`
	Users    map[string]*User    `json:"users"`
	Acls     map[string]*ACL     `json:"acls"`
	dataDir  string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Clusters: make(map[string]*Cluster),
		Users:    make(map[string]*User),
		Acls:     make(map[string]*ACL),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "memorydb", s)
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

	if m.Users == nil {
		m.Users = make(map[string]*User)
	}

	if m.Acls == nil {
		m.Acls = make(map[string]*ACL)
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

	_ = storage.SaveBytes(m.dataDir, "memorydb", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "memorydb", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// buildClusterBase creates a Cluster with core fields from a CreateClusterRequest.
func buildClusterBase(req *CreateClusterRequest) *Cluster {
	numShards := int32(1)
	if req.NumShards != nil {
		numShards = *req.NumShards
	}

	port := int32(6379)
	if req.Port != nil {
		port = *req.Port
	}

	engine := "redis"
	if req.Engine != "" {
		engine = req.Engine
	}

	engineVersion := "7.1"
	if req.EngineVersion != "" {
		engineVersion = req.EngineVersion
	}

	return &Cluster{
		ACLName:            req.ACLName,
		ARN:                fmt.Sprintf("arn:aws:memorydb:%s:%s:cluster/%s", defaultRegion, defaultAccountID, req.ClusterName),
		AvailabilityMode:   "singleaz",
		ClusterEndpoint:    &Endpoint{Address: fmt.Sprintf("%s.memorydb.%s.amazonaws.com", req.ClusterName, defaultRegion), Port: port},
		Description:        req.Description,
		Engine:             engine,
		EngineVersion:      engineVersion,
		KmsKeyID:           req.KmsKeyID,
		MaintenanceWindow:  req.MaintenanceWindow,
		Name:               req.ClusterName,
		NodeType:           req.NodeType,
		NumberOfShards:     numShards,
		ParameterGroupName: req.ParameterGroupName,
		Status:             "available",
		SubnetGroupName:    req.SubnetGroupName,
	}
}

// applyOptionalClusterFields sets optional fields on a Cluster from a CreateClusterRequest.
func applyOptionalClusterFields(cluster *Cluster, req *CreateClusterRequest) {
	if req.AutoMinorVersionUpgrade != nil {
		cluster.AutoMinorVersionUpgrade = *req.AutoMinorVersionUpgrade
	}

	if req.TLSEnabled != nil {
		cluster.TLSEnabled = *req.TLSEnabled
	}

	if req.SnapshotRetentionLimit != nil {
		cluster.SnapshotRetentionLimit = *req.SnapshotRetentionLimit
	}

	if req.SnapshotWindow != "" {
		cluster.SnapshotWindow = req.SnapshotWindow
	}

	if len(req.SecurityGroupIDs) > 0 {
		sgs := make([]SecurityGroupMembership, 0, len(req.SecurityGroupIDs))
		for _, sgID := range req.SecurityGroupIDs {
			sgs = append(sgs, SecurityGroupMembership{
				SecurityGroupID: sgID,
				Status:          "active",
			})
		}

		cluster.SecurityGroups = sgs
	}
}

// buildCluster creates a Cluster from a CreateClusterRequest.
func buildCluster(req *CreateClusterRequest) *Cluster {
	cluster := buildClusterBase(req)
	applyOptionalClusterFields(cluster, req)

	return cluster
}

// CreateCluster creates a new cluster.
func (m *MemoryStorage) CreateCluster(_ context.Context, req *CreateClusterRequest) (*Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Clusters[req.ClusterName]; exists {
		return nil, &ServiceError{
			Code:    errClusterExists,
			Message: fmt.Sprintf("Cluster %s already exists", req.ClusterName),
		}
	}

	cluster := buildCluster(req)
	m.Clusters[req.ClusterName] = cluster

	m.saveLocked()

	return cluster, nil
}

// DescribeClusters returns clusters matching the filter.
func (m *MemoryStorage) DescribeClusters(_ context.Context, clusterName string) ([]Cluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if clusterName != "" {
		cluster, exists := m.Clusters[clusterName]
		if !exists {
			return nil, &ServiceError{
				Code:    errClusterNotFound,
				Message: fmt.Sprintf("Cluster %s not found", clusterName),
			}
		}

		return []Cluster{*cluster}, nil
	}

	clusters := make([]Cluster, 0, len(m.Clusters))
	for _, c := range m.Clusters {
		clusters = append(clusters, *c)
	}

	return clusters, nil
}

// UpdateCluster updates an existing cluster.
func (m *MemoryStorage) UpdateCluster(_ context.Context, req *UpdateClusterRequest) (*Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.Clusters[req.ClusterName]
	if !exists {
		return nil, &ServiceError{
			Code:    errClusterNotFound,
			Message: fmt.Sprintf("Cluster %s not found", req.ClusterName),
		}
	}

	applyClusterUpdates(cluster, req)

	m.saveLocked()

	return cluster, nil
}

// applyClusterUpdates applies non-empty fields from the update request to the cluster.
func applyClusterUpdates(cluster *Cluster, req *UpdateClusterRequest) {
	if req.Description != "" {
		cluster.Description = req.Description
	}

	if req.ACLName != "" {
		cluster.ACLName = req.ACLName
	}

	if req.EngineVersion != "" {
		cluster.EngineVersion = req.EngineVersion
	}

	if req.MaintenanceWindow != "" {
		cluster.MaintenanceWindow = req.MaintenanceWindow
	}

	if req.NodeType != "" {
		cluster.NodeType = req.NodeType
	}

	if req.ParameterGroupName != "" {
		cluster.ParameterGroupName = req.ParameterGroupName
	}

	if req.SnapshotRetentionLimit != nil {
		cluster.SnapshotRetentionLimit = *req.SnapshotRetentionLimit
	}

	if req.SnapshotWindow != "" {
		cluster.SnapshotWindow = req.SnapshotWindow
	}

	if req.SnsTopicArn != "" {
		cluster.SnsTopicArn = req.SnsTopicArn
	}

	if len(req.SecurityGroupIDs) > 0 {
		sgs := make([]SecurityGroupMembership, 0, len(req.SecurityGroupIDs))
		for _, sgID := range req.SecurityGroupIDs {
			sgs = append(sgs, SecurityGroupMembership{
				SecurityGroupID: sgID,
				Status:          "active",
			})
		}

		cluster.SecurityGroups = sgs
	}
}

// DeleteCluster deletes a cluster.
func (m *MemoryStorage) DeleteCluster(_ context.Context, clusterName string) (*Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.Clusters[clusterName]
	if !exists {
		return nil, &ServiceError{
			Code:    errClusterNotFound,
			Message: fmt.Sprintf("Cluster %s not found", clusterName),
		}
	}

	cluster.Status = statusDeleting

	delete(m.Clusters, clusterName)

	m.saveLocked()

	return cluster, nil
}

// CreateUser creates a new user.
func (m *MemoryStorage) CreateUser(_ context.Context, req *CreateUserRequest) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Users[req.UserName]; exists {
		return nil, &ServiceError{
			Code:    errUserExists,
			Message: fmt.Sprintf("User %s already exists", req.UserName),
		}
	}

	passwordCount := int32(0)
	authType := "no-password"

	if req.AuthenticationMode != nil {
		if req.AuthenticationMode.Type != "" {
			authType = req.AuthenticationMode.Type
		}

		passwordCount = int32(min(len(req.AuthenticationMode.Passwords), 2)) //nolint:gosec // MemoryDB allows max 2 passwords
	}

	user := &User{
		ARN:          fmt.Sprintf("arn:aws:memorydb:%s:%s:user/%s", defaultRegion, defaultAccountID, req.UserName),
		AccessString: req.AccessString,
		Authentication: &Authentication{
			PasswordCount: passwordCount,
			Type:          authType,
		},
		Name:     req.UserName,
		Status:   "active",
		ACLNames: []string{},
	}

	m.Users[req.UserName] = user

	m.saveLocked()

	return user, nil
}

// DescribeUsers returns users matching the filter.
func (m *MemoryStorage) DescribeUsers(_ context.Context, userName string) ([]User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if userName != "" {
		user, exists := m.Users[userName]
		if !exists {
			return nil, &ServiceError{
				Code:    errUserNotFound,
				Message: fmt.Sprintf("User %s not found", userName),
			}
		}

		return []User{*user}, nil
	}

	users := make([]User, 0, len(m.Users))
	for _, u := range m.Users {
		users = append(users, *u)
	}

	return users, nil
}

// DeleteUser deletes a user.
func (m *MemoryStorage) DeleteUser(_ context.Context, userName string) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.Users[userName]
	if !exists {
		return nil, &ServiceError{
			Code:    errUserNotFound,
			Message: fmt.Sprintf("User %s not found", userName),
		}
	}

	user.Status = statusDeleting

	delete(m.Users, userName)

	m.saveLocked()

	return user, nil
}

// CreateACL creates a new ACL.
func (m *MemoryStorage) CreateACL(_ context.Context, req *CreateACLRequest) (*ACL, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Acls[req.ACLName]; exists {
		return nil, &ServiceError{
			Code:    errACLExists,
			Message: fmt.Sprintf("ACL %s already exists", req.ACLName),
		}
	}

	userNames := req.UserNames
	if userNames == nil {
		userNames = []string{}
	}

	acl := &ACL{
		ARN:                  fmt.Sprintf("arn:aws:memorydb:%s:%s:acl/%s", defaultRegion, defaultAccountID, req.ACLName),
		Clusters:             []string{},
		MinimumEngineVersion: "6.2.6",
		Name:                 req.ACLName,
		Status:               "active",
		UserNames:            userNames,
	}

	m.Acls[req.ACLName] = acl

	m.saveLocked()

	return acl, nil
}

// DescribeACLs returns ACLs matching the filter.
func (m *MemoryStorage) DescribeACLs(_ context.Context, aclName string) ([]ACL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if aclName != "" {
		acl, exists := m.Acls[aclName]
		if !exists {
			return nil, &ServiceError{
				Code:    errACLNotFound,
				Message: fmt.Sprintf("ACL %s not found", aclName),
			}
		}

		return []ACL{*acl}, nil
	}

	acls := make([]ACL, 0, len(m.Acls))
	for _, a := range m.Acls {
		acls = append(acls, *a)
	}

	return acls, nil
}

// DeleteACL deletes an ACL.
func (m *MemoryStorage) DeleteACL(_ context.Context, aclName string) (*ACL, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	acl, exists := m.Acls[aclName]
	if !exists {
		return nil, &ServiceError{
			Code:    errACLNotFound,
			Message: fmt.Sprintf("ACL %s not found", aclName),
		}
	}

	acl.Status = statusDeleting

	delete(m.Acls, aclName)

	m.saveLocked()

	return acl, nil
}
