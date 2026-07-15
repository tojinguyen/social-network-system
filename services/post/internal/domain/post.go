package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Post represents a user's post/feed item entity.
type Post struct {
	ID          primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	AuthorID    primitive.ObjectID   `bson:"author_id" json:"author_id"`
	Content     string               `bson:"content" json:"content"`
	MediaURLs   []string             `bson:"media_urls" json:"media_urls"`
	LikeUserIDs []primitive.ObjectID `bson:"like_user_ids" json:"-"`
	LikeCount   int                  `bson:"like_count" json:"like_count"`
	CreatedAt   time.Time            `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time            `bson:"updated_at" json:"updated_at"`
}

// PostRepository defines the database contract for Post entity operations.
type PostRepository interface {
	Create(ctx context.Context, post *Post) error
	FindByID(ctx context.Context, id string) (*Post, error)
	FindByAuthorIDs(ctx context.Context, authorIDs []primitive.ObjectID, cursor time.Time, limit int) ([]*Post, error)
	Delete(ctx context.Context, id string, authorID string) error
}
