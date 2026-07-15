package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserFollow represents the follow relationship between two users.
type UserFollow struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FollowerID primitive.ObjectID `bson:"follower_id" json:"follower_id"`
	TargetID   primitive.ObjectID `bson:"target_id" json:"target_id"`
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
}

// FollowRepository defines the database contract for follow relationship operations.
type FollowRepository interface {
	Follow(ctx context.Context, followerID, targetID string) error
	Unfollow(ctx context.Context, followerID, targetID string) error
	GetFollowing(ctx context.Context, userID string) ([]primitive.ObjectID, error)
	GetFollowers(ctx context.Context, userID string) ([]primitive.ObjectID, error)
	IsFollowing(ctx context.Context, followerID, targetID string) (bool, error)
}
