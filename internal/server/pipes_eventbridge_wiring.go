package server

import (
	"context"
	"fmt"

	"github.com/sivchari/kumo/internal/service"
	"github.com/sivchari/kumo/internal/service/eventbridge"
	"github.com/sivchari/kumo/internal/service/pipes"
)

// wirePipesToEventBridge connects the EventBridge Pipes service to the
// EventBridge service so that a pipe whose source is a DynamoDB stream and
// whose target is an EventBridge event bus actually polls the stream and
// publishes events onto the bus.
//
// Without this wiring, Pipes only stores the pipe definition and never moves
// records, so the downstream rule/target chain (in the example audit pipeline:
// rule -> audit-forwarder Lambda -> Firehose -> S3) is never triggered.
func wirePipesToEventBridge(registry *service.Registry) {
	pipesSvc, ok := registry.Get("pipes")
	if !ok {
		return
	}

	ebSvc, ok := registry.Get("events")
	if !ok {
		return
	}

	pipesTyped, ok := pipesSvc.(*pipes.Service)
	if !ok {
		return
	}

	ebTyped, ok := ebSvc.(*eventbridge.Service)
	if !ok {
		return
	}

	pipesStorage, ok := pipesTyped.Storage().(*pipes.MemoryStorage)
	if !ok {
		return
	}

	ebStorage, ok := ebTyped.Storage().(*eventbridge.MemoryStorage)
	if !ok {
		return
	}

	pipesStorage.SetEventPublisher(&pipesToEventBridgePublisher{storage: ebStorage})
}

// pipesToEventBridgePublisher adapts EventBridge's PutEvents to the
// pipes.EventPublisher interface.
type pipesToEventBridgePublisher struct {
	storage *eventbridge.MemoryStorage
}

// PutEvents publishes a single event entry onto the named bus.
func (p *pipesToEventBridgePublisher) PutEvents(ctx context.Context, busName, source, detailType, detail string) error {
	_, err := p.storage.PutEvents(ctx, []eventbridge.PutEventsRequestEntry{
		{
			Source:       source,
			DetailType:   detailType,
			Detail:       detail,
			EventBusName: busName,
		},
	})
	if err != nil {
		return fmt.Errorf("pipes event publish failed: %w", err)
	}

	return nil
}
