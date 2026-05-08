package cloudwatchlogs

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "000000000000"
	defaultLimit     = 50
	maxLimit         = 10000
)

// Storage defines the CloudWatch Logs storage interface.
type Storage interface {
	CreateLogGroup(ctx context.Context, req *CreateLogGroupRequest) error
	DeleteLogGroup(ctx context.Context, name string) error
	CreateLogStream(ctx context.Context, groupName, streamName string) error
	DeleteLogStream(ctx context.Context, groupName, streamName string) error
	PutLogEvents(ctx context.Context, groupName, streamName string, events []InputLogEvent, sequenceToken string) (*PutLogEventsResponse, error)
	GetLogEvents(ctx context.Context, req *GetLogEventsRequest) (*GetLogEventsResponse, error)
	FilterLogEvents(ctx context.Context, req *FilterLogEventsRequest) (*FilterLogEventsResponse, error)
	DescribeLogGroups(ctx context.Context, req *DescribeLogGroupsRequest) (*DescribeLogGroupsResponse, error)
	DescribeLogStreams(ctx context.Context, req *DescribeLogStreamsRequest) (*DescribeLogStreamsResponse, error)
	PutRetentionPolicy(ctx context.Context, groupName string, retentionInDays int32) error
	DeleteRetentionPolicy(ctx context.Context, groupName string) error
}

// LogStreamData holds log stream data with events.
type LogStreamData struct {
	Stream *LogStream  `json:"stream"`
	Events []*LogEvent `json:"events"`
}

// LogGroupData holds log group data with streams.
type LogGroupData struct {
	Group   *LogGroup                 `json:"group"`
	Streams map[string]*LogStreamData `json:"streams"`
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
	mu        sync.RWMutex             `json:"-"`
	LogGroups map[string]*LogGroupData `json:"logGroups"`
	baseURL   string
	dataDir   string
}

// NewMemoryStorage creates a new in-memory CloudWatch Logs storage.
func NewMemoryStorage(baseURL string, opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		LogGroups: make(map[string]*LogGroupData),
		baseURL:   baseURL,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "logs", s)
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

	if m.LogGroups == nil {
		m.LogGroups = make(map[string]*LogGroupData)
	}

	return nil
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "logs", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateLogGroup creates a new log group.
func (m *MemoryStorage) CreateLogGroup(_ context.Context, req *CreateLogGroupRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.LogGroups[req.LogGroupName]; exists {
		return &LogsError{
			Code:    "ResourceAlreadyExistsException",
			Message: fmt.Sprintf("The specified log group already exists: %s", req.LogGroupName),
		}
	}

	now := time.Now().UnixMilli()
	logGroup := &LogGroup{
		LogGroupName:  req.LogGroupName,
		LogGroupARN:   m.buildLogGroupARN(req.LogGroupName),
		CreationTime:  now,
		KmsKeyID:      req.KmsKeyID,
		LogGroupClass: req.LogGroupClass,
	}

	m.LogGroups[req.LogGroupName] = &LogGroupData{
		Group:   logGroup,
		Streams: make(map[string]*LogStreamData),
	}

	return nil
}

// DeleteLogGroup deletes a log group.
func (m *MemoryStorage) DeleteLogGroup(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.LogGroups[name]; !exists {
		return &LogsError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("The specified log group does not exist: %s", name),
		}
	}

	delete(m.LogGroups, name)

	return nil
}

// CreateLogStream creates a new log stream in a log group.
func (m *MemoryStorage) CreateLogStream(_ context.Context, groupName, streamName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	groupData, exists := m.LogGroups[groupName]
	if !exists {
		return &LogsError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("The specified log group does not exist: %s", groupName),
		}
	}

	if _, exists := groupData.Streams[streamName]; exists {
		return &LogsError{
			Code:    "ResourceAlreadyExistsException",
			Message: fmt.Sprintf("The specified log stream already exists: %s", streamName),
		}
	}

	now := time.Now().UnixMilli()
	stream := &LogStream{
		LogStreamName:       streamName,
		CreationTime:        now,
		UploadSequenceToken: uuid.New().String(),
		LogStreamARN:        m.buildLogStreamARN(groupName, streamName),
	}

	groupData.Streams[streamName] = &LogStreamData{
		Stream: stream,
		Events: make([]*LogEvent, 0),
	}

	return nil
}

// DeleteLogStream deletes a log stream from a log group.
func (m *MemoryStorage) DeleteLogStream(_ context.Context, groupName, streamName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	groupData, exists := m.LogGroups[groupName]
	if !exists {
		return &LogsError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("The specified log group does not exist: %s", groupName),
		}
	}

	if _, exists := groupData.Streams[streamName]; !exists {
		return &LogsError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("The specified log stream does not exist: %s", streamName),
		}
	}

	delete(groupData.Streams, streamName)

	return nil
}

// PutLogEvents puts log events into a log stream.
func (m *MemoryStorage) PutLogEvents(_ context.Context, groupName, streamName string, events []InputLogEvent, _ string) (*PutLogEventsResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	groupData, exists := m.LogGroups[groupName]
	if !exists {
		return nil, &LogsError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("The specified log group does not exist: %s", groupName),
		}
	}

	streamData, exists := groupData.Streams[streamName]
	if !exists {
		return nil, &LogsError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("The specified log stream does not exist: %s", streamName),
		}
	}

	now := time.Now().UnixMilli()

	for _, event := range events {
		logEvent := &LogEvent{
			Timestamp: event.Timestamp,
			Message:   event.Message,
		}
		streamData.Events = append(streamData.Events, logEvent)

		// Update stream timestamps
		if streamData.Stream.FirstEventTimestamp == nil {
			streamData.Stream.FirstEventTimestamp = &event.Timestamp
		}

		streamData.Stream.LastEventTimestamp = &event.Timestamp
		streamData.Stream.LastIngestionTime = &now
		streamData.Stream.StoredBytes += int64(len(event.Message))
	}

	// Update group stored bytes
	groupData.Group.StoredBytes += sumEventBytes(events)

	// Generate new sequence token
	newToken := uuid.New().String()
	streamData.Stream.UploadSequenceToken = newToken

	return &PutLogEventsResponse{
		NextSequenceToken: newToken,
	}, nil
}

// GetLogEvents retrieves log events from a log stream.
func (m *MemoryStorage) GetLogEvents(_ context.Context, req *GetLogEventsRequest) (*GetLogEventsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groupData, exists := m.LogGroups[req.LogGroupName]
	if !exists {
		return nil, &LogsError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("The specified log group does not exist: %s", req.LogGroupName),
		}
	}

	streamData, exists := groupData.Streams[req.LogStreamName]
	if !exists {
		return nil, &LogsError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("The specified log stream does not exist: %s", req.LogStreamName),
		}
	}

	limit := defaultLimit
	if req.Limit != nil && *req.Limit > 0 {
		limit = min(int(*req.Limit), maxLimit)
	}

	// Filter events by time range
	filteredEvents := filterEventsByTime(streamData.Events, req.StartTime, req.EndTime)

	// Sort events
	startFromHead := req.StartFromHead != nil && *req.StartFromHead
	if startFromHead {
		sort.Slice(filteredEvents, func(i, j int) bool {
			return filteredEvents[i].Timestamp < filteredEvents[j].Timestamp
		})
	} else {
		sort.Slice(filteredEvents, func(i, j int) bool {
			return filteredEvents[i].Timestamp > filteredEvents[j].Timestamp
		})
	}

	// Apply limit
	if len(filteredEvents) > limit {
		filteredEvents = filteredEvents[:limit]
	}

	// Convert to output format
	outputEvents := make([]OutputLogEvent, 0, len(filteredEvents))
	now := time.Now().UnixMilli()

	for _, event := range filteredEvents {
		outputEvents = append(outputEvents, OutputLogEvent{
			Timestamp:     event.Timestamp,
			Message:       event.Message,
			IngestionTime: now,
		})
	}

	return &GetLogEventsResponse{
		Events:            outputEvents,
		NextForwardToken:  uuid.New().String(),
		NextBackwardToken: uuid.New().String(),
	}, nil
}

// FilterLogEvents filters log events across log streams.
func (m *MemoryStorage) FilterLogEvents(_ context.Context, req *FilterLogEventsRequest) (*FilterLogEventsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groupName := req.LogGroupName
	if groupName == "" {
		groupName = req.LogGroupIdentifier
	}

	groupData, exists := m.LogGroups[groupName]
	if !exists {
		return nil, &LogsError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("The specified log group does not exist: %s", groupName),
		}
	}

	limit := getLimit(req.Limit)
	allEvents, searchedStreams := m.filterEventsFromStreams(groupData, req)

	// Sort by timestamp
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp < allEvents[j].Timestamp
	})

	// Apply limit
	if len(allEvents) > limit {
		allEvents = allEvents[:limit]
	}

	return &FilterLogEventsResponse{
		Events:             allEvents,
		SearchedLogStreams: searchedStreams,
	}, nil
}

// filterEventsFromStreams filters events from log streams based on request criteria.
func (m *MemoryStorage) filterEventsFromStreams(groupData *LogGroupData, req *FilterLogEventsRequest) ([]FilteredLogEvent, []SearchedLogStream) {
	var allEvents []FilteredLogEvent

	var searchedStreams []SearchedLogStream

	for streamName, streamData := range groupData.Streams {
		if !m.shouldIncludeStream(streamName, req.LogStreamNames, req.LogStreamNamePrefix) {
			continue
		}

		searchedStreams = append(searchedStreams, SearchedLogStream{
			LogStreamName:      streamName,
			SearchedCompletely: true,
		})

		events := m.filterStreamEvents(streamName, streamData.Events, req)
		allEvents = append(allEvents, events...)
	}

	return allEvents, searchedStreams
}

// shouldIncludeStream checks if a stream should be included in the filter results.
func (m *MemoryStorage) shouldIncludeStream(streamName string, streamNames []string, prefix string) bool {
	if len(streamNames) > 0 && !slices.Contains(streamNames, streamName) {
		return false
	}

	if prefix != "" && !strings.HasPrefix(streamName, prefix) {
		return false
	}

	return true
}

// filterStreamEvents filters events from a single stream.
func (m *MemoryStorage) filterStreamEvents(streamName string, events []*LogEvent, req *FilterLogEventsRequest) []FilteredLogEvent {
	var result []FilteredLogEvent

	for i, event := range events {
		if !matchesTimeRange(event.Timestamp, req.StartTime, req.EndTime) {
			continue
		}

		if req.FilterPattern != "" && !strings.Contains(event.Message, req.FilterPattern) {
			continue
		}

		result = append(result, FilteredLogEvent{
			LogStreamName: streamName,
			Timestamp:     event.Timestamp,
			Message:       event.Message,
			IngestionTime: time.Now().UnixMilli(),
			EventID:       fmt.Sprintf("%d-%d", event.Timestamp, i),
		})
	}

	return result
}

// matchesTimeRange checks if a timestamp is within the given time range.
func matchesTimeRange(timestamp int64, startTime, endTime *int64) bool {
	if startTime != nil && timestamp < *startTime {
		return false
	}

	if endTime != nil && timestamp > *endTime {
		return false
	}

	return true
}

// getLimit returns the limit value from the request or the default.
func getLimit(limit *int32) int {
	if limit != nil && *limit > 0 {
		return min(int(*limit), maxLimit)
	}

	return defaultLimit
}

// DescribeLogGroups describes log groups.
func (m *MemoryStorage) DescribeLogGroups(_ context.Context, req *DescribeLogGroupsRequest) (*DescribeLogGroupsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := getLimit(req.Limit)
	groups := m.filterLogGroups(req)

	// Sort by name
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].LogGroupName < groups[j].LogGroupName
	})

	result, nextToken := paginateLogGroups(groups, req.NextToken, limit)

	return &DescribeLogGroupsResponse{
		LogGroups: result,
		NextToken: nextToken,
	}, nil
}

// filterLogGroups filters log groups based on request criteria.
func (m *MemoryStorage) filterLogGroups(req *DescribeLogGroupsRequest) []LogGroupResponse {
	var groups []LogGroupResponse

	for name, groupData := range m.LogGroups {
		if !m.matchLogGroupFilters(name, groupData.Group, req) {
			continue
		}

		groups = append(groups, buildLogGroupResponse(groupData.Group))
	}

	return groups
}

// matchLogGroupFilters checks if a log group matches the filter criteria.
func (m *MemoryStorage) matchLogGroupFilters(name string, group *LogGroup, req *DescribeLogGroupsRequest) bool {
	if req.LogGroupNamePrefix != "" && !strings.HasPrefix(name, req.LogGroupNamePrefix) {
		return false
	}

	if req.LogGroupNamePattern != "" && !strings.Contains(name, req.LogGroupNamePattern) {
		return false
	}

	if req.LogGroupClass != "" && group.LogGroupClass != req.LogGroupClass {
		return false
	}

	return true
}

// buildLogGroupResponse builds a LogGroupResponse from a LogGroup.
func buildLogGroupResponse(group *LogGroup) LogGroupResponse {
	return LogGroupResponse{
		LogGroupName:      group.LogGroupName,
		LogGroupARN:       group.LogGroupARN,
		CreationTime:      group.CreationTime,
		RetentionInDays:   group.RetentionInDays,
		MetricFilterCount: group.MetricFilterCount,
		StoredBytes:       group.StoredBytes,
		KmsKeyID:          group.KmsKeyID,
		LogGroupClass:     group.LogGroupClass,
	}
}

// paginateLogGroups applies pagination to log groups.
func paginateLogGroups(groups []LogGroupResponse, nextToken string, limit int) ([]LogGroupResponse, string) {
	startIdx := findLogGroupStartIndex(groups, nextToken)
	endIdx := min(startIdx+limit, len(groups))
	result := groups[startIdx:endIdx]

	var newNextToken string

	if endIdx < len(groups) {
		newNextToken = groups[endIdx].LogGroupName
	}

	return result, newNextToken
}

// findLogGroupStartIndex finds the starting index for pagination.
func findLogGroupStartIndex(groups []LogGroupResponse, nextToken string) int {
	if nextToken == "" {
		return 0
	}

	for i := range groups {
		if groups[i].LogGroupName == nextToken {
			return i
		}
	}

	return 0
}

// DescribeLogStreams describes log streams in a log group.
func (m *MemoryStorage) DescribeLogStreams(_ context.Context, req *DescribeLogStreamsRequest) (*DescribeLogStreamsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groupName := req.LogGroupName
	if groupName == "" {
		groupName = req.LogGroupIdentifier
	}

	groupData, exists := m.LogGroups[groupName]
	if !exists {
		return nil, &LogsError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("The specified log group does not exist: %s", groupName),
		}
	}

	limit := getLimit(req.Limit)
	streams := m.filterLogStreams(groupData, req.LogStreamNamePrefix)
	sortLogStreams(streams, req.OrderBy, req.Descending)
	result, nextToken := paginateLogStreams(streams, req.NextToken, limit)

	return &DescribeLogStreamsResponse{
		LogStreams: result,
		NextToken:  nextToken,
	}, nil
}

// filterLogStreams filters log streams based on prefix.
func (m *MemoryStorage) filterLogStreams(groupData *LogGroupData, prefix string) []LogStreamResponse {
	var streams []LogStreamResponse

	for name, streamData := range groupData.Streams {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}

		streams = append(streams, buildLogStreamResponse(streamData.Stream))
	}

	return streams
}

// buildLogStreamResponse builds a LogStreamResponse from a LogStream.
func buildLogStreamResponse(stream *LogStream) LogStreamResponse {
	return LogStreamResponse{
		LogStreamName:       stream.LogStreamName,
		CreationTime:        stream.CreationTime,
		FirstEventTimestamp: stream.FirstEventTimestamp,
		LastEventTimestamp:  stream.LastEventTimestamp,
		LastIngestionTime:   stream.LastIngestionTime,
		UploadSequenceToken: stream.UploadSequenceToken,
		Arn:                 stream.LogStreamARN,
		StoredBytes:         stream.StoredBytes,
	}
}

// sortLogStreams sorts log streams by the specified order.
func sortLogStreams(streams []LogStreamResponse, orderBy string, descending *bool) {
	if orderBy == "" {
		orderBy = "LogStreamName"
	}

	desc := descending != nil && *descending

	switch orderBy {
	case "LastEventTime":
		sortByLastEventTime(streams, desc)
	default:
		sortByLogStreamName(streams, desc)
	}
}

// sortByLastEventTime sorts streams by last event timestamp.
func sortByLastEventTime(streams []LogStreamResponse, descending bool) {
	sort.Slice(streams, func(i, j int) bool {
		iTime := getLastEventTime(streams[i].LastEventTimestamp)
		jTime := getLastEventTime(streams[j].LastEventTimestamp)

		if descending {
			return iTime > jTime
		}

		return iTime < jTime
	})
}

// sortByLogStreamName sorts streams by name.
func sortByLogStreamName(streams []LogStreamResponse, descending bool) {
	sort.Slice(streams, func(i, j int) bool {
		if descending {
			return streams[i].LogStreamName > streams[j].LogStreamName
		}

		return streams[i].LogStreamName < streams[j].LogStreamName
	})
}

// getLastEventTime returns the last event timestamp or 0.
func getLastEventTime(timestamp *int64) int64 {
	if timestamp != nil {
		return *timestamp
	}

	return 0
}

// paginateLogStreams applies pagination to log streams.
func paginateLogStreams(streams []LogStreamResponse, nextToken string, limit int) ([]LogStreamResponse, string) {
	startIdx := findLogStreamStartIndex(streams, nextToken)
	endIdx := min(startIdx+limit, len(streams))
	result := streams[startIdx:endIdx]

	var newNextToken string

	if endIdx < len(streams) {
		newNextToken = streams[endIdx].LogStreamName
	}

	return result, newNextToken
}

// findLogStreamStartIndex finds the starting index for pagination.
func findLogStreamStartIndex(streams []LogStreamResponse, nextToken string) int {
	if nextToken == "" {
		return 0
	}

	for i := range streams {
		if streams[i].LogStreamName == nextToken {
			return i
		}
	}

	return 0
}

// buildLogGroupARN builds an ARN for a log group.
func (m *MemoryStorage) buildLogGroupARN(name string) string {
	return fmt.Sprintf("arn:aws:logs:%s:%s:log-group:%s",
		defaultRegion, defaultAccountID, name)
}

// buildLogStreamARN builds an ARN for a log stream.
func (m *MemoryStorage) buildLogStreamARN(groupName, streamName string) string {
	return fmt.Sprintf("arn:aws:logs:%s:%s:log-group:%s:log-stream:%s",
		defaultRegion, defaultAccountID, groupName, streamName)
}

// filterEventsByTime filters events by time range.
func filterEventsByTime(events []*LogEvent, startTime, endTime *int64) []*LogEvent {
	var result []*LogEvent

	for _, event := range events {
		if startTime != nil && event.Timestamp < *startTime {
			continue
		}

		if endTime != nil && event.Timestamp > *endTime {
			continue
		}

		result = append(result, event)
	}

	return result
}

// sumEventBytes sums the bytes of all events.
func sumEventBytes(events []InputLogEvent) int64 {
	var total int64

	for _, event := range events {
		total += int64(len(event.Message))
	}

	return total
}

// PutRetentionPolicy sets the retention period (in days) for a log group.
// AWS accepts a fixed set of values (1, 3, 5, 7, 14, 30, 60, 90, 120, 150,
// 180, 365, 400, 545, 731, 1827, 3653) but does not reject other positive
// integers; the kumo emulator stores whatever value the caller supplied.
func (m *MemoryStorage) PutRetentionPolicy(_ context.Context, groupName string, retentionInDays int32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.LogGroups[groupName]
	if !ok {
		return &LogsError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("The specified log group %s does not exist.", groupName),
		}
	}

	v := retentionInDays
	group.Group.RetentionInDays = &v

	return nil
}

// DeleteRetentionPolicy clears the retention period on a log group, returning
// the group to "never expire".
func (m *MemoryStorage) DeleteRetentionPolicy(_ context.Context, groupName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.LogGroups[groupName]
	if !ok {
		return &LogsError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("The specified log group %s does not exist.", groupName),
		}
	}

	group.Group.RetentionInDays = nil

	return nil
}
