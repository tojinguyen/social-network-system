package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"social-network-system/services/feed/internal/domain"
)

// userFollow is an internal struct to deserialize follow documents.
type userFollow struct {
	FollowerID primitive.ObjectID `bson:"follower_id"`
	TargetID   primitive.ObjectID `bson:"target_id"`
}

type feedRepo struct {
	db              *mongo.Database
	postsCollection *mongo.Collection
	followCollection *mongo.Collection
}

// NewFeedRepository creates a new FeedRepository instance.
func NewFeedRepository(db *mongo.Database) domain.FeedRepository {
	return &feedRepo{
		db:              db,
		postsCollection: db.Collection("posts"),
		followCollection: db.Collection("user_follows"),
	}
}

func (r *feedRepo) GetFollowingIDs(ctx context.Context, userID string) ([]primitive.ObjectID, error) {
	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	cur, err := r.followCollection.Find(ctx, bson.M{"follower_id": objID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var follows []userFollow
	if err := cur.All(ctx, &follows); err != nil {
		return nil, err
	}

	ids := make([]primitive.ObjectID, len(follows))
	for i, f := range follows {
		ids[i] = f.TargetID
	}
	return ids, nil
}

func (r *feedRepo) GetPostsByAuthorIDs(ctx context.Context, authorIDs []primitive.ObjectID, cursor time.Time, limit int) ([]*domain.Post, error) {
	if len(authorIDs) == 0 {
		return []*domain.Post{}, nil
	}

	filter := bson.M{
		"author_id": bson.M{"$in": authorIDs},
	}

	// Apply cursor-based pagination: only fetch posts older than cursor
	if !cursor.IsZero() {
		filter["created_at"] = bson.M{"$lt": cursor}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cur, err := r.postsCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var posts []*domain.Post
	if err := cur.All(ctx, &posts); err != nil {
		return nil, err
	}

	return posts, nil
}
