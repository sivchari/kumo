package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errResourceNotFound    = "ResourceNotFoundException"
	errConflictException   = "ConflictException"
	errValidationException = "ValidationException"
)

// Default values.
const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "123456789012"
	defaultGroupName = "default"
	defaultTimezone  = "UTC"
)

// Storage defines the Scheduler service storage interface.
type Storage interface {
	// Schedule operations
	CreateSchedule(ctx context.Context, name string, req *CreateScheduleRequest) (*Schedule, error)
	GetSchedule(ctx context.Context, name, groupName string) (*Schedule, error)
	UpdateSchedule(ctx context.Context, name string, req *UpdateScheduleRequest) (*Schedule, error)
	DeleteSchedule(ctx context.Context, name, groupName string) error
	ListSchedules(ctx context.Context, groupName string, limit int32) ([]*Schedule, error)

	// Schedule group operations
	CreateScheduleGroup(ctx context.Context, name string, req *CreateScheduleGroupRequest) (*ScheduleGroup, error)
	GetScheduleGroup(ctx context.Context, name string) (*ScheduleGroup, error)
	DeleteScheduleGroup(ctx context.Context, name string) error
	ListScheduleGroups(ctx context.Context, limit int32) ([]*ScheduleGroup, error)
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
	mu             sync.RWMutex              `json:"-"`
	Schedules      map[string]*Schedule      `json:"schedules"`      // key: groupName/scheduleName
	ScheduleGroups map[string]*ScheduleGroup `json:"scheduleGroups"` // key: groupName
	region         string
	accountID      string
	dataDir        string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	ms := &MemoryStorage{
		Schedules:      make(map[string]*Schedule),
		ScheduleGroups: make(map[string]*ScheduleGroup),
		region:         region,
		accountID:      defaultAccountID,
	}

	for _, o := range opts {
		o(ms)
	}

	if ms.dataDir != "" {
		_ = storage.Load(ms.dataDir, "scheduler", ms)
	}

	// Create default schedule group.
	ms.ScheduleGroups[defaultGroupName] = &ScheduleGroup{
		Name:         defaultGroupName,
		ARN:          generateScheduleGroupARN(ms.region, defaultAccountID, defaultGroupName),
		State:        ScheduleGroupStateActive,
		CreationDate: time.Now(),
	}

	return ms
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

	if m.Schedules == nil {
		m.Schedules = make(map[string]*Schedule)
	}

	if m.ScheduleGroups == nil {
		m.ScheduleGroups = make(map[string]*ScheduleGroup)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "scheduler", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "scheduler", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateSchedule creates a new schedule.
func (m *MemoryStorage) CreateSchedule(_ context.Context, name string, req *CreateScheduleRequest) (*Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	groupName := defaultString(req.GroupName, defaultGroupName)
	key := scheduleKey(groupName, name)

	if _, exists := m.Schedules[key]; exists {
		return nil, &Error{Code: errConflictException, Message: "Schedule already exists: " + name}
	}

	// Check if group exists.
	if _, exists := m.ScheduleGroups[groupName]; !exists {
		return nil, &Error{Code: errResourceNotFound, Message: "ScheduleGroup not found: " + groupName}
	}

	scheduleARN := generateScheduleARN(m.region, m.accountID, groupName, name)
	now := time.Now()

	schedule := &Schedule{
		Name:                       name,
		ARN:                        scheduleARN,
		GroupName:                  groupName,
		Description:                req.Description,
		ScheduleExpression:         req.ScheduleExpression,
		ScheduleExpressionTimezone: defaultString(req.ScheduleExpressionTimezone, defaultTimezone),
		State:                      defaultString(req.State, StateEnabled),
		FlexibleTimeWindow:         req.FlexibleTimeWindow,
		Target:                     req.Target,
		KmsKeyArn:                  req.KmsKeyArn,
		ActionAfterCompletion:      defaultString(req.ActionAfterCompletion, ActionNone),
		CreationDate:               now,
		LastModificationDate:       now,
	}

	if req.StartDate != nil {
		t, err := time.Parse(time.RFC3339, *req.StartDate)
		if err == nil {
			schedule.StartDate = &t
		}
	}

	if req.EndDate != nil {
		t, err := time.Parse(time.RFC3339, *req.EndDate)
		if err == nil {
			schedule.EndDate = &t
		}
	}

	m.Schedules[key] = schedule

	m.saveLocked()

	return schedule, nil
}

// GetSchedule gets a schedule.
func (m *MemoryStorage) GetSchedule(_ context.Context, name, groupName string) (*Schedule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groupName = defaultString(groupName, defaultGroupName)
	key := scheduleKey(groupName, name)

	schedule, exists := m.Schedules[key]
	if !exists {
		return nil, &Error{Code: errResourceNotFound, Message: "Schedule not found: " + name}
	}

	return schedule, nil
}

// UpdateSchedule updates a schedule.
func (m *MemoryStorage) UpdateSchedule(_ context.Context, name string, req *UpdateScheduleRequest) (*Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	groupName := defaultString(req.GroupName, defaultGroupName)
	key := scheduleKey(groupName, name)

	schedule, exists := m.Schedules[key]
	if !exists {
		return nil, &Error{Code: errResourceNotFound, Message: "Schedule not found: " + name}
	}

	// Update fields.
	schedule.Description = req.Description
	schedule.ScheduleExpression = req.ScheduleExpression
	schedule.ScheduleExpressionTimezone = defaultString(req.ScheduleExpressionTimezone, defaultTimezone)
	schedule.State = defaultString(req.State, schedule.State)
	schedule.FlexibleTimeWindow = req.FlexibleTimeWindow
	schedule.Target = req.Target
	schedule.KmsKeyArn = req.KmsKeyArn
	schedule.ActionAfterCompletion = defaultString(req.ActionAfterCompletion, schedule.ActionAfterCompletion)
	schedule.LastModificationDate = time.Now()

	if req.StartDate != nil {
		t, err := time.Parse(time.RFC3339, *req.StartDate)
		if err == nil {
			schedule.StartDate = &t
		}
	} else {
		schedule.StartDate = nil
	}

	if req.EndDate != nil {
		t, err := time.Parse(time.RFC3339, *req.EndDate)
		if err == nil {
			schedule.EndDate = &t
		}
	} else {
		schedule.EndDate = nil
	}

	m.saveLocked()

	return schedule, nil
}

// DeleteSchedule deletes a schedule.
func (m *MemoryStorage) DeleteSchedule(_ context.Context, name, groupName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	groupName = defaultString(groupName, defaultGroupName)
	key := scheduleKey(groupName, name)

	if _, exists := m.Schedules[key]; !exists {
		return &Error{Code: errResourceNotFound, Message: "Schedule not found: " + name}
	}

	delete(m.Schedules, key)

	m.saveLocked()

	return nil
}

// ListSchedules lists schedules.
func (m *MemoryStorage) ListSchedules(_ context.Context, groupName string, limit int32) ([]*Schedule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Schedule, 0, len(m.Schedules))

	for _, schedule := range m.Schedules {
		if groupName != "" && schedule.GroupName != groupName {
			continue
		}

		result = append(result, schedule)

		if limit > 0 && len(result) >= int(limit) {
			break
		}
	}

	return result, nil
}

// CreateScheduleGroup creates a new schedule group.
func (m *MemoryStorage) CreateScheduleGroup(_ context.Context, name string, _ *CreateScheduleGroupRequest) (*ScheduleGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.ScheduleGroups[name]; exists {
		return nil, &Error{Code: errConflictException, Message: "ScheduleGroup already exists: " + name}
	}

	groupARN := generateScheduleGroupARN(m.region, m.accountID, name)
	now := time.Now()

	group := &ScheduleGroup{
		Name:         name,
		ARN:          groupARN,
		State:        ScheduleGroupStateActive,
		CreationDate: now,
	}

	m.ScheduleGroups[name] = group

	m.saveLocked()

	return group, nil
}

// GetScheduleGroup gets a schedule group.
func (m *MemoryStorage) GetScheduleGroup(_ context.Context, name string) (*ScheduleGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, exists := m.ScheduleGroups[name]
	if !exists {
		return nil, &Error{Code: errResourceNotFound, Message: "ScheduleGroup not found: " + name}
	}

	return group, nil
}

// DeleteScheduleGroup deletes a schedule group.
func (m *MemoryStorage) DeleteScheduleGroup(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == defaultGroupName {
		return &Error{Code: errValidationException, Message: "Cannot delete default schedule group"}
	}

	if _, exists := m.ScheduleGroups[name]; !exists {
		return &Error{Code: errResourceNotFound, Message: "ScheduleGroup not found: " + name}
	}

	// Check if any schedules use this group.
	for _, schedule := range m.Schedules {
		if schedule.GroupName == name {
			return &Error{Code: errConflictException, Message: "ScheduleGroup has schedules: " + name}
		}
	}

	delete(m.ScheduleGroups, name)

	m.saveLocked()

	return nil
}

// ListScheduleGroups lists schedule groups.
func (m *MemoryStorage) ListScheduleGroups(_ context.Context, limit int32) ([]*ScheduleGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ScheduleGroup, 0, len(m.ScheduleGroups))

	for _, group := range m.ScheduleGroups {
		result = append(result, group)

		if limit > 0 && len(result) >= int(limit) {
			break
		}
	}

	return result, nil
}

// Helper functions.

func generateScheduleARN(region, accountID, groupName, scheduleName string) string {
	return fmt.Sprintf("arn:aws:scheduler:%s:%s:schedule/%s/%s", region, accountID, groupName, scheduleName)
}

func generateScheduleGroupARN(region, accountID, groupName string) string {
	return fmt.Sprintf("arn:aws:scheduler:%s:%s:schedule-group/%s", region, accountID, groupName)
}

func scheduleKey(groupName, scheduleName string) string {
	return groupName + "/" + scheduleName
}

func defaultString(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}

	return value
}

// Ensure MemoryStorage implements Storage.
var _ Storage = (*MemoryStorage)(nil)
