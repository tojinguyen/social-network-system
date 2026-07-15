package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	pkgkafka "social-network-system/pkg/kafka"
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
			log.Println("Ants pool received invalid type payload")
			return
		}
		w.processMessage(ctx, msg)
	})
	if err != nil {
		log.Fatalf("Failed to initialize ants pool: %v", err)
	}
	defer pool.Release()

	log.Printf("Fan-out Worker started with ants pool size %d, listening to topic: %s", w.cfg.WorkerPoolSize, w.cfg.PostCreatedTopic)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Fan-out consumer loop...")
			return
		default:
			msg, err := w.consumer.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("Error fetching message: %v", err)
				time.Sleep(1 * time.Second) // backoff on connection issue
				continue
			}

			// Invoke task in pool. This is blocking if no goroutines are available
			if err := pool.Invoke(msg); err != nil {
				log.Printf("Failed to invoke task in ants pool: %v", err)
			}
		}
	}
}

func (w *FanoutWorker) processMessage(ctx context.Context, msg kafka.Message) {
	db := w.mongoClient.Database(w.cfg.MongoDBName)
	followColl := db.Collection("user_follows")

	var event PostCreatedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.Printf("Error unmarshalling event: %v", err)
		// Commit corrupt message to avoid infinite retry loops
		_ = w.consumer.CommitMessages(ctx, msg)
		return
	}

	log.Printf("Processing post created event: Post=%s, Author=%s", event.PostID, event.AuthorID)

	authorObjID, err := primitive.ObjectIDFromHex(event.AuthorID)
	if err != nil {
		log.Printf("Invalid author ID: %v", err)
		_ = w.consumer.CommitMessages(ctx, msg)
		return
	}

	// 1. Evaluate celebrity threshold status
	followerCount, err := followColl.CountDocuments(ctx, bson.M{"target_id": authorObjID})
	if err != nil {
		log.Printf("Error counting followers: %v", err)
		return // retry later by not committing
	}

	if int(followerCount) >= w.cfg.CelebrityThreshold {
		log.Printf("Author %s is a Celebrity (%d followers). Skipping fan-out (Pull model will apply).", event.AuthorID, followerCount)
		_ = w.consumer.CommitMessages(ctx, msg)
		return
	}

	// 2. Query followers
	cursor, err := followColl.Find(ctx, bson.M{"target_id": authorObjID})
	if err != nil {
		log.Printf("Error finding followers: %v", err)
		return // retry
	}

	var follows []UserFollow
	if err := cursor.All(ctx, &follows); err != nil {
		log.Printf("Error decoding followers: %v", err)
		cursor.Close(ctx)
		return
	}
	cursor.Close(ctx)

	if len(follows) == 0 {
		log.Printf("Author %s has 0 followers. Skipping fan-out.", event.AuthorID)
		_ = w.consumer.CommitMessages(ctx, msg)
		return
	}

	// 3. Write to follower Redis Feed ZSET caches with Pipeline
	pipe := w.redisClient.Pipeline()
	score := float64(event.CreatedAt.UnixNano())

	for _, follow := range follows {
		followerKey := fmt.Sprintf("feed:user:%s", follow.FollowerID.Hex())

		// Push new post to feed cache
		pipe.ZAdd(ctx, followerKey, redis.Z{
			Score:  score,
			Member: event.PostID,
		})

		// Limit max cache to 500 items to conserve memory
		pipe.ZRemRangeByRank(ctx, followerKey, 0, -501)

		// Set cache expiration (7 days)
		pipe.Expire(ctx, followerKey, 7*24*time.Hour)
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		log.Printf("Error executing Redis pipeline: %v", err)
		return // retry
	}

	log.Printf("Successfully fanned out post %s to %d followers", event.PostID, len(follows))

	// 4. Commit message offset
	if err := w.consumer.CommitMessages(ctx, msg); err != nil {
		log.Printf("Error committing offset: %v", err)
	}
}
