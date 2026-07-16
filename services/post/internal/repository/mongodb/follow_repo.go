package mongodb

import (
	"context"
	"time"

	"social-network-system/services/post/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type followRepo struct {
	db         *mongo.Database
	collection *mongo.Collection
}

// NewFollowRepository creates a new FollowRepository instance and initializes indexes.
func NewFollowRepository(db *mongo.Database) domain.FollowRepository {
	collection := db.Collection("user_follows")

	// Create unique compound index to prevent duplicate follow relationships
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, _ = collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
			{
				Keys: bson.D{
					{Key: "follower_id", Value: 1},
					{Key: "target_id", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
			{
				Keys: bson.D{{Key: "follower_id", Value: 1}},
			},
			{
				Keys: bson.D{{Key: "target_id", Value: 1}},
			},
		})
	}()

	return &followRepo{
		db:         db,
		collection: collection,
	}
}

func (r *followRepo) Follow(ctx context.Context, followerID, targetID string) error {
	followerObjID, err := primitive.ObjectIDFromHex(followerID)
	if err != nil {
		return err
	}

	targetObjID, err := primitive.ObjectIDFromHex(targetID)
	if err != nil {
		return err
	}

	follow := &domain.UserFollow{
		ID:         primitive.NewObjectID(),
		FollowerID: followerObjID,
		TargetID:   targetObjID,
		CreatedAt:  time.Now(),
	}

	_, err = r.collection.InsertOne(ctx, follow)
	if mongo.IsDuplicateKeyError(err) {
		return nil // Already following, treat as idempotent success
	}
	return err
}

func (r *followRepo) Unfollow(ctx context.Context, followerID, targetID string) error {
	followerObjID, err := primitive.ObjectIDFromHex(followerID)
	if err != nil {
		return err
	}

	targetObjID, err := primitive.ObjectIDFromHex(targetID)
	if err != nil {
		return err
	}

	_, err = r.collection.DeleteOne(ctx, bson.M{
		"follower_id": followerObjID,
		"target_id":   targetObjID,
	})
	return err
}

func (r *followRepo) GetFollowing(ctx context.Context, userID string) ([]primitive.ObjectID, error) {
	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	cur, err := r.collection.Find(ctx, bson.M{"follower_id": objID})
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = cur.Close(ctx)
	}()

	var follows []domain.UserFollow
	if err := cur.All(ctx, &follows); err != nil {
		return nil, err
	}

	ids := make([]primitive.ObjectID, len(follows))
	for i, f := range follows {
		ids[i] = f.TargetID
	}
	return ids, nil
}

func (r *followRepo) GetFollowers(ctx context.Context, userID string) ([]primitive.ObjectID, error) {
	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	cur, err := r.collection.Find(ctx, bson.M{"target_id": objID})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cur.Close(ctx) }()

	var follows []domain.UserFollow
	if err := cur.All(ctx, &follows); err != nil {
		return nil, err
	}

	ids := make([]primitive.ObjectID, len(follows))
	for i, f := range follows {
		ids[i] = f.FollowerID
	}
	return ids, nil
}

func (r *followRepo) IsFollowing(ctx context.Context, followerID, targetID string) (bool, error) {
	followerObjID, err := primitive.ObjectIDFromHex(followerID)
	if err != nil {
		return false, err
	}

	targetObjID, err := primitive.ObjectIDFromHex(targetID)
	if err != nil {
		return false, err
	}

	count, err := r.collection.CountDocuments(ctx, bson.M{
		"follower_id": followerObjID,
		"target_id":   targetObjID,
	})
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
