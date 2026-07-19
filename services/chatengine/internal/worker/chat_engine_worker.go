package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	pkgkafka "social-network-system/pkg/kafka"
	"social-network-system/pkg/tracing"
	"social-network-system/services/chatengine/config"
)

// ChatIncomingEvent matches the event structure published to Kafka topic.
type ChatIncomingEvent struct {
	ID          string    `json:"id"`
	SenderID    string    `json:"sender_id"`
	RecipientID string    `json:"recipient_id"`
	ContentType string    `json:"content_type"`
	Content     string    `json:"content"`
	MediaURL    string    `json:"media_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ChatReceivePayload is the payload published to Redis Pub/Sub for online users.
type ChatReceivePayload struct {
	MsgID       string    `json:"msg_id"`
	SenderID    string    `json:"sender_id"`
	RecipientID string    `json:"recipient_id,omitempty"`
	ContentType string    `json:"content_type"`
	Content     string    `json:"content"`
	MediaURL    string    `json:"media_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Message schema for MongoDB
type Message struct {
	ID             primitive.ObjectID `bson:"_id"`
	ConversationID string             `bson:"conversation_id"`
	SenderID       primitive.ObjectID `bson:"sender_id"`
	RecipientID    primitive.ObjectID `bson:"recipient_id"`
	ContentType    string             `bson:"content_type"`
	Content        string             `bson:"content"`
	MediaURL       string             `bson:"media_url,omitempty"`
	CreatedAt      time.Time          `bson:"created_at"`
}

// ChatEngineWorker handles saving incoming chats to MongoDB and routing them.
type ChatEngineWorker struct {
	cfg         *config.Config
	mongoClient *mongo.Client
	redisClient *redis.Client
	consumer    pkgkafka.Consumer
}

// NewChatEngineWorker creates a new ChatEngineWorker instance.
func NewChatEngineWorker(
	cfg *config.Config,
	mongoClient *mongo.Client,
	redisClient *redis.Client,
	consumer pkgkafka.Consumer,
) *ChatEngineWorker {
	return &ChatEngineWorker{
		cfg:         cfg,
		mongoClient: mongoClient,
		redisClient: redisClient,
		consumer:    consumer,
	}
}

// Start runs the Kafka consumer loop.
func (w *ChatEngineWorker) Start(ctx context.Context) {
	pool, err := ants.NewPoolWithFunc(w.cfg.WorkerPoolSize, func(i interface{}) {
		msg, ok := i.(kafka.Message)
		if !ok {
			slog.Error("Ants pool received invalid type payload")
			return
		}
		w.processMessage(ctx, msg)
	})
	if err != nil {
		slog.Error("Failed to initialize ants pool", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Release()

	// Ensure MongoDB messages collection index is created
	w.ensureIndexes()

	slog.Info("Chat Engine Worker started", slog.Int("pool_size", w.cfg.WorkerPoolSize), slog.String("topic", w.cfg.KafkaTopicChatIncoming))

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping Chat Engine consumer loop...")
			return
		default:
			msg, err := w.consumer.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("Error fetching message from Kafka", slog.Any("error", err))
				time.Sleep(1 * time.Second) // Backoff
				continue
			}

			if err := pool.Invoke(msg); err != nil {
				slog.Error("Failed to invoke task in ants pool", slog.Any("error", err))
			}
		}
	}
}

func (w *ChatEngineWorker) processMessage(ctx context.Context, msg kafka.Message) {
	traceCtx := ctx
	if os.Getenv("OTEL_ENABLED") == "true" {
		traceCtx = tracing.ExtractKafkaHeaders(ctx, msg.Headers)
		tracer := otel.Tracer("chatengine-worker")
		var span trace.Span
		traceCtx, span = tracer.Start(traceCtx, "process_chat_incoming_event", trace.WithSpanKind(trace.SpanKindConsumer))
		defer span.End()
		span.SetAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", w.cfg.KafkaTopicChatIncoming),
			attribute.String("messaging.operation", "process"),
		)
	}

	var event ChatIncomingEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		slog.ErrorContext(traceCtx, "Error unmarshalling event", slog.Any("error", err))
		// Commit bad format messages to avoid blocking
		_ = w.consumer.CommitMessages(traceCtx, msg)
		return
	}

	if os.Getenv("OTEL_ENABLED") == "true" {
		span := trace.SpanFromContext(traceCtx)
		span.SetAttributes(
			attribute.String("message.id", event.ID),
			attribute.String("sender.id", event.SenderID),
			attribute.String("recipient.id", event.RecipientID),
		)
	}

	slog.InfoContext(traceCtx, "Processing chat incoming event",
		slog.String("msg_id", event.ID),
		slog.String("sender", event.SenderID),
		slog.String("recipient", event.RecipientID))

	msgObjID, err := primitive.ObjectIDFromHex(event.ID)
	if err != nil {
		slog.ErrorContext(traceCtx, "Invalid message ID", slog.Any("error", err))
		_ = w.consumer.CommitMessages(traceCtx, msg)
		return
	}

	senderObjID, err := primitive.ObjectIDFromHex(event.SenderID)
	if err != nil {
		slog.ErrorContext(traceCtx, "Invalid sender ID", slog.Any("error", err))
		_ = w.consumer.CommitMessages(traceCtx, msg)
		return
	}

	recipientObjID, err := primitive.ObjectIDFromHex(event.RecipientID)
	if err != nil {
		slog.ErrorContext(traceCtx, "Invalid recipient ID", slog.Any("error", err))
		_ = w.consumer.CommitMessages(traceCtx, msg)
		return
	}

	// 1. Ghi tin nhắn vào MongoDB
	db := w.mongoClient.Database(w.cfg.MongoDBName)
	messagesColl := db.Collection("messages")

	conversationID := getConversationID(event.SenderID, event.RecipientID)
	dbMessage := Message{
		ID:             msgObjID,
		ConversationID: conversationID,
		SenderID:       senderObjID,
		RecipientID:    recipientObjID,
		ContentType:    event.ContentType,
		Content:        event.Content,
		MediaURL:       event.MediaURL,
		CreatedAt:      event.CreatedAt,
	}

	_, err = messagesColl.InsertOne(traceCtx, dbMessage)
	if err != nil {
		// Log but do not skip/commit offset in case of temporary DB failure to allow retry
		slog.ErrorContext(traceCtx, "Failed to save message to MongoDB", slog.Any("error", err))
		return
	}

	// 2. Tra cứu trạng thái online của recipient trong Redis
	presenceKey := fmt.Sprintf("presence:user:%s", event.RecipientID)
	nodeID, err := w.redisClient.Get(traceCtx, presenceKey).Result()
	if err == nil && nodeID != "" {
		// User online -> Routing qua Redis Pub/Sub
		routeChannel := fmt.Sprintf("chat_node:%s", nodeID)

		pubPayload := ChatReceivePayload{
			MsgID:       event.ID,
			SenderID:    event.SenderID,
			RecipientID: event.RecipientID,
			ContentType: event.ContentType,
			Content:     event.Content,
			MediaURL:    event.MediaURL,
			CreatedAt:   event.CreatedAt,
		}

		payloadBytes, err := json.Marshal(pubPayload)
		if err == nil {
			err = w.redisClient.Publish(traceCtx, routeChannel, payloadBytes).Err()
			if err != nil {
				slog.ErrorContext(traceCtx, "Failed to publish message routing to Redis", slog.Any("error", err))
			} else {
				slog.InfoContext(traceCtx, "Message routed online",
					slog.String("node_id", nodeID),
					slog.String("recipient", event.RecipientID))
			}
		}
	} else {
		// User offline -> Giả lập offline notification
		slog.InfoContext(traceCtx, "User is offline. Triggering Offline Push Notification",
			slog.String("recipient", event.RecipientID),
			slog.String("sender", event.SenderID))
	}

	// 3. Commit offset lên Kafka
	if err := w.consumer.CommitMessages(traceCtx, msg); err != nil {
		slog.ErrorContext(traceCtx, "Error committing Kafka offset", slog.Any("error", err))
	}
}

func (w *ChatEngineWorker) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := w.mongoClient.Database(w.cfg.MongoDBName).Collection("messages")
	_, _ = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "conversation_id", Value: 1},
			{Key: "created_at", Value: -1},
		},
	})
}

// getConversationID creates a unique conversation identifier between two users
func getConversationID(userA, userB string) string {
	slice := []string{userA, userB}
	sort.Strings(slice)
	return slice[0] + "_" + slice[1]
}
