//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/xray"
	"github.com/aws/aws-sdk-go-v2/service/xray/types"
	"github.com/sivchari/golden"
)

func TestXRay_PutTraceSegments(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createXRayClient(t, ctx)

	// Create a segment document.
	segmentDoc := map[string]any{
		"id":          "1234567890abcdef",
		"trace_id":    "1-5e6b4c2a-1234567890abcdef12345678",
		"name":        "test-service",
		"start_time":  float64(time.Now().Add(-1 * time.Second).Unix()),
		"end_time":    float64(time.Now().Unix()),
		"in_progress": false,
		"http": map[string]any{
			"request": map[string]any{
				"method":    "GET",
				"url":       "https://example.com/api",
				"client_ip": "192.168.1.1",
			},
			"response": map[string]any{
				"status": 200,
			},
		},
	}

	docBytes, err := json.Marshal(segmentDoc)
	if err != nil {
		t.Fatal(err)
	}

	// Put trace segments.
	result, err := client.PutTraceSegments(ctx, &xray.PutTraceSegmentsInput{
		TraceSegmentDocuments: []string{string(docBytes)},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), result)
}

func TestXRay_GetTraceSummaries(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createXRayClient(t, ctx)

	// First, put some trace segments.
	segmentDoc := map[string]any{
		"id":          "abcdef1234567890",
		"trace_id":    "1-5e6b4c2b-abcdef1234567890abcdef12",
		"name":        "summary-test-service",
		"start_time":  float64(time.Now().Add(-1 * time.Second).Unix()),
		"end_time":    float64(time.Now().Unix()),
		"in_progress": false,
	}

	docBytes, err := json.Marshal(segmentDoc)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.PutTraceSegments(ctx, &xray.PutTraceSegmentsInput{
		TraceSegmentDocuments: []string{string(docBytes)},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get trace summaries.
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)

	result, err := client.GetTraceSummaries(ctx, &xray.GetTraceSummariesInput{
		StartTime: &startTime,
		EndTime:   &endTime,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"TraceSummaries",
		"TracesProcessedCount",
		"ApproximateTime",
		"Id",
		"Duration",
		"ResponseTime",
		"Http",
		"Annotations",
		"Users",
		"ServiceIds",
		"EntryPoint",
		"AvailabilityZones",
		"InstanceIds",
		"ResourceARNs",
		"ResultMetadata",
	)).Assert(t.Name(), result)
}

func TestXRay_BatchGetTraces(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createXRayClient(t, ctx)

	traceID := "1-5e6b4c2c-fedcba0987654321fedcba09"

	// First, put a trace segment.
	segmentDoc := map[string]any{
		"id":          "fedcba0987654321",
		"trace_id":    traceID,
		"name":        "batch-test-service",
		"start_time":  float64(time.Now().Add(-1 * time.Second).Unix()),
		"end_time":    float64(time.Now().Unix()),
		"in_progress": false,
	}

	docBytes, err := json.Marshal(segmentDoc)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.PutTraceSegments(ctx, &xray.PutTraceSegmentsInput{
		TraceSegmentDocuments: []string{string(docBytes)},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Batch get traces.
	result, err := client.BatchGetTraces(ctx, &xray.BatchGetTracesInput{
		TraceIds: []string{traceID},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Duration",
		"Segments",
		"ResultMetadata",
	)).Assert(t.Name(), result)
}

func TestXRay_GetServiceGraph(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createXRayClient(t, ctx)

	// First, put some trace segments with service info.
	segmentDoc := map[string]any{
		"id":          "0123456789abcdef",
		"trace_id":    "1-5e6b4c2d-0123456789abcdef01234567",
		"name":        "graph-test-service",
		"start_time":  float64(time.Now().Add(-1 * time.Second).Unix()),
		"end_time":    float64(time.Now().Unix()),
		"in_progress": false,
		"origin":      "AWS::EC2::Instance",
	}

	docBytes, err := json.Marshal(segmentDoc)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.PutTraceSegments(ctx, &xray.PutTraceSegmentsInput{
		TraceSegmentDocuments: []string{string(docBytes)},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get service graph.
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)

	result, err := client.GetServiceGraph(ctx, &xray.GetServiceGraphInput{
		StartTime: &startTime,
		EndTime:   &endTime,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"StartTime",
		"EndTime",
		"Services",
		"ResultMetadata",
	)).Assert(t.Name(), result)
}

func TestXRay_CreateGroup(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createXRayClient(t, ctx)

	groupName := "test-group-" + time.Now().Format("20060102150405")

	// Create a group.
	result, err := client.CreateGroup(ctx, &xray.CreateGroupInput{
		GroupName:        aws.String(groupName),
		FilterExpression: aws.String("service(id(name: \"test-service\"))"),
		InsightsConfiguration: &types.InsightsConfiguration{
			InsightsEnabled:      aws.Bool(true),
			NotificationsEnabled: aws.Bool(false),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"GroupARN",
		"GroupName",
		"ResultMetadata",
	)).Assert(t.Name(), result)

	// Clean up: delete the group.
	_, err = client.DeleteGroup(ctx, &xray.DeleteGroupInput{
		GroupName: aws.String(groupName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestXRay_DeleteGroup(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createXRayClient(t, ctx)

	groupName := "delete-test-group-" + time.Now().Format("20060102150405")

	// Create a group first.
	_, err := client.CreateGroup(ctx, &xray.CreateGroupInput{
		GroupName: aws.String(groupName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete the group.
	_, err = client.DeleteGroup(ctx, &xray.DeleteGroupInput{
		GroupName: aws.String(groupName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Try to delete again - should fail.
	_, err = client.DeleteGroup(ctx, &xray.DeleteGroupInput{
		GroupName: aws.String(groupName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func createXRayClient(t *testing.T, _ context.Context) *xray.Client {
	t.Helper()

	return xray.NewFromConfig(awsConfig(t), func(o *xray.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}
