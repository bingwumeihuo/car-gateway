package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"vehicle-gateway/internal/config"
	"vehicle-gateway/internal/infra/mq"
)

type KafkaProducer struct {
	writer *kafka.Writer
	logger *zap.Logger
	topic  string
}

// Ensure KafkaProducer implements mq.Producer
var _ mq.Producer = (*KafkaProducer)(nil)

func NewKafkaProducer(cfg config.KafkaConfig, logger *zap.Logger) (*KafkaProducer, error) {
	w := &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Brokers...),
		Topic:                  cfg.Topic, // Default topic
		Balancer:               &kafka.LeastBytes{},
		WriteTimeout:           10 * time.Second,
		RequiredAcks:           kafka.RequireOne,
		AllowAutoTopicCreation: true,
		Async:                  true, // Async writing for better performance
	}

	logger.Info("Initialized Kafka producer", zap.Strings("brokers", cfg.Brokers), zap.String("topic", cfg.Topic))

	return &KafkaProducer{
		writer: w,
		logger: logger,
		topic:  cfg.Topic,
	}, nil
}

func (p *KafkaProducer) Produce(ctx context.Context, topic string, key string, data interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	targetTopic := p.topic
	if topic != "" {
		targetTopic = topic
	}

	err = p.writer.WriteMessages(ctx,
		kafka.Message{
			Topic: targetTopic,
			Key:   []byte(key),
			Value: body,
		},
	)

	if err != nil {
		p.logger.Error("Failed to produce message to Kafka", zap.Error(err), zap.String("topic", targetTopic))
		return err
	}

	p.logger.Debug("Produced message to Kafka", zap.String("topic", targetTopic), zap.String("key", key))
	return nil
}

func (p *KafkaProducer) Close() {
	if err := p.writer.Close(); err != nil {
		p.logger.Error("Failed to close Kafka writer", zap.Error(err))
	}
}
