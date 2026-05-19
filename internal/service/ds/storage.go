package ds

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Default values.
const defaultRegion = "us-east-1"

// Storage defines the Directory Service storage interface.
type Storage interface {
	CreateDirectory(ctx context.Context, req *CreateDirectoryRequest) (*Directory, error)
	DescribeDirectories(ctx context.Context, directoryIDs []string, limit int, nextToken string) ([]*Directory, string, error)
	DeleteDirectory(ctx context.Context, directoryID string) error
	CreateSnapshot(ctx context.Context, directoryID, name string) (*Snapshot, error)
	DescribeSnapshots(ctx context.Context, directoryID string, snapshotIDs []string, limit int, nextToken string) ([]*Snapshot, string, error)
	DeleteSnapshot(ctx context.Context, snapshotID string) error
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
	mu          sync.RWMutex          `json:"-"`
	Directories map[string]*Directory `json:"directories"`
	Snapshots   map[string]*Snapshot  `json:"snapshots"`
	region      string
	accountID   string
	dataDir     string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Directories: make(map[string]*Directory),
		Snapshots:   make(map[string]*Snapshot),
		region:      region,
		accountID:   "123456789012",
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "ds", s)
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

	if s.Directories == nil {
		s.Directories = make(map[string]*Directory)
	}

	if s.Snapshots == nil {
		s.Snapshots = make(map[string]*Snapshot)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	type alias MemoryStorage

	data, err := json.Marshal(&struct{ *alias }{alias: (*alias)(s)})
	if err != nil {
		return
	}

	_ = storage.SaveBytes(s.dataDir, "ds", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "ds", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateDirectory creates a new directory.
func (s *MemoryStorage) CreateDirectory(_ context.Context, req *CreateDirectoryRequest) (*Directory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate directory name.
	for _, d := range s.Directories {
		if d.Name == req.Name {
			return nil, &Error{
				Type:    ErrEntityAlreadyExists,
				Message: fmt.Sprintf("Directory with name %s already exists", req.Name),
			}
		}
	}

	directoryID := "d-" + uuid.New().String()[:10]

	var vpcSettings *DirectoryVPCSettings
	if req.VPCSettings != nil {
		vpcSettings = &DirectoryVPCSettings{
			VPCID:             req.VPCSettings.VPCID,
			SubnetIDs:         req.VPCSettings.SubnetIDs,
			SecurityGroupID:   "sg-" + uuid.New().String()[:8],
			AvailabilityZones: []string{"us-east-1a", "us-east-1b"},
		}
	}

	shortName := req.ShortName
	if shortName == "" {
		shortName = req.Name
	}

	now := time.Now().UTC()
	directory := &Directory{
		DirectoryID:        directoryID,
		Name:               req.Name,
		ShortName:          shortName,
		Password:           req.Password,
		Description:        req.Description,
		Size:               req.Size,
		Type:               DirectoryTypeSimpleAD,
		Stage:              DirectoryStateActive,
		DNSIPAddrs:         []string{"10.0.0.1", "10.0.0.2"},
		SSOEnabled:         false,
		DesiredNumberOfDCs: 2,
		VPCSettings:        vpcSettings,
		LaunchTime:         now,
		StageLastUpdatedAt: now,
	}

	s.Directories[directoryID] = directory

	s.saveLocked()

	return directory, nil
}

// DescribeDirectories returns directories matching the given IDs.
func (s *MemoryStorage) DescribeDirectories(_ context.Context, directoryIDs []string, limit int, nextToken string) ([]*Directory, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit == 0 {
		limit = 100
	}

	var directories []*Directory

	if len(directoryIDs) > 0 {
		// Return specific directories.
		for _, id := range directoryIDs {
			if d, exists := s.Directories[id]; exists {
				directories = append(directories, d)
			}
		}
	} else {
		// Return all directories.
		for _, d := range s.Directories {
			directories = append(directories, d)
		}
	}

	// Sort by directory ID for consistent pagination.
	sort.Slice(directories, func(i, j int) bool {
		return directories[i].DirectoryID < directories[j].DirectoryID
	})

	// Handle pagination.
	start := 0

	if nextToken != "" {
		for i, d := range directories {
			if d.DirectoryID == nextToken {
				start = i

				break
			}
		}
	}

	end := min(start+limit, len(directories))
	result := directories[start:end]
	newNextToken := ""

	if end < len(directories) {
		newNextToken = directories[end].DirectoryID
	}

	return result, newNextToken, nil
}

// DeleteDirectory deletes a directory.
func (s *MemoryStorage) DeleteDirectory(_ context.Context, directoryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Directories[directoryID]; !exists {
		return &Error{
			Type:    ErrEntityDoesNotExist,
			Message: fmt.Sprintf("Directory %s does not exist", directoryID),
		}
	}

	delete(s.Directories, directoryID)

	s.saveLocked()

	return nil
}

// CreateSnapshot creates a new snapshot for a directory.
func (s *MemoryStorage) CreateSnapshot(_ context.Context, directoryID, name string) (*Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Directories[directoryID]; !exists {
		return nil, &Error{
			Type:    ErrEntityDoesNotExist,
			Message: fmt.Sprintf("Directory %s does not exist", directoryID),
		}
	}

	snapshotID := "s-" + uuid.New().String()[:10]
	now := time.Now().UTC()

	snapshot := &Snapshot{
		SnapshotID:  snapshotID,
		DirectoryID: directoryID,
		Name:        name,
		Type:        SnapshotTypeManual,
		Status:      SnapshotStateCompleted,
		StartTime:   now,
	}

	s.Snapshots[snapshotID] = snapshot

	s.saveLocked()

	return snapshot, nil
}

// DescribeSnapshots returns snapshots matching the given criteria.
func (s *MemoryStorage) DescribeSnapshots(_ context.Context, directoryID string, snapshotIDs []string, limit int, nextToken string) ([]*Snapshot, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit == 0 {
		limit = 100
	}

	var snapshots []*Snapshot

	if len(snapshotIDs) > 0 {
		// Return specific snapshots.
		for _, id := range snapshotIDs {
			snap, exists := s.Snapshots[id]
			if !exists {
				continue
			}

			if directoryID != "" && snap.DirectoryID != directoryID {
				continue
			}

			snapshots = append(snapshots, snap)
		}
	} else {
		// Return all snapshots.
		for _, snap := range s.Snapshots {
			if directoryID != "" && snap.DirectoryID != directoryID {
				continue
			}

			snapshots = append(snapshots, snap)
		}
	}

	// Sort by snapshot ID for consistent pagination.
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].SnapshotID < snapshots[j].SnapshotID
	})

	// Handle pagination.
	start := 0

	if nextToken != "" {
		for i, snap := range snapshots {
			if snap.SnapshotID == nextToken {
				start = i

				break
			}
		}
	}

	end := min(start+limit, len(snapshots))
	result := snapshots[start:end]
	newNextToken := ""

	if end < len(snapshots) {
		newNextToken = snapshots[end].SnapshotID
	}

	return result, newNextToken, nil
}

// DeleteSnapshot deletes a snapshot.
func (s *MemoryStorage) DeleteSnapshot(_ context.Context, snapshotID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Snapshots[snapshotID]; !exists {
		return &Error{
			Type:    ErrEntityDoesNotExist,
			Message: fmt.Sprintf("Snapshot %s does not exist", snapshotID),
		}
	}

	delete(s.Snapshots, snapshotID)

	s.saveLocked()

	return nil
}
