package ebs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultAccountID = "000000000000"
	defaultBlockSize = 524288 // 512 KiB

	errSnapshotNotFound = "ResourceNotFoundException"
)

// ServiceError represents an EBS service error.
type ServiceError struct {
	Code    string
	Message string
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Storage defines the EBS storage interface.
type Storage interface {
	StartSnapshot(ctx context.Context, req *StartSnapshotRequest) (*Snapshot, error)
	CompleteSnapshot(ctx context.Context, snapshotID string) (*Snapshot, error)
	ListSnapshotBlocks(ctx context.Context, snapshotID string) (*ListSnapshotBlocksResponse, error)
	PutSnapshotBlock(ctx context.Context, snapshotID string, blockIndex int32, data []byte, checksum string) error
	GetSnapshotBlock(ctx context.Context, snapshotID string, blockIndex int32) ([]byte, string, error)
	ListChangedBlocks(ctx context.Context, firstSnapshotID, secondSnapshotID string) (*ListChangedBlocksResponse, error)
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

// blockData stores the raw data and checksum for a single snapshot block.
type blockData struct {
	Data     []byte `json:"data"`
	Checksum string `json:"checksum"`
}

// MemoryStorage implements Storage with in-memory data.
type MemoryStorage struct {
	mu        sync.RWMutex                    `json:"-"`
	Snapshots map[string]*Snapshot            `json:"snapshots"`
	Blocks    map[string]map[int32]*blockData `json:"blocks"`
	dataDir   string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Snapshots: make(map[string]*Snapshot),
		Blocks:    make(map[string]map[int32]*blockData),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "ebs", s)
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

	if m.Snapshots == nil {
		m.Snapshots = make(map[string]*Snapshot)
	}

	if m.Blocks == nil {
		m.Blocks = make(map[string]map[int32]*blockData)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "ebs", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "ebs", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// StartSnapshot starts a new snapshot.
func (m *MemoryStorage) StartSnapshot(_ context.Context, req *StartSnapshotRequest) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshotID := "snap-" + uuid.New().String()[:12]
	snapshot := &Snapshot{
		BlockSize:        defaultBlockSize,
		Description:      req.Description,
		KmsKeyArn:        req.KmsKeyArn,
		OwnerID:          defaultAccountID,
		ParentSnapshotID: req.ParentSnapshotID,
		SnapshotID:       snapshotID,
		StartTime:        time.Now().Unix(),
		Status:           "pending",
		Tags:             req.Tags,
		VolumeSize:       req.VolumeSize,
	}

	m.Snapshots[snapshotID] = snapshot

	m.saveLocked()

	return snapshot, nil
}

// CompleteSnapshot completes a snapshot.
func (m *MemoryStorage) CompleteSnapshot(_ context.Context, snapshotID string) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshot, exists := m.Snapshots[snapshotID]
	if !exists {
		return nil, &ServiceError{
			Code:    errSnapshotNotFound,
			Message: fmt.Sprintf("Snapshot %s does not exist.", snapshotID),
		}
	}

	snapshot.Status = "completed"

	m.saveLocked()

	return snapshot, nil
}

// ListSnapshotBlocks lists blocks in a snapshot.
func (m *MemoryStorage) ListSnapshotBlocks(_ context.Context, snapshotID string) (*ListSnapshotBlocksResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot, exists := m.Snapshots[snapshotID]
	if !exists {
		return nil, &ServiceError{
			Code:    errSnapshotNotFound,
			Message: fmt.Sprintf("Snapshot %s does not exist.", snapshotID),
		}
	}

	snapshotBlocks := m.Blocks[snapshotID]

	indices := make([]int, 0, len(snapshotBlocks))

	for idx := range snapshotBlocks {
		indices = append(indices, int(idx))
	}

	sort.Ints(indices)

	blocks := make([]Block, 0, len(indices))

	for _, idx := range indices {
		token := base64.StdEncoding.EncodeToString([]byte(uuid.New().String()))
		blocks = append(blocks, Block{
			BlockIndex: int32(idx), //nolint:gosec // idx is always a non-negative block index
			BlockToken: token,
		})
	}

	return &ListSnapshotBlocksResponse{
		Blocks:     blocks,
		BlockSize:  snapshot.BlockSize,
		VolumeSize: snapshot.VolumeSize,
	}, nil
}

// PutSnapshotBlock stores a block in the specified snapshot.
func (m *MemoryStorage) PutSnapshotBlock(_ context.Context, snapshotID string, blockIndex int32, data []byte, checksum string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshot, exists := m.Snapshots[snapshotID]
	if !exists {
		return &ServiceError{
			Code:    errSnapshotNotFound,
			Message: fmt.Sprintf("Snapshot %s does not exist.", snapshotID),
		}
	}

	if snapshot.Status != "pending" {
		return &ServiceError{
			Code:    errValidation,
			Message: fmt.Sprintf("Snapshot %s is not in pending state.", snapshotID),
		}
	}

	if m.Blocks[snapshotID] == nil {
		m.Blocks[snapshotID] = make(map[int32]*blockData)
	}

	m.Blocks[snapshotID][blockIndex] = &blockData{
		Data:     data,
		Checksum: checksum,
	}

	m.saveLocked()

	return nil
}

// GetSnapshotBlock retrieves a block from the specified snapshot.
func (m *MemoryStorage) GetSnapshotBlock(_ context.Context, snapshotID string, blockIndex int32) ([]byte, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.Snapshots[snapshotID]
	if !exists {
		return nil, "", &ServiceError{
			Code:    errSnapshotNotFound,
			Message: fmt.Sprintf("Snapshot %s does not exist.", snapshotID),
		}
	}

	snapshotBlocks, ok := m.Blocks[snapshotID]
	if !ok {
		return nil, "", &ServiceError{
			Code:    errSnapshotNotFound,
			Message: fmt.Sprintf("Block index %d does not exist in snapshot %s.", blockIndex, snapshotID),
		}
	}

	block, ok := snapshotBlocks[blockIndex]
	if !ok {
		return nil, "", &ServiceError{
			Code:    errSnapshotNotFound,
			Message: fmt.Sprintf("Block index %d does not exist in snapshot %s.", blockIndex, snapshotID),
		}
	}

	return block.Data, block.Checksum, nil
}

// collectSortedIndices merges block indices from two block maps and returns them sorted.
func collectSortedIndices(first, second map[int32]*blockData) []int {
	indexSet := make(map[int32]struct{})

	for idx := range first {
		indexSet[idx] = struct{}{}
	}

	for idx := range second {
		indexSet[idx] = struct{}{}
	}

	indices := make([]int, 0, len(indexSet))

	for idx := range indexSet {
		indices = append(indices, int(idx))
	}

	sort.Ints(indices)

	return indices
}

// buildChangedBlocks builds a list of ChangedBlock entries by comparing two block maps.
func buildChangedBlocks(indices []int, first, second map[int32]*blockData) []ChangedBlock {
	changedBlocks := make([]ChangedBlock, 0, len(indices))

	for _, idx := range indices {
		i := int32(idx) //nolint:gosec // idx is always a non-negative block index
		_, inFirst := first[i]
		_, inSecond := second[i]

		if !inFirst && !inSecond {
			continue
		}

		cb := ChangedBlock{BlockIndex: i}

		if inFirst {
			cb.FirstBlockToken = base64.StdEncoding.EncodeToString([]byte(uuid.New().String()))
		}

		if inSecond {
			cb.SecondBlockToken = base64.StdEncoding.EncodeToString([]byte(uuid.New().String()))
		}

		changedBlocks = append(changedBlocks, cb)
	}

	return changedBlocks
}

// ListChangedBlocks compares two snapshots and returns changed blocks.
func (m *MemoryStorage) ListChangedBlocks(_ context.Context, firstSnapshotID, secondSnapshotID string) (*ListChangedBlocksResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	secondSnapshot, exists := m.Snapshots[secondSnapshotID]
	if !exists {
		return nil, &ServiceError{
			Code:    errSnapshotNotFound,
			Message: fmt.Sprintf("Snapshot %s does not exist.", secondSnapshotID),
		}
	}

	var firstBlocks map[int32]*blockData

	if firstSnapshotID != "" {
		if _, ok := m.Snapshots[firstSnapshotID]; !ok {
			return nil, &ServiceError{
				Code:    errSnapshotNotFound,
				Message: fmt.Sprintf("Snapshot %s does not exist.", firstSnapshotID),
			}
		}

		firstBlocks = m.Blocks[firstSnapshotID]
	}

	secondBlocks := m.Blocks[secondSnapshotID]
	indices := collectSortedIndices(firstBlocks, secondBlocks)
	changedBlocks := buildChangedBlocks(indices, firstBlocks, secondBlocks)

	return &ListChangedBlocksResponse{
		BlockSize:     secondSnapshot.BlockSize,
		ChangedBlocks: changedBlocks,
		VolumeSize:    secondSnapshot.VolumeSize,
	}, nil
}
