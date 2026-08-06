//go:build integration

package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/pinpointsmsvoicev2"
)

func newPinpointSMSVoiceV2Client(t *testing.T) *pinpointsmsvoicev2.Client {
	t.Helper()

	return pinpointsmsvoicev2.NewFromConfig(awsConfig(t), func(o *pinpointsmsvoicev2.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestPinpointSMSVoiceV2_SendTextMessage(t *testing.T) {
	client := newPinpointSMSVoiceV2Client(t)
	ctx := t.Context()

	output, err := client.SendTextMessage(ctx, &pinpointsmsvoicev2.SendTextMessageInput{
		DestinationPhoneNumber: aws.String("+1234567890"),
		MessageBody:            aws.String("Hello from kumo"),
		OriginationIdentity:    aws.String("+0987654321"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if output.MessageId == nil || *output.MessageId == "" {
		t.Fatal("expected non-empty MessageId")
	}
}

func TestPinpointSMSVoiceV2_GetSentTextMessages(t *testing.T) {
	client := newPinpointSMSVoiceV2Client(t)
	ctx := t.Context()

	// Send a text message.
	destPhone := "+1112223333"
	msgBody := "Test SMS body"

	_, err := client.SendTextMessage(ctx, &pinpointsmsvoicev2.SendTextMessageInput{
		DestinationPhoneNumber: aws.String(destPhone),
		MessageBody:            aws.String(msgBody),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get sent messages via kumo-specific endpoint.
	resp, err := http.Get(testEndpoint() + "/kumo/pinpointsmsvoicev2/sent-messages")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status %d, got %d, body: %s", http.StatusOK, resp.StatusCode, body)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	sentMessages, ok := result["SentTextMessages"]
	if !ok {
		t.Fatal("SentTextMessages field not found in response")
	}

	messages, ok := sentMessages.([]interface{})
	if !ok {
		t.Fatal("SentTextMessages is not an array")
	}

	if len(messages) == 0 {
		t.Fatal("no sent messages found")
	}

	// Find our message.
	var found bool

	for _, m := range messages {
		msg, ok := m.(map[string]interface{})
		if !ok {
			continue
		}

		if msg["DestinationPhoneNumber"] == destPhone && msg["MessageBody"] == msgBody {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("sent message to %s with body '%s' not found", destPhone, msgBody)
	}
}
