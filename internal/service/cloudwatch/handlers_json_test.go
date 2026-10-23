package cloudwatch

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sivchari/kumo/internal/service"
)

var _ service.JSONProtocolService = (*Service)(nil)

func postJSON(t *testing.T, svc *Service, action, body string) (int, string) {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "GraniteServiceVersion20100801."+action)

	rec := httptest.NewRecorder()
	svc.DispatchAction(rec, req)

	return rec.Code, rec.Body.String()
}

func TestJSONProtocol_MetricDataRoundTripWithEpochTimestamps(t *testing.T) {
	svc := New(NewMemoryStorage(""))

	code, body := postJSON(t, svc, actionPutMetricData, `{
		"Namespace": "Kumo/Test",
		"MetricData": [{
			"MetricName": "ConsoleLoginFailure",
			"Dimensions": [{"Name": "Account", "Value": "123"}],
			"Timestamp": 1760608800.5,
			"Value": 2
		}]
	}`)
	if code != http.StatusOK {
		t.Fatalf("PutMetricData: code=%d body=%s", code, body)
	}

	code, body = postJSON(t, svc, actionGetMetricData, `{
		"StartTime": 1760608700,
		"EndTime": "2025-10-16T10:05:00Z",
		"MetricDataQueries": [{
			"Id": "q1",
			"MetricStat": {
				"Metric": {
					"Namespace": "Kumo/Test",
					"MetricName": "ConsoleLoginFailure",
					"Dimensions": [{"Name": "Account", "Value": "123"}]
				},
				"Period": 60,
				"Stat": "Sum"
			}
		}]
	}`)
	if code != http.StatusOK {
		t.Fatalf("GetMetricData: code=%d body=%s", code, body)
	}

	var resp struct {
		MetricDataResults []struct {
			Timestamps []float64
			Values     []float64
		}
	}

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("GetMetricData response timestamps are not epoch numbers: %v body=%s", err, body)
	}

	if len(resp.MetricDataResults) != 1 {
		t.Fatalf("expected 1 result, got body=%s", body)
	}

	result := resp.MetricDataResults[0]
	if len(result.Timestamps) != 1 || result.Timestamps[0] != 1760608800.5 {
		t.Errorf("Timestamps = %v, want [1760608800.5]", result.Timestamps)
	}

	if len(result.Values) != 1 || result.Values[0] != 2 {
		t.Errorf("Values = %v, want [2]", result.Values)
	}
}

func TestJSONProtocol_GetMetricStatisticsReturnsEpochTimestamps(t *testing.T) {
	svc := New(NewMemoryStorage(""))

	code, body := postJSON(t, svc, actionPutMetricData, `{
		"Namespace": "Kumo/Test",
		"MetricData": [{"MetricName": "Latency", "Timestamp": 1760608800, "Value": 7}]
	}`)
	if code != http.StatusOK {
		t.Fatalf("PutMetricData: code=%d body=%s", code, body)
	}

	code, body = postJSON(t, svc, actionGetMetricStatistics, `{
		"Namespace": "Kumo/Test",
		"MetricName": "Latency",
		"StartTime": 1760608700,
		"EndTime": 1760608900,
		"Period": 60,
		"Statistics": ["Sum"]
	}`)
	if code != http.StatusOK {
		t.Fatalf("GetMetricStatistics: code=%d body=%s", code, body)
	}

	var resp struct {
		Datapoints []struct {
			Timestamp float64
			Sum       float64
		}
	}

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("GetMetricStatistics response timestamp is not an epoch number: %v body=%s", err, body)
	}

	if len(resp.Datapoints) != 1 || resp.Datapoints[0].Timestamp != 1760608800 || resp.Datapoints[0].Sum != 7 {
		t.Errorf("Datapoints = %+v, want one at 1760608800 with Sum 7", resp.Datapoints)
	}
}

func TestJSONProtocol_DescribeAlarmsReturnsJSONWithEpochTimestamps(t *testing.T) {
	svc := New(NewMemoryStorage(""))

	code, body := postJSON(t, svc, actionPutMetricAlarm, `{
		"AlarmName": "login-failures",
		"MetricName": "ConsoleLoginFailure",
		"Namespace": "Kumo/Test",
		"Statistic": "Sum",
		"Period": 60,
		"EvaluationPeriods": 1,
		"Threshold": 1,
		"ComparisonOperator": "GreaterThanOrEqualToThreshold"
	}`)
	if code != http.StatusOK || body != "{}\n" {
		t.Fatalf("PutMetricAlarm: code=%d body=%s", code, body)
	}

	code, body = postJSON(t, svc, actionDescribeAlarms, `{"AlarmNames": ["login-failures"]}`)
	if code != http.StatusOK {
		t.Fatalf("DescribeAlarms: code=%d body=%s", code, body)
	}

	var resp struct {
		MetricAlarms []struct {
			AlarmName                          string
			StateUpdatedTimestamp              float64
			AlarmConfigurationUpdatedTimestamp float64
		}
	}

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("DescribeAlarms response is not JSON with epoch timestamps: %v body=%s", err, body)
	}

	if len(resp.MetricAlarms) != 1 {
		t.Fatalf("expected 1 alarm, got body=%s", body)
	}

	alarm := resp.MetricAlarms[0]
	if alarm.AlarmName != "login-failures" || alarm.StateUpdatedTimestamp <= 0 || alarm.AlarmConfigurationUpdatedTimestamp <= 0 {
		t.Errorf("alarm = %+v, want login-failures with epoch timestamps", alarm)
	}
}

func TestQueryProtocol_DescribeAlarmsStillReturnsXML(t *testing.T) {
	svc := New(NewMemoryStorage(""))

	code, body := postJSON(t, svc, actionPutMetricAlarm, `{
		"AlarmName": "login-failures",
		"MetricName": "ConsoleLoginFailure",
		"Namespace": "Kumo/Test",
		"ComparisonOperator": "GreaterThanOrEqualToThreshold"
	}`)
	if code != http.StatusOK {
		t.Fatalf("PutMetricAlarm: code=%d body=%s", code, body)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"AlarmNames": ["login-failures"]}`))
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "GraniteServiceVersion20100801."+actionDescribeAlarms)

	rec := httptest.NewRecorder()
	svc.DispatchAction(rec, service.WithQueryProtocol(req))

	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/xml") {
		t.Fatalf("Content-Type = %q, want text/xml; body=%s", rec.Header().Get("Content-Type"), rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "<StateUpdatedTimestamp>") {
		t.Errorf("body missing RFC3339 StateUpdatedTimestamp: %s", rec.Body.String())
	}
}

func TestJSONProtocol_NullTimestampsAreUnset(t *testing.T) {
	svc := New(NewMemoryStorage(""))

	code, body := postJSON(t, svc, actionPutMetricData, `{
		"Namespace": "Kumo/Test",
		"MetricData": [{"MetricName": "Latency", "Timestamp": null, "Value": 7}]
	}`)
	if code != http.StatusOK {
		t.Fatalf("PutMetricData: code=%d body=%s", code, body)
	}

	code, body = postJSON(t, svc, actionGetMetricStatistics, `{
		"Namespace": "Kumo/Test",
		"MetricName": "Latency",
		"StartTime": null,
		"EndTime": null,
		"Period": 60,
		"Statistics": ["Sum"]
	}`)
	if code != http.StatusOK {
		t.Fatalf("GetMetricStatistics: code=%d body=%s", code, body)
	}

	var resp struct {
		Datapoints []struct {
			Timestamp float64
			Sum       float64
		}
	}

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}

	if len(resp.Datapoints) != 1 || resp.Datapoints[0].Timestamp <= 0 || resp.Datapoints[0].Sum != 7 {
		t.Errorf("Datapoints = %+v, want one stamped with the current time and Sum 7", resp.Datapoints)
	}
}

func TestJSONProtocol_InvalidTimestampIsRejected(t *testing.T) {
	svc := New(NewMemoryStorage(""))

	code, body := postJSON(t, svc, actionGetMetricStatistics, `{
		"Namespace": "Kumo/Test",
		"MetricName": "Latency",
		"StartTime": "yesterday",
		"Period": 60
	}`)
	if code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400; body=%s", code, body)
	}
}

func TestJSONProtocol_StringAndEpochTimestampsCompareChronologically(t *testing.T) {
	svc := New(NewMemoryStorage(""))

	code, body := postJSON(t, svc, actionPutMetricData, `{
		"Namespace": "Kumo/Test",
		"MetricData": [
			{"MetricName": "Latency", "Timestamp": "2025-10-16T10:00:00Z", "Value": 1},
			{"MetricName": "Latency", "Timestamp": 1760608800.5, "Value": 2},
			{"MetricName": "Latency", "Timestamp": "2025-10-16T10:00:01Z", "Value": 3}
		]
	}`)
	if code != http.StatusOK {
		t.Fatalf("PutMetricData: code=%d body=%s", code, body)
	}

	code, body = postJSON(t, svc, actionGetMetricData, `{
		"StartTime": "2025-10-16T10:00:00.25Z",
		"EndTime": 1760608800.75,
		"MetricDataQueries": [{
			"Id": "q1",
			"MetricStat": {
				"Metric": {"Namespace": "Kumo/Test", "MetricName": "Latency"},
				"Period": 60,
				"Stat": "Sum"
			}
		}]
	}`)
	if code != http.StatusOK {
		t.Fatalf("GetMetricData: code=%d body=%s", code, body)
	}

	var resp struct {
		MetricDataResults []struct {
			Timestamps []float64
			Values     []float64
		}
	}

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}

	if len(resp.MetricDataResults) != 1 {
		t.Fatalf("expected 1 result, got body=%s", body)
	}

	result := resp.MetricDataResults[0]
	if len(result.Values) != 1 || result.Values[0] != 2 || result.Timestamps[0] != 1760608800.5 {
		t.Errorf("result = %+v, want only the datapoint at 1760608800.5", result)
	}
}

func TestJSONTimestamp_RoundTripsAtMillisecondPrecision(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: `1760608800`, want: `1760608800`},
		{in: `1760608800.5`, want: `1760608800.5`},
		{in: `1760608800.0006`, want: `1760608800.001`},
		{in: `"2025-10-16T10:00:00Z"`, want: `1760608800`},
		{in: `"2025-10-16T10:00:00.0006Z"`, want: `1760608800.001`},
		{in: `"2025-10-16T10:00:00.0004Z"`, want: `1760608800`},
	}

	for _, tt := range tests {
		var ts jsonTimestamp
		if err := json.Unmarshal([]byte(tt.in), &ts); err != nil {
			t.Fatalf("unmarshal %s: %v", tt.in, err)
		}

		got, err := json.Marshal(ts)
		if err != nil {
			t.Fatalf("marshal %s: %v", tt.in, err)
		}

		if string(got) != tt.want {
			t.Errorf("round trip %s = %s, want %s", tt.in, got, tt.want)
		}
	}
}
