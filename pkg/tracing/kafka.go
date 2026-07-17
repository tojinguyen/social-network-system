package tracing

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
)

// KafkaHeaderCarrier implements propagation.TextMapCarrier for Kafka headers.
type KafkaHeaderCarrier []kafka.Header

// Get retrieves the value associated with the given key.
func (c *KafkaHeaderCarrier) Get(key string) string {
	for _, h := range *c {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

// Set sets the key-value pair.
func (c *KafkaHeaderCarrier) Set(key string, value string) {
	for i, h := range *c {
		if h.Key == key {
			(*c)[i].Value = []byte(value)
			return
		}
	}
	*c = append(*c, kafka.Header{
		Key:   key,
		Value: []byte(value),
	})
}

// Keys lists the keys stored in this carrier.
func (c *KafkaHeaderCarrier) Keys() []string {
	keys := make([]string, len(*c))
	for i, h := range *c {
		keys[i] = h.Key
	}
	return keys
}

// InjectKafkaHeaders injects the trace context from ctx into the Kafka headers.
func InjectKafkaHeaders(ctx context.Context, headers *[]kafka.Header) {
	if headers == nil {
		return
	}
	carrier := KafkaHeaderCarrier(*headers)
	otel.GetTextMapPropagator().Inject(ctx, &carrier)
	*headers = []kafka.Header(carrier)
}

// ExtractKafkaHeaders extracts the trace context from Kafka headers and returns a new context.
func ExtractKafkaHeaders(ctx context.Context, headers []kafka.Header) context.Context {
	carrier := KafkaHeaderCarrier(headers)
	return otel.GetTextMapPropagator().Extract(ctx, &carrier)
}
