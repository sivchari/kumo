package eks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	statusCreating = "CREATING"
	statusActive   = "ACTIVE"
	statusDeleting = "DELETING"

	defaultRegion            = "us-east-1"
	defaultKubernetesVersion = "1.29"
	defaultPlatformVersion   = "eks.1"
)

// Storage defines the EKS storage interface.
type Storage interface {
	CreateCluster(ctx context.Context, req *CreateClusterRequest) (*Cluster, error)
	DeleteCluster(ctx context.Context, name string) (*Cluster, error)
	DescribeCluster(ctx context.Context, name string) (*Cluster, error)
	ListClusters(ctx context.Context, maxResults int, nextToken string) ([]string, string, error)
	CreateNodegroup(ctx context.Context, req *CreateNodegroupRequest) (*Nodegroup, error)
	DeleteNodegroup(ctx context.Context, clusterName, nodegroupName string) (*Nodegroup, error)
	DescribeNodegroup(ctx context.Context, clusterName, nodegroupName string) (*Nodegroup, error)
	ListNodegroups(ctx context.Context, clusterName string, maxResults int, nextToken string) ([]string, string, error)
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
	mu         sync.RWMutex                     `json:"-"`
	Clusters   map[string]*Cluster              `json:"clusters"`
	Nodegroups map[string]map[string]*Nodegroup `json:"nodegroups"`
	region     string
	accountID  string
	dataDir    string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Clusters:   make(map[string]*Cluster),
		Nodegroups: make(map[string]map[string]*Nodegroup),
		region:     region,
		accountID:  "123456789012",
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "eks", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (s *MemoryStorage) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(s)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (s *MemoryStorage) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(s)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if s.Clusters == nil {
		s.Clusters = make(map[string]*Cluster)
	}

	if s.Nodegroups == nil {
		s.Nodegroups = make(map[string]map[string]*Nodegroup)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "eks", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "eks", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateCluster creates a new EKS cluster.
func (s *MemoryStorage) CreateCluster(_ context.Context, req *CreateClusterRequest) (*Cluster, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Clusters[req.Name]; exists {
		return nil, &Error{
			Code:    "ResourceInUseException",
			Message: fmt.Sprintf("Cluster already exists with name: %s", req.Name),
		}
	}

	cluster := s.buildCluster(req)
	cluster.Status = statusActive

	s.Clusters[req.Name] = cluster
	s.Nodegroups[req.Name] = make(map[string]*Nodegroup)

	s.saveLocked()

	return cluster, nil
}

// buildCluster builds a Cluster from a CreateClusterRequest.
func (s *MemoryStorage) buildCluster(req *CreateClusterRequest) *Cluster {
	now := NewEpochTime(time.Now())

	version := req.Version
	if version == "" {
		version = defaultKubernetesVersion
	}

	clusterArn := fmt.Sprintf("arn:aws:eks:%s:%s:cluster/%s", s.region, s.accountID, req.Name)
	endpoint := fmt.Sprintf("https://%s.gr7.%s.eks.amazonaws.com", uuid.New().String()[:8], s.region)
	oidcIssuer := fmt.Sprintf("https://oidc.eks.%s.amazonaws.com/id/%s", s.region, uuid.New().String()[:32])
	caData := base64.StdEncoding.EncodeToString([]byte("fake-certificate-authority-data"))

	cluster := &Cluster{
		Name:                    req.Name,
		Arn:                     clusterArn,
		CreatedAt:               &now,
		Version:                 version,
		Endpoint:                endpoint,
		RoleArn:                 req.RoleArn,
		ResourcesVpcConfig:      s.buildVpcConfig(req.ResourcesVpcConfig),
		KubernetesNetworkConfig: &KubernetesNetworkConfig{ServiceIpv4Cidr: "10.100.0.0/16", IPFamily: "ipv4"},
		Identity:                &Identity{Oidc: &OIDC{Issuer: oidcIssuer}},
		Status:                  statusCreating,
		CertificateAuthority:    &Certificate{Data: caData},
		PlatformVersion:         defaultPlatformVersion,
		Tags:                    req.Tags,
		EncryptionConfig:        req.EncryptionConfig,
		Health:                  &ClusterHealth{Issues: []ClusterIssue{}},
		Logging:                 req.Logging,
	}

	if req.AccessConfig != nil {
		cluster.AccessConfig = &AccessConfigResponse{
			BootstrapClusterCreatorAdminPermissions: req.AccessConfig.BootstrapClusterCreatorAdminPermissions,
			AuthenticationMode:                      req.AccessConfig.AuthenticationMode,
		}
	}

	return cluster
}

// buildVpcConfig builds a VpcConfigResponse from a VpcConfigRequest.
func (s *MemoryStorage) buildVpcConfig(req *VpcConfigRequest) *VpcConfigResponse {
	if req == nil {
		return nil
	}

	endpointPublicAccess := true
	endpointPrivateAccess := false

	if req.EndpointPublicAccess != nil {
		endpointPublicAccess = *req.EndpointPublicAccess
	}

	if req.EndpointPrivateAccess != nil {
		endpointPrivateAccess = *req.EndpointPrivateAccess
	}

	publicCidrs := req.PublicAccessCidrs
	if len(publicCidrs) == 0 {
		publicCidrs = []string{"0.0.0.0/0"}
	}

	return &VpcConfigResponse{
		SubnetIDs:              req.SubnetIDs,
		SecurityGroupIDs:       req.SecurityGroupIDs,
		ClusterSecurityGroupID: fmt.Sprintf("sg-%s", uuid.New().String()[:17]),
		VpcID:                  fmt.Sprintf("vpc-%s", uuid.New().String()[:17]),
		EndpointPublicAccess:   endpointPublicAccess,
		EndpointPrivateAccess:  endpointPrivateAccess,
		PublicAccessCidrs:      publicCidrs,
	}
}

// DeleteCluster deletes an EKS cluster.
func (s *MemoryStorage) DeleteCluster(_ context.Context, name string) (*Cluster, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cluster, exists := s.Clusters[name]
	if !exists {
		return nil, &Error{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("No cluster found for name: %s", name),
		}
	}

	// Check if there are any nodegroups.
	if nodegroups, ok := s.Nodegroups[name]; ok && len(nodegroups) > 0 {
		return nil, &Error{
			Code:    "ResourceInUseException",
			Message: "Cluster has nodegroups attached",
		}
	}

	cluster.Status = statusDeleting

	delete(s.Clusters, name)
	delete(s.Nodegroups, name)

	s.saveLocked()

	return cluster, nil
}

// DescribeCluster describes an EKS cluster.
func (s *MemoryStorage) DescribeCluster(_ context.Context, name string) (*Cluster, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cluster, exists := s.Clusters[name]
	if !exists {
		return nil, &Error{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("No cluster found for name: %s", name),
		}
	}

	return cluster, nil
}

// ListClusters lists all EKS clusters.
func (s *MemoryStorage) ListClusters(_ context.Context, _ int, _ string) ([]string, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.Clusters))
	for name := range s.Clusters {
		names = append(names, name)
	}

	return names, "", nil
}

// CreateNodegroup creates a new EKS node group.
func (s *MemoryStorage) CreateNodegroup(_ context.Context, req *CreateNodegroupRequest) (*Nodegroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cluster, exists := s.Clusters[req.ClusterName]
	if !exists {
		return nil, &Error{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("No cluster found for name: %s", req.ClusterName),
		}
	}

	if _, exists := s.Nodegroups[req.ClusterName][req.NodegroupName]; exists {
		return nil, &Error{
			Code:    "ResourceInUseException",
			Message: fmt.Sprintf("Nodegroup already exists with name: %s", req.NodegroupName),
		}
	}

	nodegroup := s.buildNodegroup(req, cluster.Version)
	nodegroup.Status = statusActive

	s.Nodegroups[req.ClusterName][req.NodegroupName] = nodegroup

	s.saveLocked()

	return nodegroup, nil
}

// buildNodegroup builds a Nodegroup from a CreateNodegroupRequest.
func (s *MemoryStorage) buildNodegroup(req *CreateNodegroupRequest, clusterVersion string) *Nodegroup {
	now := NewEpochTime(time.Now())
	nodegroupArn := fmt.Sprintf("arn:aws:eks:%s:%s:nodegroup/%s/%s/%s",
		s.region, s.accountID, req.ClusterName, req.NodegroupName, uuid.New().String()[:8])

	instanceTypes := req.InstanceTypes
	if len(instanceTypes) == 0 {
		instanceTypes = []string{"t3.medium"}
	}

	capacityType := req.CapacityType
	if capacityType == "" {
		capacityType = "ON_DEMAND"
	}

	amiType := req.AmiType
	if amiType == "" {
		amiType = "AL2_x86_64"
	}

	scalingConfig := req.ScalingConfig
	if scalingConfig == nil {
		minSize := 1
		maxSize := 2
		desiredSize := 1
		scalingConfig = &NodegroupScaling{MinSize: &minSize, MaxSize: &maxSize, DesiredSize: &desiredSize}
	}

	return &Nodegroup{
		NodegroupName:  req.NodegroupName,
		NodegroupArn:   nodegroupArn,
		ClusterName:    req.ClusterName,
		Version:        clusterVersion,
		ReleaseVersion: fmt.Sprintf("%s-20231116", clusterVersion),
		CreatedAt:      &now,
		ModifiedAt:     &now,
		Status:         statusCreating,
		CapacityType:   capacityType,
		ScalingConfig:  scalingConfig,
		InstanceTypes:  instanceTypes,
		Subnets:        req.Subnets,
		RemoteAccess:   req.RemoteAccess,
		AmiType:        amiType,
		NodeRole:       req.NodeRole,
		Labels:         req.Labels,
		Taints:         req.Taints,
		DiskSize:       req.DiskSize,
		UpdateConfig:   req.UpdateConfig,
		LaunchTemplate: req.LaunchTemplate,
		Tags:           req.Tags,
		Resources: &NodegroupResources{
			AutoScalingGroups: []AutoScalingGroup{{Name: fmt.Sprintf("eks-%s-%s", req.NodegroupName, uuid.New().String()[:8])}},
		},
		Health: &NodegroupHealth{Issues: []Issue{}},
	}
}

// DeleteNodegroup deletes an EKS node group.
func (s *MemoryStorage) DeleteNodegroup(_ context.Context, clusterName, nodegroupName string) (*Nodegroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Clusters[clusterName]; !exists {
		return nil, &Error{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("No cluster found for name: %s", clusterName),
		}
	}

	nodegroups, exists := s.Nodegroups[clusterName]
	if !exists {
		return nil, &Error{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("No nodegroup found for name: %s", nodegroupName),
		}
	}

	nodegroup, exists := nodegroups[nodegroupName]
	if !exists {
		return nil, &Error{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("No nodegroup found for name: %s", nodegroupName),
		}
	}

	nodegroup.Status = statusDeleting

	delete(s.Nodegroups[clusterName], nodegroupName)

	s.saveLocked()

	return nodegroup, nil
}

// DescribeNodegroup describes an EKS node group.
func (s *MemoryStorage) DescribeNodegroup(_ context.Context, clusterName, nodegroupName string) (*Nodegroup, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.Clusters[clusterName]; !exists {
		return nil, &Error{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("No cluster found for name: %s", clusterName),
		}
	}

	nodegroups, exists := s.Nodegroups[clusterName]
	if !exists {
		return nil, &Error{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("No nodegroup found for name: %s", nodegroupName),
		}
	}

	nodegroup, exists := nodegroups[nodegroupName]
	if !exists {
		return nil, &Error{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("No nodegroup found for name: %s", nodegroupName),
		}
	}

	return nodegroup, nil
}

// ListNodegroups lists all EKS node groups for a cluster.
func (s *MemoryStorage) ListNodegroups(_ context.Context, clusterName string, _ int, _ string) ([]string, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.Clusters[clusterName]; !exists {
		return nil, "", &Error{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("No cluster found for name: %s", clusterName),
		}
	}

	nodegroups, exists := s.Nodegroups[clusterName]
	if !exists {
		return []string{}, "", nil
	}

	names := make([]string, 0, len(nodegroups))
	for name := range nodegroups {
		names = append(names, name)
	}

	return names, "", nil
}
