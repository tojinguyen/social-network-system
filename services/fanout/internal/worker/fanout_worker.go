package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
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
	"social-network-system/services/fanout/config"
)

// PostCreatedEvent matches the schema emitted by the Post Service.
type PostCreatedEvent struct {
	PostID    string    `json:"post_id"`
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}

// UserFollow maps follow relationship in database.
type UserFollow struct {
	FollowerID primitive.ObjectID `bson:"follower_id"`
	TargetID   primitive.ObjectID `bson:"target_id"`
}

// FanoutWorker handles fanning out posts to followers' feed cache.
type FanoutWorker struct {
	cfg         *config.Config
	mongoClient *mongo.Client
	redisClient *redis.Client
	consumer    pkgkafka.Consumer
}

// NewFanoutWorker creates a new FanoutWorker.
func NewFanoutWorker(
	cfg *config.Config,
	mongoClient *mongo.Client,
	redisClient *redis.Client,
	consumer pkgkafka.Consumer,
) *FanoutWorker {
	return &FanoutWorker{
		cfg:         cfg,
		mongoClient: mongoClient,
		redisClient: redisClient,
		consumer:    consumer,
	}
}

// Start initiates the consumer loop, fetching events and distributing them to the ants worker pool.
func (w *FanoutWorker) Start(ctx context.Context) {
	// Initialize ants pool with function
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

	slog.Info("Fan-out Worker started", slog.Int("pool_size", w.cfg.WorkerPoolSize), slog.String("topic", w.cfg.PostCreatedTopic))

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping Fan-out consumer loop...")
			return
		default:
			msg, err := w.consumer.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("Error fetching message", slog.Any("error", err))
				time.Sleep(1 * time.Second) // backoff on connection issue
				continue
			}

			// Invoke task in pool. This is blocking if no goroutines are available
			if err := pool.Invoke(msg); err != nil {
				slog.Error("Failed to invoke task in ants pool", slog.Any("error", err))
			}
		}
	}
}

func (w *FanoutWorker) processMessage(ctx context.Context, msg kafka.Message) {
	traceCtx := ctx
	if os.Getenv("OTEL_ENABLED") == "true" {
		traceCtx = tracing.ExtractKafkaHeaders(ctx, msg.Headers)
		tracer := otel.Tracer("fanout-worker")
		var span trace.Span
		traceCtx, span = tracer.Start(traceCtx, "process_post_created_event", trace.WithSpanKind(trace.SpanKindConsumer))
		defer span.End()
		span.SetAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", w.cfg.PostCreatedTopic),
			attribute.String("messaging.operation", "process"),
		)
	}

	db := w.mongoClient.Database(w.cfg.MongoDBName)
	followColl := db.Collection("user_follows")

	var event PostCreatedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		slog.ErrorContext(traceCtx, "Error unmarshalling event", slog.Any("error", err))
		// Commit corrupt message to avoid infinite retry loops
		_ = w.consumer.CommitMessages(traceCtx, msg)
		return
	}

	if os.Getenv("OTEL_ENABLED") == "true" {
		span := trace.SpanFromContext(traceCtx)
		span.SetAttributes(
			attribute.String("post.id", event.PostID),
			attribute.String("author.id", event.AuthorID),
		)
	}

	slog.InfoContext(traceCtx, "Processing post created event", slog.String("post_id", event.PostID), slog.String("author_id", event.AuthorID))

	authorObjID, err := primitive.ObjectIDFromHex(event.AuthorID)
	if err != nil {
		slog.ErrorContext(traceCtx, "Invalid author ID", slog.Any("error", err))
		_ = w.consumer.CommitMessages(traceCtx, msg)
		return
	}

	// 1. Evaluate celebrity threshold status
	followerCount, err := followColl.CountDocuments(traceCtx, bson.M{"target_id": authorObjID})
	if err != nil {
		slog.ErrorContext(traceCtx, "Error counting followers", slog.Any("error", err))
		return // retry later by not committing
	}

	if int(followerCount) >= w.cfg.CelebrityThreshold {
		slog.InfoContext(traceCtx, "Author is a Celebrity, skipping fan-out (Pull model will apply)",
			slog.String("author_id", event.AuthorID),
			slog.Int64("followers", followerCount))
		_ = w.consumer.CommitMessages(traceCtx, msg)
		return
	}

	// 2. Query followers
	cursor, err := followColl.Find(traceCtx, bson.M{"target_id": authorObjID})
	if err != nil {
		slog.ErrorContext(traceCtx, "Error finding followers", slog.Any("error", err))
		return // retry
	}

	var follows []UserFollow
	if err := cursor.All(traceCtx, &follows); err != nil {
		slog.ErrorContext(traceCtx, "Error decoding followers", slog.Any("error", err))
		cursor.Close(traceCtx)
		return
	}
	cursor.Close(traceCtx)

	if len(follows) == 0 {
		slog.InfoContext(traceCtx, "Author has 0 followers, skipping fan-out", slog.String("author_id", event.AuthorID))
		_ = w.consumer.CommitMessages(traceCtx, msg)
		return
	}

	// 3. Write to follower Redis Feed ZSET caches with Pipeline
	pipe := w.redisClient.Pipeline()
	score := float64(event.CreatedAt.UnixNano())

	for _, follow := range follows {
		followerKey := fmt.Sprintf("feed:user:%s", follow.FollowerID.Hex())

		// Push new post to feed cache
		pipe.ZAdd(traceCtx, followerKey, redis.Z{
			Score:  score,
			Member: event.PostID,
		})

		// Limit max cache to 500 items to conserve memory
		pipe.ZRemRangeByRank(traceCtx, followerKey, 0, -501)

		// Set cache expiration (7 days)
		pipe.Expire(traceCtx, followerKey, 7*24*time.Hour)
	}

	_, err = pipe.Exec(traceCtx)
	if err != nil {
		slog.ErrorContext(traceCtx, "Error executing Redis pipeline", slog.Any("error", err))
		return // retry
	}

	slog.InfoContext(traceCtx, "Successfully fanned out post",
		slog.String("post_id", event.PostID),
		slog.Int("followers", len(follows)))

	// 4. Commit message offset
	if err := w.consumer.CommitMessages(traceCtx, msg); err != nil {
		slog.ErrorContext(traceCtx, "Error committing offset", slog.Any("error", err))
	}
}
