package usecase

import (
	"context"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"social-network-system/services/feed/config"
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
	cfg       *config.Config
	feedRepo  domain.FeedRepository
	cacheRepo domain.FeedCacheRepository
}

// NewFeedUseCase creates a new FeedUseCase instance.
func NewFeedUseCase(cfg *config.Config, feedRepo domain.FeedRepository, cacheRepo domain.FeedCacheRepository) FeedUseCase {
	return &feedUseCase{
		cfg:       cfg,
		feedRepo:  feedRepo,
		cacheRepo: cacheRepo,
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

	// Step 1: Get list of users followed by current user along with their follower count
	connections, err := u.feedRepo.GetFollowingConnections(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(connections) == 0 {
		return &domain.FeedResponse{
			Items: []*domain.FeedItem{},
		}, nil
	}

	// Separate normal users (push) from celebrities (pull)
	var celebrityIDs []primitive.ObjectID
	for _, conn := range connections {
		if conn.FollowerCount >= u.cfg.CelebrityThreshold {
			celebrityIDs = append(celebrityIDs, conn.TargetID)
		}
	}

	// Step 2: Read cached post IDs from Redis Feed Cache (ZSET)
	cachedPostIDs, err := u.cacheRepo.GetFeedCache(ctx, userID, cursorTime, size)
	if err != nil {
		return nil, err
	}

	var normalPosts []*domain.Post
	if len(cachedPostIDs) > 0 {
		normalPosts, err = u.feedRepo.GetPostsByIDs(ctx, cachedPostIDs)
		if err != nil {
			return nil, err
		}
	}

	// Step 3: Pull celebrity posts from MongoDB
	var celebrityPosts []*domain.Post
	if len(celebrityIDs) > 0 {
		celebrityPosts, err = u.feedRepo.GetPostsByAuthorIDs(ctx, celebrityIDs, cursorTime, size)
		if err != nil {
			return nil, err
		}
	}

	// Step 4: Merge both list of posts and sort descending by creation time
	var mergedPosts []*domain.Post
	mergedPosts = append(mergedPosts, normalPosts...)
	mergedPosts = append(mergedPosts, celebrityPosts...)

	sort.Slice(mergedPosts, func(i, j int) bool {
		return mergedPosts[i].CreatedAt.After(mergedPosts[j].CreatedAt)
	})

	// Truncate to request size
	if len(mergedPosts) > size {
		mergedPosts = mergedPosts[:size]
	}

	// Step 5: Build feed response
	items := make([]*domain.FeedItem, len(mergedPosts))
	for i, post := range mergedPosts {
		items[i] = &domain.FeedItem{
			ID:        post.ID.Hex(),
			AuthorID:  post.AuthorID.Hex(),
			Content:   post.Content,
			MediaURLs: post.MediaURLs,
			LikeCount: post.LikeCount,
			CreatedAt: post.CreatedAt.Format(time.RFC3339Nano),
		}
	}

	resp := &domain.FeedResponse{
		Items: items,
	}

	// Determine next cursor from the oldest post in the batch
	if len(mergedPosts) == size {
		lastPost := mergedPosts[len(mergedPosts)-1]
		resp.NextCursor = lastPost.CreatedAt.Format(time.RFC3339Nano)
	}

	return resp, nil
}
