package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"social-network-system/services/chat/internal/domain"
)

type chatRepo struct {
	db         *mongo.Database
	collection *mongo.Collection
}

// NewChatRepository creates a new ChatRepository instance and configures indexes.
func NewChatRepository(db *mongo.Database) domain.ChatRepository {
	collection := db.Collection("messages")

	// Create index asynchronously for performance
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, _ = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{
				{Key: "conversation_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
		})
	}()

	return &chatRepo{
		db:         db,
		collection: collection,
	}
}

func (r *chatRepo) Create(ctx context.Context, msg *domain.Message) error {
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	_, err := r.collection.InsertOne(ctx, msg)
	return err
}

func (r *chatRepo) GetHistory(ctx context.Context, conversationID string, cursor time.Time, limit int) ([]*domain.Message, error) {
	filter := bson.M{
		"conversation_id": conversationID,
	}

	// Pagination using cursor
	if !cursor.IsZero() {
		filter["created_at"] = bson.M{"$lt": cursor}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cur, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cur.Close(ctx) }()

	var messages []*domain.Message
	if err := cur.All(ctx, &messages); err != nil {
		return nil, err
	}

	// Reverse slice to return chronological order (oldest to newest for user experience)
	// MongoDB returns them newest to oldest because of Sort: created_at -1.
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}
