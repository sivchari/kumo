package eventbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

//nolint:funlen // Table-driven test with comprehensive InputPath coverage.
func TestResolveInputPath(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"version":"0","id":"abc","source":"my.app","detail-type":"OrderCreated","detail":{"orderId":"123","nested":{"key":"val"}},"region":"us-east-1","account":"000000000000","time":"2026-01-01T00:00:00Z"}`)

	tests := []struct {
		name      string
		inputPath string
		wantNil   bool
		want      string
	}{
		{
			name:      "empty path returns original",
			inputPath: "",
			want:      string(payload),
		},
		{
			name:      "dollar only returns original",
			inputPath: "$",
			want:      string(payload),
		},
		{
			name:      "extract detail",
			inputPath: "$.detail",
			want:      `{"nested":{"key":"val"},"orderId":"123"}`,
		},
		{
			name:      "extract nested field",
			inputPath: "$.detail.nested",
			want:      `{"key":"val"}`,
		},
		{
			name:      "extract scalar field",
			inputPath: "$.detail.orderId",
			want:      `"123"`,
		},
		{
			name:      "extract source",
			inputPath: "$.source",
			want:      `"my.app"`,
		},
		{
			name:      "non-existent path returns nil",
			inputPath: "$.nonexistent",
			wantNil:   true,
		},
		{
			name:      "non-existent nested path returns nil",
			inputPath: "$.detail.nonexistent.deep",
			wantNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := resolveInputPath(payload, tt.inputPath)

			if tt.wantNil {
				if got != nil {
					t.Errorf("resolveInputPath() = %s, want nil", string(got))
				}

				return
			}

			if string(got) != tt.want {
				t.Errorf("resolveInputPath() = %s, want %s", string(got), tt.want)
			}
		})
	}
}

//nolint:funlen // Table-driven test with comprehensive extractJSONPath coverage.
func TestExtractJSONPath(t *testing.T) {
	t.Parallel()

	event := map[string]any{
		"version":     "0",
		"id":          "abc-123",
		"source":      "my.app",
		"detail-type": "OrderCreated",
		"detail": map[string]any{
			"marker": "test-marker-value",
			"nested": map[string]any{
				"key": "val",
			},
			"count": float64(42),
		},
	}

	tests := []struct {
		name string
		path string
		want any
	}{
		{
			name: "extract top-level string field",
			path: "$.source",
			want: "my.app",
		},
		{
			name: "extract nested string field",
			path: "$.detail.marker",
			want: "test-marker-value",
		},
		{
			name: "extract deeply nested field",
			path: "$.detail.nested.key",
			want: "val",
		},
		{
			name: "extract nested object",
			path: "$.detail.nested",
			want: map[string]any{"key": "val"},
		},
		{
			name: "extract numeric field",
			path: "$.detail.count",
			want: float64(42),
		},
		{
			name: "non-existent field returns nil",
			path: "$.nonexistent",
			want: nil,
		},
		{
			name: "non-existent nested path returns nil",
			path: "$.detail.nonexistent.deep",
			want: nil,
		},
		{
			name: "empty path returns entire object",
			path: "",
			want: event,
		},
		{
			name: "dollar only returns entire object",
			path: "$",
			want: event,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := extractJSONPath(event, tt.path)

			if tt.want == nil {
				if got != nil {
					t.Errorf("extractJSONPath() = %v, want nil", got)
				}

				return
			}

			// For map comparison, marshal both to JSON.
			if _, ok := tt.want.(map[string]any); ok {
				gotJSON, _ := json.Marshal(got)
				wantJSON, _ := json.Marshal(tt.want)

				if !bytes.Equal(gotJSON, wantJSON) {
					t.Errorf("extractJSONPath() = %s, want %s", gotJSON, wantJSON)
				}

				return
			}

			if got != tt.want {
				t.Errorf("extractJSONPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

//nolint:funlen // Table-driven test with comprehensive InputTransformer coverage.
func TestApplyInputTransformer(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"version":"0","id":"abc","source":"my.app","detail-type":"OrderCreated","detail":{"marker":"test-marker-123","nested":{"key":"val"}},"region":"us-east-1","account":"000000000000","time":"2026-01-01T00:00:00Z"}`)

	tests := []struct {
		name        string
		transformer *InputTransformer
		want        string
	}{
		{
			name: "simple string replacement",
			transformer: &InputTransformer{
				InputPathsMap: map[string]string{
					"marker": "$.detail.marker",
				},
				InputTemplate: `{"transformedMarker": <marker>, "source": "custom-bus"}`,
			},
			want: `{"transformedMarker": "test-marker-123", "source": "custom-bus"}`,
		},
		{
			name: "multiple replacements",
			transformer: &InputTransformer{
				InputPathsMap: map[string]string{
					"marker": "$.detail.marker",
					"src":    "$.source",
				},
				InputTemplate: `{"marker": <marker>, "src": <src>}`,
			},
			want: `{"marker": "test-marker-123", "src": "my.app"}`,
		},
		{
			name: "object replacement",
			transformer: &InputTransformer{
				InputPathsMap: map[string]string{
					"nested": "$.detail.nested",
				},
				InputTemplate: `{"data": <nested>}`,
			},
			want: `{"data": {"key":"val"}}`,
		},
		{
			name: "non-existent path yields null",
			transformer: &InputTransformer{
				InputPathsMap: map[string]string{
					"missing": "$.nonexistent",
				},
				InputTemplate: `{"value": <missing>}`,
			},
			want: `{"value": null}`,
		},
		{
			name: "empty InputPathsMap with no placeholders",
			transformer: &InputTransformer{
				InputPathsMap: nil,
				InputTemplate: `{"static": "value"}`,
			},
			want: `{"static": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := applyInputTransformer(payload, tt.transformer)
			if string(got) != tt.want {
				t.Errorf("applyInputTransformer() = %s, want %s", string(got), tt.want)
			}
		})
	}
}

func TestIsLambdaArn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		arn  string
		want bool
	}{
		{
			name: "Lambda function ARN",
			arn:  "arn:aws:lambda:us-east-1:000000000000:function:my-func",
			want: true,
		},
		{
			name: "Lambda qualified ARN",
			arn:  "arn:aws:lambda:us-east-1:000000000000:function:my-func:PROD",
			want: true,
		},
		{
			name: "SQS ARN",
			arn:  "arn:aws:sqs:us-east-1:000000000000:my-queue",
			want: false,
		},
		{
			name: "empty string",
			arn:  "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isLambdaArn(tt.arn); got != tt.want {
				t.Errorf("isLambdaArn(%q) = %v, want %v", tt.arn, got, tt.want)
			}
		})
	}
}

func TestIsSQSArn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		arn  string
		want bool
	}{
		{
			name: "SQS ARN",
			arn:  "arn:aws:sqs:us-east-1:000000000000:my-queue",
			want: true,
		},
		{
			name: "Lambda ARN",
			arn:  "arn:aws:lambda:us-east-1:000000000000:function:my-func",
			want: false,
		},
		{
			name: "API destination ARN",
			arn:  "arn:aws:events:us-east-1:000000000000:api-destination/my-dest",
			want: false,
		},
		{
			name: "empty string",
			arn:  "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isSQSArn(tt.arn); got != tt.want {
				t.Errorf("isSQSArn(%q) = %v, want %v", tt.arn, got, tt.want)
			}
		})
	}
}

const (
	regionAPNE1 = "ap-northeast-1"
	regionEUW1  = "eu-west-1"
	regionUSE1  = "us-east-1"
)

// TestNewMemoryStorage_RegionFromEnv verifies that NewMemoryStorage honours
// AWS_DEFAULT_REGION and falls back to us-east-1 when the env var is empty
// or unset, matching the convention established by sfn (PR #520).
func TestNewMemoryStorage_RegionFromEnv(t *testing.T) {
	//nolint:paralleltest // t.Setenv is incompatible with t.Parallel.
	tests := []struct {
		name       string
		envValue   string
		wantRegion string
	}{
		{
			name:       "AWS_DEFAULT_REGION " + regionAPNE1,
			envValue:   regionAPNE1,
			wantRegion: regionAPNE1,
		},
		{
			name:       "AWS_DEFAULT_REGION " + regionEUW1,
			envValue:   regionEUW1,
			wantRegion: regionEUW1,
		},
		{
			name:       "AWS_DEFAULT_REGION empty falls back to " + regionUSE1,
			envValue:   "",
			wantRegion: regionUSE1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AWS_DEFAULT_REGION", tt.envValue)

			s := NewMemoryStorage()
			if s.region != tt.wantRegion {
				t.Errorf("region = %q, want %q", s.region, tt.wantRegion)
			}

			wantArn := "arn:aws:events:" + tt.wantRegion + ":000000000000:event-bus/default"
			if got := s.EventBuses[defaultEventBusName].Arn; got != wantArn {
				t.Errorf("default event bus ARN = %q, want %q", got, wantArn)
			}
		})
	}
}

// TestPutEvents_APIDestinationCrossRegion is the regression test for the
// silent-drop bug where a target ARN whose region differs from the storage's
// own region failed strict ARN comparison in resolveAPIDestination, causing
// matchAndDeliver to skip dispatch while PutEvents still reported success.
//
//nolint:funlen // End-to-end test with setup, dispatch, and assertion.
func TestPutEvents_APIDestinationCrossRegion(t *testing.T) {
	t.Parallel()

	received := make(chan []byte, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		select {
		case received <- body:
		default:
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := NewMemoryStorage()
	ctx := context.Background()

	conn, err := s.CreateConnection(ctx, &CreateConnectionRequest{
		Name:              "test-conn",
		AuthorizationType: "API_KEY",
		AuthParameters: AuthParameters{
			APIKeyAuthParameters: &APIKeyAuthParameters{
				APIKeyName:  "X-Api-Key",
				APIKeyValue: "secret",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}

	dest, err := s.CreateAPIDestination(ctx, &CreateAPIDestinationRequest{
		Name:               "test-dest",
		ConnectionArn:      conn.Arn,
		InvocationEndpoint: server.URL,
		HTTPMethod:         http.MethodPost,
	})
	if err != nil {
		t.Fatalf("CreateAPIDestination: %v", err)
	}

	// The storage emits the API destination ARN under its own region. Pick a
	// different region for the target ARN so we exercise the cross-region path.
	otherRegion := regionAPNE1
	if s.region == otherRegion {
		otherRegion = regionEUW1
	}

	if strings.Contains(dest.Arn, ":"+otherRegion+":") {
		t.Fatalf("test precondition failed: dest.Arn %q should not contain %q", dest.Arn, otherRegion)
	}

	crossRegionTargetArn := "arn:aws:events:" + otherRegion + ":000000000000:api-destination/" + dest.Name

	if _, err := s.PutRule(ctx, &PutRuleRequest{
		Name:         "test-rule",
		EventPattern: `{"source":["my.test"]}`,
		State:        "ENABLED",
	}); err != nil {
		t.Fatalf("PutRule: %v", err)
	}

	if _, err := s.PutTargets(ctx, "", "test-rule", []TargetInput{
		{ID: "target-1", Arn: crossRegionTargetArn},
	}); err != nil {
		t.Fatalf("PutTargets: %v", err)
	}

	if _, err := s.PutEvents(ctx, []PutEventsRequestEntry{
		{Source: "my.test", DetailType: "TestEvent", Detail: `{"hello":"world"}`},
	}); err != nil {
		t.Fatalf("PutEvents: %v", err)
	}

	select {
	case body := <-received:
		var ev map[string]any
		if err := json.Unmarshal(body, &ev); err != nil {
			t.Fatalf("dispatched body not valid JSON: %v", err)
		}

		if ev["source"] != "my.test" {
			t.Errorf("source = %v, want my.test", ev["source"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive dispatched event within timeout (silent drop)")
	}
}
