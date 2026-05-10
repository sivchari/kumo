package eventbridge

import (
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
