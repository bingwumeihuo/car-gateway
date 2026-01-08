package mq

import (
	"context"
)

// Producer defines the interface for message queue producers
type Producer interface {
	Produce(ctx context.Context, topic string, key string, data interface{}) error
	Close()
}

// NoOpProducer is a dummy producer used when MQ is disabled
type NoOpProducer struct{}

func NewNoOpProducer() *NoOpProducer {
	return &NoOpProducer{}
}

func (p *NoOpProducer) Produce(ctx context.Context, topic string, key string, data interface{}) error {
	// Do nothing
	return nil
}

func (p *NoOpProducer) Close() {
	// Do nothing
}
