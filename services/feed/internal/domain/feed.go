package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FeedItem represents a single item in the user's news feed.
type FeedItem struct {
	ID        string   `json:"id"`
	AuthorID  string   `json:"author_id"`
	Content   string   `json:"content"`
	MediaURLs []string `json:"media_urls"`
	LikeCount int      `json:"like_count"`
	CreatedAt string   `json:"created_at"`
}

// FeedResponse represents the paginated feed response.
type FeedResponse struct {
	Items      []*FeedItem `json:"items"`
	NextCursor string      `json:"next_cursor,omitempty"`
}

// FollowConnection represents a user that is followed, along with their follower count.
type FollowConnection struct {
	TargetID      primitive.ObjectID
	FollowerCount int
}

// FeedCacheRepository defines the contract for reading/writing the feed cache.
type FeedCacheRepository interface {
	GetFeedCache(ctx context.Context, userID string, cursor time.Time, limit int) ([]string, error)
}

// FeedRepository defines the database contract for feed data retrieval.
type FeedRepository interface {
	// GetFollowingConnections returns the list of followed target IDs along with their follower counts.
	GetFollowingConnections(ctx context.Context, userID string) ([]FollowConnection, error)
	// GetPostsByAuthorIDs returns posts from the given authors, sorted by created_at desc,
	// with cursor-based pagination.
	GetPostsByAuthorIDs(ctx context.Context, authorIDs []primitive.ObjectID, cursor time.Time, limit int) ([]*Post, error)
	// GetPostsByIDs retrieves posts matching the given post IDs.
	GetPostsByIDs(ctx context.Context, ids []string) ([]*Post, error)
}

