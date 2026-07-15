package kafka

import (
	"context"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// Consumer defines the interface for consuming events from a Kafka topic.
type Consumer interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type kafkaConsumer struct {
	reader *kafka.Reader
}

// NewConsumer creates and configures a new Kafka Consumer with manual commit strategy.
func NewConsumer(brokers string, groupID string, topic string) Consumer {
	brokerList := strings.Split(brokers, ",")
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokerList,
		GroupID:        groupID,
		Topic:          topic,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		CommitInterval: 0,    // Disabled auto-commit to support manual transaction boundary control
		MaxWait:        1 * time.Second,
	})

	return &kafkaConsumer{reader: reader}
}

func (c *kafkaConsumer) FetchMessage(ctx context.Context) (kafka.Message, error) {
	return c.reader.FetchMessage(ctx)
}

func (c *kafkaConsumer) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	return c.reader.CommitMessages(ctx, msgs...)
}

func (c *kafkaConsumer) Close() error {
	return c.reader.Close()
}
