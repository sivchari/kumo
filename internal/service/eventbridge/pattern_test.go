package eventbridge

import (
	"testing"
)

//nolint:funlen // Table-driven test with comprehensive pattern matching coverage.
func TestMatchEventPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		event   PutEventsRequestEntry
		want    bool
	}{
		{
			name:    "empty pattern matches everything",
			pattern: "",
			event:   PutEventsRequestEntry{Source: "my.app", DetailType: "OrderCreated"},
			want:    true,
		},
		{
			name:    "source match",
			pattern: `{"source": ["my.app"]}`,
			event:   PutEventsRequestEntry{Source: "my.app", DetailType: "OrderCreated"},
			want:    true,
		},
		{
			name:    "source mismatch",
			pattern: `{"source": ["other.app"]}`,
			event:   PutEventsRequestEntry{Source: "my.app", DetailType: "OrderCreated"},
			want:    false,
		},
		{
			name:    "source multiple values",
			pattern: `{"source": ["app.a", "app.b", "my.app"]}`,
			event:   PutEventsRequestEntry{Source: "my.app"},
			want:    true,
		},
		{
			name:    "detail-type match",
			pattern: `{"detail-type": ["OrderCreated"]}`,
			event:   PutEventsRequestEntry{Source: "my.app", DetailType: "OrderCreated"},
			want:    true,
		},
		{
			name:    "detail-type mismatch",
			pattern: `{"detail-type": ["OrderDeleted"]}`,
			event:   PutEventsRequestEntry{Source: "my.app", DetailType: "OrderCreated"},
			want:    false,
		},
		{
			name:    "source AND detail-type both match",
			pattern: `{"source": ["my.app"], "detail-type": ["OrderCreated"]}`,
			event:   PutEventsRequestEntry{Source: "my.app", DetailType: "OrderCreated"},
			want:    true,
		},
		{
			name:    "source matches but detail-type does not",
			pattern: `{"source": ["my.app"], "detail-type": ["OrderDeleted"]}`,
			event:   PutEventsRequestEntry{Source: "my.app", DetailType: "OrderCreated"},
			want:    false,
		},
		{
			name:    "detail field match",
			pattern: `{"source": ["my.app"], "detail": {"status": ["completed"]}}`,
			event:   PutEventsRequestEntry{Source: "my.app", Detail: `{"status": "completed", "amount": 100}`},
			want:    true,
		},
		{
			name:    "detail field mismatch",
			pattern: `{"detail": {"status": ["pending"]}}`,
			event:   PutEventsRequestEntry{Source: "my.app", Detail: `{"status": "completed"}`},
			want:    false,
		},
		{
			name:    "detail nested object match",
			pattern: `{"detail": {"order": {"type": ["premium"]}}}`,
			event:   PutEventsRequestEntry{Detail: `{"order": {"type": "premium", "id": "123"}}`},
			want:    true,
		},
		{
			name:    "detail number match",
			pattern: `{"detail": {"count": [1, 2, 3]}}`,
			event:   PutEventsRequestEntry{Detail: `{"count": 2}`},
			want:    true,
		},
		{
			name:    "invalid pattern JSON",
			pattern: `{invalid}`,
			event:   PutEventsRequestEntry{Source: "my.app"},
			want:    false,
		},
		{
			name:    "prefix on source",
			pattern: `{"source": [{"prefix": "my."}]}`,
			event:   PutEventsRequestEntry{Source: "my.app"},
			want:    true,
		},
		{
			name:    "prefix on source mismatch",
			pattern: `{"source": [{"prefix": "other."}]}`,
			event:   PutEventsRequestEntry{Source: "my.app"},
			want:    false,
		},
		{
			name:    "suffix on detail field",
			pattern: `{"detail": {"name": [{"suffix": ".png"}]}}`,
			event:   PutEventsRequestEntry{Source: "s3", Detail: `{"name": "photo.png"}`},
			want:    true,
		},
		{
			name:    "exists true matches present field",
			pattern: `{"detail": {"userId": [{"exists": true}]}}`,
			event:   PutEventsRequestEntry{Detail: `{"userId": "u1"}`},
			want:    true,
		},
		{
			name:    "exists false matches missing field",
			pattern: `{"detail": {"userId": [{"exists": false}]}}`,
			event:   PutEventsRequestEntry{Detail: `{"orderId": "o1"}`},
			want:    true,
		},
		{
			name:    "exists true rejects missing field",
			pattern: `{"detail": {"userId": [{"exists": true}]}}`,
			event:   PutEventsRequestEntry{Detail: `{"orderId": "o1"}`},
			want:    false,
		},
		{
			name:    "anything-but single value",
			pattern: `{"detail": {"status": [{"anything-but": "draft"}]}}`,
			event:   PutEventsRequestEntry{Detail: `{"status": "published"}`},
			want:    true,
		},
		{
			name:    "anything-but single value rejects",
			pattern: `{"detail": {"status": [{"anything-but": "draft"}]}}`,
			event:   PutEventsRequestEntry{Detail: `{"status": "draft"}`},
			want:    false,
		},
		{
			name:    "anything-but list",
			pattern: `{"detail": {"status": [{"anything-but": ["draft", "archived"]}]}}`,
			event:   PutEventsRequestEntry{Detail: `{"status": "published"}`},
			want:    true,
		},
		{
			name:    "numeric range match",
			pattern: `{"detail": {"price": [{"numeric": [">", 0, "<=", 100]}]}}`,
			event:   PutEventsRequestEntry{Detail: `{"price": 50}`},
			want:    true,
		},
		{
			name:    "numeric range out of bounds",
			pattern: `{"detail": {"price": [{"numeric": [">", 0, "<=", 100]}]}}`,
			event:   PutEventsRequestEntry{Detail: `{"price": 200}`},
			want:    false,
		},
		{
			name:    "numeric equality",
			pattern: `{"detail": {"count": [{"numeric": ["=", 5]}]}}`,
			event:   PutEventsRequestEntry{Detail: `{"count": 5}`},
			want:    true,
		},
		{
			name:    "equals-ignore-case",
			pattern: `{"detail": {"region": [{"equals-ignore-case": "us-east-1"}]}}`,
			event:   PutEventsRequestEntry{Detail: `{"region": "US-EAST-1"}`},
			want:    true,
		},
		{
			name:    "literal and content filter mixed",
			pattern: `{"detail": {"state": ["pending", {"prefix": "process"}]}}`,
			event:   PutEventsRequestEntry{Detail: `{"state": "processing"}`},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := matchEventPattern(tt.pattern, &tt.event)
			if got != tt.want {
				t.Errorf("matchEventPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}
