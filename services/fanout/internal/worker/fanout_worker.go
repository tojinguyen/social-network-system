package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"social-network-system/pkg/kafka"
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
	consumer    kafka.Consumer
}

// NewFanoutWorker creates a new FanoutWorker.
func NewFanoutWorker(
	cfg *config.Config,
	mongoClient *mongo.Client,
	redisClient *redis.Client,
	consumer kafka.Consumer,
) *FanoutWorker {
	return &FanoutWorker{
		cfg:         cfg,
		mongoClient: mongoClient,
		redisClient: redisClient,
		consumer:    consumer,
	}
}

// Start initiates the consumer loop, fetching events and writing to Redis ZSET.
func (w *FanoutWorker) Start(ctx context.Context) {
	log.Printf("Fan-out Worker started, listening to topic: %s", w.cfg.PostCreatedTopic)

	db := w.mongoClient.Database(w.cfg.MongoDBName)
	followColl := db.Collection("user_follows")

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Fan-out Worker...")
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

			var event PostCreatedEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf("Error unmarshalling event: %v", err)
				// Commit corrupt message to avoid infinite retry loops
				_ = w.consumer.CommitMessages(ctx, msg)
				continue
			}

			log.Printf("Processing post created event: Post=%s, Author=%s", event.PostID, event.AuthorID)

			authorObjID, err := primitive.ObjectIDFromHex(event.AuthorID)
			if err != nil {
				log.Printf("Invalid author ID: %v", err)
				_ = w.consumer.CommitMessages(ctx, msg)
				continue
			}

			// 1. Evaluate celebrity threshold status
			followerCount, err := followColl.CountDocuments(ctx, bson.M{"target_id": authorObjID})
			if err != nil {
				log.Printf("Error counting followers: %v", err)
				continue // retry later
			}

			if int(followerCount) >= w.cfg.CelebrityThreshold {
				log.Printf("Author %s is a Celebrity (%d followers). Skipping fan-out (Pull model will apply).", event.AuthorID, followerCount)
				_ = w.consumer.CommitMessages(ctx, msg)
				continue
			}

			// 2. Query followers
			cursor, err := followColl.Find(ctx, bson.M{"target_id": authorObjID})
			if err != nil {
				log.Printf("Error finding followers: %v", err)
				continue // retry
			}

			var follows []UserFollow
			if err := cursor.All(ctx, &follows); err != nil {
				log.Printf("Error decoding followers: %v", err)
				cursor.Close(ctx)
				continue
			}
			cursor.Close(ctx)

			if len(follows) == 0 {
				log.Printf("Author %s has 0 followers. Skipping fan-out.", event.AuthorID)
				_ = w.consumer.CommitMessages(ctx, msg)
				continue
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
				continue // retry
			}

			log.Printf("Successfully fanned out post %s to %d followers", event.PostID, len(follows))

			// 4. Commit message offset
			if err := w.consumer.CommitMessages(ctx, msg); err != nil {
				log.Printf("Error committing offset: %v", err)
			}
		}
	}
}
