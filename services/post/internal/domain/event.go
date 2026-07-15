package domain

import "time"

// PostCreatedEvent represents the message payload published when a post is created.
type PostCreatedEvent struct {
	PostID    string    `json:"post_id"`
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
