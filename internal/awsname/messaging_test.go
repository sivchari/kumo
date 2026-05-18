package awsname

import (
	"strings"
	"testing"
)

func TestIsValidMessagingResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		maxLen int
		want   bool
	}{
		{name: "plain", input: "topic", maxLen: 256, want: true},
		{name: "fifo", input: "topic.fifo", maxLen: 256, want: true},
		{name: "alphanumeric and separators", input: "Topic_1-2", maxLen: 256, want: true},
		{name: "empty", input: "", maxLen: 256, want: false},
		{name: "only .fifo", input: ".fifo", maxLen: 256, want: false},
		{name: "embedded dot", input: "to.pic", maxLen: 256, want: false},
		{name: "invalid char", input: "topic/sub", maxLen: 256, want: false},
		{name: "space", input: "topic name", maxLen: 256, want: false},
		{name: "exactly maxLen", input: strings.Repeat("a", 80), maxLen: 80, want: true},
		{name: "over maxLen", input: strings.Repeat("a", 81), maxLen: 80, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := IsValidMessagingResource(tt.input, tt.maxLen); got != tt.want {
				t.Errorf("IsValidMessagingResource(%q, %d) = %v, want %v", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
