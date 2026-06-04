package configservice

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

// Error codes.
const (
	errNoSuchConfigurationRecorder               = "NoSuchConfigurationRecorderException"
	errMaxNumberOfConfigurationRecordersExceeded = "MaxNumberOfConfigurationRecordersExceededException"
	errNoSuchConfigRule                          = "NoSuchConfigRuleException"
	errInvalidParameterValue                     = "InvalidParameterValueException"
)

// Default values.
const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "123456789012"
)

// Storage defines the Config service storage interface.
type Storage interface {
	// Configuration Recorder operations
	PutConfigurationRecorder(ctx context.Context, req *PutConfigurationRecorderRequest) error
	DeleteConfigurationRecorder(ctx context.Context, name string) error
	DescribeConfigurationRecorders(ctx context.Context, names []string) ([]*ConfigurationRecorder, error)
	StartConfigurationRecorder(ctx context.Context, name string) error
	StopConfigurationRecorder(ctx context.Context, name string) error

	// Config Rule operations
	PutConfigRule(ctx context.Context, req *PutConfigRuleRequest) (*ConfigRule, error)
	DeleteConfigRule(ctx context.Context, name string) error
	DescribeConfigRules(ctx context.Context, names []string) ([]*ConfigRule, error)
	GetComplianceDetailsByConfigRule(ctx context.Context, req *GetComplianceDetailsByConfigRuleRequest) ([]*EvaluationResult, string, error)
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
	mu               sync.RWMutex                            `json:"-"`
	Recorders        map[string]*ConfigurationRecorder       `json:"recorders"`
	RecorderStatuses map[string]*ConfigurationRecorderStatus `json:"recorderStatuses"`
	Rules            map[string]*ConfigRule                  `json:"rules"`
	region           string
	accountID        string
	dataDir          string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Recorders:        make(map[string]*ConfigurationRecorder),
		RecorderStatuses: make(map[string]*ConfigurationRecorderStatus),
		Rules:            make(map[string]*ConfigRule),
		region:           region,
		accountID:        defaultAccountID,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "configservice", s)
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

	if m.Recorders == nil {
		m.Recorders = make(map[string]*ConfigurationRecorder)
	}

	if m.RecorderStatuses == nil {
		m.RecorderStatuses = make(map[string]*ConfigurationRecorderStatus)
	}

	if m.Rules == nil {
		m.Rules = make(map[string]*ConfigRule)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "configservice", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "configservice", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// PutConfigurationRecorder creates or updates a configuration recorder.
func (m *MemoryStorage) PutConfigurationRecorder(_ context.Context, req *PutConfigurationRecorderRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.ConfigurationRecorder == nil {
		return &Error{Code: errInvalidParameterValue, Message: "ConfigurationRecorder is required"}
	}

	input := req.ConfigurationRecorder

	if input.Name == "" {
		return &Error{Code: errInvalidParameterValue, Message: "Configuration recorder name is required"}
	}

	// AWS Config allows only one configuration recorder per region
	if len(m.Recorders) > 0 {
		if _, exists := m.Recorders[input.Name]; !exists {
			return &Error{Code: errMaxNumberOfConfigurationRecordersExceeded, Message: "Only one configuration recorder is allowed per region"}
		}
	}

	recorder := &ConfigurationRecorder{
		Name:    input.Name,
		RoleARN: input.RoleARN,
	}

	if input.RecordingGroup != nil {
		recorder.RecordingGroup = &RecordingGroup{
			AllSupported:               defaultBool(input.RecordingGroup.AllSupported, true),
			IncludeGlobalResourceTypes: defaultBool(input.RecordingGroup.IncludeGlobalResourceTypes, false),
			ResourceTypes:              input.RecordingGroup.ResourceTypes,
		}
	} else {
		recorder.RecordingGroup = &RecordingGroup{
			AllSupported:               true,
			IncludeGlobalResourceTypes: false,
		}
	}

	m.Recorders[input.Name] = recorder

	// Initialize status if not exists
	if _, exists := m.RecorderStatuses[input.Name]; !exists {
		m.RecorderStatuses[input.Name] = &ConfigurationRecorderStatus{
			Name:       input.Name,
			Recording:  false,
			LastStatus: "Pending",
		}
	}

	m.saveLocked()

	return nil
}

// DeleteConfigurationRecorder deletes a configuration recorder.
func (m *MemoryStorage) DeleteConfigurationRecorder(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Recorders[name]; !exists {
		return &Error{Code: errNoSuchConfigurationRecorder, Message: "Configuration recorder not found"}
	}

	delete(m.Recorders, name)
	delete(m.RecorderStatuses, name)

	m.saveLocked()

	return nil
}

// DescribeConfigurationRecorders describes configuration recorders.
func (m *MemoryStorage) DescribeConfigurationRecorders(_ context.Context, names []string) ([]*ConfigurationRecorder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(names) == 0 {
		// Return all recorders
		result := make([]*ConfigurationRecorder, 0, len(m.Recorders))
		for _, recorder := range m.Recorders {
			result = append(result, recorder)
		}

		return result, nil
	}

	// Return specified recorders
	result := make([]*ConfigurationRecorder, 0, len(names))

	for _, name := range names {
		if recorder, exists := m.Recorders[name]; exists {
			result = append(result, recorder)
		}
	}

	return result, nil
}

// StartConfigurationRecorder starts recording for a configuration recorder.
func (m *MemoryStorage) StartConfigurationRecorder(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Recorders[name]; !exists {
		return &Error{Code: errNoSuchConfigurationRecorder, Message: "Configuration recorder not found"}
	}

	status := m.RecorderStatuses[name]
	status.Recording = true
	status.LastStatus = "SUCCESS"

	now := time.Now()
	status.LastStartTime = &now

	m.saveLocked()

	return nil
}

// StopConfigurationRecorder stops recording for a configuration recorder.
func (m *MemoryStorage) StopConfigurationRecorder(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Recorders[name]; !exists {
		return &Error{Code: errNoSuchConfigurationRecorder, Message: "Configuration recorder not found"}
	}

	status := m.RecorderStatuses[name]
	status.Recording = false

	now := time.Now()
	status.LastStopTime = &now

	m.saveLocked()

	return nil
}

// PutConfigRule creates or updates a config rule.
func (m *MemoryStorage) PutConfigRule(_ context.Context, req *PutConfigRuleRequest) (*ConfigRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.ConfigRule == nil {
		return nil, &Error{Code: errInvalidParameterValue, Message: "ConfigRule is required"}
	}

	input := req.ConfigRule

	if input.ConfigRuleName == "" {
		return nil, &Error{Code: errInvalidParameterValue, Message: "Config rule name is required"}
	}

	if input.Source == nil {
		return nil, &Error{Code: errInvalidParameterValue, Message: "Source is required"}
	}

	// Check if rule exists (update case)
	existing, exists := m.Rules[input.ConfigRuleName]

	var ruleID string

	var ruleARN string

	if exists {
		ruleID = existing.ConfigRuleID
		ruleARN = existing.ConfigRuleARN
	} else {
		ruleID = "config-rule-" + uuid.New().String()[:8]
		ruleARN = generateConfigRuleARN(m.region, m.accountID, ruleID)
	}

	rule := &ConfigRule{
		ConfigRuleName:  input.ConfigRuleName,
		ConfigRuleARN:   ruleARN,
		ConfigRuleID:    ruleID,
		Description:     input.Description,
		ConfigRuleState: "ACTIVE",
		Source: &Source{
			Owner:            input.Source.Owner,
			SourceIdentifier: input.Source.SourceIdentifier,
		},
	}

	if input.Scope != nil {
		rule.Scope = &Scope{
			ComplianceResourceTypes: input.Scope.ComplianceResourceTypes,
		}
	}

	m.Rules[input.ConfigRuleName] = rule

	m.saveLocked()

	return rule, nil
}

// DeleteConfigRule deletes a config rule.
func (m *MemoryStorage) DeleteConfigRule(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Rules[name]; !exists {
		return &Error{Code: errNoSuchConfigRule, Message: "Config rule not found"}
	}

	delete(m.Rules, name)

	m.saveLocked()

	return nil
}

// DescribeConfigRules describes config rules.
func (m *MemoryStorage) DescribeConfigRules(_ context.Context, names []string) ([]*ConfigRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(names) == 0 {
		// Return all rules
		result := make([]*ConfigRule, 0, len(m.Rules))
		for _, rule := range m.Rules {
			result = append(result, rule)
		}

		return result, nil
	}

	// Return specified rules
	result := make([]*ConfigRule, 0, len(names))

	for _, name := range names {
		if rule, exists := m.Rules[name]; exists {
			result = append(result, rule)
		}
	}

	return result, nil
}

// GetComplianceDetailsByConfigRule gets compliance details for a config rule.
// For MVP, this returns an empty list as we don't track actual compliance.
func (m *MemoryStorage) GetComplianceDetailsByConfigRule(_ context.Context, req *GetComplianceDetailsByConfigRuleRequest) ([]*EvaluationResult, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.Rules[req.ConfigRuleName]; !exists {
		return nil, "", &Error{Code: errNoSuchConfigRule, Message: "Config rule not found"}
	}

	// Return empty results for MVP
	return []*EvaluationResult{}, "", nil
}

// Helper functions.

func generateConfigRuleARN(region, accountID, ruleID string) string {
	return "arn:aws:config:" + region + ":" + accountID + ":config-rule/" + ruleID
}

func defaultBool(ptr *bool, defaultValue bool) bool {
	if ptr == nil {
		return defaultValue
	}

	return *ptr
}
