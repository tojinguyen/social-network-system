package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Post represents a post entity read from the shared posts collection.
// This is a read-only model used by Feed Service to deserialize post data.
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
