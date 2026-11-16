package cloudwatch

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/sivchari/kumo/internal/server"
	"github.com/sivchari/kumo/internal/service"
)

func postQuery(t *testing.T, svc *Service, action, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "GraniteServiceVersion20100801."+action)

	rec := httptest.NewRecorder()
	svc.DispatchAction(rec, service.WithQueryProtocol(req))

	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/xml") {
		t.Fatalf("%s: Content-Type = %q, want text/xml; body=%s", action, rec.Header().Get("Content-Type"), rec.Body.String())
	}

	return rec
}

// postQueryForm sends form through the Query protocol dispatcher, as the AWS
// CLI and older SDKs do.
func postQueryForm(t *testing.T, svc *Service, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	dispatcher := server.NewQueryProtocolDispatcher()
	for _, action := range svc.Actions() {
		dispatcher.RegisterAction(action, svc.TargetPrefix(), svc.ServiceIdentifier(), svc.DispatchAction)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "aws-cli/2.27.25 api/cloudwatch#2010-08-01")

	rec := httptest.NewRecorder()
	dispatcher.ServeHTTP(rec, req)

	return rec
}

func putTestAlarm(t *testing.T, svc *Service, name string) {
	t.Helper()

	code, body := postJSON(t, svc, actionPutMetricAlarm, `{
		"AlarmName": "`+name+`",
		"MetricName": "ConsoleLoginFailure",
		"Namespace": "Kumo/Test",
		"ComparisonOperator": "GreaterThanOrEqualToThreshold"
	}`)
	if code != http.StatusOK {
		t.Fatalf("PutMetricAlarm: code=%d body=%s", code, body)
	}
}

func TestQueryProtocol_GetMetricDataReturnsXMLWithISOTimestamps(t *testing.T) {
	svc := New(NewMemoryStorage(""))

	rec := postQuery(t, svc, actionPutMetricData, `{
		"Namespace": "Kumo/Test",
		"MetricData": [{"MetricName": "Latency", "Timestamp": "2025-10-16T10:00:00.5Z", "Value": 2}]
	}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<PutMetricDataResponse") {
		t.Fatalf("PutMetricData: code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = postQuery(t, svc, actionGetMetricData, `{
		"StartTime": "2025-10-16T09:59:00Z",
		"EndTime": "2025-10-16T10:05:00Z",
		"MetricDataQueries": [{
			"Id": "q1",
			"MetricStat": {
				"Metric": {"Namespace": "Kumo/Test", "MetricName": "Latency"},
				"Period": 60,
				"Stat": "Sum"
			}
		}]
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("GetMetricData: code=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		XMLName xml.Name `xml:"GetMetricDataResponse"`
		Results []struct {
			ID         string    `xml:"Id"`
			Timestamps []string  `xml:"Timestamps>member"`
			Values     []float64 `xml:"Values>member"`
		} `xml:"GetMetricDataResult>MetricDataResults>member"`
	}

	if err := xml.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, body=%s", rec.Body.String())
	}

	got := resp.Results[0]
	if got.ID != "q1" || len(got.Timestamps) != 1 || got.Timestamps[0] != "2025-10-16T10:00:00.5Z" || got.Values[0] != 2 {
		t.Errorf("result = %+v, want q1 with one datapoint at 2025-10-16T10:00:00.5Z valued 2", got)
	}
}

func TestQueryProtocol_GetMetricDataFromFormBody(t *testing.T) {
	svc := New(NewMemoryStorage(""))

	code, body := postJSON(t, svc, actionPutMetricData, `{
		"Namespace": "Kumo/Test",
		"MetricData": [{
			"MetricName": "Latency",
			"Dimensions": [{"Name": "Account", "Value": "acct-a"}],
			"Timestamp": "2025-10-16T10:00:00Z",
			"Value": 7
		}]
	}`)
	if code != http.StatusOK {
		t.Fatalf("PutMetricData: code=%d body=%s", code, body)
	}

	rec := postQueryForm(t, svc, url.Values{
		"Action":                        {actionGetMetricData},
		"Version":                       {"2010-08-01"},
		"StartTime":                     {"2025-10-16T09:59:00Z"},
		"EndTime":                       {"2025-10-16T10:05:00Z"},
		"MetricDataQueries.member.1.Id": {"m"},
		"MetricDataQueries.member.1.MetricStat.Metric.Namespace":                 {"Kumo/Test"},
		"MetricDataQueries.member.1.MetricStat.Metric.MetricName":                {"Latency"},
		"MetricDataQueries.member.1.MetricStat.Metric.Dimensions.member.1.Name":  {"Account"},
		"MetricDataQueries.member.1.MetricStat.Metric.Dimensions.member.1.Value": {"acct-a"},
		"MetricDataQueries.member.1.MetricStat.Period":                           {"60"},
		"MetricDataQueries.member.1.MetricStat.Stat":                             {"Sum"},
		"MetricDataQueries.member.2.Id":                                          {"empty"},
		"MetricDataQueries.member.2.MetricStat.Metric.Namespace":                 {"Kumo/Test"},
		"MetricDataQueries.member.2.MetricStat.Metric.MetricName":                {"Unknown"},
		"MetricDataQueries.member.2.MetricStat.Period":                           {"60"},
		"MetricDataQueries.member.2.MetricStat.Stat":                             {"Sum"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("GetMetricData: code=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Results []struct {
			ID         string    `xml:"Id"`
			StatusCode string    `xml:"StatusCode"`
			Values     []float64 `xml:"Values>member"`
		} `xml:"GetMetricDataResult>MetricDataResults>member"`
	}

	if err := xml.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}

	if len(resp.Results) != 2 {
		t.Fatalf("expected one result per query, body=%s", rec.Body.String())
	}

	if got := resp.Results[0]; got.ID != "m" || len(got.Values) != 1 || got.Values[0] != 7 {
		t.Errorf("first result = %+v, want m with the stored value 7", got)
	}

	if got := resp.Results[1]; got.ID != "empty" || got.StatusCode != "Complete" || len(got.Values) != 0 {
		t.Errorf("second result = %+v, want a complete result with no values", got)
	}
}

func TestQueryProtocol_ListMetricsReturnsXMLWithDimensions(t *testing.T) {
	svc := New(NewMemoryStorage(""))

	code, body := postJSON(t, svc, actionPutMetricData, `{
		"Namespace": "Kumo/Test",
		"MetricData": [{"MetricName": "Latency", "Dimensions": [{"Name": "Account", "Value": "123"}], "Value": 2}]
	}`)
	if code != http.StatusOK {
		t.Fatalf("PutMetricData: code=%d body=%s", code, body)
	}

	rec := postQuery(t, svc, actionListMetrics, `{"Namespace": "Kumo/Test"}`)

	var resp struct {
		Metrics []struct {
			MetricName string      `xml:"MetricName"`
			Dimensions []Dimension `xml:"Dimensions>member"`
		} `xml:"ListMetricsResult>Metrics>member"`
	}

	if err := xml.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}

	if len(resp.Metrics) != 1 || resp.Metrics[0].MetricName != "Latency" ||
		len(resp.Metrics[0].Dimensions) != 1 || resp.Metrics[0].Dimensions[0] != (Dimension{Name: "Account", Value: "123"}) {
		t.Errorf("metrics = %+v, body=%s", resp.Metrics, rec.Body.String())
	}
}

func TestQueryProtocol_SetAlarmStateReturnsXML(t *testing.T) {
	svc := New(NewMemoryStorage(""))
	putTestAlarm(t, svc, "login-failures")

	rec := postQuery(t, svc, actionSetAlarmState, `{
		"AlarmName": "login-failures",
		"StateValue": "ALARM",
		"StateReason": "test"
	}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<SetAlarmStateResponse") {
		t.Fatalf("SetAlarmState: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestQueryProtocol_ErrorsAreXML(t *testing.T) {
	svc := New(NewMemoryStorage(""))

	tests := []struct {
		name       string
		action     string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "resource not found",
			action:     actionSetAlarmState,
			body:       `{"AlarmName": "missing", "StateValue": "ALARM", "StateReason": "test"}`,
			wantStatus: http.StatusNotFound,
			wantCode:   errResourceNotFound,
		},
		{
			name:       "missing parameter",
			action:     actionPutMetricData,
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   errMissingParameter,
		},
		{
			name:       "unknown action",
			action:     "DescribeAlarmHistory",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   errInvalidAction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := postQuery(t, svc, tt.action, tt.body)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			var resp struct {
				XMLName   xml.Name `xml:"ErrorResponse"`
				Type      string   `xml:"Error>Type"`
				Code      string   `xml:"Error>Code"`
				RequestID string   `xml:"RequestId"`
			}

			if err := xml.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
			}

			if resp.Type != "Sender" || resp.Code != tt.wantCode || resp.RequestID == "" {
				t.Errorf("error = %+v, want Sender %s with a RequestId", resp, tt.wantCode)
			}
		})
	}
}

func TestJSONProtocol_ErrorsCarryQueryErrorHeader(t *testing.T) {
	svc := New(NewMemoryStorage(""))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"AlarmName": "missing", "StateValue": "ALARM", "StateReason": "test"}`))
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "GraniteServiceVersion20100801."+actionSetAlarmState)

	rec := httptest.NewRecorder()
	svc.DispatchAction(rec, req)

	if got := rec.Header().Get("x-amzn-query-error"); got != "ResourceNotFound;Sender" {
		t.Errorf("x-amzn-query-error = %q, want ResourceNotFound;Sender", got)
	}

	if !strings.Contains(rec.Body.String(), `"__type":"ResourceNotFound"`) {
		t.Errorf("body = %s, want JSON error", rec.Body.String())
	}
}

func TestPutMetricAlarm_ARNUsesConfiguredRegion(t *testing.T) {
	t.Setenv("AWS_DEFAULT_REGION", "eu-west-1")

	svc := New(NewMemoryStorage(""))
	putTestAlarm(t, svc, "login-failures")

	code, body := postJSON(t, svc, actionDescribeAlarms, `{"AlarmNames": ["login-failures"]}`)
	if code != http.StatusOK {
		t.Fatalf("DescribeAlarms: code=%d body=%s", code, body)
	}

	if !strings.Contains(body, `"AlarmArn":"arn:aws:cloudwatch:eu-west-1:`) {
		t.Errorf("body = %s, want an eu-west-1 alarm ARN", body)
	}
}
