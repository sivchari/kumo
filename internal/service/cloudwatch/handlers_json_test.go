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
