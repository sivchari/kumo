package globalaccelerator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errNotFound           = "AcceleratorNotFoundException"
	errListenerNotFound   = "ListenerNotFoundException"
	errEndpointNotFound   = "EndpointGroupNotFoundException"
	errAcceleratorEnabled = "AcceleratorNotDisabledException"

	defaultAccountID = "000000000000"
)

// Storage defines the Global Accelerator storage interface.
type Storage interface {
	// Accelerator operations.
	CreateAccelerator(ctx context.Context, req *CreateAcceleratorRequest) (*Accelerator, error)
	GetAccelerator(ctx context.Context, arn string) (*Accelerator, error)
	ListAccelerators(ctx context.Context, maxResults int32, nextToken string) ([]*Accelerator, string, error)
	UpdateAccelerator(ctx context.Context, arn, name, ipAddressType string, enabled *bool) (*Accelerator, error)
	DeleteAccelerator(ctx context.Context, arn string) error

	// Listener operations.
	CreateListener(ctx context.Context, req *CreateListenerRequest) (*Listener, error)
	GetListener(ctx context.Context, arn string) (*Listener, error)
	ListListeners(ctx context.Context, acceleratorArn string, maxResults int32, nextToken string) ([]*Listener, string, error)
	UpdateListener(ctx context.Context, req *UpdateListenerRequest) (*Listener, error)
	DeleteListener(ctx context.Context, arn string) error

	// EndpointGroup operations.
	CreateEndpointGroup(ctx context.Context, req *CreateEndpointGroupRequest) (*EndpointGroup, error)
	GetEndpointGroup(ctx context.Context, arn string) (*EndpointGroup, error)
	ListEndpointGroups(ctx context.Context, listenerArn string, maxResults int32, nextToken string) ([]*EndpointGroup, string, error)
	UpdateEndpointGroup(ctx context.Context, req *UpdateEndpointGroupRequest) (*EndpointGroup, error)
	DeleteEndpointGroup(ctx context.Context, arn string) error
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
	Accelerators   map[string]*Accelerator   `json:"accelerators"`
	Listeners      map[string]*Listener      `json:"listeners"`
	EndpointGroups map[string]*EndpointGroup `json:"endpointGroups"`
	dataDir        string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Accelerators:   make(map[string]*Accelerator),
		Listeners:      make(map[string]*Listener),
		EndpointGroups: make(map[string]*EndpointGroup),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "globalaccelerator", s)
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

	if s.Accelerators == nil {
		s.Accelerators = make(map[string]*Accelerator)
	}

	if s.Listeners == nil {
		s.Listeners = make(map[string]*Listener)
	}

	if s.EndpointGroups == nil {
		s.EndpointGroups = make(map[string]*EndpointGroup)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "globalaccelerator", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "globalaccelerator", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateAccelerator creates a new accelerator.
func (s *MemoryStorage) CreateAccelerator(_ context.Context, req *CreateAcceleratorRequest) (*Accelerator, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acceleratorID := uuid.New().String()
	arn := fmt.Sprintf("arn:aws:globalaccelerator::%s:accelerator/%s", defaultAccountID, acceleratorID)

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	ipAddressType := IPAddressTypeIPv4
	if req.IPAddressType != "" {
		ipAddressType = IPAddressType(req.IPAddressType)
	}

	// Generate static IP addresses.
	ipSets := []IPSet{
		{
			IPFamily:        "IPv4",
			IPAddresses:     []string{generateIPAddress(), generateIPAddress()},
			IPAddressFamily: "IPv4",
		},
	}

	if ipAddressType == IPAddressTypeDualStack {
		ipSets = append(ipSets, IPSet{
			IPFamily:        "IPv6",
			IPAddresses:     []string{generateIPv6Address()},
			IPAddressFamily: "IPv6",
		})
	}

	now := time.Now()
	accelerator := &Accelerator{
		AcceleratorArn: arn,
		Name:           req.Name,
		IPAddressType:  ipAddressType,
		Enabled:        enabled,
		IPSets:         ipSets,
		DNSName:        fmt.Sprintf("%s.awsglobalaccelerator.com", acceleratorID[:8]),
		Status:         AcceleratorStatusDeployed,
		CreatedTime:    now,
		LastModified:   now,
	}

	if ipAddressType == IPAddressTypeDualStack {
		accelerator.DualStackDNS = fmt.Sprintf("%s.dualstack.awsglobalaccelerator.com", acceleratorID[:8])
	}

	s.Accelerators[arn] = accelerator

	s.saveLocked()

	return accelerator, nil
}

// GetAccelerator retrieves an accelerator by ARN.
func (s *MemoryStorage) GetAccelerator(_ context.Context, arn string) (*Accelerator, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	accelerator, ok := s.Accelerators[arn]
	if !ok {
		return nil, &ServiceError{Code: errNotFound, Message: "Accelerator not found"}
	}

	return accelerator, nil
}

// ListAccelerators lists all accelerators with pagination support.
func (s *MemoryStorage) ListAccelerators(_ context.Context, maxResults int32, nextToken string) ([]*Accelerator, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Default maxResults per AWS API documentation.
	if maxResults <= 0 {
		maxResults = 100
	}

	// Collect all ARNs and sort for consistent pagination.
	arns := make([]string, 0, len(s.Accelerators))
	for arn := range s.Accelerators {
		arns = append(arns, arn)
	}

	sort.Strings(arns)

	// Find starting index based on nextToken.
	startIdx := 0

	if nextToken != "" {
		for i, arn := range arns {
			if arn == nextToken {
				startIdx = i

				break
			}
		}
	}

	// Collect accelerators from startIdx up to maxResults.
	accelerators := make([]*Accelerator, 0, maxResults)
	for i := startIdx; i < len(arns) && len(accelerators) < int(maxResults); i++ {
		accelerators = append(accelerators, s.Accelerators[arns[i]])
	}

	// Determine next token.
	var newNextToken string
	if startIdx+int(maxResults) < len(arns) {
		newNextToken = arns[startIdx+int(maxResults)]
	}

	return accelerators, newNextToken, nil
}

// UpdateAccelerator updates an accelerator.
func (s *MemoryStorage) UpdateAccelerator(_ context.Context, arn, name, ipAddressType string, enabled *bool) (*Accelerator, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accelerator, ok := s.Accelerators[arn]
	if !ok {
		return nil, &ServiceError{Code: errNotFound, Message: "Accelerator not found"}
	}

	if name != "" {
		accelerator.Name = name
	}

	if ipAddressType != "" {
		accelerator.IPAddressType = IPAddressType(ipAddressType)
	}

	if enabled != nil {
		accelerator.Enabled = *enabled
	}

	accelerator.LastModified = time.Now()

	s.saveLocked()

	return accelerator, nil
}

// DeleteAccelerator deletes an accelerator.
func (s *MemoryStorage) DeleteAccelerator(_ context.Context, arn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	accelerator, ok := s.Accelerators[arn]
	if !ok {
		return &ServiceError{Code: errNotFound, Message: "Accelerator not found"}
	}

	if accelerator.Enabled {
		return &ServiceError{Code: errAcceleratorEnabled, Message: "Accelerator must be disabled before deletion"}
	}

	// Delete associated listeners and endpoint groups.
	for listenerArn, listener := range s.Listeners {
		if listener.AcceleratorArn == arn {
			// Delete endpoint groups for this listener.
			for egArn, eg := range s.EndpointGroups {
				if eg.ListenerArn == listenerArn {
					delete(s.EndpointGroups, egArn)
				}
			}

			delete(s.Listeners, listenerArn)
		}
	}

	delete(s.Accelerators, arn)

	s.saveLocked()

	return nil
}

// CreateListener creates a new listener.
func (s *MemoryStorage) CreateListener(_ context.Context, req *CreateListenerRequest) (*Listener, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify accelerator exists.
	if _, ok := s.Accelerators[req.AcceleratorArn]; !ok {
		return nil, &ServiceError{Code: errNotFound, Message: "Accelerator not found"}
	}

	listenerID := uuid.New().String()
	arn := fmt.Sprintf("%s/listener/%s", req.AcceleratorArn, listenerID)

	portRanges := make([]PortRange, len(req.PortRanges))
	for i, pr := range req.PortRanges {
		portRanges[i] = PortRange(pr)
	}

	clientAffinity := ClientAffinityNone
	if req.ClientAffinity != "" {
		clientAffinity = ClientAffinity(req.ClientAffinity)
	}

	listener := &Listener{
		ListenerArn:    arn,
		AcceleratorArn: req.AcceleratorArn,
		PortRanges:     portRanges,
		Protocol:       Protocol(req.Protocol),
		ClientAffinity: clientAffinity,
	}

	s.Listeners[arn] = listener

	s.saveLocked()

	return listener, nil
}

// GetListener retrieves a listener by ARN.
func (s *MemoryStorage) GetListener(_ context.Context, arn string) (*Listener, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	listener, ok := s.Listeners[arn]
	if !ok {
		return nil, &ServiceError{Code: errListenerNotFound, Message: "Listener not found"}
	}

	return listener, nil
}

// ListListeners lists listeners for an accelerator.
func (s *MemoryStorage) ListListeners(_ context.Context, acceleratorArn string, maxResults int32, _ string) ([]*Listener, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults <= 0 {
		maxResults = 100
	}

	listeners := make([]*Listener, 0)

	for _, listener := range s.Listeners {
		if listener.AcceleratorArn == acceleratorArn {
			listeners = append(listeners, listener)
			if len(listeners) >= int(maxResults) {
				break
			}
		}
	}

	return listeners, "", nil
}

// UpdateListener updates a listener.
func (s *MemoryStorage) UpdateListener(_ context.Context, req *UpdateListenerRequest) (*Listener, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	listener, ok := s.Listeners[req.ListenerArn]
	if !ok {
		return nil, &ServiceError{Code: errListenerNotFound, Message: "Listener not found"}
	}

	if len(req.PortRanges) > 0 {
		portRanges := make([]PortRange, len(req.PortRanges))
		for i, pr := range req.PortRanges {
			portRanges[i] = PortRange(pr)
		}

		listener.PortRanges = portRanges
	}

	if req.Protocol != "" {
		listener.Protocol = Protocol(req.Protocol)
	}

	if req.ClientAffinity != "" {
		listener.ClientAffinity = ClientAffinity(req.ClientAffinity)
	}

	s.saveLocked()

	return listener, nil
}

// DeleteListener deletes a listener.
func (s *MemoryStorage) DeleteListener(_ context.Context, arn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.Listeners[arn]; !ok {
		return &ServiceError{Code: errListenerNotFound, Message: "Listener not found"}
	}

	// Delete associated endpoint groups.
	for egArn, eg := range s.EndpointGroups {
		if eg.ListenerArn == arn {
			delete(s.EndpointGroups, egArn)
		}
	}

	delete(s.Listeners, arn)

	s.saveLocked()

	return nil
}

// CreateEndpointGroup creates a new endpoint group.
func (s *MemoryStorage) CreateEndpointGroup(_ context.Context, req *CreateEndpointGroupRequest) (*EndpointGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify listener exists.
	if _, ok := s.Listeners[req.ListenerArn]; !ok {
		return nil, &ServiceError{Code: errListenerNotFound, Message: "Listener not found"}
	}

	endpointGroupID := uuid.New().String()
	arn := fmt.Sprintf("%s/endpoint-group/%s", req.ListenerArn, endpointGroupID)

	trafficDialPercentage := 100.0
	if req.TrafficDialPercentage != nil {
		trafficDialPercentage = *req.TrafficDialPercentage
	}

	healthCheckProtocol := HealthCheckProtocolTCP
	if req.HealthCheckProtocol != "" {
		healthCheckProtocol = HealthCheckProtocol(req.HealthCheckProtocol)
	}

	healthCheckIntervalSeconds := int32(30)
	if req.HealthCheckIntervalSeconds != nil {
		healthCheckIntervalSeconds = *req.HealthCheckIntervalSeconds
	}

	thresholdCount := int32(3)
	if req.ThresholdCount != nil {
		thresholdCount = *req.ThresholdCount
	}

	endpointGroup := &EndpointGroup{
		EndpointGroupArn:           arn,
		ListenerArn:                req.ListenerArn,
		EndpointGroupRegion:        req.EndpointGroupRegion,
		EndpointDescriptions:       convertEndpointConfigs(req.EndpointConfigurations),
		TrafficDialPercentage:      trafficDialPercentage,
		HealthCheckPort:            req.HealthCheckPort,
		HealthCheckProtocol:        healthCheckProtocol,
		HealthCheckPath:            req.HealthCheckPath,
		HealthCheckIntervalSeconds: healthCheckIntervalSeconds,
		ThresholdCount:             thresholdCount,
		PortOverrides:              convertPortOverrides(req.PortOverrides),
	}

	s.EndpointGroups[arn] = endpointGroup

	s.saveLocked()

	return endpointGroup, nil
}

// GetEndpointGroup retrieves an endpoint group by ARN.
func (s *MemoryStorage) GetEndpointGroup(_ context.Context, arn string) (*EndpointGroup, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	endpointGroup, ok := s.EndpointGroups[arn]
	if !ok {
		return nil, &ServiceError{Code: errEndpointNotFound, Message: "Endpoint group not found"}
	}

	return endpointGroup, nil
}

// ListEndpointGroups lists endpoint groups for a listener.
func (s *MemoryStorage) ListEndpointGroups(_ context.Context, listenerArn string, maxResults int32, _ string) ([]*EndpointGroup, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults <= 0 {
		maxResults = 100
	}

	endpointGroups := make([]*EndpointGroup, 0)

	for _, eg := range s.EndpointGroups {
		if eg.ListenerArn == listenerArn {
			endpointGroups = append(endpointGroups, eg)
			if len(endpointGroups) >= int(maxResults) {
				break
			}
		}
	}

	return endpointGroups, "", nil
}

// UpdateEndpointGroup updates an endpoint group.
func (s *MemoryStorage) UpdateEndpointGroup(_ context.Context, req *UpdateEndpointGroupRequest) (*EndpointGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	eg, ok := s.EndpointGroups[req.EndpointGroupArn]
	if !ok {
		return nil, &ServiceError{Code: errEndpointNotFound, Message: "Endpoint group not found"}
	}

	if len(req.EndpointConfigurations) > 0 {
		eg.EndpointDescriptions = convertEndpointConfigs(req.EndpointConfigurations)
	}

	if req.TrafficDialPercentage != nil {
		eg.TrafficDialPercentage = *req.TrafficDialPercentage
	}

	if req.HealthCheckPort != nil {
		eg.HealthCheckPort = req.HealthCheckPort
	}

	if req.HealthCheckProtocol != "" {
		eg.HealthCheckProtocol = HealthCheckProtocol(req.HealthCheckProtocol)
	}

	if req.HealthCheckPath != "" {
		eg.HealthCheckPath = req.HealthCheckPath
	}

	if req.HealthCheckIntervalSeconds != nil {
		eg.HealthCheckIntervalSeconds = *req.HealthCheckIntervalSeconds
	}

	if req.ThresholdCount != nil {
		eg.ThresholdCount = *req.ThresholdCount
	}

	if len(req.PortOverrides) > 0 {
		eg.PortOverrides = convertPortOverrides(req.PortOverrides)
	}

	s.saveLocked()

	return eg, nil
}

// DeleteEndpointGroup deletes an endpoint group.
func (s *MemoryStorage) DeleteEndpointGroup(_ context.Context, arn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.EndpointGroups[arn]; !ok {
		return &ServiceError{Code: errEndpointNotFound, Message: "Endpoint group not found"}
	}

	delete(s.EndpointGroups, arn)

	s.saveLocked()

	return nil
}

// generateIPAddress generates a simulated static IP address.
func generateIPAddress() string {
	// Use 75.2.x.x range which is typical for Global Accelerator.
	return fmt.Sprintf("75.2.%d.%d", randomByte(), randomByte())
}

// generateIPv6Address generates a simulated IPv6 address.
func generateIPv6Address() string {
	return fmt.Sprintf("2600:9000:a%03x::%d", randomByte(), randomByte())
}

// randomByte generates a random byte value using UUID.
func randomByte() int {
	id := uuid.New()

	return int(id[0])
}

// convertEndpointConfigs converts EndpointConfigInput to EndpointDescription.
func convertEndpointConfigs(configs []EndpointConfigInput) []EndpointDescription {
	endpoints := make([]EndpointDescription, len(configs))

	for i, ec := range configs {
		clientIPPreservation := false
		if ec.ClientIPPreservationEnabled != nil {
			clientIPPreservation = *ec.ClientIPPreservationEnabled
		}

		endpoints[i] = EndpointDescription{
			EndpointID:                  ec.EndpointID,
			Weight:                      ec.Weight,
			HealthState:                 HealthStateInitial,
			ClientIPPreservationEnabled: clientIPPreservation,
		}
	}

	return endpoints
}

// convertPortOverrides converts PortOverrideInput to PortOverride.
func convertPortOverrides(overrides []PortOverrideInput) []PortOverride {
	portOverrides := make([]PortOverride, len(overrides))

	for i, po := range overrides {
		portOverrides[i] = PortOverride(po)
	}

	return portOverrides
}
