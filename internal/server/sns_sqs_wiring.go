package server

import (
	"context"
	"strings"

	"github.com/sivchari/kumo/internal/service"
	"github.com/sivchari/kumo/internal/service/sns"
	"github.com/sivchari/kumo/internal/service/sqs"
)

// wireSNStoSQS connects the SNS service to the SQS service so SNS topic
// subscriptions with protocol=sqs actually deliver messages into the
// target queue.
//
// Without this wiring, SNS Publish silently drops all messages destined
// for SQS subscribers (sub.Endpoint is the queue ARN, but
// MemoryStorage.Publish only iterates subscribers and calls
// SqsPublisher.PublishToSQS — and SqsPublisher is nil unless something
// installs it). Found while running a tofu serverless stack against kumo
// and watching CLI sqs receive-message return zero messages after
// sns publish.
func wireSNStoSQS(registry *service.Registry) {
	snsSvc, ok := registry.Get("sns")
	if !ok {
		return
	}

	sqsSvc, ok := registry.Get("sqs")
	if !ok {
		return
	}

	snsTyped, ok := snsSvc.(*sns.Service)
	if !ok {
		return
	}

	sqsTyped, ok := sqsSvc.(*sqs.Service)
	if !ok {
		return
	}

	snsStorage, ok := snsTyped.Storage().(*sns.MemoryStorage)
	if !ok {
		return
	}

	snsStorage.SetSQSPublisher(&snsToSQSPublisher{
		storage: sqsTyped.Storage(),
		baseURL: sqsTyped.BaseURL(),
	})
}

// snsToSQSPublisher adapts the SQS storage layer to the SNS
// SQSPublisher interface. It accepts either a queue URL or an SQS ARN
// in the endpoint argument; ARNs are translated to URLs against the
// configured base URL because that's how SNS subscriptions store the
// SQS endpoint.
type snsToSQSPublisher struct {
	storage sqs.Storage
	baseURL string
}

// PublishToSQS hands a single message to the SQS storage layer. The
// MessageId / Subject attributes the SNS layer attaches are forwarded as
// SQS message attributes (String type) so subscribers can read them.
func (p *snsToSQSPublisher) PublishToSQS(ctx context.Context, endpoint, body string, attrs map[string]string) error {
	queueURL := p.endpointToQueueURL(endpoint)

	mAttrs := make(map[string]sqs.MessageAttributeValue, len(attrs))
	for k, v := range attrs {
		mAttrs[k] = sqs.MessageAttributeValue{DataType: "String", StringValue: v}
	}

	_, err := p.storage.SendMessage(ctx, queueURL, body, 0, mAttrs, "", "")
	if err != nil {
		return err //nolint:wrapcheck // adapter is a thin pass-through
	}

	return nil
}

// endpointToQueueURL converts an SQS ARN to a queue URL, returning the
// input unchanged when it is already a URL (subscribers may send either
// shape; AWS console shows ARNs, terraform sends ARNs, raw API calls
// often pass URLs).
func (p *snsToSQSPublisher) endpointToQueueURL(endpoint string) string {
	if !strings.HasPrefix(endpoint, "arn:") {
		return endpoint
	}

	// arn:aws:sqs:<region>:<account>:<name> → <baseURL>/<account>/<name>
	parts := strings.Split(endpoint, ":")
	if len(parts) < 6 {
		return endpoint
	}

	account := parts[4]
	name := parts[5]

	return p.baseURL + "/" + account + "/" + name
}
