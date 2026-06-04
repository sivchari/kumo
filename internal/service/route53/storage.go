package route53

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/sivchari/kumo/internal/storage"
)

var (
	// ErrHostedZoneNotFound is returned when a hosted zone is not found.
	ErrHostedZoneNotFound = errors.New("hosted zone not found")
	// ErrHostedZoneAlreadyExists is returned when a hosted zone already exists.
	ErrHostedZoneAlreadyExists = errors.New("hosted zone already exists")
	// ErrHostedZoneNotEmpty is returned when trying to delete a non-empty hosted zone.
	ErrHostedZoneNotEmpty = errors.New("hosted zone is not empty")
	// ErrRecordSetNotFound is returned when a record set is not found.
	ErrRecordSetNotFound = errors.New("record set not found")
	// ErrRecordSetAlreadyExists is returned when a record set already exists.
	ErrRecordSetAlreadyExists = errors.New("record set already exists")
	// ErrInvalidInput is returned when input is invalid.
	ErrInvalidInput = errors.New("invalid input")
	// ErrChangeNotFound is returned when a change is not found.
	ErrChangeNotFound = errors.New("change not found")
)

// Storage defines the interface for Route 53 storage operations.
type Storage interface {
	CreateHostedZone(zone *HostedZone) error
	GetHostedZone(id string) (*HostedZone, error)
	ListHostedZones() ([]*HostedZone, error)
	DeleteHostedZone(id string) error
	GetRecordSets(hostedZoneID string) ([]ResourceRecordSet, error)
	ChangeRecordSets(hostedZoneID string, changes []Change) error
	PutChange(change *ChangeInfo) error
	GetChange(id string) (*ChangeInfo, error)
	ListTagsForResource(resourceType, resourceID string) ([]Tag, error)
	ChangeTagsForResource(resourceType, resourceID string, addTags []Tag, removeKeys []string) error
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

// MemoryStorage is an in-memory implementation of Storage.
type MemoryStorage struct {
	mu          sync.RWMutex                   `json:"-"`
	HostedZones map[string]*HostedZone         `json:"hostedZones"`
	RecordSets  map[string][]ResourceRecordSet `json:"recordSets"` // key: hostedZoneID
	Changes     map[string]*ChangeInfo         `json:"changes"`    // key: bare change ID
	Tags        map[string][]Tag               `json:"tags"`       // key: "{type}/{id}"
	dataDir     string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		HostedZones: make(map[string]*HostedZone),
		RecordSets:  make(map[string][]ResourceRecordSet),
		Changes:     make(map[string]*ChangeInfo),
		Tags:        make(map[string][]Tag),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "route53", s)
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

	if s.HostedZones == nil {
		s.HostedZones = make(map[string]*HostedZone)
	}

	if s.RecordSets == nil {
		s.RecordSets = make(map[string][]ResourceRecordSet)
	}

	if s.Changes == nil {
		s.Changes = make(map[string]*ChangeInfo)
	}

	if s.Tags == nil {
		s.Tags = make(map[string][]Tag)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "route53", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "route53", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateHostedZone creates a new hosted zone.
func (s *MemoryStorage) CreateHostedZone(zone *HostedZone) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.HostedZones[zone.ID]; exists {
		return ErrHostedZoneAlreadyExists
	}

	s.HostedZones[zone.ID] = zone
	s.RecordSets[zone.ID] = []ResourceRecordSet{}

	s.saveLocked()

	return nil
}

// GetHostedZone retrieves a hosted zone by ID.
func (s *MemoryStorage) GetHostedZone(id string) (*HostedZone, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	zone, exists := s.HostedZones[id]
	if !exists {
		return nil, ErrHostedZoneNotFound
	}

	return zone, nil
}

// ListHostedZones lists all hosted zones.
func (s *MemoryStorage) ListHostedZones() ([]*HostedZone, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	zones := make([]*HostedZone, 0, len(s.HostedZones))
	for _, zone := range s.HostedZones {
		zones = append(zones, zone)
	}

	return zones, nil
}

// DeleteHostedZone deletes a hosted zone.
func (s *MemoryStorage) DeleteHostedZone(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.HostedZones[id]; !exists {
		return ErrHostedZoneNotFound
	}

	// Check if hosted zone has record sets (other than NS and SOA)
	if records, ok := s.RecordSets[id]; ok {
		for i := range records {
			if records[i].Type != "NS" && records[i].Type != "SOA" {
				return ErrHostedZoneNotEmpty
			}
		}
	}

	delete(s.HostedZones, id)
	delete(s.RecordSets, id)

	// Clean up tags associated with the hosted zone.
	bareID := strings.TrimPrefix(id, "/hostedzone/")
	delete(s.Tags, "hostedzone/"+bareID)

	s.saveLocked()

	return nil
}

// GetRecordSets retrieves all record sets for a hosted zone.
func (s *MemoryStorage) GetRecordSets(hostedZoneID string) ([]ResourceRecordSet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.HostedZones[hostedZoneID]; !exists {
		return nil, ErrHostedZoneNotFound
	}

	records, ok := s.RecordSets[hostedZoneID]
	if !ok {
		return []ResourceRecordSet{}, nil
	}

	return records, nil
}

// ChangeRecordSets applies changes to record sets.
func (s *MemoryStorage) ChangeRecordSets(hostedZoneID string, changes []Change) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.HostedZones[hostedZoneID]; !exists {
		return ErrHostedZoneNotFound
	}

	records := s.RecordSets[hostedZoneID]

	for i := range changes {
		switch changes[i].Action {
		case "CREATE":
			if s.findRecordIndex(records, changes[i].ResourceRecordSet.Name, changes[i].ResourceRecordSet.Type) >= 0 {
				return ErrRecordSetAlreadyExists
			}

			records = append(records, changes[i].ResourceRecordSet)
		case "DELETE":
			idx := s.findRecordIndex(records, changes[i].ResourceRecordSet.Name, changes[i].ResourceRecordSet.Type)
			if idx < 0 {
				return ErrRecordSetNotFound
			}

			records = append(records[:idx], records[idx+1:]...)
		case "UPSERT":
			idx := s.findRecordIndex(records, changes[i].ResourceRecordSet.Name, changes[i].ResourceRecordSet.Type)
			if idx >= 0 {
				records[idx] = changes[i].ResourceRecordSet
			} else {
				records = append(records, changes[i].ResourceRecordSet)
			}
		default:
			return ErrInvalidInput
		}
	}

	s.RecordSets[hostedZoneID] = records

	// Update record count
	if zone, exists := s.HostedZones[hostedZoneID]; exists {
		zone.ResourceRecordSetCount = int64(len(records))
	}

	s.saveLocked()

	return nil
}

// PutChange stores a change.
func (s *MemoryStorage) PutChange(change *ChangeInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := strings.TrimPrefix(change.ID, "/change/")
	s.Changes[id] = change

	s.saveLocked()

	return nil
}

// GetChange retrieves a change by ID.
func (s *MemoryStorage) GetChange(id string) (*ChangeInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.Changes[id]
	if !ok {
		return nil, ErrChangeNotFound
	}

	return c, nil
}

// findRecordIndex finds the index of a record set by name and type.
func (s *MemoryStorage) findRecordIndex(records []ResourceRecordSet, name, recordType string) int {
	for i := range records {
		if records[i].Name == name && records[i].Type == recordType {
			return i
		}
	}

	return -1
}

// ListTagsForResource returns the tags for the given resource.
func (s *MemoryStorage) ListTagsForResource(resourceType, resourceID string) ([]Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := resourceType + "/" + resourceID
	tags := s.Tags[key]

	if tags == nil {
		return []Tag{}, nil
	}

	// Return a copy.
	out := make([]Tag, len(tags))
	copy(out, tags)

	return out, nil
}

// ChangeTagsForResource adds and/or removes tags for the given resource.
func (s *MemoryStorage) ChangeTagsForResource(resourceType, resourceID string, addTags []Tag, removeKeys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := resourceType + "/" + resourceID

	tags := s.Tags[key]

	// Remove tags by key.
	for _, rk := range removeKeys {
		for i := range tags {
			if tags[i].Key == rk {
				tags = append(tags[:i], tags[i+1:]...)

				break
			}
		}
	}

	// Add/update tags.
	for _, at := range addTags {
		found := false

		for i := range tags {
			if tags[i].Key == at.Key {
				tags[i].Value = at.Value
				found = true

				break
			}
		}

		if !found {
			tags = append(tags, at)
		}
	}

	s.Tags[key] = tags

	s.saveLocked()

	return nil
}
