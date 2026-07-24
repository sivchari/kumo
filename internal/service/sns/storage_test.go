package sns

import (
	"errors"
	"testing"
)

func TestCreateTopicRejectsDelimiterNames(t *testing.T) {
	t.Parallel()

	tests := []string{
		"bad:name",
		"bad/name",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := NewMemoryStorage("http://localhost:4566")

			_, err := store.CreateTopic(t.Context(), name, nil, nil)
			expectTopicErrorCode(t, err, "InvalidParameter")
		})
	}
}

func TestCreateTopicAcceptsValidNames(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage("http://localhost:4566")

	for _, name := range []string{"topic_name-1", "topic-name.fifo"} {
		topic, err := store.CreateTopic(t.Context(), name, nil, nil)
		if err != nil {
			t.Fatalf("CreateTopic(%q): %v", name, err)
		}

		if topic.Name != name {
			t.Fatalf("CreateTopic(%q) name = %q", name, topic.Name)
		}
	}
}

func expectTopicErrorCode(t *testing.T, err error, code string) {
	t.Helper()

	var topicErr *TopicError
	if !errors.As(err, &topicErr) {
		t.Fatalf("got err %v, want TopicError code %s", err, code)
	}

	if topicErr.Code != code {
		t.Fatalf("got TopicError code %s, want %s", topicErr.Code, code)
	}
}
