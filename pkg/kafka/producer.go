package kafka

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"social-network-system/pkg/tracing"
)

// Producer defines the interface for publishing events to a Kafka topic.
type Producer interface {
	Publish(ctx context.Context, key string, value interface{}) error
	Close() error
}

type kafkaProducer struct {
	writer *kafka.Writer
}

// NewProducer creates and configures a new Kafka Producer.
func NewProducer(brokers string, topic string) Producer {
	brokerList := strings.Split(brokers, ",")
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokerList...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		WriteTimeout: 10 * time.Second,
		RequiredAcks: kafka.RequireAll,
		Async:        false,
	}

	return &kafkaProducer{writer: writer}
}

func (p *kafkaProducer) Publish(ctx context.Context, key string, value interface{}) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: payload,
	}

	if os.Getenv("OTEL_ENABLED") == "true" {
		tracing.InjectKafkaHeaders(ctx, &msg.Headers)
	}

	return p.writer.WriteMessages(ctx, msg)
}

func (p *kafkaProducer) Close() error {
	return p.writer.Close()
}
