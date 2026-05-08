package elbv2

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "000000000000"
)

// Storage defines the storage interface for ELB v2 service.
type Storage interface {
	CreateLoadBalancer(ctx context.Context, req *CreateLoadBalancerRequest) (*LoadBalancer, error)
	DeleteLoadBalancer(ctx context.Context, loadBalancerArn string) error
	DescribeLoadBalancers(ctx context.Context, arns, names []string) ([]*LoadBalancer, error)

	CreateTargetGroup(ctx context.Context, req *CreateTargetGroupRequest) (*TargetGroup, error)
	DeleteTargetGroup(ctx context.Context, targetGroupArn string) error
	DescribeTargetGroups(ctx context.Context, arns, names []string, lbArn string) ([]*TargetGroup, error)

	RegisterTargets(ctx context.Context, targetGroupArn string, targets []Target) error
	DeregisterTargets(ctx context.Context, targetGroupArn string, targets []Target) error

	CreateListener(ctx context.Context, req *CreateListenerRequest) (*Listener, error)
	DeleteListener(ctx context.Context, listenerArn string) error

	CreateRule(ctx context.Context, listenerArn, priority string, conditions []RuleCondition, actions []Action) (*Rule, error)
	DescribeRules(ctx context.Context, listenerArn string, ruleArns []string) ([]Rule, error)
	ModifyRule(ctx context.Context, ruleArn string, conditions []RuleCondition, actions []Action) (*Rule, error)
	DeleteRule(ctx context.Context, ruleArn string) error
	SetRulePriorities(ctx context.Context, priorities map[string]string) ([]Rule, error)

	ModifyLoadBalancerAttributes(ctx context.Context, lbArn string, attrs map[string]string) (map[string]string, error)
	DescribeLoadBalancerAttributes(ctx context.Context, lbArn string) (map[string]string, error)
	ModifyTargetGroupAttributes(ctx context.Context, tgArn string, attrs map[string]string) (map[string]string, error)
	DescribeTargetGroupAttributes(ctx context.Context, tgArn string) (map[string]string, error)

	DescribeListeners(ctx context.Context, listenerArns []string, lbArn string) ([]*Listener, error)
	ModifyListener(ctx context.Context, listenerArn string, port int, protocol string, defaultActions []Action) (*Listener, error)
	DescribeTargetHealth(ctx context.Context, targetGroupArn string, targets []Target) ([]TargetDescription, error)
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
	mu            sync.RWMutex             `json:"-"`
	LoadBalancers map[string]*LoadBalancer `json:"loadBalancers"` // keyed by ARN
	TargetGroups  map[string]*TargetGroup  `json:"targetGroups"`  // keyed by ARN
	Listeners     map[string]*Listener     `json:"listeners"`     // keyed by ARN
	Targets       map[string][]Target      `json:"targets"`       // keyed by targetGroupArn
	dataDir       string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		LoadBalancers: make(map[string]*LoadBalancer),
		TargetGroups:  make(map[string]*TargetGroup),
		Listeners:     make(map[string]*Listener),
		Targets:       make(map[string][]Target),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "elbv2", s)
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

	if m.LoadBalancers == nil {
		m.LoadBalancers = make(map[string]*LoadBalancer)
	}

	if m.TargetGroups == nil {
		m.TargetGroups = make(map[string]*TargetGroup)
	}

	if m.Listeners == nil {
		m.Listeners = make(map[string]*Listener)
	}

	if m.Targets == nil {
		m.Targets = make(map[string][]Target)
	}

	return nil
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "elbv2", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// loadBalancerDefaults holds default values for load balancer creation.
type loadBalancerDefaults struct {
	lbType        string
	scheme        string
	ipAddressType string
}

// getLoadBalancerDefaults returns default values for load balancer fields.
func getLoadBalancerDefaults(req *CreateLoadBalancerRequest) loadBalancerDefaults {
	defaults := loadBalancerDefaults{
		lbType:        req.Type,
		scheme:        req.Scheme,
		ipAddressType: req.IPAddressType,
	}

	if defaults.lbType == "" {
		defaults.lbType = "application"
	}

	if defaults.scheme == "" {
		defaults.scheme = "internet-facing"
	}

	if defaults.ipAddressType == "" {
		defaults.ipAddressType = "ipv4"
	}

	return defaults
}

// CreateLoadBalancer creates a new load balancer.
func (m *MemoryStorage) CreateLoadBalancer(_ context.Context, req *CreateLoadBalancerRequest) (*LoadBalancer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkDuplicateLoadBalancerName(req.Name); err != nil {
		return nil, err
	}

	defaults := getLoadBalancerDefaults(req)
	lb := m.buildLoadBalancer(req, defaults)
	m.LoadBalancers[lb.LoadBalancerArn] = lb

	return lb, nil
}

// checkDuplicateLoadBalancerName checks if a load balancer with the given name already exists.
func (m *MemoryStorage) checkDuplicateLoadBalancerName(name string) error {
	for _, lb := range m.LoadBalancers {
		if lb.LoadBalancerName == name {
			return &Error{
				Code:    "DuplicateLoadBalancerName",
				Message: fmt.Sprintf("A load balancer with the name '%s' already exists", name),
			}
		}
	}

	return nil
}

// buildLoadBalancer constructs a LoadBalancer from request and defaults.
func (m *MemoryStorage) buildLoadBalancer(req *CreateLoadBalancerRequest, defaults loadBalancerDefaults) *LoadBalancer {
	lbID := uuid.New().String()[:17]
	arn := fmt.Sprintf("arn:aws:elasticloadbalancing:%s:%s:loadbalancer/%s/%s/%s",
		defaultRegion, defaultAccountID, defaults.lbType[:3], req.Name, lbID)
	dnsName := fmt.Sprintf("%s-%s.%s.elb.amazonaws.com", req.Name, lbID[:8], defaultRegion)

	azs := make([]AvailabilityZone, 0, len(req.Subnets))
	for i, subnet := range req.Subnets {
		azs = append(azs, AvailabilityZone{
			ZoneName: fmt.Sprintf("%s%c", defaultRegion, 'a'+byte(i%3)),
			SubnetID: subnet,
		})
	}

	return &LoadBalancer{
		LoadBalancerArn:       arn,
		DNSName:               dnsName,
		CanonicalHostedZoneID: "Z35SXDOTRQ7X7K",
		CreatedTime:           time.Now(),
		LoadBalancerName:      req.Name,
		Scheme:                defaults.scheme,
		VpcID:                 "vpc-" + uuid.New().String()[:8],
		State:                 LoadBalancerState{Code: "active"},
		Type:                  defaults.lbType,
		AvailabilityZones:     azs,
		SecurityGroups:        req.SecurityGroups,
		IPAddressType:         defaults.ipAddressType,
	}
}

// DeleteLoadBalancer deletes a load balancer.
func (m *MemoryStorage) DeleteLoadBalancer(_ context.Context, loadBalancerArn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.LoadBalancers[loadBalancerArn]; !ok {
		return &Error{
			Code:    "LoadBalancerNotFound",
			Message: fmt.Sprintf("Load balancer '%s' not found", loadBalancerArn),
		}
	}

	// Delete associated listeners.
	for arn, listener := range m.Listeners {
		if listener.LoadBalancerArn == loadBalancerArn {
			delete(m.Listeners, arn)
		}
	}

	delete(m.LoadBalancers, loadBalancerArn)

	return nil
}

// DescribeLoadBalancers describes load balancers.
func (m *MemoryStorage) DescribeLoadBalancers(_ context.Context, arns, names []string) ([]*LoadBalancer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*LoadBalancer, 0)

	if len(arns) == 0 && len(names) == 0 {
		// Return all load balancers.
		for _, lb := range m.LoadBalancers {
			result = append(result, lb)
		}

		return result, nil
	}

	// Filter by ARNs.
	arnSet := make(map[string]bool)
	for _, arn := range arns {
		arnSet[arn] = true
	}

	// Filter by names.
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	for _, lb := range m.LoadBalancers {
		if len(arns) > 0 && arnSet[lb.LoadBalancerArn] {
			result = append(result, lb)

			continue
		}

		if len(names) > 0 && nameSet[lb.LoadBalancerName] {
			result = append(result, lb)
		}
	}

	return result, nil
}

// targetGroupDefaults holds default values for target group creation.
type targetGroupDefaults struct {
	targetType          string
	healthCheckPort     string
	healthCheckProtocol string
	healthCheckPath     string
	healthCheckInterval int
	healthCheckTimeout  int
	healthyThreshold    int
	unhealthyThreshold  int
}

// getTargetGroupDefaults returns default values for target group fields.
func getTargetGroupDefaults(req *CreateTargetGroupRequest) targetGroupDefaults {
	defaults := targetGroupDefaults{
		targetType:          req.TargetType,
		healthCheckPort:     req.HealthCheckPort,
		healthCheckProtocol: req.HealthCheckProtocol,
		healthCheckPath:     req.HealthCheckPath,
		healthCheckInterval: req.HealthCheckIntervalSeconds,
		healthCheckTimeout:  req.HealthCheckTimeoutSeconds,
		healthyThreshold:    req.HealthyThresholdCount,
		unhealthyThreshold:  req.UnhealthyThresholdCount,
	}

	if defaults.targetType == "" {
		defaults.targetType = "instance"
	}

	if defaults.healthCheckPort == "" {
		defaults.healthCheckPort = "traffic-port"
	}

	if defaults.healthCheckProtocol == "" {
		defaults.healthCheckProtocol = req.Protocol
		if defaults.healthCheckProtocol == "" {
			defaults.healthCheckProtocol = "HTTP"
		}
	}

	if defaults.healthCheckPath == "" && (defaults.healthCheckProtocol == "HTTP" || defaults.healthCheckProtocol == "HTTPS") {
		defaults.healthCheckPath = "/"
	}

	if defaults.healthCheckInterval == 0 {
		defaults.healthCheckInterval = 30
	}

	if defaults.healthCheckTimeout == 0 {
		defaults.healthCheckTimeout = 5
	}

	if defaults.healthyThreshold == 0 {
		defaults.healthyThreshold = 5
	}

	if defaults.unhealthyThreshold == 0 {
		defaults.unhealthyThreshold = 2
	}

	return defaults
}

// CreateTargetGroup creates a new target group.
func (m *MemoryStorage) CreateTargetGroup(_ context.Context, req *CreateTargetGroupRequest) (*TargetGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkDuplicateTargetGroupName(req.Name); err != nil {
		return nil, err
	}

	defaults := getTargetGroupDefaults(req)
	tg := m.buildTargetGroup(req, &defaults)
	m.TargetGroups[tg.TargetGroupArn] = tg
	m.Targets[tg.TargetGroupArn] = []Target{}

	return tg, nil
}

// checkDuplicateTargetGroupName checks if a target group with the given name already exists.
func (m *MemoryStorage) checkDuplicateTargetGroupName(name string) error {
	for _, tg := range m.TargetGroups {
		if tg.TargetGroupName == name {
			return &Error{
				Code:    "DuplicateTargetGroupName",
				Message: fmt.Sprintf("A target group with the name '%s' already exists", name),
			}
		}
	}

	return nil
}

// buildTargetGroup constructs a TargetGroup from request and defaults.
func (m *MemoryStorage) buildTargetGroup(req *CreateTargetGroupRequest, defaults *targetGroupDefaults) *TargetGroup {
	tgID := uuid.New().String()[:17]
	arn := fmt.Sprintf("arn:aws:elasticloadbalancing:%s:%s:targetgroup/%s/%s",
		defaultRegion, defaultAccountID, req.Name, tgID)

	return &TargetGroup{
		TargetGroupArn:             arn,
		TargetGroupName:            req.Name,
		Protocol:                   req.Protocol,
		Port:                       req.Port,
		VpcID:                      req.VpcID,
		HealthCheckEnabled:         true,
		HealthCheckIntervalSeconds: defaults.healthCheckInterval,
		HealthCheckPath:            defaults.healthCheckPath,
		HealthCheckPort:            defaults.healthCheckPort,
		HealthCheckProtocol:        defaults.healthCheckProtocol,
		HealthCheckTimeoutSeconds:  defaults.healthCheckTimeout,
		HealthyThresholdCount:      defaults.healthyThreshold,
		UnhealthyThresholdCount:    defaults.unhealthyThreshold,
		TargetType:                 defaults.targetType,
		LoadBalancerArns:           []string{},
	}
}

// DeleteTargetGroup deletes a target group.
func (m *MemoryStorage) DeleteTargetGroup(_ context.Context, targetGroupArn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.TargetGroups[targetGroupArn]; !ok {
		return &Error{
			Code:    "TargetGroupNotFound",
			Message: fmt.Sprintf("Target group '%s' not found", targetGroupArn),
		}
	}

	delete(m.TargetGroups, targetGroupArn)
	delete(m.Targets, targetGroupArn)

	return nil
}

// DescribeTargetGroups describes target groups.
func (m *MemoryStorage) DescribeTargetGroups(_ context.Context, arns, names []string, lbArn string) ([]*TargetGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*TargetGroup, 0)

	if len(arns) == 0 && len(names) == 0 && lbArn == "" {
		// Return all target groups.
		for _, tg := range m.TargetGroups {
			result = append(result, tg)
		}

		return result, nil
	}

	// Filter by ARNs.
	arnSet := make(map[string]bool)
	for _, arn := range arns {
		arnSet[arn] = true
	}

	// Filter by names.
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	for _, tg := range m.TargetGroups {
		if len(arns) > 0 && arnSet[tg.TargetGroupArn] {
			result = append(result, tg)

			continue
		}

		if len(names) > 0 && nameSet[tg.TargetGroupName] {
			result = append(result, tg)

			continue
		}

		if lbArn != "" && slices.Contains(tg.LoadBalancerArns, lbArn) {
			result = append(result, tg)
		}
	}

	return result, nil
}

// RegisterTargets registers targets with a target group.
func (m *MemoryStorage) RegisterTargets(_ context.Context, targetGroupArn string, targets []Target) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.TargetGroups[targetGroupArn]; !ok {
		return &Error{
			Code:    "TargetGroupNotFound",
			Message: fmt.Sprintf("Target group '%s' not found", targetGroupArn),
		}
	}

	existingTargets := m.Targets[targetGroupArn]
	existingSet := make(map[string]bool)

	for _, t := range existingTargets {
		existingSet[t.ID] = true
	}

	for _, t := range targets {
		if !existingSet[t.ID] {
			existingTargets = append(existingTargets, t)
		}
	}

	m.Targets[targetGroupArn] = existingTargets

	return nil
}

// DeregisterTargets deregisters targets from a target group.
func (m *MemoryStorage) DeregisterTargets(_ context.Context, targetGroupArn string, targets []Target) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.TargetGroups[targetGroupArn]; !ok {
		return &Error{
			Code:    "TargetGroupNotFound",
			Message: fmt.Sprintf("Target group '%s' not found", targetGroupArn),
		}
	}

	removeSet := make(map[string]bool)
	for _, t := range targets {
		removeSet[t.ID] = true
	}

	existingTargets := m.Targets[targetGroupArn]
	newTargets := make([]Target, 0, len(existingTargets))

	for _, t := range existingTargets {
		if !removeSet[t.ID] {
			newTargets = append(newTargets, t)
		}
	}

	m.Targets[targetGroupArn] = newTargets

	return nil
}

// CreateListener creates a new listener.
func (m *MemoryStorage) CreateListener(_ context.Context, req *CreateListenerRequest) (*Listener, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lb, ok := m.LoadBalancers[req.LoadBalancerArn]
	if !ok {
		return nil, &Error{
			Code:    "LoadBalancerNotFound",
			Message: fmt.Sprintf("Load balancer '%s' not found", req.LoadBalancerArn),
		}
	}

	listenerID := uuid.New().String()[:17]

	// Parse load balancer ID from ARN for listener ARN.
	lbIDStart := len(req.LoadBalancerArn) - 17
	lbID := req.LoadBalancerArn[lbIDStart:]

	// Get load balancer type from the ARN.
	lbType := lb.Type[:3]

	arn := fmt.Sprintf("arn:aws:elasticloadbalancing:%s:%s:listener/%s/%s/%s/%s",
		defaultRegion, defaultAccountID, lbType, lb.LoadBalancerName, lbID, listenerID)

	listener := &Listener{
		ListenerArn:     arn,
		LoadBalancerArn: req.LoadBalancerArn,
		Port:            req.Port,
		Protocol:        req.Protocol,
		DefaultActions:  req.DefaultActions,
	}

	m.Listeners[arn] = listener

	// Update target group's load balancer ARNs.
	for _, action := range req.DefaultActions {
		if action.TargetGroupArn != "" {
			if tg, exists := m.TargetGroups[action.TargetGroupArn]; exists {
				if !slices.Contains(tg.LoadBalancerArns, req.LoadBalancerArn) {
					tg.LoadBalancerArns = append(tg.LoadBalancerArns, req.LoadBalancerArn)
				}
			}
		}
	}

	return listener, nil
}

// DeleteListener deletes a listener.
func (m *MemoryStorage) DeleteListener(_ context.Context, listenerArn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.Listeners[listenerArn]; !ok {
		return &Error{
			Code:    "ListenerNotFound",
			Message: fmt.Sprintf("Listener '%s' not found", listenerArn),
		}
	}

	delete(m.Listeners, listenerArn)

	return nil
}

// CreateRule attaches a new rule to a listener.
func (m *MemoryStorage) CreateRule(_ context.Context, listenerArn, priority string, conditions []RuleCondition, actions []Action) (*Rule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	listener, ok := m.Listeners[listenerArn]
	if !ok {
		return nil, &Error{Code: "ListenerNotFound", Message: "Listener '" + listenerArn + "' not found"}
	}

	rule := Rule{
		RuleArn:    listenerArn + "/" + uuidLite(),
		Priority:   priority,
		Conditions: append([]RuleCondition(nil), conditions...),
		Actions:    append([]Action(nil), actions...),
		IsDefault:  false,
	}
	listener.Rules = append(listener.Rules, rule)

	return &listener.Rules[len(listener.Rules)-1], nil
}

// DescribeRules returns rules for a listener (or specific rule ARNs across
// all listeners).
func (m *MemoryStorage) DescribeRules(_ context.Context, listenerArn string, ruleArns []string) ([]Rule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Rule, 0)

	if listenerArn != "" {
		listener, ok := m.Listeners[listenerArn]
		if !ok {
			return nil, &Error{Code: "ListenerNotFound", Message: "Listener '" + listenerArn + "' not found"}
		}

		out = append(out, defaultRuleFor(listener))
		out = append(out, listener.Rules...)

		return out, nil
	}

	if len(ruleArns) == 0 {
		return out, nil
	}

	for _, listener := range m.Listeners {
		for _, rule := range listener.Rules {
			for _, want := range ruleArns {
				if rule.RuleArn == want {
					out = append(out, rule)
				}
			}
		}
	}

	return out, nil
}

// defaultRuleFor synthesizes the implicit default rule that AWS surfaces
// alongside explicitly-created rules.
func defaultRuleFor(listener *Listener) Rule {
	return Rule{
		RuleArn:   listener.ListenerArn + "/default",
		Priority:  "default",
		Actions:   append([]Action(nil), listener.DefaultActions...),
		IsDefault: true,
	}
}

// ModifyRule replaces the conditions and/or actions on a rule. nil slices
// leave the corresponding field as-is, matching AWS one-field-per-call
// semantics.
func (m *MemoryStorage) ModifyRule(_ context.Context, ruleArn string, conditions []RuleCondition, actions []Action) (*Rule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, listener := range m.Listeners {
		for i := range listener.Rules {
			if listener.Rules[i].RuleArn != ruleArn {
				continue
			}

			if conditions != nil {
				listener.Rules[i].Conditions = append([]RuleCondition(nil), conditions...)
			}

			if actions != nil {
				listener.Rules[i].Actions = append([]Action(nil), actions...)
			}

			return &listener.Rules[i], nil
		}
	}

	return nil, &Error{Code: "RuleNotFound", Message: "Rule '" + ruleArn + "' not found"}
}

// DeleteRule removes a rule by ARN.
func (m *MemoryStorage) DeleteRule(_ context.Context, ruleArn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, listener := range m.Listeners {
		for i := range listener.Rules {
			if listener.Rules[i].RuleArn == ruleArn {
				listener.Rules = append(listener.Rules[:i], listener.Rules[i+1:]...)

				return nil
			}
		}
	}

	return &Error{Code: "RuleNotFound", Message: "Rule '" + ruleArn + "' not found"}
}

// SetRulePriorities updates the priorities of one or more rules atomically.
func (m *MemoryStorage) SetRulePriorities(_ context.Context, priorities map[string]string) ([]Rule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	updated := make([]Rule, 0, len(priorities))

	for arn, prio := range priorities {
		found := false

		for _, listener := range m.Listeners {
			for i := range listener.Rules {
				if listener.Rules[i].RuleArn == arn {
					listener.Rules[i].Priority = prio
					updated = append(updated, listener.Rules[i])
					found = true

					break
				}
			}

			if found {
				break
			}
		}

		if !found {
			return nil, &Error{Code: "RuleNotFound", Message: "Rule '" + arn + "' not found"}
		}
	}

	return updated, nil
}

func uuidLite() string {
	return fmt.Sprintf("rule-%d", time.Now().UnixNano())
}

// defaultLoadBalancerAttributes returns the AWS-default attribute set
// surfaced on a freshly-created load balancer.
func defaultLoadBalancerAttributes() map[string]string {
	return map[string]string{
		"access_logs.s3.enabled":                          "false",
		"access_logs.s3.bucket":                           "",
		"access_logs.s3.prefix":                           "",
		"deletion_protection.enabled":                     "false",
		"idle_timeout.timeout_seconds":                    "60",
		"routing.http2.enabled":                           "true",
		"routing.http.drop_invalid_header_fields.enabled": "false",
		"load_balancing.cross_zone.enabled":               "true",
	}
}

// defaultTargetGroupAttributes returns the AWS-default attribute set for a
// freshly-created target group.
func defaultTargetGroupAttributes() map[string]string {
	return map[string]string{
		"deregistration_delay.timeout_seconds":  "300",
		"stickiness.enabled":                    "false",
		"stickiness.type":                       "lb_cookie",
		"stickiness.lb_cookie.duration_seconds": "86400",
		"slow_start.duration_seconds":           "0",
		"load_balancing.algorithm.type":         "round_robin",
		"proxy_protocol_v2.enabled":             "false",
	}
}

// ModifyLoadBalancerAttributes upserts attributes on a load balancer.
func (m *MemoryStorage) ModifyLoadBalancerAttributes(_ context.Context, lbArn string, attrs map[string]string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lb, ok := m.LoadBalancers[lbArn]
	if !ok {
		return nil, &Error{Code: "LoadBalancerNotFound", Message: "Load balancer '" + lbArn + "' not found"}
	}

	if lb.Attributes == nil {
		lb.Attributes = defaultLoadBalancerAttributes()
	}

	for k, v := range attrs {
		lb.Attributes[k] = v
	}

	return cloneAttributes(lb.Attributes), nil
}

// DescribeLoadBalancerAttributes returns the attribute set, lazy-initializing
// to the AWS defaults if Modify has never been called.
func (m *MemoryStorage) DescribeLoadBalancerAttributes(_ context.Context, lbArn string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lb, ok := m.LoadBalancers[lbArn]
	if !ok {
		return nil, &Error{Code: "LoadBalancerNotFound", Message: "Load balancer '" + lbArn + "' not found"}
	}

	if lb.Attributes == nil {
		lb.Attributes = defaultLoadBalancerAttributes()
	}

	return cloneAttributes(lb.Attributes), nil
}

// ModifyTargetGroupAttributes upserts attributes on a target group.
func (m *MemoryStorage) ModifyTargetGroupAttributes(_ context.Context, tgArn string, attrs map[string]string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tg, ok := m.TargetGroups[tgArn]
	if !ok {
		return nil, &Error{Code: "TargetGroupNotFound", Message: "Target group '" + tgArn + "' not found"}
	}

	if tg.Attributes == nil {
		tg.Attributes = defaultTargetGroupAttributes()
	}

	for k, v := range attrs {
		tg.Attributes[k] = v
	}

	return cloneAttributes(tg.Attributes), nil
}

// DescribeTargetGroupAttributes returns the attribute set with AWS defaults.
func (m *MemoryStorage) DescribeTargetGroupAttributes(_ context.Context, tgArn string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tg, ok := m.TargetGroups[tgArn]
	if !ok {
		return nil, &Error{Code: "TargetGroupNotFound", Message: "Target group '" + tgArn + "' not found"}
	}

	if tg.Attributes == nil {
		tg.Attributes = defaultTargetGroupAttributes()
	}

	return cloneAttributes(tg.Attributes), nil
}

func cloneAttributes(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}

	return out
}

// DescribeListeners returns listeners by ARN list or by parent load balancer.
func (m *MemoryStorage) DescribeListeners(_ context.Context, listenerArns []string, lbArn string) ([]*Listener, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*Listener, 0)

	if len(listenerArns) > 0 {
		for _, arn := range listenerArns {
			listener, ok := m.Listeners[arn]
			if !ok {
				return nil, &Error{Code: "ListenerNotFound", Message: "Listener '" + arn + "' not found"}
			}

			out = append(out, listener)
		}

		return out, nil
	}

	for _, listener := range m.Listeners {
		if lbArn == "" || listener.LoadBalancerArn == lbArn {
			out = append(out, listener)
		}
	}

	return out, nil
}

// ModifyListener replaces port / protocol / default actions on a listener.
// A zero port means "do not change"; an empty protocol or nil DefaultActions
// likewise.
func (m *MemoryStorage) ModifyListener(_ context.Context, listenerArn string, port int, protocol string, defaultActions []Action) (*Listener, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	listener, ok := m.Listeners[listenerArn]
	if !ok {
		return nil, &Error{Code: "ListenerNotFound", Message: "Listener '" + listenerArn + "' not found"}
	}

	if port != 0 {
		listener.Port = port
	}

	if protocol != "" {
		listener.Protocol = protocol
	}

	if defaultActions != nil {
		listener.DefaultActions = append([]Action(nil), defaultActions...)
	}

	return listener, nil
}

// DescribeTargetHealth returns the health of registered targets in a target
// group. kumo does not run real health checks, so every registered target is
// reported as "healthy". An empty Targets request returns the full set; a
// non-empty Targets request filters to those exact targets and reports
// "unused" for any target not currently registered.
func (m *MemoryStorage) DescribeTargetHealth(_ context.Context, targetGroupArn string, targets []Target) ([]TargetDescription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.TargetGroups[targetGroupArn]; !ok {
		return nil, &Error{Code: "TargetGroupNotFound", Message: "Target group '" + targetGroupArn + "' not found"}
	}

	registered := m.Targets[targetGroupArn]

	if len(targets) == 0 {
		out := make([]TargetDescription, 0, len(registered))
		for _, t := range registered {
			out = append(out, TargetDescription{Target: t, HealthState: "healthy"})
		}

		return out, nil
	}

	out := make([]TargetDescription, 0, len(targets))

	for _, want := range targets {
		state := "unused"

		for _, t := range registered {
			if t.ID == want.ID && (want.Port == 0 || t.Port == want.Port) {
				state = "healthy"

				break
			}
		}

		out = append(out, TargetDescription{Target: want, HealthState: state})
	}

	return out, nil
}
