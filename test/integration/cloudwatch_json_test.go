//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// postCloudWatchJSON sends a SigV4-signed AWS JSON 1.0 request, the wire
// format aws-sdk-js-v3 uses for CloudWatch, and returns the response body.
func postCloudWatchJSON(ctx context.Context, t *testing.T, action, body string) []byte {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, testEndpoint()+"/", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "GraniteServiceVersion20100801."+action)

	creds, err := awsConfig(t).Credentials.Retrieve(ctx)
	if err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256([]byte(body))
	if err := v4.NewSigner().SignHTTP(ctx, creds, req, hex.EncodeToString(sum[:]), "monitoring", testRegion(), time.Now()); err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: status %d body=%s", action, resp.StatusCode, respBody)
	}

	if got := resp.Header.Get("Content-Type"); got != "application/x-amz-json-1.0" {
		t.Fatalf("%s: Content-Type = %q, want application/x-amz-json-1.0; body=%s", action, got, respBody)
	}

	return respBody
}

func TestCloudWatch_JSONProtocol_AlarmLifecycle(t *testing.T) {
	alarmName := "json-protocol-alarm"

	postCloudWatchJSON(t.Context(), t, "PutMetricAlarm", `{
		"AlarmName": "`+alarmName+`",
		"MetricName": "ConsoleLoginFailure",
		"Namespace": "Kumo/JSONProtocol",
		"Statistic": "Sum",
		"Period": 60,
		"EvaluationPeriods": 1,
		"Threshold": 1,
		"ComparisonOperator": "GreaterThanOrEqualToThreshold"
	}`)

	t.Cleanup(func() {
		postCloudWatchJSON(context.Background(), t, "DeleteAlarms", `{"AlarmNames": ["`+alarmName+`"]}`)
	})

	var described struct {
		MetricAlarms []struct {
			AlarmName                          string
			StateValue                         string
			StateUpdatedTimestamp              float64
			AlarmConfigurationUpdatedTimestamp float64
		}
	}

	body := postCloudWatchJSON(t.Context(), t, "DescribeAlarms", `{"AlarmNames": ["`+alarmName+`"]}`)
	if err := json.Unmarshal(body, &described); err != nil {
		t.Fatalf("DescribeAlarms is not JSON with epoch timestamps: %v body=%s", err, body)
	}

	if len(described.MetricAlarms) != 1 {
		t.Fatalf("expected 1 alarm, body=%s", body)
	}

	alarm := described.MetricAlarms[0]
	if alarm.AlarmName != alarmName || alarm.StateValue != "INSUFFICIENT_DATA" {
		t.Errorf("alarm = %+v", alarm)
	}

	if alarm.StateUpdatedTimestamp <= 0 || alarm.AlarmConfigurationUpdatedTimestamp <= 0 {
		t.Errorf("alarm timestamps = %v / %v, want epoch seconds", alarm.StateUpdatedTimestamp, alarm.AlarmConfigurationUpdatedTimestamp)
	}
}

func TestCloudWatch_JSONProtocol_MetricDataRoundTrip(t *testing.T) {
	postCloudWatchJSON(t.Context(), t, "PutMetricData", `{
		"Namespace": "Kumo/JSONProtocol",
		"MetricData": [{"MetricName": "Latency", "Timestamp": 1760608800.5, "Value": 7}]
	}`)

	var result struct {
		MetricDataResults []struct {
			Timestamps []float64
			Values     []float64
		}
	}

	body := postCloudWatchJSON(t.Context(), t, "GetMetricData", `{
		"StartTime": 1760608700,
		"EndTime": 1760608900,
		"MetricDataQueries": [{
			"Id": "q1",
			"MetricStat": {
				"Metric": {"Namespace": "Kumo/JSONProtocol", "MetricName": "Latency"},
				"Period": 60,
				"Stat": "Sum"
			}
		}]
	}`)
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("GetMetricData is not JSON with epoch timestamps: %v body=%s", err, body)
	}

	if len(result.MetricDataResults) != 1 {
		t.Fatalf("expected 1 result, body=%s", body)
	}

	got := result.MetricDataResults[0]
	if len(got.Timestamps) != 1 || got.Timestamps[0] != 1760608800.5 || got.Values[0] != 7 {
		t.Errorf("result = %+v, want one datapoint at 1760608800.5 with value 7", got)
	}
}

// postCloudWatchQuery sends a SigV4-signed AWS Query request, the wire format
// of older SDKs and terraform-provider-aws, and returns the status and body.
func postCloudWatchQuery(ctx context.Context, t *testing.T, action string, params url.Values) (int, []byte) {
	t.Helper()

	form := url.Values{"Action": {action}, "Version": {"2010-08-01"}}
	for k, v := range params {
		form[k] = v
	}

	body := form.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, testEndpoint()+"/", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "aws-sdk-go-v2/1.0.0 api/cloudwatch#1.0.0")

	creds, err := awsConfig(t).Credentials.Retrieve(ctx)
	if err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256([]byte(body))
	if err := v4.NewSigner().SignHTTP(ctx, creds, req, hex.EncodeToString(sum[:]), "monitoring", testRegion(), time.Now()); err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/xml") {
		t.Fatalf("%s: Content-Type = %q, want text/xml; body=%s", action, got, respBody)
	}

	return resp.StatusCode, respBody
}

func TestCloudWatch_QueryProtocol_MetricsAndAlarmState(t *testing.T) {
	ctx := t.Context()
	timestamp := time.Now().UTC().Truncate(time.Minute)

	status, body := postCloudWatchQuery(ctx, t, "PutMetricData", url.Values{
		"Namespace":                      {"Kumo/QueryProtocol"},
		"MetricData.member.1.MetricName": {"Latency"},
		"MetricData.member.1.Timestamp":  {timestamp.Format(time.RFC3339)},
		"MetricData.member.1.Value":      {"7"},
	})
	if status != http.StatusOK {
		t.Fatalf("PutMetricData: status %d body=%s", status, body)
	}

	status, body = postCloudWatchQuery(ctx, t, "GetMetricStatistics", url.Values{
		"Namespace":           {"Kumo/QueryProtocol"},
		"MetricName":          {"Latency"},
		"StartTime":           {timestamp.Add(-time.Minute).Format(time.RFC3339)},
		"EndTime":             {timestamp.Add(time.Minute).Format(time.RFC3339)},
		"Period":              {"60"},
		"Statistics.member.1": {"Sum"},
	})
	if status != http.StatusOK {
		t.Fatalf("GetMetricStatistics: status %d body=%s", status, body)
	}

	var stats struct {
		Datapoints []struct {
			Timestamp string  `xml:"Timestamp"`
			Sum       float64 `xml:"Sum"`
		} `xml:"GetMetricStatisticsResult>Datapoints>member"`
	}

	if err := xml.Unmarshal(body, &stats); err != nil {
		t.Fatalf("GetMetricStatistics is not XML: %v body=%s", err, body)
	}

	if len(stats.Datapoints) != 1 || stats.Datapoints[0].Sum != 7 || stats.Datapoints[0].Timestamp != timestamp.Format(time.RFC3339) {
		t.Errorf("datapoints = %+v, want Sum 7 at %s", stats.Datapoints, timestamp.Format(time.RFC3339))
	}

	status, body = postCloudWatchQuery(ctx, t, "SetAlarmState", url.Values{
		"AlarmName":   {"query-protocol-missing-alarm"},
		"StateValue":  {"ALARM"},
		"StateReason": {"test"},
	})

	var errResp struct {
		Code string `xml:"Error>Code"`
	}

	if err := xml.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("SetAlarmState error is not XML: %v body=%s", err, body)
	}

	if status != http.StatusNotFound || errResp.Code != "ResourceNotFound" {
		t.Errorf("SetAlarmState on a missing alarm: status %d code %q, want 404 ResourceNotFound", status, errResp.Code)
	}
}
