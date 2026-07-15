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

// FeedRepository defines the database contract for feed data retrieval.
type FeedRepository interface {
	// GetFollowingIDs returns the list of user IDs that the given user is following.
	GetFollowingIDs(ctx context.Context, userID string) ([]primitive.ObjectID, error)
	// GetPostsByAuthorIDs returns posts from the given authors, sorted by created_at desc,
	// with cursor-based pagination.
	GetPostsByAuthorIDs(ctx context.Context, authorIDs []primitive.ObjectID, cursor time.Time, limit int) ([]*Post, error)
}
