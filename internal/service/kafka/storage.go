package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	statusActive   = "ACTIVE"
	statusDeleting = "DELETING"

	defaultRegion       = "us-east-1"
	defaultKafkaVersion = "3.6.0"
	defaultInstanceType = "kafka.m5.large"
	defaultVolumeSize   = 100
)

// Error codes.
const (
	errBadRequest    = "BadRequestException"
	errNotFound      = "NotFoundException"
	errConflict      = "ConflictException"
	errInternalError = "InternalServerErrorException"
)

// Storage defines the MSK storage interface.
type Storage interface {
	CreateCluster(ctx context.Context, req *CreateClusterRequest) (*CreateClusterResponse, error)
	DescribeCluster(ctx context.Context, clusterArn string) (*ClusterInfo, error)
	DeleteCluster(ctx context.Context, clusterArn string) (*DeleteClusterResponse, error)
	ListClusters(ctx context.Context, maxResults int, nextToken string) ([]ClusterInfo, string, error)
	GetBootstrapBrokers(ctx context.Context, clusterArn string) (*GetBootstrapBrokersResponse, error)
	UpdateClusterConfiguration(ctx context.Context, clusterArn string, req *UpdateClusterConfigurationRequest) (*UpdateClusterConfigurationResponse, error)
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
	mu        sync.RWMutex            `json:"-"`
	Clusters  map[string]*ClusterInfo `json:"clusters"` // keyed by clusterArn
	region    string
	accountID string
	dataDir   string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Clusters:  make(map[string]*ClusterInfo),
		region:    region,
		accountID: "123456789012",
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "kafka", s)
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
		s.Clusters = make(map[string]*ClusterInfo)
	}

	return nil
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "kafka", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateCluster creates a new MSK cluster.
func (s *MemoryStorage) CreateCluster(_ context.Context, req *CreateClusterRequest) (*CreateClusterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate cluster name.
	for _, c := range s.Clusters {
		if c.ClusterName == req.ClusterName {
			return nil, &Error{
				Code:    errConflict,
				Message: fmt.Sprintf("Cluster name %s already exists", req.ClusterName),
			}
		}
	}

	clusterUUID := uuid.New().String()
	clusterArn := fmt.Sprintf("arn:aws:kafka:%s:%s:cluster/%s/%s", s.region, s.accountID, req.ClusterName, clusterUUID)

	kafkaVersion := req.KafkaVersion
	if kafkaVersion == "" {
		kafkaVersion = defaultKafkaVersion
	}

	numberOfBrokerNodes := req.NumberOfBrokerNodes
	if numberOfBrokerNodes == 0 {
		numberOfBrokerNodes = 3
	}

	brokerNodeGroupInfo := req.BrokerNodeGroupInfo
	if brokerNodeGroupInfo != nil && brokerNodeGroupInfo.InstanceType == "" {
		brokerNodeGroupInfo.InstanceType = defaultInstanceType
	}

	if brokerNodeGroupInfo != nil && brokerNodeGroupInfo.StorageInfo == nil {
		brokerNodeGroupInfo.StorageInfo = &StorageInfo{
			EBSStorageInfo: &EBSStorageInfo{
				VolumeSize: defaultVolumeSize,
			},
		}
	}

	cluster := &ClusterInfo{
		ClusterArn:     clusterArn,
		ClusterName:    req.ClusterName,
		CreationTime:   time.Now().UTC().Format(time.RFC3339),
		CurrentVersion: "K1" + clusterUUID[:8],
		State:          statusActive,
		CurrentBrokerSoftwareInfo: &BrokerSoftwareInfo{
			KafkaVersion: kafkaVersion,
		},
		NumberOfBrokerNodes: numberOfBrokerNodes,
		BrokerNodeGroupInfo: brokerNodeGroupInfo,
		EncryptionInfo:      req.EncryptionInfo,
		Tags:                req.Tags,
	}

	s.Clusters[clusterArn] = cluster

	return &CreateClusterResponse{
		ClusterArn:  clusterArn,
		ClusterName: req.ClusterName,
		State:       statusActive,
	}, nil
}

// DescribeCluster describes an MSK cluster.
func (s *MemoryStorage) DescribeCluster(_ context.Context, clusterArn string) (*ClusterInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cluster, exists := s.Clusters[clusterArn]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Cluster %s not found", clusterArn),
		}
	}

	return cluster, nil
}

// DeleteCluster deletes an MSK cluster.
func (s *MemoryStorage) DeleteCluster(_ context.Context, clusterArn string) (*DeleteClusterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cluster, exists := s.Clusters[clusterArn]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Cluster %s not found", clusterArn),
		}
	}

	cluster.State = statusDeleting

	delete(s.Clusters, clusterArn)

	return &DeleteClusterResponse{
		ClusterArn: clusterArn,
		State:      statusDeleting,
	}, nil
}

// ListClusters lists all MSK clusters.
func (s *MemoryStorage) ListClusters(_ context.Context, _ int, _ string) ([]ClusterInfo, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clusters := make([]ClusterInfo, 0, len(s.Clusters))
	for _, c := range s.Clusters {
		clusters = append(clusters, *c)
	}

	return clusters, "", nil
}

// GetBootstrapBrokers returns bootstrap broker connection strings.
func (s *MemoryStorage) GetBootstrapBrokers(_ context.Context, clusterArn string) (*GetBootstrapBrokersResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cluster, exists := s.Clusters[clusterArn]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Cluster %s not found", clusterArn),
		}
	}

	brokers := make([]string, 0, cluster.NumberOfBrokerNodes)
	brokersTLS := make([]string, 0, cluster.NumberOfBrokerNodes)

	for i := range cluster.NumberOfBrokerNodes {
		brokerHost := fmt.Sprintf("b-%d.%s.%s.kafka.%s.amazonaws.com",
			i+1, cluster.ClusterName, uuid.New().String()[:8], s.region)
		brokers = append(brokers, fmt.Sprintf("%s:9092", brokerHost))
		brokersTLS = append(brokersTLS, fmt.Sprintf("%s:9094", brokerHost))
	}

	return &GetBootstrapBrokersResponse{
		BootstrapBrokerString:    strings.Join(brokers, ","),
		BootstrapBrokerStringTLS: strings.Join(brokersTLS, ","),
	}, nil
}

// UpdateClusterConfiguration updates the configuration of an MSK cluster.
func (s *MemoryStorage) UpdateClusterConfiguration(_ context.Context, clusterArn string, req *UpdateClusterConfigurationRequest) (*UpdateClusterConfigurationResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cluster, exists := s.Clusters[clusterArn]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Cluster %s not found", clusterArn),
		}
	}

	if cluster.CurrentVersion != req.CurrentVersion {
		return nil, &Error{
			Code:    errBadRequest,
			Message: fmt.Sprintf("Current version %s does not match cluster version %s", req.CurrentVersion, cluster.CurrentVersion),
		}
	}

	// Update version.
	cluster.CurrentVersion = "K2" + uuid.New().String()[:8]

	operationArn := fmt.Sprintf("arn:aws:kafka:%s:%s:cluster-operation/%s/%s",
		s.region, s.accountID, cluster.ClusterName, uuid.New().String())

	return &UpdateClusterConfigurationResponse{
		ClusterArn:          clusterArn,
		ClusterOperationArn: operationArn,
	}, nil
}
