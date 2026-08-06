package cligen

import "testing"

func TestToKebabCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{"QueueName", "queue-name"},
		{"QueueURL", "queue-url"},
		{"QueueOwnerAWSAccountId", "queue-owner-aws-account-id"},
		{"GetQueueUrl", "get-queue-url"},
		{"CreateQueue", "create-queue"},
		{"TopicArn", "topic-arn"},
		{"TableClass", "table-class"},
		{"KeyId", "key-id"},
		{"ShardCount", "shard-count"},
		{"MaxRecordSizeInKiB", "max-record-size-in-ki-b"},
		{"EventBusName", "event-bus-name"},
		{"CIDRBlock", "cidr-block"},
		{"VpcId", "vpc-id"},
		{"TTL", "ttl"},
		{"ID", "id"},
		{"URL", "url"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := toKebabCase(tt.name); got != tt.want {
				t.Errorf("toKebabCase(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestToWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{"QueueName", "Queue name"},
		{"QueueURL", "Queue url"},
		{"ShardCount", "Shard count"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := toWords(tt.name); got != tt.want {
				t.Errorf("toWords(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
