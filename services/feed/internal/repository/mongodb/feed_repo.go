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

// followConnBSON represents follow relationship aggregation schema in BSON.
type followConnBSON struct {
	TargetID      primitive.ObjectID `bson:"target_id"`
	FollowerCount int                `bson:"follower_count"`
}

type feedRepo struct {
	db               *mongo.Database
	postsCollection  *mongo.Collection
	followCollection *mongo.Collection
}

// NewFeedRepository creates a new FeedRepository instance.
func NewFeedRepository(db *mongo.Database) domain.FeedRepository {
	return &feedRepo{
		db:               db,
		postsCollection:  db.Collection("posts"),
		followCollection: db.Collection("user_follows"),
	}
}

func (r *feedRepo) GetFollowingConnections(ctx context.Context, userID string) ([]domain.FollowConnection, error) {
	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	// Match followed users of the current user, lookup to aggregate their follower counts
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "follower_id", Value: objID}}}},
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "user_follows"},
			{Key: "localField", Value: "target_id"},
			{Key: "foreignField", Value: "target_id"},
			{Key: "as", Value: "followers"},
		}}},
		{{Key: "$project", Value: bson.D{
			{Key: "target_id", Value: 1},
			{Key: "follower_count", Value: bson.D{{Key: "$size", Value: "$followers"}}},
		}}},
	}

	cur, err := r.followCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var conns []followConnBSON
	if err := cur.All(ctx, &conns); err != nil {
		return nil, err
	}

	result := make([]domain.FollowConnection, len(conns))
	for i, c := range conns {
		result[i] = domain.FollowConnection{
			TargetID:      c.TargetID,
			FollowerCount: c.FollowerCount,
		}
	}
	return result, nil
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

func (r *feedRepo) GetPostsByIDs(ctx context.Context, ids []string) ([]*domain.Post, error) {
	if len(ids) == 0 {
		return []*domain.Post{}, nil
	}

	objIDs := make([]primitive.ObjectID, len(ids))
	for i, id := range ids {
		objID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return nil, err
		}
		objIDs[i] = objID
	}

	filter := bson.M{
		"_id": bson.M{"$in": objIDs},
	}

	cur, err := r.postsCollection.Find(ctx, filter)
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

