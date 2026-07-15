package usecase

import (
	"context"
	"time"

	"social-network-system/services/feed/internal/domain"
)

const (
	// DefaultFeedSize is the default number of posts to return per feed request.
	DefaultFeedSize = 20
	// MaxFeedSize is the maximum number of posts allowed per request.
	MaxFeedSize = 50
)

// FeedUseCase defines the business logic contract for feed operations.
type FeedUseCase interface {
	GetFeed(ctx context.Context, userID string, cursor string, size int) (*domain.FeedResponse, error)
}

type feedUseCase struct {
	feedRepo domain.FeedRepository
}

// NewFeedUseCase creates a new FeedUseCase instance.
func NewFeedUseCase(feedRepo domain.FeedRepository) FeedUseCase {
	return &feedUseCase{
		feedRepo: feedRepo,
	}
}

func (u *feedUseCase) GetFeed(ctx context.Context, userID string, cursor string, size int) (*domain.FeedResponse, error) {
	// Validate and normalize size
	if size <= 0 {
		size = DefaultFeedSize
	}
	if size > MaxFeedSize {
		size = MaxFeedSize
	}

	// Parse cursor timestamp
	var cursorTime time.Time
	if cursor != "" {
		parsed, err := time.Parse(time.RFC3339Nano, cursor)
		if err != nil {
			return nil, err
		}
		cursorTime = parsed
	}

	// Step 1: Get list of users that the current user is following
	followingIDs, err := u.feedRepo.GetFollowingIDs(ctx, userID)
	if err != nil {
		return nil, err
	}

	// If user follows nobody, return empty feed
	if len(followingIDs) == 0 {
		return &domain.FeedResponse{
			Items: []*domain.FeedItem{},
		}, nil
	}

	// Step 2: Query posts from followed users with cursor pagination
	posts, err := u.feedRepo.GetPostsByAuthorIDs(ctx, followingIDs, cursorTime, size)
	if err != nil {
		return nil, err
	}

	// Step 3: Build feed response
	items := make([]*domain.FeedItem, len(posts))
	for i, post := range posts {
		items[i] = &domain.FeedItem{
			ID:        post.ID.Hex(),
			AuthorID:  post.AuthorID.Hex(),
			Content:   post.Content,
			MediaURLs: post.MediaURLs,
			LikeCount: post.LikeCount,
			CreatedAt: post.CreatedAt.Format(time.RFC3339Nano),
		}
	}

	// Step 4: Determine next cursor from the last post's created_at
	resp := &domain.FeedResponse{
		Items: items,
	}

	if len(posts) == size {
		lastPost := posts[len(posts)-1]
		resp.NextCursor = lastPost.CreatedAt.Format(time.RFC3339Nano)
	}

	return resp, nil
}
