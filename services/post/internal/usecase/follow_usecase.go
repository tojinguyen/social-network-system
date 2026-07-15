package usecase

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"social-network-system/services/post/internal/domain"
)

var (
	// ErrSelfFollow is returned when a user tries to follow themselves.
	ErrSelfFollow = errors.New("cannot follow yourself")
)

// FollowUseCase defines the business logic contract for follow operations.
type FollowUseCase interface {
	Follow(ctx context.Context, followerID, targetID string) error
	Unfollow(ctx context.Context, followerID, targetID string) error
	GetFollowing(ctx context.Context, userID string) ([]primitive.ObjectID, error)
	GetFollowers(ctx context.Context, userID string) ([]primitive.ObjectID, error)
}

type followUseCase struct {
	followRepo domain.FollowRepository
}

// NewFollowUseCase creates a new FollowUseCase instance.
func NewFollowUseCase(followRepo domain.FollowRepository) FollowUseCase {
	return &followUseCase{
		followRepo: followRepo,
	}
}

func (u *followUseCase) Follow(ctx context.Context, followerID, targetID string) error {
	if followerID == targetID {
		return ErrSelfFollow
	}
	return u.followRepo.Follow(ctx, followerID, targetID)
}

func (u *followUseCase) Unfollow(ctx context.Context, followerID, targetID string) error {
	return u.followRepo.Unfollow(ctx, followerID, targetID)
}

func (u *followUseCase) GetFollowing(ctx context.Context, userID string) ([]primitive.ObjectID, error) {
	return u.followRepo.GetFollowing(ctx, userID)
}

func (u *followUseCase) GetFollowers(ctx context.Context, userID string) ([]primitive.ObjectID, error) {
	return u.followRepo.GetFollowers(ctx, userID)
}
