package eventbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Default event bus name.
const defaultEventBusName = "default"

// Error codes.
const (
	errEventBusNotFound      = "ResourceNotFoundException"
	errEventBusAlreadyExists = "ResourceAlreadyExistsException"
	errRuleNotFound          = "ResourceNotFoundException"
	errInvalidParameter      = "ValidationException"
)

// Storage defines the EventBridge storage interface.
type Storage interface {
	// Event Bus operations.
	CreateEventBus(ctx context.Context, req *CreateEventBusRequest) (*EventBus, error)
	DeleteEventBus(ctx context.Context, name string) error
	DescribeEventBus(ctx context.Context, name string) (*EventBus, error)
	ListEventBuses(ctx context.Context, namePrefix string, limit int32, nextToken string) ([]*EventBus, string, error)

	// Rule operations.
	PutRule(ctx context.Context, req *PutRuleRequest) (*Rule, error)
	DeleteRule(ctx context.Context, eventBusName, ruleName string, force bool) error
	DescribeRule(ctx context.Context, eventBusName, ruleName string) (*Rule, error)
	ListRules(ctx context.Context, eventBusName, namePrefix string, limit int32, nextToken string) ([]*Rule, string, error)

	// Target operations.
	PutTargets(ctx context.Context, eventBusName, ruleName string, targets []TargetInput) ([]PutTargetsResultEntry, error)
	RemoveTargets(ctx context.Context, eventBusName, ruleName string, ids []string, force bool) ([]RemoveTargetsResultEntry, error)
	ListTargetsByRule(ctx context.Context, eventBusName, ruleName string, limit int32, nextToken string) ([]*Target, string, error)

	// Event operations.
	PutEvents(ctx context.Context, entries []PutEventsRequestEntry) ([]PutEventsResultEntry, error)
	GetDeliveredEvents(ctx context.Context) []DeliveredEvent

	// Connection operations.
	CreateConnection(ctx context.Context, req *CreateConnectionRequest) (*Connection, error)
	DescribeConnection(ctx context.Context, name string) (*Connection, error)
	DeleteConnection(ctx context.Context, name string) (*Connection, error)

	// API Destination operations.
	CreateAPIDestination(ctx context.Context, req *CreateAPIDestinationRequest) (*APIDestination, error)
	DescribeAPIDestination(ctx context.Context, name string) (*APIDestination, error)
	DeleteAPIDestination(ctx context.Context, name string) error

	// DispatchAction dispatches the request to the appropriate handler.
	DispatchAction(action string) bool
}

// Option is a configuration option for MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables persistent storage in the specified directory.
func WithDataDir(dir string) Option {
	return func(s *MemoryStorage) {
		s.dataDir = dir
	}
}

// WithBaseURL sets the base URL for internal service-to-service communication.
func WithBaseURL(url string) Option {
	return func(s *MemoryStorage) {
		s.baseURL = url
	}
}

// Compile-time interface checks.
var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MemoryStorage implements Storage with in-memory data.
type MemoryStorage struct {
	mu              sync.RWMutex                    `json:"-"`
	EventBuses      map[string]*EventBus            `json:"eventBuses"`
	Rules           map[string]map[string]*Rule     `json:"rules"`
	Targets         map[string]map[string][]*Target `json:"targets"`
	Connections     map[string]*Connection          `json:"connections"`
	APIDestinations map[string]*APIDestination      `json:"apiDestinations"`
	DeliveredEvents []DeliveredEvent                `json:"deliveredEvents"`
	region          string
	accountID       string
	dataDir         string
	baseURL         string
	logger          *slog.Logger
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		EventBuses:      make(map[string]*EventBus),
		Rules:           make(map[string]map[string]*Rule),
		Targets:         make(map[string]map[string][]*Target),
		Connections:     make(map[string]*Connection),
		APIDestinations: make(map[string]*APIDestination),
		region:          "us-east-1",
		accountID:       "000000000000",
		baseURL:         "http://localhost:4566",
		logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "eventbridge", s)
	}

	// Create default event bus if not present.
	if _, exists := s.EventBuses[defaultEventBusName]; !exists {
		now := time.Now()
		s.EventBuses[defaultEventBusName] = &EventBus{
			Name:         defaultEventBusName,
			Arn:          fmt.Sprintf("arn:aws:events:%s:%s:event-bus/%s", s.region, s.accountID, defaultEventBusName),
			CreationTime: now,
			LastModified: now,
		}
		s.Rules[defaultEventBusName] = make(map[string]*Rule)
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

	if s.EventBuses == nil {
		s.EventBuses = make(map[string]*EventBus)
	}

	if s.Rules == nil {
		s.Rules = make(map[string]map[string]*Rule)
	}

	if s.Targets == nil {
		s.Targets = make(map[string]map[string][]*Target)
	}

	return nil
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "eventbridge", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateEventBus creates a new event bus.
func (s *MemoryStorage) CreateEventBus(_ context.Context, req *CreateEventBusRequest) (*EventBus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.EventBuses[req.Name]; exists {
		return nil, &ServiceError{Code: errEventBusAlreadyExists, Message: "Event bus already exists"}
	}

	now := time.Now()
	eventBus := &EventBus{
		Name:         req.Name,
		Arn:          fmt.Sprintf("arn:aws:events:%s:%s:event-bus/%s", s.region, s.accountID, req.Name),
		Description:  req.Description,
		CreationTime: now,
		LastModified: now,
	}

	s.EventBuses[req.Name] = eventBus
	s.Rules[req.Name] = make(map[string]*Rule)

	return eventBus, nil
}

// DeleteEventBus deletes an event bus.
func (s *MemoryStorage) DeleteEventBus(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == defaultEventBusName {
		return &ServiceError{Code: errInvalidParameter, Message: "Cannot delete the default event bus"}
	}

	if _, exists := s.EventBuses[name]; !exists {
		return &ServiceError{Code: errEventBusNotFound, Message: "Event bus not found"}
	}

	delete(s.EventBuses, name)
	delete(s.Rules, name)

	// Delete all targets for rules on this event bus.
	for key := range s.Targets {
		if strings.HasPrefix(key, name+":") {
			delete(s.Targets, key)
		}
	}

	return nil
}

// DescribeEventBus describes an event bus.
func (s *MemoryStorage) DescribeEventBus(_ context.Context, name string) (*EventBus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if name == "" {
		name = defaultEventBusName
	}

	eventBus, exists := s.EventBuses[name]
	if !exists {
		return nil, &ServiceError{Code: errEventBusNotFound, Message: "Event bus not found"}
	}

	return eventBus, nil
}

// ListEventBuses lists event buses.
func (s *MemoryStorage) ListEventBuses(_ context.Context, namePrefix string, limit int32, _ string) ([]*EventBus, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	var eventBuses []*EventBus

	for _, eb := range s.EventBuses {
		if namePrefix == "" || strings.HasPrefix(eb.Name, namePrefix) {
			eventBuses = append(eventBuses, eb)
		}

		if int32(len(eventBuses)) >= limit { //nolint:gosec // slice length bounded by limit parameter
			break
		}
	}

	return eventBuses, "", nil
}

// PutRule creates or updates a rule.
func (s *MemoryStorage) PutRule(_ context.Context, req *PutRuleRequest) (*Rule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	eventBusName := req.EventBusName
	if eventBusName == "" {
		eventBusName = defaultEventBusName
	}

	if _, exists := s.EventBuses[eventBusName]; !exists {
		return nil, &ServiceError{Code: errEventBusNotFound, Message: "Event bus not found"}
	}

	now := time.Now()
	state := RuleStateEnabled

	if req.State == "DISABLED" {
		state = RuleStateDisabled
	}

	rule := &Rule{
		Name:               req.Name,
		Arn:                fmt.Sprintf("arn:aws:events:%s:%s:rule/%s/%s", s.region, s.accountID, eventBusName, req.Name),
		EventBusName:       eventBusName,
		EventPattern:       req.EventPattern,
		ScheduleExpression: req.ScheduleExpression,
		State:              state,
		Description:        req.Description,
		RoleArn:            req.RoleArn,
		CreationTime:       now,
		LastModified:       now,
	}

	if s.Rules[eventBusName] == nil {
		s.Rules[eventBusName] = make(map[string]*Rule)
	}

	s.Rules[eventBusName][req.Name] = rule

	return rule, nil
}

// DeleteRule deletes a rule.
func (s *MemoryStorage) DeleteRule(_ context.Context, eventBusName, ruleName string, _ bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if eventBusName == "" {
		eventBusName = defaultEventBusName
	}

	rules, exists := s.Rules[eventBusName]
	if !exists {
		return &ServiceError{Code: errRuleNotFound, Message: "Rule not found"}
	}

	if _, exists := rules[ruleName]; !exists {
		return &ServiceError{Code: errRuleNotFound, Message: "Rule not found"}
	}

	delete(rules, ruleName)

	// Delete targets for this rule.
	targetKey := eventBusName + ":" + ruleName
	delete(s.Targets, targetKey)

	return nil
}

// DescribeRule describes a rule.
func (s *MemoryStorage) DescribeRule(_ context.Context, eventBusName, ruleName string) (*Rule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if eventBusName == "" {
		eventBusName = defaultEventBusName
	}

	rules, exists := s.Rules[eventBusName]
	if !exists {
		return nil, &ServiceError{Code: errRuleNotFound, Message: "Rule not found"}
	}

	rule, exists := rules[ruleName]
	if !exists {
		return nil, &ServiceError{Code: errRuleNotFound, Message: "Rule not found"}
	}

	return rule, nil
}

// ListRules lists rules for an event bus.
func (s *MemoryStorage) ListRules(_ context.Context, eventBusName, namePrefix string, limit int32, _ string) ([]*Rule, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if eventBusName == "" {
		eventBusName = defaultEventBusName
	}

	if limit <= 0 {
		limit = 10
	}

	rules, exists := s.Rules[eventBusName]
	if !exists {
		return nil, "", nil
	}

	var result []*Rule

	for _, rule := range rules {
		if namePrefix == "" || strings.HasPrefix(rule.Name, namePrefix) {
			result = append(result, rule)
		}

		if int32(len(result)) >= limit { //nolint:gosec // slice length bounded by limit parameter
			break
		}
	}

	return result, "", nil
}

// PutTargets adds targets to a rule.
func (s *MemoryStorage) PutTargets(_ context.Context, eventBusName, ruleName string, targets []TargetInput) ([]PutTargetsResultEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if eventBusName == "" {
		eventBusName = defaultEventBusName
	}

	rules, exists := s.Rules[eventBusName]
	if !exists {
		return nil, &ServiceError{Code: errRuleNotFound, Message: "Rule not found"}
	}

	if _, exists := rules[ruleName]; !exists {
		return nil, &ServiceError{Code: errRuleNotFound, Message: "Rule not found"}
	}

	targetKey := eventBusName + ":" + ruleName

	if s.Targets[targetKey] == nil {
		s.Targets[targetKey] = make(map[string][]*Target)
	}

	var failedEntries []PutTargetsResultEntry

	for _, t := range targets {
		target := &Target{
			ID:             t.ID,
			Arn:            t.Arn,
			RoleArn:        t.RoleArn,
			Input:          t.Input,
			InputPath:      t.InputPath,
			HTTPParameters: t.HTTPParameters,
		}

		// Find and update existing target or add new one.
		found := false
		existingTargets := s.Targets[targetKey][ruleName]

		for i, existing := range existingTargets {
			if existing.ID == t.ID {
				existingTargets[i] = target
				found = true

				break
			}
		}

		if !found {
			s.Targets[targetKey][ruleName] = append(s.Targets[targetKey][ruleName], target)
		}
	}

	return failedEntries, nil
}

// RemoveTargets removes targets from a rule.
func (s *MemoryStorage) RemoveTargets(_ context.Context, eventBusName, ruleName string, ids []string, _ bool) ([]RemoveTargetsResultEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if eventBusName == "" {
		eventBusName = defaultEventBusName
	}

	targetKey := eventBusName + ":" + ruleName

	var failedEntries []RemoveTargetsResultEntry

	if s.Targets[targetKey] == nil {
		return failedEntries, nil
	}

	existingTargets := s.Targets[targetKey][ruleName]

	var newTargets []*Target

	idsToRemove := make(map[string]bool)
	for _, id := range ids {
		idsToRemove[id] = true
	}

	for _, target := range existingTargets {
		if !idsToRemove[target.ID] {
			newTargets = append(newTargets, target)
		}
	}

	s.Targets[targetKey][ruleName] = newTargets

	return failedEntries, nil
}

// ListTargetsByRule lists targets for a rule.
func (s *MemoryStorage) ListTargetsByRule(_ context.Context, eventBusName, ruleName string, limit int32, _ string) ([]*Target, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if eventBusName == "" {
		eventBusName = defaultEventBusName
	}

	if limit <= 0 {
		limit = 100
	}

	targetKey := eventBusName + ":" + ruleName

	if s.Targets[targetKey] == nil {
		return nil, "", nil
	}

	targets := s.Targets[targetKey][ruleName]

	if int32(len(targets)) > limit { //nolint:gosec // slice length bounded by limit parameter
		targets = targets[:limit]
	}

	return targets, "", nil
}

// PutEvents puts events to the event bus, matches against rules, and records deliveries.
func (s *MemoryStorage) PutEvents(_ context.Context, entries []PutEventsRequestEntry) ([]PutEventsResultEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := make([]PutEventsResultEntry, len(entries))

	for i, entry := range entries {
		eventID := uuid.New().String()
		results[i] = PutEventsResultEntry{EventID: eventID}

		eventBusName := entry.EventBusName
		if eventBusName == "" {
			eventBusName = defaultEventBusName
		}

		s.matchAndDeliver(eventID, eventBusName, &entry)
	}

	return results, nil
}

// matchAndDeliver matches an event against rules, records deliveries, and performs HTTP delivery for API destinations. Must be called under lock.
func (s *MemoryStorage) matchAndDeliver(eventID, eventBusName string, entry *PutEventsRequestEntry) {
	rules, exists := s.Rules[eventBusName]
	if !exists {
		return
	}

	for _, rule := range rules {
		if rule.State != RuleStateEnabled {
			continue
		}

		if !matchEventPattern(rule.EventPattern, entry) {
			continue
		}

		targetKey := eventBusName + ":" + rule.Name
		ruleTargets := s.Targets[targetKey][rule.Name]

		var eventTime string
		if entry.Time != nil {
			eventTime = entry.Time.Format(time.RFC3339)
		}

		for _, target := range ruleTargets {
			s.DeliveredEvents = append(s.DeliveredEvents, DeliveredEvent{
				EventID:      eventID,
				Source:       entry.Source,
				DetailType:   entry.DetailType,
				Detail:       entry.Detail,
				EventBusName: eventBusName,
				RuleName:     rule.Name,
				TargetID:     target.ID,
				TargetArn:    target.Arn,
				Time:         eventTime,
			})

			payload := s.buildEventPayload(eventID, eventBusName, target, entry)

			// Deliver to API Destination via HTTP if the target ARN is an API destination.
			if dest := s.resolveAPIDestination(target.Arn); dest != nil {
				go s.deliverToHTTP(dest, target, payload)
			}

			// Deliver to SQS if the target ARN is an SQS queue.
			if isSQSArn(target.Arn) {
				go s.deliverToSQS(target, payload)
			}

			// Deliver to Lambda if the target ARN is a Lambda function.
			if isLambdaArn(target.Arn) {
				go s.deliverToLambda(target, payload)
			}
		}
	}
}

// buildEventPayload builds the CloudWatch Events envelope for delivery.
func (s *MemoryStorage) buildEventPayload(eventID, eventBusName string, target *Target, entry *PutEventsRequestEntry) []byte {
	payload := map[string]any{
		"version":     "0",
		"id":          eventID,
		"source":      entry.Source,
		"detail-type": entry.DetailType,
		"detail":      json.RawMessage(entry.Detail),
		"region":      s.region,
		"account":     s.accountID,
		"time":        time.Now().Format(time.RFC3339),
	}

	if eventBusName != defaultEventBusName {
		payload["event-bus-name"] = eventBusName
	}

	body, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("failed to marshal event payload", "error", err)

		return nil
	}

	// Apply InputPath if set on the target.
	if target.InputPath != "" {
		if resolved := resolveInputPath(body, target.InputPath); resolved != nil {
			return resolved
		}
	}

	return body
}

// resolveInputPath extracts a sub-field from payload using a simple JSONPath expression.
// Supports paths like "$.detail", "$.detail.payload", "$.source".
func resolveInputPath(payload []byte, inputPath string) []byte {
	path := strings.TrimPrefix(inputPath, "$.")
	if path == "" || path == "$" {
		return payload
	}

	parts := strings.Split(path, ".")

	var current any
	if err := json.Unmarshal(payload, &current); err != nil {
		return nil
	}

	for _, part := range parts {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil
		}

		current, ok = obj[part]
		if !ok {
			return nil
		}
	}

	result, err := json.Marshal(current)
	if err != nil {
		return nil
	}

	return result
}

// deliverToHTTP sends an event to an API Destination's HTTP endpoint.
func (s *MemoryStorage) deliverToHTTP(dest *APIDestination, target *Target, payload []byte) {
	if payload == nil {
		return
	}

	endpoint := dest.InvocationEndpoint
	method := dest.HTTPMethod

	if method == "" {
		method = http.MethodPost
	}

	req, err := http.NewRequestWithContext(context.Background(), method, endpoint, bytes.NewReader(payload))
	if err != nil {
		s.logger.Error("failed to create HTTP request for API destination", "error", err, "endpoint", endpoint)

		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Amazon/EventBridge/ApiDestinations")
	applyHTTPParameters(req, target.HTTPParameters)

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("failed to deliver event to API destination", "error", err, "endpoint", endpoint)

		return
	}

	defer func() { _ = resp.Body.Close() }()

	s.logger.Info("delivered event to API destination",
		"endpoint", endpoint,
		"status", resp.StatusCode,
	)
}

// isSQSArn returns true if the ARN is an SQS queue ARN.
func isSQSArn(arn string) bool {
	return strings.Contains(arn, ":sqs:")
}

// deliverToSQS sends an event to an SQS queue via the local kumo SQS endpoint.
func (s *MemoryStorage) deliverToSQS(target *Target, payload []byte) {
	if payload == nil {
		return
	}

	// Extract queue name from ARN: arn:aws:sqs:region:account:queue-name
	parts := strings.Split(target.Arn, ":")
	if len(parts) < 6 {
		s.logger.Error("invalid SQS ARN", "arn", target.Arn)

		return
	}

	queueName := parts[len(parts)-1]
	sqsEndpoint := fmt.Sprintf("%s/000000000000/%s", s.baseURL, queueName)

	reqBody := map[string]any{
		"QueueUrl":    sqsEndpoint,
		"MessageBody": string(payload),
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		s.logger.Error("failed to marshal SQS SendMessage request", "error", err)

		return
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, s.baseURL+"/", bytes.NewReader(body))
	if err != nil {
		s.logger.Error("failed to create SQS request", "error", err, "queue", queueName)

		return
	}

	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("failed to deliver event to SQS", "error", err, "queue", queueName)

		return
	}

	defer func() { _ = resp.Body.Close() }()

	s.logger.Info("delivered event to SQS",
		"queue", queueName,
		"status", resp.StatusCode,
	)
}

// isLambdaArn returns true if the ARN is a Lambda function ARN.
func isLambdaArn(arn string) bool {
	return strings.Contains(arn, ":lambda:")
}

// deliverToLambda invokes a Lambda function asynchronously via the local kumo Lambda endpoint.
func (s *MemoryStorage) deliverToLambda(target *Target, payload []byte) {
	if payload == nil {
		return
	}

	// Lambda ARN: arn:aws:lambda:region:account:function:function-name[:qualifier]
	parts := strings.Split(target.Arn, ":")
	if len(parts) < 7 || parts[5] != "function" {
		s.logger.Error("invalid Lambda ARN", "arn", target.Arn)

		return
	}

	functionName := parts[6]
	endpoint := fmt.Sprintf("%s/lambda/2015-03-31/functions/%s/invocations", s.baseURL, functionName)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		s.logger.Error("failed to create Lambda invoke request", "error", err, "function", functionName)

		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Amz-Invocation-Type", "Event")

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("failed to deliver event to Lambda", "error", err, "function", functionName)

		return
	}

	defer func() { _ = resp.Body.Close() }()

	s.logger.Info("delivered event to Lambda",
		"function", functionName,
		"status", resp.StatusCode,
	)
}

// applyHTTPParameters applies HTTP parameters from a target to an HTTP request.
func applyHTTPParameters(req *http.Request, params *HTTPParameters) {
	if params == nil {
		return
	}

	for k, v := range params.HeaderParameters {
		req.Header.Set(k, v)
	}

	if len(params.QueryStringParameters) > 0 {
		q := req.URL.Query()
		for k, v := range params.QueryStringParameters {
			q.Set(k, v)
		}

		req.URL.RawQuery = q.Encode()
	}
}

// GetDeliveredEvents returns all delivered events.
func (s *MemoryStorage) GetDeliveredEvents(_ context.Context) []DeliveredEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]DeliveredEvent, len(s.DeliveredEvents))
	copy(result, s.DeliveredEvents)

	return result
}

// CreateConnection creates a new connection.
func (s *MemoryStorage) CreateConnection(_ context.Context, req *CreateConnectionRequest) (*Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Connections[req.Name]; exists {
		return nil, &ServiceError{Code: errEventBusAlreadyExists, Message: "Connection already exists"}
	}

	now := time.Now()
	conn := &Connection{
		Name:               req.Name,
		Arn:                fmt.Sprintf("arn:aws:events:%s:%s:connection/%s", s.region, s.accountID, req.Name),
		ConnectionState:    "AUTHORIZED",
		AuthorizationType:  req.AuthorizationType,
		AuthParameters:     req.AuthParameters,
		CreationTime:       now,
		LastModifiedTime:   now,
		LastAuthorizedTime: now,
	}

	s.Connections[req.Name] = conn

	return conn, nil
}

// DescribeConnection describes a connection.
func (s *MemoryStorage) DescribeConnection(_ context.Context, name string) (*Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conn, exists := s.Connections[name]
	if !exists {
		return nil, &ServiceError{Code: errEventBusNotFound, Message: "Connection not found"}
	}

	return conn, nil
}

// DeleteConnection deletes a connection.
func (s *MemoryStorage) DeleteConnection(_ context.Context, name string) (*Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, exists := s.Connections[name]
	if !exists {
		return nil, &ServiceError{Code: errEventBusNotFound, Message: "Connection not found"}
	}

	delete(s.Connections, name)

	return conn, nil
}

// CreateAPIDestination creates a new API destination.
func (s *MemoryStorage) CreateAPIDestination(_ context.Context, req *CreateAPIDestinationRequest) (*APIDestination, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.APIDestinations[req.Name]; exists {
		return nil, &ServiceError{Code: errEventBusAlreadyExists, Message: "API destination already exists"}
	}

	rateLimit := req.InvocationRateLimitPerSecond
	if rateLimit == 0 {
		rateLimit = 300
	}

	now := time.Now()
	dest := &APIDestination{
		Name:                         req.Name,
		Arn:                          fmt.Sprintf("arn:aws:events:%s:%s:api-destination/%s", s.region, s.accountID, req.Name),
		ConnectionArn:                req.ConnectionArn,
		InvocationEndpoint:           req.InvocationEndpoint,
		HTTPMethod:                   req.HTTPMethod,
		InvocationRateLimitPerSecond: rateLimit,
		APIDestinationState:          "ACTIVE",
		CreationTime:                 now,
		LastModifiedTime:             now,
	}

	s.APIDestinations[req.Name] = dest

	return dest, nil
}

// DescribeAPIDestination describes an API destination.
func (s *MemoryStorage) DescribeAPIDestination(_ context.Context, name string) (*APIDestination, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dest, exists := s.APIDestinations[name]
	if !exists {
		return nil, &ServiceError{Code: errEventBusNotFound, Message: "API destination not found"}
	}

	return dest, nil
}

// DeleteAPIDestination deletes an API destination.
func (s *MemoryStorage) DeleteAPIDestination(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.APIDestinations[name]; !exists {
		return &ServiceError{Code: errEventBusNotFound, Message: "API destination not found"}
	}

	delete(s.APIDestinations, name)

	return nil
}

// resolveAPIDestination finds the API destination and its endpoint from a target ARN.
func (s *MemoryStorage) resolveAPIDestination(targetArn string) *APIDestination {
	for _, dest := range s.APIDestinations {
		if dest.Arn == targetArn {
			return dest
		}
	}

	return nil
}

// DispatchAction checks if the action is valid.
func (s *MemoryStorage) DispatchAction(_ string) bool {
	return true
}
