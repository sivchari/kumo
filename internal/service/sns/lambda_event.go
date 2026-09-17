package sns

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	lambdaEventSource   = "aws:sns"
	lambdaEventVersion  = "1.0"
	lambdaTimestampFmt  = "2006-01-02T15:04:05.000Z"
	dataTypeBinary      = "Binary"
	dataTypeNumber      = "Number"
	dataTypeStringArray = "String.Array"
)

// deliverToLambda invokes the subscription's function asynchronously with the
// event Lambda receives from SNS. Without a configured invoker the delivery
// is skipped, like SQS deliveries without a publisher.
func (m *MemoryStorage) deliverToLambda(ctx context.Context, sub *Subscription, message, subject, messageID string, attributes map[string]MessageAttribute) error {
	if m.lambdaInvoker == nil {
		return nil
	}

	payload, err := json.Marshal(buildLambdaEvent(sub, message, subject, messageID, attributes))
	if err != nil {
		return fmt.Errorf("failed to marshal lambda event: %w", err)
	}

	if err := m.lambdaInvoker.InvokeAsync(ctx, sub.Endpoint, payload); err != nil {
		return fmt.Errorf("failed to invoke lambda: %w", err)
	}

	return nil
}

// buildLambdaEvent builds the SNS event for a lambda-protocol subscription.
// See https://docs.aws.amazon.com/lambda/latest/dg/with-sns.html
func buildLambdaEvent(sub *Subscription, message, subject, messageID string, attributes map[string]MessageAttribute) snsLambdaEvent {
	entity := snsLambdaMessage{
		Type:              snsNotificationType,
		MessageID:         messageID,
		TopicArn:          sub.TopicARN,
		Message:           message,
		Timestamp:         time.Now().UTC().Format(lambdaTimestampFmt),
		SignatureVersion:  "1",
		Signature:         "EXAMPLE",
		SigningCertURL:    signingCertURLPlaceholder,
		UnsubscribeURL:    unsubscribeURL(sub.ARN),
		MessageAttributes: lambdaMessageAttributes(attributes),
	}

	if subject != "" {
		entity.Subject = &subject
	}

	return snsLambdaEvent{Records: []snsLambdaRecord{{
		EventSource:          lambdaEventSource,
		EventVersion:         lambdaEventVersion,
		EventSubscriptionArn: sub.ARN,
		Sns:                  entity,
	}}}
}

// lambdaMessageAttributes projects publish attributes the way SNS delivers
// them to Lambda: Number and String.Array are not supported for lambda
// subscriptions and are passed as String, Binary values are base64-encoded.
func lambdaMessageAttributes(attributes map[string]MessageAttribute) map[string]snsNotificationAttribute {
	result := make(map[string]snsNotificationAttribute, len(attributes))

	for name, attr := range attributes {
		result[name] = lambdaMessageAttribute(attr)
	}

	return result
}

func lambdaMessageAttribute(attr MessageAttribute) snsNotificationAttribute {
	switch {
	case strings.HasPrefix(attr.DataType, dataTypeBinary):
		return snsNotificationAttribute{Type: attr.DataType, Value: base64.StdEncoding.EncodeToString(attr.BinaryValue)}
	case attr.DataType == "", strings.HasPrefix(attr.DataType, dataTypeNumber), attr.DataType == dataTypeStringArray:
		return snsNotificationAttribute{Type: dataTypeString, Value: attr.StringValue}
	default:
		return snsNotificationAttribute{Type: attr.DataType, Value: attr.StringValue}
	}
}

// unsubscribeURL builds the Unsubscribe link SNS embeds in notifications.
func unsubscribeURL(subscriptionARN string) string {
	return "https://sns.us-east-1.amazonaws.com/?Action=Unsubscribe&SubscriptionArn=" + subscriptionARN
}
