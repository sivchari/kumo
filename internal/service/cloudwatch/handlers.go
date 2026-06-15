package cloudwatch

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/server"
	"github.com/sivchari/kumo/internal/service"
)

// Error codes for CloudWatch.
const (
	errInvalidParameter     = "InvalidParameterValue"
	errMissingParameter     = "MissingParameter"
	errInternalServiceError = "InternalServiceError"
	errInvalidAction        = "InvalidAction"
	errResourceNotFound     = "ResourceNotFound"
)

// PutMetricData handles the PutMetricData action.
func (s *Service) PutMetricData(w http.ResponseWriter, r *http.Request) {
	var req PutMetricDataRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCloudWatchError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Namespace == "" {
		writeCloudWatchError(w, errMissingParameter, "The parameter Namespace is required", http.StatusBadRequest)

		return
	}

	if len(req.MetricData) == 0 {
		writeCloudWatchError(w, errMissingParameter, "The parameter MetricData is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.PutMetricData(r.Context(), req.Namespace, req.MetricData); err != nil {
		handleCloudWatchError(w, err)

		return
	}

	// PutMetricData returns an empty response on success.
	writeJSONResponse(w, struct{}{})
}

// GetMetricData handles the GetMetricData action.
func (s *Service) GetMetricData(w http.ResponseWriter, r *http.Request) {
	var req GetMetricDataRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCloudWatchError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.MetricDataQueries) == 0 {
		writeCloudWatchError(w, errMissingParameter, "The parameter MetricDataQueries is required", http.StatusBadRequest)

		return
	}

	result, err := s.storage.GetMetricData(r.Context(), &req)
	if err != nil {
		handleCloudWatchError(w, err)

		return
	}

	writeJSONResponse(w, GetMetricDataResponse{
		MetricDataResults: result.MetricDataResults,
		NextToken:         result.NextToken,
	})
}

// GetMetricStatistics handles the GetMetricStatistics action.
func (s *Service) GetMetricStatistics(w http.ResponseWriter, r *http.Request) {
	var req GetMetricStatisticsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCloudWatchError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Namespace == "" {
		writeCloudWatchError(w, errMissingParameter, "The parameter Namespace is required", http.StatusBadRequest)

		return
	}

	if req.MetricName == "" {
		writeCloudWatchError(w, errMissingParameter, "The parameter MetricName is required", http.StatusBadRequest)

		return
	}

	result, err := s.storage.GetMetricStatistics(r.Context(), &req)
	if err != nil {
		handleCloudWatchError(w, err)

		return
	}

	writeJSONResponse(w, GetMetricStatisticsResponse{
		Label:      result.Label,
		Datapoints: result.Datapoints,
	})
}

// ListMetrics handles the ListMetrics action.
func (s *Service) ListMetrics(w http.ResponseWriter, r *http.Request) {
	var req ListMetricsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCloudWatchError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	result, err := s.storage.ListMetrics(r.Context(), &req)
	if err != nil {
		handleCloudWatchError(w, err)

		return
	}

	writeJSONResponse(w, ListMetricsResponse{
		Metrics:        result.Metrics,
		NextToken:      result.NextToken,
		OwningAccounts: result.OwningAccounts,
	})
}

// PutMetricAlarm handles the PutMetricAlarm action.
func (s *Service) PutMetricAlarm(w http.ResponseWriter, r *http.Request) {
	var req PutMetricAlarmRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCloudWatchError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.AlarmName == "" {
		writeCloudWatchError(w, errMissingParameter, "The parameter AlarmName is required", http.StatusBadRequest)

		return
	}

	if req.MetricName == "" {
		writeCloudWatchError(w, errMissingParameter, "The parameter MetricName is required", http.StatusBadRequest)

		return
	}

	if req.Namespace == "" {
		writeCloudWatchError(w, errMissingParameter, "The parameter Namespace is required", http.StatusBadRequest)

		return
	}

	if req.ComparisonOperator == "" {
		writeCloudWatchError(w, errMissingParameter, "The parameter ComparisonOperator is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.PutMetricAlarm(r.Context(), &req); err != nil {
		handleCloudWatchError(w, err)

		return
	}

	// PutMetricAlarm returns an empty response on success — XML for the
	// Query-protocol path that terraform-provider-aws uses.
	writeCloudWatchXML(w, xmlPutMetricAlarmResponse{
		Xmlns:            cloudWatchXMLNS,
		ResponseMetadata: xmlResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteAlarms handles the DeleteAlarms action.
func (s *Service) DeleteAlarms(w http.ResponseWriter, r *http.Request) {
	var req DeleteAlarmsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCloudWatchError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.AlarmNames) == 0 {
		writeCloudWatchError(w, errMissingParameter, "The parameter AlarmNames is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteAlarms(r.Context(), req.AlarmNames); err != nil {
		handleCloudWatchError(w, err)

		return
	}

	// DeleteAlarms returns an empty response on success — XML for the
	// Query-protocol path that terraform-provider-aws uses.
	writeCloudWatchXML(w, xmlDeleteAlarmsResponse{
		Xmlns:            cloudWatchXMLNS,
		ResponseMetadata: xmlResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeAlarms handles the DescribeAlarms action.
func (s *Service) DescribeAlarms(w http.ResponseWriter, r *http.Request) {
	var req DescribeAlarmsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCloudWatchError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	result, err := s.storage.DescribeAlarms(r.Context(), &req)
	if err != nil {
		handleCloudWatchError(w, err)

		return
	}

	// XML for the Query-protocol path that terraform-provider-aws uses.
	writeCloudWatchXML(w, xmlDescribeAlarmsResponse{
		Xmlns: cloudWatchXMLNS,
		DescribeAlarmsResult: xmlDescribeAlarmsResult{
			MetricAlarms: xmlMetricAlarmList{Members: metricAlarmsToXML(result.MetricAlarms)},
			NextToken:    result.NextToken,
		},
		ResponseMetadata: xmlResponseMetadata{RequestID: uuid.New().String()},
	})
}

// SetAlarmState handles the SetAlarmState action via JSON protocol.
func (s *Service) SetAlarmState(w http.ResponseWriter, r *http.Request) {
	var req SetAlarmStateRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCloudWatchError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.AlarmName == "" {
		writeCloudWatchError(w, errMissingParameter, "The parameter AlarmName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.SetAlarmState(r.Context(), req.AlarmName, req.StateValue, req.StateReason); err != nil {
		handleCloudWatchError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// ListTagsForResource returns the tags attached to a CloudWatch resource.
func (s *Service) ListTagsForResource(w http.ResponseWriter, r *http.Request) {
	var req ListTagsForResourceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCloudWatchError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResourceARN == "" {
		writeCloudWatchError(w, errMissingParameter, "The parameter ResourceARN is required", http.StatusBadRequest)

		return
	}

	tags, err := s.storage.ListTagsForResource(r.Context(), req.ResourceARN)
	if err != nil {
		handleCloudWatchError(w, err)

		return
	}

	members := make([]xmlTagMember, 0, len(tags))
	for _, tag := range tags {
		members = append(members, xmlTagMember(tag))
	}

	writeCloudWatchXML(w, xmlListTagsForResourceResponse{
		Xmlns: cloudWatchXMLNS,
		ListTagsForResourceResult: xmlListTagsForResourceResult{
			Tags: xmlTagList{Members: members},
		},
		ResponseMetadata: xmlResponseMetadata{RequestID: uuid.New().String()},
	})
}

// TagResource attaches tags to a CloudWatch resource.
func (s *Service) TagResource(w http.ResponseWriter, r *http.Request) {
	var req TagResourceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCloudWatchError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResourceARN == "" {
		writeCloudWatchError(w, errMissingParameter, "The parameter ResourceARN is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.TagResource(r.Context(), req.ResourceARN, req.Tags); err != nil {
		handleCloudWatchError(w, err)

		return
	}

	writeCloudWatchXML(w, xmlTagResourceResponse{
		Xmlns:            cloudWatchXMLNS,
		ResponseMetadata: xmlResponseMetadata{RequestID: uuid.New().String()},
	})
}

// UntagResource removes tags from a CloudWatch resource.
func (s *Service) UntagResource(w http.ResponseWriter, r *http.Request) {
	var req UntagResourceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCloudWatchError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResourceARN == "" {
		writeCloudWatchError(w, errMissingParameter, "The parameter ResourceARN is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.UntagResource(r.Context(), req.ResourceARN, req.TagKeys); err != nil {
		handleCloudWatchError(w, err)

		return
	}

	writeCloudWatchXML(w, xmlUntagResourceResponse{
		Xmlns:            cloudWatchXMLNS,
		ResponseMetadata: xmlResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DispatchAction routes the request to the appropriate handler based on X-Amz-Target header.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "GraniteServiceVersion20100801.")

	switch action {
	case "PutMetricData":
		s.PutMetricData(w, r)
	case "GetMetricData":
		s.GetMetricData(w, r)
	case "GetMetricStatistics":
		s.GetMetricStatistics(w, r)
	case "ListMetrics":
		s.ListMetrics(w, r)
	case "PutMetricAlarm":
		s.PutMetricAlarm(w, r)
	case "DeleteAlarms":
		s.DeleteAlarms(w, r)
	case "DescribeAlarms":
		s.DescribeAlarms(w, r)
	case "SetAlarmState":
		s.SetAlarmState(w, r)
	case "ListTagsForResource":
		s.ListTagsForResource(w, r)
	case "TagResource":
		s.TagResource(w, r)
	case "UntagResource":
		s.UntagResource(w, r)
	default:
		writeCloudWatchError(w, errInvalidAction, "The action "+action+" is not valid", http.StatusBadRequest)
	}
}

// handleCloudWatchError handles CloudWatch errors.
func handleCloudWatchError(w http.ResponseWriter, err error) {
	var cwErr *Error
	if errors.As(err, &cwErr) {
		status := http.StatusBadRequest
		if cwErr.Code == errResourceNotFound {
			status = http.StatusNotFound
		}

		writeCloudWatchError(w, cwErr.Code, cwErr.Message, status)

		return
	}

	writeCloudWatchError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)
}

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	service.WriteJSONResponse(w, service.ContentTypeAmzJSON10, v)
}

// writeCloudWatchError writes a CloudWatch error response in JSON format.
func writeCloudWatchError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeAmzJSON10, code, message, status)
}

// CBOR Protocol Handlers for RPC v2 CBOR

// PutMetricDataCBOR handles the PutMetricData action with CBOR protocol.
func (s *Service) PutMetricDataCBOR(w http.ResponseWriter, r *http.Request) {
	var req PutMetricDataRequest
	if err := server.DecodeCBORRequest(r, &req); err != nil {
		server.WriteCBORError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Namespace == "" {
		server.WriteCBORError(w, errMissingParameter, "The parameter Namespace is required", http.StatusBadRequest)

		return
	}

	if len(req.MetricData) == 0 {
		server.WriteCBORError(w, errMissingParameter, "The parameter MetricData is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.PutMetricData(r.Context(), req.Namespace, req.MetricData); err != nil {
		handleCloudWatchCBORError(w, err)

		return
	}

	// PutMetricData returns an empty response on success.
	server.WriteCBORResponse(w, struct{}{})
}

// GetMetricDataCBOR handles the GetMetricData action with CBOR protocol.
func (s *Service) GetMetricDataCBOR(w http.ResponseWriter, r *http.Request) {
	var req GetMetricDataCBORRequest
	if err := server.DecodeCBORRequest(r, &req); err != nil {
		server.WriteCBORError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.MetricDataQueries) == 0 {
		server.WriteCBORError(w, errMissingParameter, "The parameter MetricDataQueries is required", http.StatusBadRequest)

		return
	}

	// Convert CBOR request to storage request
	storageReq := &GetMetricDataRequest{
		MetricDataQueries: req.MetricDataQueries,
		StartTime:         req.StartTime.Format(time.RFC3339),
		EndTime:           req.EndTime.Format(time.RFC3339),
		NextToken:         req.NextToken,
		MaxDatapoints:     req.MaxDatapoints,
	}

	result, err := s.storage.GetMetricData(r.Context(), storageReq)
	if err != nil {
		handleCloudWatchCBORError(w, err)

		return
	}

	// Convert result to CBOR response
	cborResults := make([]MetricDataCBORResult, len(result.MetricDataResults))

	for i := range result.MetricDataResults {
		r := result.MetricDataResults[i]
		timestamps := make([]time.Time, len(r.Timestamps))

		for j := range r.Timestamps {
			t, _ := parseTimestamp(r.Timestamps[j])
			timestamps[j] = t
		}

		cborResults[i] = MetricDataCBORResult{
			ID:         r.ID,
			Label:      r.Label,
			Timestamps: timestamps,
			Values:     r.Values,
			StatusCode: r.StatusCode,
		}
	}

	server.WriteCBORResponse(w, GetMetricDataCBORResponse{
		MetricDataResults: cborResults,
		NextToken:         result.NextToken,
	})
}

// GetMetricStatisticsCBOR handles the GetMetricStatistics action with CBOR protocol.
func (s *Service) GetMetricStatisticsCBOR(w http.ResponseWriter, r *http.Request) {
	var req GetMetricStatisticsCBORRequest
	if err := server.DecodeCBORRequest(r, &req); err != nil {
		server.WriteCBORError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Namespace == "" {
		server.WriteCBORError(w, errMissingParameter, "The parameter Namespace is required", http.StatusBadRequest)

		return
	}

	if req.MetricName == "" {
		server.WriteCBORError(w, errMissingParameter, "The parameter MetricName is required", http.StatusBadRequest)

		return
	}

	// Convert CBOR request to storage request
	storageReq := &GetMetricStatisticsRequest{
		Namespace:  req.Namespace,
		MetricName: req.MetricName,
		Dimensions: req.Dimensions,
		StartTime:  req.StartTime.Format(time.RFC3339),
		EndTime:    req.EndTime.Format(time.RFC3339),
		Period:     req.Period,
		Statistics: req.Statistics,
		Unit:       req.Unit,
	}

	result, err := s.storage.GetMetricStatistics(r.Context(), storageReq)
	if err != nil {
		handleCloudWatchCBORError(w, err)

		return
	}

	// Convert result to CBOR response
	cborDatapoints := make([]CBORDatapoint, len(result.Datapoints))

	for i := range result.Datapoints {
		dp := result.Datapoints[i]
		t, _ := parseTimestamp(dp.Timestamp)
		cborDatapoints[i] = CBORDatapoint{
			Timestamp:   t,
			SampleCount: dp.SampleCount,
			Average:     dp.Average,
			Sum:         dp.Sum,
			Minimum:     dp.Minimum,
			Maximum:     dp.Maximum,
			Unit:        dp.Unit,
		}
	}

	server.WriteCBORResponse(w, GetMetricStatisticsCBORResponse{
		Label:      result.Label,
		Datapoints: cborDatapoints,
	})
}

// ListMetricsCBOR handles the ListMetrics action with CBOR protocol.
func (s *Service) ListMetricsCBOR(w http.ResponseWriter, r *http.Request) {
	var req ListMetricsRequest
	if err := server.DecodeCBORRequest(r, &req); err != nil {
		server.WriteCBORError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	result, err := s.storage.ListMetrics(r.Context(), &req)
	if err != nil {
		handleCloudWatchCBORError(w, err)

		return
	}

	server.WriteCBORResponse(w, ListMetricsResponse{
		Metrics:        result.Metrics,
		NextToken:      result.NextToken,
		OwningAccounts: result.OwningAccounts,
	})
}

// PutMetricAlarmCBOR handles the PutMetricAlarm action with CBOR protocol.
func (s *Service) PutMetricAlarmCBOR(w http.ResponseWriter, r *http.Request) {
	var req PutMetricAlarmRequest
	if err := server.DecodeCBORRequest(r, &req); err != nil {
		server.WriteCBORError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.AlarmName == "" {
		server.WriteCBORError(w, errMissingParameter, "The parameter AlarmName is required", http.StatusBadRequest)

		return
	}

	if req.MetricName == "" {
		server.WriteCBORError(w, errMissingParameter, "The parameter MetricName is required", http.StatusBadRequest)

		return
	}

	if req.Namespace == "" {
		server.WriteCBORError(w, errMissingParameter, "The parameter Namespace is required", http.StatusBadRequest)

		return
	}

	if req.ComparisonOperator == "" {
		server.WriteCBORError(w, errMissingParameter, "The parameter ComparisonOperator is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.PutMetricAlarm(r.Context(), &req); err != nil {
		handleCloudWatchCBORError(w, err)

		return
	}

	// PutMetricAlarm returns an empty response on success.
	server.WriteCBORResponse(w, struct{}{})
}

// DeleteAlarmsCBOR handles the DeleteAlarms action with CBOR protocol.
func (s *Service) DeleteAlarmsCBOR(w http.ResponseWriter, r *http.Request) {
	var req DeleteAlarmsRequest
	if err := server.DecodeCBORRequest(r, &req); err != nil {
		server.WriteCBORError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.AlarmNames) == 0 {
		server.WriteCBORError(w, errMissingParameter, "The parameter AlarmNames is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteAlarms(r.Context(), req.AlarmNames); err != nil {
		handleCloudWatchCBORError(w, err)

		return
	}

	// DeleteAlarms returns an empty response on success.
	server.WriteCBORResponse(w, struct{}{})
}

// DescribeAlarmsCBOR handles the DescribeAlarms action with CBOR protocol.
func (s *Service) DescribeAlarmsCBOR(w http.ResponseWriter, r *http.Request) {
	var req DescribeAlarmsRequest
	if err := server.DecodeCBORRequest(r, &req); err != nil {
		server.WriteCBORError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	result, err := s.storage.DescribeAlarms(r.Context(), &req)
	if err != nil {
		handleCloudWatchCBORError(w, err)

		return
	}

	// Convert result to CBOR response
	cborAlarms := make([]MetricAlarmCBOR, len(result.MetricAlarms))

	for i := range result.MetricAlarms {
		alarm := &result.MetricAlarms[i]
		stateUpdated, _ := parseTimestamp(alarm.StateUpdatedTimestamp)
		configUpdated, _ := parseTimestamp(alarm.AlarmConfigurationUpdatedTimestamp)
		cborAlarms[i] = MetricAlarmCBOR{
			AlarmName:                          alarm.AlarmName,
			AlarmArn:                           alarm.AlarmArn,
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
			StateUpdatedTimestamp:              stateUpdated,
			AlarmConfigurationUpdatedTimestamp: configUpdated,
		}
	}

	server.WriteCBORResponse(w, DescribeAlarmsCBORResponse{
		MetricAlarms: cborAlarms,
		NextToken:    result.NextToken,
	})
}

// SetAlarmStateCBOR handles the SetAlarmState action via CBOR protocol.
func (s *Service) SetAlarmStateCBOR(w http.ResponseWriter, r *http.Request) {
	var req SetAlarmStateCBORRequest
	if err := server.DecodeCBORRequest(r, &req); err != nil {
		server.WriteCBORError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.AlarmName == "" {
		server.WriteCBORError(w, errInvalidParameter, "AlarmName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.SetAlarmState(r.Context(), req.AlarmName, req.StateValue, req.StateReason); err != nil {
		handleCloudWatchCBORError(w, err)

		return
	}

	server.WriteCBORResponse(w, struct{}{})
}

// ListTagsForResourceCBOR handles the ListTagsForResource action with CBOR protocol.
func (s *Service) ListTagsForResourceCBOR(w http.ResponseWriter, r *http.Request) {
	var req ListTagsForResourceRequest
	if err := server.DecodeCBORRequest(r, &req); err != nil {
		server.WriteCBORError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResourceARN == "" {
		server.WriteCBORError(w, errMissingParameter, "The parameter ResourceARN is required", http.StatusBadRequest)

		return
	}

	tags, err := s.storage.ListTagsForResource(r.Context(), req.ResourceARN)
	if err != nil {
		handleCloudWatchCBORError(w, err)

		return
	}

	server.WriteCBORResponse(w, ListTagsForResourceCBORResponse{
		Tags: tags,
	})
}

// TagResourceCBOR handles the TagResource action with CBOR protocol.
func (s *Service) TagResourceCBOR(w http.ResponseWriter, r *http.Request) {
	var req TagResourceRequest
	if err := server.DecodeCBORRequest(r, &req); err != nil {
		server.WriteCBORError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResourceARN == "" {
		server.WriteCBORError(w, errMissingParameter, "The parameter ResourceARN is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.TagResource(r.Context(), req.ResourceARN, req.Tags); err != nil {
		handleCloudWatchCBORError(w, err)

		return
	}

	server.WriteCBORResponse(w, struct{}{})
}

// UntagResourceCBOR handles the UntagResource action with CBOR protocol.
func (s *Service) UntagResourceCBOR(w http.ResponseWriter, r *http.Request) {
	var req UntagResourceRequest
	if err := server.DecodeCBORRequest(r, &req); err != nil {
		server.WriteCBORError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResourceARN == "" {
		server.WriteCBORError(w, errMissingParameter, "The parameter ResourceARN is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.UntagResource(r.Context(), req.ResourceARN, req.TagKeys); err != nil {
		handleCloudWatchCBORError(w, err)

		return
	}

	server.WriteCBORResponse(w, struct{}{})
}

// handleCloudWatchCBORError handles CloudWatch errors for CBOR protocol.
func handleCloudWatchCBORError(w http.ResponseWriter, err error) {
	var cwErr *Error
	if errors.As(err, &cwErr) {
		status := http.StatusBadRequest
		if cwErr.Code == errResourceNotFound {
			status = http.StatusNotFound
		}

		server.WriteCBORError(w, cwErr.Code, cwErr.Message, status)

		return
	}

	server.WriteCBORError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)
}

// parseTimestamp parses a timestamp string in various formats.
func parseTimestamp(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// Try RFC3339Nano
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}

	// Try ISO8601 without timezone
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", s)
}
