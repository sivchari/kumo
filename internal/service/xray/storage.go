package xray

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errInvalidRequest = "InvalidRequestException"
	errNotFound       = "InvalidRequestException"
)

// Storage defines the interface for X-Ray storage operations.
type Storage interface {
	PutTraceSegments(ctx context.Context, documents []string) ([]UnprocessedTraceSegment, error)
	GetTraceSummaries(ctx context.Context, startTime, endTime time.Time) ([]TraceSummary, error)
	BatchGetTraces(ctx context.Context, traceIDs []string) ([]*Trace, []string, error)
	GetServiceGraph(ctx context.Context, startTime, endTime time.Time, groupName string) ([]ServiceNode, error)
	CreateGroup(ctx context.Context, input *CreateGroupInput) (*Group, error)
	DeleteGroup(ctx context.Context, groupName, groupARN string) error
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

// MemoryStorage implements Storage with in-memory data structures.
type MemoryStorage struct {
	mu       sync.RWMutex        `json:"-"`
	Traces   map[string]*Trace   `json:"traces"`   // key: traceID
	Segments map[string]*Segment `json:"segments"` // key: segmentID
	Groups   map[string]*Group   `json:"groups"`   // key: groupName
	dataDir  string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Traces:   make(map[string]*Trace),
		Segments: make(map[string]*Segment),
		Groups:   make(map[string]*Group),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "xray", s)
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

	if s.Traces == nil {
		s.Traces = make(map[string]*Trace)
	}

	if s.Segments == nil {
		s.Segments = make(map[string]*Segment)
	}

	if s.Groups == nil {
		s.Groups = make(map[string]*Group)
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

	_ = storage.SaveBytes(s.dataDir, "xray", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "xray", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// SegmentDocument represents the structure of a segment document.
type SegmentDocument struct {
	ID         string  `json:"id"`
	TraceID    string  `json:"trace_id"`
	Name       string  `json:"name"`
	StartTime  float64 `json:"start_time"`
	EndTime    float64 `json:"end_time"`
	InProgress bool    `json:"in_progress"`
	Service    struct {
		Version string `json:"version"`
	} `json:"service"`
	User     string `json:"user"`
	Origin   string `json:"origin"`
	ParentID string `json:"parent_id"`
	HTTP     struct {
		Request struct {
			Method   string `json:"method"`
			URL      string `json:"url"`
			ClientIP string `json:"client_ip"`
		} `json:"request"`
		Response struct {
			Status int `json:"status"`
		} `json:"response"`
	} `json:"http"`
	Fault    bool `json:"fault"`
	Error    bool `json:"error"`
	Throttle bool `json:"throttle"`
}

// PutTraceSegments stores trace segments.
func (s *MemoryStorage) PutTraceSegments(_ context.Context, documents []string) ([]UnprocessedTraceSegment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var unprocessed []UnprocessedTraceSegment

	for _, doc := range documents {
		var segDoc SegmentDocument
		if err := json.Unmarshal([]byte(doc), &segDoc); err != nil {
			unprocessed = append(unprocessed, UnprocessedTraceSegment{
				ID:        "",
				ErrorCode: "InvalidSegmentDocument",
				Message:   "Invalid segment document format",
			})

			continue
		}

		if segDoc.ID == "" {
			segDoc.ID = uuid.New().String()[:16]
		}

		if segDoc.TraceID == "" {
			segDoc.TraceID = fmt.Sprintf("1-%x-%s", time.Now().Unix(), uuid.New().String()[:24])
		}

		segment := &Segment{
			ID:       segDoc.ID,
			Document: doc,
		}

		s.Segments[segDoc.ID] = segment

		// Create or update trace.
		trace, exists := s.Traces[segDoc.TraceID]
		if !exists {
			trace = &Trace{
				ID:       segDoc.TraceID,
				Segments: []*Segment{},
			}
			s.Traces[segDoc.TraceID] = trace
		}

		trace.Segments = append(trace.Segments, segment)

		// Calculate duration.
		if segDoc.EndTime > 0 && segDoc.StartTime > 0 {
			duration := segDoc.EndTime - segDoc.StartTime
			if duration > trace.Duration {
				trace.Duration = duration
			}
		}
	}

	s.saveLocked()

	return unprocessed, nil
}

// GetTraceSummaries retrieves trace summaries.
func (s *MemoryStorage) GetTraceSummaries(_ context.Context, _, _ time.Time) ([]TraceSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summaries := make([]TraceSummary, 0, len(s.Traces))

	for _, trace := range s.Traces {
		summary := TraceSummary{
			ID:       trace.ID,
			Duration: trace.Duration,
		}

		// Parse first segment to get additional info.
		if len(trace.Segments) > 0 {
			var segDoc SegmentDocument
			if err := json.Unmarshal([]byte(trace.Segments[0].Document), &segDoc); err == nil {
				summary.HasFault = segDoc.Fault
				summary.HasError = segDoc.Error
				summary.HasThrottle = segDoc.Throttle
				summary.HTTP = &HTTPInfo{
					HTTPMethod: segDoc.HTTP.Request.Method,
					HTTPURL:    segDoc.HTTP.Request.URL,
					ClientIP:   segDoc.HTTP.Request.ClientIP,
					//nolint:gosec // G115: HTTP status codes are always in range 100-599, safe for int32.
					HTTPStatus: int32(segDoc.HTTP.Response.Status),
				}
			}
		}

		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// BatchGetTraces retrieves traces by ID.
func (s *MemoryStorage) BatchGetTraces(_ context.Context, traceIDs []string) ([]*Trace, []string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var traces []*Trace

	var unprocessed []string

	for _, id := range traceIDs {
		if trace, exists := s.Traces[id]; exists {
			traces = append(traces, trace)
		} else {
			unprocessed = append(unprocessed, id)
		}
	}

	return traces, unprocessed, nil
}

// GetServiceGraph retrieves the service graph.
func (s *MemoryStorage) GetServiceGraph(_ context.Context, _, _ time.Time, _ string) ([]ServiceNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Build service graph from traces.
	serviceMap := make(map[string]*ServiceNode)

	for _, trace := range s.Traces {
		for _, segment := range trace.Segments {
			var segDoc SegmentDocument
			if err := json.Unmarshal([]byte(segment.Document), &segDoc); err != nil {
				continue
			}

			serviceName := segDoc.Name
			if serviceName == "" {
				serviceName = "unknown"
			}

			if _, exists := serviceMap[serviceName]; !exists {
				serviceMap[serviceName] = &ServiceNode{
					//nolint:gosec // G115: Service count is bounded by trace data, won't exceed int32 range.
					ReferenceID: int32(len(serviceMap) + 1),
					Name:        serviceName,
					Type:        segDoc.Origin,
					Root:        segDoc.ParentID == "",
					SummaryStatistics: &ServiceStats{
						TotalCount: 0,
						OkCount:    0,
					},
				}
			}

			svc := serviceMap[serviceName]
			svc.SummaryStatistics.TotalCount++

			if !segDoc.Fault && !segDoc.Error {
				svc.SummaryStatistics.OkCount++
			}
		}
	}

	services := make([]ServiceNode, 0, len(serviceMap))
	for _, svc := range serviceMap {
		services = append(services, *svc)
	}

	return services, nil
}

// CreateGroup creates a new group.
func (s *MemoryStorage) CreateGroup(_ context.Context, input *CreateGroupInput) (*Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.GroupName == "" {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: "GroupName is required",
		}
	}

	if _, exists := s.Groups[input.GroupName]; exists {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: fmt.Sprintf("Group %s already exists", input.GroupName),
		}
	}

	groupARN := fmt.Sprintf("arn:aws:xray:us-east-1:000000000000:group/%s/%s",
		input.GroupName, uuid.New().String())

	group := &Group{
		GroupName:             input.GroupName,
		GroupARN:              groupARN,
		FilterExpression:      input.FilterExpression,
		InsightsConfiguration: input.InsightsConfiguration,
	}

	s.Groups[input.GroupName] = group

	s.saveLocked()

	return group, nil
}

// DeleteGroup deletes a group.
func (s *MemoryStorage) DeleteGroup(_ context.Context, groupName, groupARN string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find group by name or ARN.
	var targetName string

	if groupName != "" {
		targetName = groupName
	} else if groupARN != "" {
		for name, group := range s.Groups {
			if group.GroupARN == groupARN {
				targetName = name

				break
			}
		}
	}

	if targetName == "" {
		return &Error{
			Code:    errNotFound,
			Message: "Group not found",
		}
	}

	if _, exists := s.Groups[targetName]; !exists {
		return &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Group %s not found", targetName),
		}
	}

	delete(s.Groups, targetName)

	s.saveLocked()

	return nil
}
