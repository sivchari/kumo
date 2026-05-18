package cloudwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/storage"
)

// defaultAccountID is the default AWS account ID used in the emulator.
const defaultAccountID = "000000000000"

// defaultRegion is the default AWS region used in the emulator.
const defaultRegion = "us-east-1"

// SNSPublisher is an interface for publishing alarm action notifications to SNS topics.
type SNSPublisher interface {
	Publish(ctx context.Context, topicARN, message, subject string) error
}

// Storage defines the CloudWatch storage interface.
type Storage interface {
	PutMetricData(ctx context.Context, namespace string, metricData []MetricDatum) error
	GetMetricData(ctx context.Context, req *GetMetricDataRequest) (*GetMetricDataResult, error)
	GetMetricStatistics(ctx context.Context, req *GetMetricStatisticsRequest) (*GetMetricStatisticsResult, error)
	ListMetrics(ctx context.Context, req *ListMetricsRequest) (*ListMetricsResult, error)
	PutMetricAlarm(ctx context.Context, req *PutMetricAlarmRequest) error
	DeleteAlarms(ctx context.Context, alarmNames []string) error
	DescribeAlarms(ctx context.Context, req *DescribeAlarmsRequest) (*DescribeAlarmsResult, error)
	SetAlarmState(ctx context.Context, alarmName, stateValue, stateReason string) error
	TagResource(ctx context.Context, resourceARN string, tags []Tag) error
	UntagResource(ctx context.Context, resourceARN string, tagKeys []string) error
	ListTagsForResource(ctx context.Context, resourceARN string) ([]Tag, error)
}

// MetricKey uniquely identifies a metric.
type MetricKey struct {
	Namespace  string `json:"namespace"`
	MetricName string `json:"metricName"`
	Dimensions string `json:"dimensions"` // sorted dimension string for consistency
}

// StoredMetric holds metric data in memory.
type StoredMetric struct {
	Namespace  string            `json:"namespace"`
	MetricName string            `json:"metricName"`
	Dimensions []Dimension       `json:"dimensions"`
	Datapoints []MetricDatapoint `json:"datapoints"`
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
	mu           sync.RWMutex                `json:"-"`
	Metrics      map[MetricKey]*StoredMetric `json:"metrics"`
	Alarms       map[string]*Alarm           `json:"alarms"`
	Tags         map[string][]Tag            `json:"tags"`
	baseURL      string
	dataDir      string
	snsPublisher SNSPublisher `json:"-"`
}

// NewMemoryStorage creates a new in-memory CloudWatch storage.
func NewMemoryStorage(baseURL string, opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Metrics: make(map[MetricKey]*StoredMetric),
		Alarms:  make(map[string]*Alarm),
		Tags:    make(map[string][]Tag),
		baseURL: baseURL,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "monitoring", s)
	}

	return s
}

// metricKeyToString converts a MetricKey to a string for JSON map keys.
func metricKeyToString(k MetricKey) string {
	return k.Namespace + "|" + k.MetricName + "|" + k.Dimensions
}

// stringToMetricKey converts a string back to a MetricKey.
func stringToMetricKey(s string) MetricKey {
	parts := strings.SplitN(s, "|", 3)
	if len(parts) != 3 {
		return MetricKey{}
	}

	return MetricKey{
		Namespace:  parts[0],
		MetricName: parts[1],
		Dimensions: parts[2],
	}
}

// marshalableStorage is a JSON-serializable representation of MemoryStorage.
type marshalableStorage struct {
	Metrics map[string]*StoredMetric `json:"metrics"`
	Alarms  map[string]*Alarm        `json:"alarms"`
	Tags    map[string][]Tag         `json:"tags"`
}

// MarshalJSON serializes the storage state to JSON.
func (s *MemoryStorage) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m := &marshalableStorage{
		Metrics: make(map[string]*StoredMetric, len(s.Metrics)),
		Alarms:  s.Alarms,
		Tags:    s.Tags,
	}

	for k, v := range s.Metrics {
		m.Metrics[metricKeyToString(k)] = v
	}

	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (s *MemoryStorage) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var m marshalableStorage

	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	s.Metrics = make(map[MetricKey]*StoredMetric, len(m.Metrics))

	for k, v := range m.Metrics {
		s.Metrics[stringToMetricKey(k)] = v
	}

	s.Alarms = m.Alarms

	if s.Alarms == nil {
		s.Alarms = make(map[string]*Alarm)
	}

	s.Tags = m.Tags

	if s.Tags == nil {
		s.Tags = make(map[string][]Tag)
	}

	return nil
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "monitoring", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// SetSNSPublisher sets the SNS publisher for alarm action notifications.
func (s *MemoryStorage) SetSNSPublisher(publisher SNSPublisher) {
	s.snsPublisher = publisher
}

// PutMetricData stores metric data.
func (s *MemoryStorage) PutMetricData(_ context.Context, namespace string, metricData []MetricDatum) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range metricData {
		datum := &metricData[i]
		key := s.makeMetricKey(namespace, datum.MetricName, datum.Dimensions)

		metric, exists := s.Metrics[key]
		if !exists {
			metric = &StoredMetric{
				Namespace:  namespace,
				MetricName: datum.MetricName,
				Dimensions: datum.Dimensions,
				Datapoints: make([]MetricDatapoint, 0),
			}
			s.Metrics[key] = metric
		}

		timestamp := datum.Timestamp
		if timestamp == "" {
			timestamp = time.Now().UTC().Format(time.RFC3339)
		}

		s.appendDatapoints(metric, datum, timestamp)
	}

	return nil
}

// appendDatapoints adds datapoints to the metric based on the datum type.
func (s *MemoryStorage) appendDatapoints(metric *StoredMetric, datum *MetricDatum, timestamp string) {
	switch {
	case datum.Value != nil:
		metric.Datapoints = append(metric.Datapoints, MetricDatapoint{
			Timestamp: timestamp,
			Value:     *datum.Value,
			Unit:      datum.Unit,
		})
	case datum.StatisticValues != nil:
		metric.Datapoints = append(metric.Datapoints, MetricDatapoint{
			Timestamp: timestamp,
			Value:     datum.StatisticValues.Sum / datum.StatisticValues.SampleCount,
			Unit:      datum.Unit,
		})
	case len(datum.Values) > 0:
		for _, v := range datum.Values {
			metric.Datapoints = append(metric.Datapoints, MetricDatapoint{
				Timestamp: timestamp,
				Value:     v,
				Unit:      datum.Unit,
			})
		}
	}
}

// GetMetricData retrieves metric data.
func (s *MemoryStorage) GetMetricData(_ context.Context, req *GetMetricDataRequest) (*GetMetricDataResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]MetricDataResult, 0, len(req.MetricDataQueries))

	for _, query := range req.MetricDataQueries {
		if query.MetricStat == nil {
			continue
		}

		key := s.makeMetricKey(
			query.MetricStat.Metric.Namespace,
			query.MetricStat.Metric.MetricName,
			query.MetricStat.Metric.Dimensions,
		)

		metric, exists := s.Metrics[key]
		if !exists {
			results = append(results, MetricDataResult{
				ID:         query.ID,
				Label:      query.Label,
				StatusCode: "Complete",
			})

			continue
		}

		// Filter datapoints by time range.
		timestamps := make([]string, 0)
		values := make([]float64, 0)

		for _, dp := range metric.Datapoints {
			if s.isInTimeRange(dp.Timestamp, req.StartTime, req.EndTime) {
				timestamps = append(timestamps, dp.Timestamp)
				values = append(values, dp.Value)
			}
		}

		label := query.Label
		if label == "" {
			label = query.MetricStat.Metric.MetricName
		}

		results = append(results, MetricDataResult{
			ID:         query.ID,
			Label:      label,
			Timestamps: timestamps,
			Values:     values,
			StatusCode: "Complete",
		})
	}

	return &GetMetricDataResult{
		MetricDataResults: results,
	}, nil
}

// GetMetricStatistics retrieves statistics for a metric.
func (s *MemoryStorage) GetMetricStatistics(_ context.Context, req *GetMetricStatisticsRequest) (*GetMetricStatisticsResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.makeMetricKey(req.Namespace, req.MetricName, req.Dimensions)

	metric, exists := s.Metrics[key]
	if !exists {
		return &GetMetricStatisticsResult{
			Label:      req.MetricName,
			Datapoints: []Datapoint{},
		}, nil
	}

	// Collect datapoints in time range.
	var filteredPoints []MetricDatapoint

	for _, dp := range metric.Datapoints {
		if s.isInTimeRange(dp.Timestamp, req.StartTime, req.EndTime) {
			filteredPoints = append(filteredPoints, dp)
		}
	}

	if len(filteredPoints) == 0 {
		return &GetMetricStatisticsResult{
			Label:      req.MetricName,
			Datapoints: []Datapoint{},
		}, nil
	}

	// Calculate statistics.
	datapoints := s.calculateStatistics(filteredPoints, req.Statistics, req.Period)

	return &GetMetricStatisticsResult{
		Label:      req.MetricName,
		Datapoints: datapoints,
	}, nil
}

// ListMetrics lists available metrics.
func (s *MemoryStorage) ListMetrics(_ context.Context, req *ListMetricsRequest) (*ListMetricsResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := make([]Metric, 0)

	for _, m := range s.Metrics {
		// Filter by namespace.
		if req.Namespace != "" && m.Namespace != req.Namespace {
			continue
		}

		// Filter by metric name.
		if req.MetricName != "" && m.MetricName != req.MetricName {
			continue
		}

		// Filter by dimensions.
		if !s.matchesDimensionFilters(m.Dimensions, req.Dimensions) {
			continue
		}

		metrics = append(metrics, Metric{
			Namespace:  m.Namespace,
			MetricName: m.MetricName,
			Dimensions: m.Dimensions,
		})
	}

	// Sort metrics for consistent output.
	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].Namespace != metrics[j].Namespace {
			return metrics[i].Namespace < metrics[j].Namespace
		}

		return metrics[i].MetricName < metrics[j].MetricName
	})

	// Build owning accounts list.
	// In a single-account emulator, all metrics are owned by the default account.
	var owningAccounts []string
	if len(metrics) > 0 {
		owningAccounts = []string{defaultAccountID}
	}

	return &ListMetricsResult{
		Metrics:        metrics,
		OwningAccounts: owningAccounts,
	}, nil
}

// PutMetricAlarm creates or updates an alarm.
func (s *MemoryStorage) PutMetricAlarm(_ context.Context, req *PutMetricAlarmRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	alarmARN := fmt.Sprintf("arn:aws:cloudwatch:us-east-1:%s:alarm:%s", defaultAccountID, req.AlarmName)

	actionsEnabled := true
	if req.ActionsEnabled != nil {
		actionsEnabled = *req.ActionsEnabled
	}

	alarm := &Alarm{
		AlarmName:          req.AlarmName,
		AlarmARN:           alarmARN,
		AlarmDescription:   req.AlarmDescription,
		MetricName:         req.MetricName,
		Namespace:          req.Namespace,
		Statistic:          req.Statistic,
		Dimensions:         req.Dimensions,
		Period:             req.Period,
		EvaluationPeriods:  req.EvaluationPeriods,
		Threshold:          req.Threshold,
		ComparisonOperator: req.ComparisonOperator,
		ActionsEnabled:     actionsEnabled,
		AlarmActions:       req.AlarmActions,
		OKActions:          req.OKActions,
		StateValue:         "INSUFFICIENT_DATA",
		StateReason:        "Unchecked: Initial alarm creation",
		StateUpdatedAt:     now,
		CreatedAt:          now,
	}

	s.Alarms[req.AlarmName] = alarm

	return nil
}

// DeleteAlarms deletes the specified alarms.
func (s *MemoryStorage) DeleteAlarms(_ context.Context, alarmNames []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, name := range alarmNames {
		if _, exists := s.Alarms[name]; !exists {
			return &Error{
				Code:    "ResourceNotFound",
				Message: fmt.Sprintf("Alarm %s does not exist", name),
			}
		}
	}

	for _, name := range alarmNames {
		delete(s.Alarms, name)
	}

	return nil
}

// SetAlarmState sets the state of an alarm and fires the corresponding
// alarm actions (AlarmActions for ALARM, OKActions for OK) by publishing
// a notification message to each SNS topic ARN listed in the action list.
func (s *MemoryStorage) SetAlarmState(ctx context.Context, alarmName, stateValue, stateReason string) error {
	s.mu.Lock()

	alarm, exists := s.Alarms[alarmName]
	if !exists {
		s.mu.Unlock()

		return &Error{
			Code:    "ResourceNotFound",
			Message: fmt.Sprintf("Alarm %s does not exist", alarmName),
		}
	}

	previousState := alarm.StateValue
	alarm.StateValue = stateValue
	alarm.StateReason = stateReason
	alarm.StateUpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// Determine which actions to fire based on the new state.
	var actionARNs []string

	if alarm.ActionsEnabled {
		switch stateValue {
		case "ALARM":
			actionARNs = append(actionARNs, alarm.AlarmActions...)
		case "OK":
			actionARNs = append(actionARNs, alarm.OKActions...)
		}
	}

	// Copy alarm fields needed for the notification while still holding
	// the lock, then release before performing I/O.
	notification := s.buildAlarmNotification(alarm, previousState)
	s.mu.Unlock()

	// Publish to each action target (SNS topic ARNs).
	if s.snsPublisher != nil && len(actionARNs) > 0 {
		body, err := json.Marshal(notification)
		if err != nil {
			return fmt.Errorf("SetAlarmState failed: %w", err)
		}

		subject := fmt.Sprintf("ALARM: %q in %s", alarmName, defaultRegion)

		for _, arn := range actionARNs {
			// Best-effort delivery: log but do not fail the SetAlarmState
			// call if a publish fails (matches AWS behaviour where action
			// delivery is asynchronous).
			_ = s.snsPublisher.Publish(ctx, arn, string(body), subject)
		}
	}

	return nil
}

// alarmNotification mirrors the JSON structure AWS CloudWatch sends to
// SNS action targets when an alarm state changes.
//
//nolint:tagliatelle // AWS notification uses PascalCase JSON fields.
type alarmNotification struct {
	AlarmName        string                   `json:"AlarmName"`
	AlarmDescription string                   `json:"AlarmDescription,omitempty"`
	AWSAccountID     string                   `json:"AWSAccountId"`
	NewStateValue    string                   `json:"NewStateValue"`
	NewStateReason   string                   `json:"NewStateReason"`
	OldStateValue    string                   `json:"OldStateValue"`
	StateChangeTime  string                   `json:"StateChangeTime"`
	Region           string                   `json:"Region"`
	AlarmARN         string                   `json:"AlarmArn"`
	Trigger          alarmNotificationTrigger `json:"Trigger"`
}

// alarmNotificationTrigger contains the metric details that triggered the
// alarm.
//
//nolint:tagliatelle // AWS notification uses PascalCase JSON fields.
type alarmNotificationTrigger struct {
	MetricName         string      `json:"MetricName"`
	Namespace          string      `json:"Namespace"`
	Statistic          string      `json:"Statistic,omitempty"`
	Dimensions         []Dimension `json:"Dimensions,omitempty"`
	Period             int32       `json:"Period"`
	EvaluationPeriods  int32       `json:"EvaluationPeriods"`
	Threshold          float64     `json:"Threshold"`
	ComparisonOperator string      `json:"ComparisonOperator"`
}

// buildAlarmNotification creates an alarm notification message from alarm
// state. The caller must hold at least a read lock on s.mu.
func (s *MemoryStorage) buildAlarmNotification(alarm *Alarm, previousState string) *alarmNotification {
	return &alarmNotification{
		AlarmName:        alarm.AlarmName,
		AlarmDescription: alarm.AlarmDescription,
		AWSAccountID:     defaultAccountID,
		NewStateValue:    alarm.StateValue,
		NewStateReason:   alarm.StateReason,
		OldStateValue:    previousState,
		StateChangeTime:  alarm.StateUpdatedAt,
		Region:           defaultRegion,
		AlarmARN:         alarm.AlarmARN,
		Trigger: alarmNotificationTrigger{
			MetricName:         alarm.MetricName,
			Namespace:          alarm.Namespace,
			Statistic:          alarm.Statistic,
			Dimensions:         alarm.Dimensions,
			Period:             alarm.Period,
			EvaluationPeriods:  alarm.EvaluationPeriods,
			Threshold:          alarm.Threshold,
			ComparisonOperator: alarm.ComparisonOperator,
		},
	}
}

// DescribeAlarms returns information about alarms.
func (s *MemoryStorage) DescribeAlarms(_ context.Context, req *DescribeAlarmsRequest) (*DescribeAlarmsResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alarms := make([]MetricAlarm, 0)

	for _, alarm := range s.Alarms {
		if !s.alarmMatchesFilter(alarm, req) {
			continue
		}

		alarms = append(alarms, convertAlarmToJSON(alarm))
	}

	// Sort alarms by name.
	sort.Slice(alarms, func(i, j int) bool {
		return alarms[i].AlarmName < alarms[j].AlarmName
	})

	// Apply MaxRecords limit.
	maxRecords := 50
	if req.MaxRecords != nil && *req.MaxRecords > 0 {
		maxRecords = int(*req.MaxRecords)
	}

	if len(alarms) > maxRecords {
		alarms = alarms[:maxRecords]
	}

	return &DescribeAlarmsResult{
		MetricAlarms: alarms,
	}, nil
}

// alarmMatchesFilter checks if an alarm matches the filter criteria.
func (s *MemoryStorage) alarmMatchesFilter(alarm *Alarm, req *DescribeAlarmsRequest) bool {
	if len(req.AlarmNames) > 0 && !slices.Contains(req.AlarmNames, alarm.AlarmName) {
		return false
	}

	if req.AlarmNamePrefix != "" && !strings.HasPrefix(alarm.AlarmName, req.AlarmNamePrefix) {
		return false
	}

	if req.StateValue != "" && alarm.StateValue != req.StateValue {
		return false
	}

	return true
}

// convertAlarmToJSON converts an Alarm to MetricAlarm JSON response.
func convertAlarmToJSON(alarm *Alarm) MetricAlarm {
	return MetricAlarm{
		AlarmName:                          alarm.AlarmName,
		AlarmArn:                           alarm.AlarmARN,
		AlarmDescription:                   alarm.AlarmDescription,
		MetricName:                         alarm.MetricName,
		Namespace:                          alarm.Namespace,
		Statistic:                          alarm.Statistic,
		Dimensions:                         alarm.Dimensions,
		Period:                             alarm.Period,
		EvaluationPeriods:                  alarm.EvaluationPeriods,
		Threshold:                          alarm.Threshold,
		ComparisonOperator:                 alarm.ComparisonOperator,
		ActionsEnabled:                     alarm.ActionsEnabled,
		AlarmActions:                       alarm.AlarmActions,
		OKActions:                          alarm.OKActions,
		StateValue:                         alarm.StateValue,
		StateReason:                        alarm.StateReason,
		StateUpdatedTimestamp:              alarm.StateUpdatedAt,
		AlarmConfigurationUpdatedTimestamp: alarm.CreatedAt,
	}
}

// makeMetricKey creates a unique key for a metric.
func (s *MemoryStorage) makeMetricKey(namespace, metricName string, dimensions []Dimension) MetricKey {
	// Sort dimensions for consistent key generation.
	sorted := make([]Dimension, len(dimensions))
	copy(sorted, dimensions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	dimParts := make([]string, 0, len(sorted))

	for _, d := range sorted {
		dimParts = append(dimParts, fmt.Sprintf("%s=%s", d.Name, d.Value))
	}

	return MetricKey{
		Namespace:  namespace,
		MetricName: metricName,
		Dimensions: strings.Join(dimParts, ","),
	}
}

// isInTimeRange checks if a timestamp is within the given range.
func (s *MemoryStorage) isInTimeRange(timestamp, startTime, endTime string) bool {
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return false
	}

	if startTime != "" {
		start, err := time.Parse(time.RFC3339, startTime)
		if err == nil && ts.Before(start) {
			return false
		}
	}

	if endTime != "" {
		end, err := time.Parse(time.RFC3339, endTime)
		if err == nil && ts.After(end) {
			return false
		}
	}

	return true
}

// matchesDimensionFilters checks if dimensions match the filters.
func (s *MemoryStorage) matchesDimensionFilters(dimensions []Dimension, filters []DimensionFilter) bool {
	if len(filters) == 0 {
		return true
	}

	dimMap := make(map[string]string)
	for _, d := range dimensions {
		dimMap[d.Name] = d.Value
	}

	for _, f := range filters {
		val, exists := dimMap[f.Name]
		if !exists {
			return false
		}

		if f.Value != "" && val != f.Value {
			return false
		}
	}

	return true
}

// calculateStatistics calculates statistics for datapoints.
func (s *MemoryStorage) calculateStatistics(points []MetricDatapoint, statistics []string, _ int32) []Datapoint {
	if len(points) == 0 {
		return nil
	}

	// Group by period (simplified: use first timestamp).
	timestamp := points[0].Timestamp
	unit := points[0].Unit
	count := float64(len(points))

	var sum float64

	minVal := points[0].Value
	maxVal := points[0].Value

	for _, p := range points {
		sum += p.Value

		if p.Value < minVal {
			minVal = p.Value
		}

		if p.Value > maxVal {
			maxVal = p.Value
		}
	}

	average := sum / count

	dp := Datapoint{
		Timestamp: timestamp,
		Unit:      unit,
	}

	// Include requested statistics.
	for _, stat := range statistics {
		switch stat {
		case "SampleCount":
			dp.SampleCount = &count
		case "Average":
			dp.Average = &average
		case "Sum":
			dp.Sum = &sum
		case "Minimum":
			dp.Minimum = &minVal
		case "Maximum":
			dp.Maximum = &maxVal
		}
	}

	// If no specific statistics requested, include all.
	if len(statistics) == 0 {
		dp.SampleCount = &count
		dp.Average = &average
		dp.Sum = &sum
		dp.Minimum = &minVal
		dp.Maximum = &maxVal
	}

	return []Datapoint{dp}
}

// TagResource adds or overwrites tags on a resource identified by its ARN.
func (s *MemoryStorage) TagResource(_ context.Context, resourceARN string, tags []Tag) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.Tags[resourceARN]
	tagMap := make(map[string]string, len(existing)+len(tags))

	for _, tag := range existing {
		tagMap[tag.Key] = tag.Value
	}

	for _, tag := range tags {
		tagMap[tag.Key] = tag.Value
	}

	merged := make([]Tag, 0, len(tagMap))

	for k, v := range tagMap {
		merged = append(merged, Tag{Key: k, Value: v})
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Key < merged[j].Key
	})

	s.Tags[resourceARN] = merged

	return nil
}

// UntagResource removes the specified tag keys from a resource.
func (s *MemoryStorage) UntagResource(_ context.Context, resourceARN string, tagKeys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.Tags[resourceARN]

	keySet := make(map[string]bool, len(tagKeys))
	for _, key := range tagKeys {
		keySet[key] = true
	}

	remaining := make([]Tag, 0, len(existing))

	for _, tag := range existing {
		if !keySet[tag.Key] {
			remaining = append(remaining, tag)
		}
	}

	s.Tags[resourceARN] = remaining

	return nil
}

// ListTagsForResource returns the tags attached to a resource.
func (s *MemoryStorage) ListTagsForResource(_ context.Context, resourceARN string) ([]Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tags := s.Tags[resourceARN]
	if tags == nil {
		tags = make([]Tag, 0)
	}

	return tags, nil
}
