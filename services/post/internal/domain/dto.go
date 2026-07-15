package domain

// CreatePostRequest defines the input fields required to create a new post.
type CreatePostRequest struct {
	Content   string   `json:"content" binding:"required,max=5000"`
	MediaURLs []string `json:"media_urls" binding:"omitempty,max=10"`
}

// FollowRequest defines the input fields required to follow a user.
type FollowRequest struct {
	TargetID string `json:"target_id" binding:"required"`
}

// PostResponse represents a single post in API responses.
type PostResponse struct {
	ID        string   `json:"id"`
	AuthorID  string   `json:"author_id"`
	Content   string   `json:"content"`
	MediaURLs []string `json:"media_urls"`
	LikeCount int      `json:"like_count"`
	CreatedAt string   `json:"created_at"`
}
