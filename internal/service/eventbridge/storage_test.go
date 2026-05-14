package eventbridge

import (
	"bytes"
	"encoding/json"
	"testing"
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
