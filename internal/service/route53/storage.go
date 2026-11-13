package route53

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
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
	// ErrRoutingPolicyConflict is returned when record sets that share a name
	// and type would use routing policies that can't be mixed.
	ErrRoutingPolicyConflict = errors.New("record sets with the same name and type use conflicting routing policies")
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

	s.normalizeLoadedRecordSets(slog.Default())

	if s.Changes == nil {
		s.Changes = make(map[string]*ChangeInfo)
	}

	if s.Tags == nil {
		s.Tags = make(map[string][]Tag)
	}

	return nil
}

// normalizeLoadedRecordSets brings the record sets loaded from a snapshot in
// line with what ChangeRecordSets stores. Loading never fails, so record sets
// that cannot be kept are dropped and logged.
func (s *MemoryStorage) normalizeLoadedRecordSets(logger *slog.Logger) {
	for zoneID, records := range s.RecordSets {
		zone, ok := s.HostedZones[zoneID]
		if !ok {
			// Every record set operation looks the zone up first, and a new
			// zone gets a freshly generated ID, so no API reaches the record
			// sets of a zone that is gone. Drop them instead of saving them
			// again, and log them so they can be restored by hand.
			logger.Warn("route53: dropped record sets of a missing hosted zone from snapshot",
				"zone", zoneID, "recordSets", snapshotJSON(records))
			delete(s.RecordSets, zoneID)

			continue
		}

		// Snapshots written before record names were normalized may hold
		// names as sent, without the trailing dot or with upper-case letters,
		// possibly for more than one spelling of a record set.
		for i := range records {
			records[i].Name = normalizeDNSName(records[i].Name)
		}

		records = dropUnnamedRecordSets(logger, zoneID, records)
		records = coalesceRecordSets(logger, zoneID, records)
		s.RecordSets[zoneID] = records
		zone.ResourceRecordSetCount = int64(len(records))
	}
}

// dropUnnamedRecordSets drops record sets with an empty name, which earlier
// versions stored as sent. ChangeRecordSets rejects a change without a Name,
// so no API could delete them, and one of a type other than NS or SOA would
// keep DeleteHostedZone from deleting the zone. Each one is logged in full so
// its content is not lost.
func dropUnnamedRecordSets(logger *slog.Logger, zoneID string, records []ResourceRecordSet) []ResourceRecordSet {
	named := make([]ResourceRecordSet, 0, len(records))

	for i := range records {
		if records[i].Name == "" {
			logger.Warn("route53: dropped a record set without a name from snapshot",
				"zone", zoneID, "recordSet", snapshotJSON(records[i]))

			continue
		}

		named = append(named, records[i])
	}

	return named
}

// coalesceRecordSets keeps one record set per name, type and set identifier.
// Earlier versions compared names exactly as sent, so a snapshot can hold
// more than one spelling of a record set, which share a key once normalized.
//
// The snapshot does not say which spelling was written last. The one later in
// the slice is kept: CREATE, and an UPSERT that matched no stored spelling,
// append, so it is the later write unless an UPSERT afterwards updated an
// earlier spelling in place. That case cannot be told apart, so a dropped
// record set that differs from the kept one is logged in full, next to the
// kept one, to be restored by hand if needed.
func coalesceRecordSets(logger *slog.Logger, zoneID string, records []ResourceRecordSet) []ResourceRecordSet {
	type recordKey struct{ name, recordType, setIdentifier string }

	kept := make(map[recordKey]int, len(records))
	keep := make([]bool, len(records))

	for i := len(records) - 1; i >= 0; i-- {
		key := recordKey{records[i].Name, records[i].Type, records[i].SetIdentifier}

		if j, ok := kept[key]; ok {
			if !reflect.DeepEqual(records[i], records[j]) {
				logger.Warn("route53: dropped an earlier conflicting record set from snapshot",
					"zone", zoneID, "dropped", snapshotJSON(records[i]), "kept", snapshotJSON(records[j]))
			}

			continue
		}

		kept[key] = i
		keep[i] = true
	}

	coalesced := make([]ResourceRecordSet, 0, len(kept))

	for i := range records {
		if keep[i] {
			coalesced = append(coalesced, records[i])
		}
	}

	return coalesced
}

// snapshotJSON renders v the way a snapshot stores it, so that a logged record
// set can be put back into the snapshot.
func snapshotJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%+v", v)
	}

	return string(data)
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

	if records, ok := s.RecordSets[id]; ok {
		for i := range records {
			if records[i].Type != "NS" && records[i].Type != "SOA" {
				return ErrHostedZoneNotEmpty
			}
		}
	}

	delete(s.HostedZones, id)
	delete(s.RecordSets, id)

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

// ChangeRecordSets applies a change batch to record sets. Route 53 makes all
// or none of the changes in a batch, so they are applied to a copy of the
// zone's record sets that replaces the stored ones only if every change
// succeeds.
func (s *MemoryStorage) ChangeRecordSets(hostedZoneID string, changes []Change) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.HostedZones[hostedZoneID]; !exists {
		return ErrHostedZoneNotFound
	}

	records := slices.Clone(s.RecordSets[hostedZoneID])

	for i := range changes {
		// Route 53 treats a record name with and without the trailing dot as
		// identical and returns it fully qualified, and stores letters in
		// lower case however they are sent.
		rrset := changes[i].ResourceRecordSet
		rrset.Name = normalizeDNSName(rrset.Name)

		switch changes[i].Action {
		case changeActionCreate:
			if s.findRecordIndex(records, rrset.Name, rrset.Type, rrset.SetIdentifier) >= 0 {
				return ErrRecordSetAlreadyExists
			}

			records = append(records, rrset)
		case changeActionDelete:
			idx := s.findRecordIndex(records, rrset.Name, rrset.Type, rrset.SetIdentifier)
			if idx < 0 {
				return ErrRecordSetNotFound
			}

			records = append(records[:idx], records[idx+1:]...)
		case changeActionUpsert:
			idx := s.findRecordIndex(records, rrset.Name, rrset.Type, rrset.SetIdentifier)
			if idx >= 0 {
				records[idx] = rrset
			} else {
				records = append(records, rrset)
			}
		default:
			return ErrInvalidInput
		}
	}

	if err := checkRoutingPolicies(records, changes); err != nil {
		return err
	}

	s.RecordSets[hostedZoneID] = records

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

// exclusiveRoutingPolicy returns the routing policy of rrset if it is one that,
// per the Route 53 documentation, no record set of another routing policy may
// share a name and type with, and "" otherwise. Of those policies kumo only
// keeps the fields of weighted, latency and failover record sets; geolocation
// and geoproximity record sets look like any other record set with a
// SetIdentifier.
func exclusiveRoutingPolicy(rrset *ResourceRecordSet) string {
	switch {
	case rrset.Weight != nil:
		return "weighted"
	case rrset.Region != "":
		return "latency"
	case rrset.Failover != "":
		return "failover"
	default:
		return ""
	}
}

// checkRoutingPolicies returns ErrRoutingPolicyConflict if a name and type
// that a CREATE or UPSERT in changes writes holds, in records, a weighted,
// latency or failover record set next to a record set of another routing
// policy. Only the names and types the batch writes are checked, so a batch is
// not refused for a conflict it neither causes nor touches.
func checkRoutingPolicies(records []ResourceRecordSet, changes []Change) error {
	type group struct{ name, recordType string }

	written := make(map[group]bool, len(changes))

	for i := range changes {
		if changes[i].Action == changeActionCreate || changes[i].Action == changeActionUpsert {
			rrset := &changes[i].ResourceRecordSet
			written[group{normalizeDNSName(rrset.Name), rrset.Type}] = true
		}
	}

	policies := make(map[group]string, len(written))

	for i := range records {
		key := group{records[i].Name, records[i].Type}
		if !written[key] {
			continue
		}

		policy := exclusiveRoutingPolicy(&records[i])

		if seen, ok := policies[key]; ok && seen != policy {
			return ErrRoutingPolicyConflict
		}

		policies[key] = policy
	}

	return nil
}

// findRecordIndex finds the index of a record set by name, type and set
// identifier, the same key coalesceRecordSets uses. Record sets with a routing
// policy other than simple share a name and type and differ only in
// SetIdentifier.
func (s *MemoryStorage) findRecordIndex(records []ResourceRecordSet, name, recordType, setIdentifier string) int {
	for i := range records {
		if records[i].Name == name && records[i].Type == recordType && records[i].SetIdentifier == setIdentifier {
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

	for _, rk := range removeKeys {
		for i := range tags {
			if tags[i].Key == rk {
				tags = append(tags[:i], tags[i+1:]...)

				break
			}
		}
	}

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
