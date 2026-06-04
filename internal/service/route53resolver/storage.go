package route53resolver

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

const (
	errResourceNotFound = "ResourceNotFoundException"
	errResourceExists   = "ResourceExistsException"
	errResourceInUse    = "ResourceInUseException"
	errInvalidParameter = "InvalidParameterException"

	defaultRegion  = "us-east-1"
	statusDeleting = "DELETING"
)

// Storage is the interface for Route 53 Resolver storage operations.
type Storage interface {
	// Resolver Endpoints
	CreateResolverEndpoint(ctx context.Context, req *CreateResolverEndpointRequest) (*ResolverEndpoint, error)
	GetResolverEndpoint(ctx context.Context, id string) (*ResolverEndpoint, error)
	DeleteResolverEndpoint(ctx context.Context, id string) (*ResolverEndpoint, error)
	ListResolverEndpoints(ctx context.Context, maxResults int, nextToken string) ([]*ResolverEndpoint, string, error)

	// Resolver Rules
	CreateResolverRule(ctx context.Context, req *CreateResolverRuleRequest) (*ResolverRule, error)
	GetResolverRule(ctx context.Context, id string) (*ResolverRule, error)
	DeleteResolverRule(ctx context.Context, id string) (*ResolverRule, error)
	ListResolverRules(ctx context.Context, maxResults int, nextToken string) ([]*ResolverRule, string, error)

	// Resolver Rule Associations
	AssociateResolverRule(ctx context.Context, req *AssociateResolverRuleRequest) (*ResolverRuleAssociation, error)
	DisassociateResolverRule(ctx context.Context, ruleID, vpcID string) (*ResolverRuleAssociation, error)
	ListResolverRuleAssociations(ctx context.Context, maxResults int, nextToken string) ([]*ResolverRuleAssociation, string, error)
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

// MemoryStorage implements in-memory storage for Route 53 Resolver.
type MemoryStorage struct {
	mu           sync.RWMutex                        `json:"-"`
	Endpoints    map[string]*ResolverEndpoint        `json:"endpoints"`
	Rules        map[string]*ResolverRule            `json:"rules"`
	Associations map[string]*ResolverRuleAssociation `json:"associations"`
	accountID    string
	region       string
	dataDir      string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Endpoints:    make(map[string]*ResolverEndpoint),
		Rules:        make(map[string]*ResolverRule),
		Associations: make(map[string]*ResolverRuleAssociation),
		accountID:    "123456789012",
		region:       region,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "route53resolver", s)
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

	if s.Endpoints == nil {
		s.Endpoints = make(map[string]*ResolverEndpoint)
	}

	if s.Rules == nil {
		s.Rules = make(map[string]*ResolverRule)
	}

	if s.Associations == nil {
		s.Associations = make(map[string]*ResolverRuleAssociation)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "route53resolver", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "route53resolver", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateResolverEndpoint creates a new resolver endpoint.
func (s *MemoryStorage) CreateResolverEndpoint(_ context.Context, req *CreateResolverEndpointRequest) (*ResolverEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := "rslvr-" + req.Direction[:2] + "-" + uuid.New().String()[:8]
	now := time.Now().UTC().Format(time.RFC3339)

	// Extract VPC ID from first subnet (simplified)
	vpcID := "vpc-" + uuid.New().String()[:8]

	ipAddresses := make([]*IPAddressResponse, 0, len(req.IPAddresses))

	for _, ipReq := range req.IPAddresses {
		ipAddr := &IPAddressResponse{
			IPAddressID:      "rslvr-ip-" + uuid.New().String()[:8],
			SubnetID:         ipReq.SubnetID,
			IP:               ipReq.IP,
			IPv6:             ipReq.IPv6,
			Status:           "ATTACHED",
			CreationTime:     now,
			ModificationTime: now,
		}

		if ipAddr.IP == "" {
			ipAddr.IP = fmt.Sprintf("10.0.%d.%d", len(ipAddresses), len(ipAddresses)+1)
		}

		ipAddresses = append(ipAddresses, ipAddr)
	}

	endpoint := &ResolverEndpoint{
		ID:                    id,
		CreatorRequestID:      req.CreatorRequestID,
		ARN:                   fmt.Sprintf("arn:aws:route53resolver:%s:%s:resolver-endpoint/%s", s.region, s.accountID, id),
		Name:                  req.Name,
		SecurityGroupIDs:      req.SecurityGroupIDs,
		Direction:             req.Direction,
		IPAddressCount:        len(ipAddresses),
		HostVPCID:             vpcID,
		Status:                "OPERATIONAL",
		CreationTime:          now,
		ModificationTime:      now,
		OutpostArn:            req.OutpostArn,
		PreferredInstanceType: req.PreferredInstanceType,
		ResolverEndpointType:  req.ResolverEndpointType,
		Protocols:             req.Protocols,
		IPAddresses:           ipAddresses,
	}

	if endpoint.ResolverEndpointType == "" {
		endpoint.ResolverEndpointType = "IPV4"
	}

	if len(endpoint.Protocols) == 0 {
		endpoint.Protocols = []string{"Do53"}
	}

	s.Endpoints[id] = endpoint

	s.saveLocked()

	return endpoint, nil
}

// GetResolverEndpoint retrieves a resolver endpoint by ID.
func (s *MemoryStorage) GetResolverEndpoint(_ context.Context, id string) (*ResolverEndpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	endpoint, exists := s.Endpoints[id]
	if !exists {
		return nil, &ResolverError{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Resolver endpoint with ID '%s' does not exist", id),
		}
	}

	return endpoint, nil
}

// DeleteResolverEndpoint deletes a resolver endpoint.
func (s *MemoryStorage) DeleteResolverEndpoint(_ context.Context, id string) (*ResolverEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	endpoint, exists := s.Endpoints[id]
	if !exists {
		return nil, &ResolverError{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Resolver endpoint with ID '%s' does not exist", id),
		}
	}

	endpoint.Status = statusDeleting

	delete(s.Endpoints, id)

	s.saveLocked()

	return endpoint, nil
}

// ListResolverEndpoints lists resolver endpoints.
func (s *MemoryStorage) ListResolverEndpoints(_ context.Context, maxResults int, _ string) ([]*ResolverEndpoint, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults == 0 {
		maxResults = 10
	}

	endpoints := make([]*ResolverEndpoint, 0, len(s.Endpoints))
	for _, endpoint := range s.Endpoints {
		endpoints = append(endpoints, endpoint)
		if len(endpoints) >= maxResults {
			break
		}
	}

	return endpoints, "", nil
}

// CreateResolverRule creates a new resolver rule.
func (s *MemoryStorage) CreateResolverRule(_ context.Context, req *CreateResolverRuleRequest) (*ResolverRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := "rslvr-rr-" + uuid.New().String()[:8]
	now := time.Now().UTC().Format(time.RFC3339)

	targetIPs := make([]*TargetAddress, 0, len(req.TargetIPs))
	for _, t := range req.TargetIPs {
		targetIPs = append(targetIPs, &TargetAddress{
			IP:       t.IP,
			Port:     t.Port,
			IPv6:     t.IPv6,
			Protocol: t.Protocol,
		})
	}

	rule := &ResolverRule{
		ID:                 id,
		CreatorRequestID:   req.CreatorRequestID,
		ARN:                fmt.Sprintf("arn:aws:route53resolver:%s:%s:resolver-rule/%s", s.region, s.accountID, id),
		DomainName:         req.DomainName,
		Status:             "COMPLETE",
		RuleType:           req.RuleType,
		Name:               req.Name,
		TargetIPs:          targetIPs,
		ResolverEndpointID: req.ResolverEndpointID,
		OwnerID:            s.accountID,
		ShareStatus:        "NOT_SHARED",
		CreationTime:       now,
		ModificationTime:   now,
	}

	s.Rules[id] = rule

	s.saveLocked()

	return rule, nil
}

// GetResolverRule retrieves a resolver rule by ID.
func (s *MemoryStorage) GetResolverRule(_ context.Context, id string) (*ResolverRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rule, exists := s.Rules[id]
	if !exists {
		return nil, &ResolverError{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Resolver rule with ID '%s' does not exist", id),
		}
	}

	return rule, nil
}

// DeleteResolverRule deletes a resolver rule.
func (s *MemoryStorage) DeleteResolverRule(_ context.Context, id string) (*ResolverRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rule, exists := s.Rules[id]
	if !exists {
		return nil, &ResolverError{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Resolver rule with ID '%s' does not exist", id),
		}
	}

	// Check if rule is associated with any VPC
	for _, assoc := range s.Associations {
		if assoc.ResolverRuleID == id {
			return nil, &ResolverError{
				Code:    errResourceInUse,
				Message: fmt.Sprintf("Resolver rule '%s' is still associated with VPCs", id),
			}
		}
	}

	rule.Status = statusDeleting

	delete(s.Rules, id)

	s.saveLocked()

	return rule, nil
}

// ListResolverRules lists resolver rules.
func (s *MemoryStorage) ListResolverRules(_ context.Context, maxResults int, _ string) ([]*ResolverRule, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults == 0 {
		maxResults = 10
	}

	rules := make([]*ResolverRule, 0, len(s.Rules))
	for _, rule := range s.Rules {
		rules = append(rules, rule)
		if len(rules) >= maxResults {
			break
		}
	}

	return rules, "", nil
}

// AssociateResolverRule associates a resolver rule with a VPC.
func (s *MemoryStorage) AssociateResolverRule(_ context.Context, req *AssociateResolverRuleRequest) (*ResolverRuleAssociation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if rule exists
	if _, exists := s.Rules[req.ResolverRuleID]; !exists {
		return nil, &ResolverError{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Resolver rule with ID '%s' does not exist", req.ResolverRuleID),
		}
	}

	// Check if association already exists
	for _, assoc := range s.Associations {
		if assoc.ResolverRuleID == req.ResolverRuleID && assoc.VPCID == req.VPCID {
			return nil, &ResolverError{
				Code:    errResourceExists,
				Message: fmt.Sprintf("Resolver rule '%s' is already associated with VPC '%s'", req.ResolverRuleID, req.VPCID),
			}
		}
	}

	id := "rslvr-rrassoc-" + uuid.New().String()[:8]

	assoc := &ResolverRuleAssociation{
		ID:             id,
		ResolverRuleID: req.ResolverRuleID,
		Name:           req.Name,
		VPCID:          req.VPCID,
		Status:         "COMPLETE",
	}

	s.Associations[id] = assoc

	s.saveLocked()

	return assoc, nil
}

// DisassociateResolverRule disassociates a resolver rule from a VPC.
func (s *MemoryStorage) DisassociateResolverRule(_ context.Context, ruleID, vpcID string) (*ResolverRuleAssociation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, assoc := range s.Associations {
		if assoc.ResolverRuleID == ruleID && assoc.VPCID == vpcID {
			assoc.Status = statusDeleting

			delete(s.Associations, id)

			s.saveLocked()

			return assoc, nil
		}
	}

	return nil, &ResolverError{
		Code:    errResourceNotFound,
		Message: fmt.Sprintf("Association between resolver rule '%s' and VPC '%s' does not exist", ruleID, vpcID),
	}
}

// ListResolverRuleAssociations lists resolver rule associations.
func (s *MemoryStorage) ListResolverRuleAssociations(_ context.Context, maxResults int, _ string) ([]*ResolverRuleAssociation, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults == 0 {
		maxResults = 10
	}

	associations := make([]*ResolverRuleAssociation, 0, len(s.Associations))
	for _, assoc := range s.Associations {
		associations = append(associations, assoc)
		if len(associations) >= maxResults {
			break
		}
	}

	return associations, "", nil
}
