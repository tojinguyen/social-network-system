package usecase

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"social-network-system/services/post/internal/domain"
)

var (
	// ErrPostNotFound is returned when a post cannot be found.
	ErrPostNotFound = errors.New("post not found")
	// ErrUnauthorized is returned when a user tries to modify a post they don't own.
	ErrUnauthorized = errors.New("unauthorized to perform this action")
)

// PostUseCase defines the business logic contract for post operations.
type PostUseCase interface {
	CreatePost(ctx context.Context, userID string, req *domain.CreatePostRequest) (*domain.Post, error)
	GetPost(ctx context.Context, postID string) (*domain.Post, error)
	DeletePost(ctx context.Context, postID string, userID string) error
}

type postUseCase struct {
	postRepo domain.PostRepository
}

// NewPostUseCase creates a new PostUseCase instance.
func NewPostUseCase(postRepo domain.PostRepository) PostUseCase {
	return &postUseCase{
		postRepo: postRepo,
	}
}

func (u *postUseCase) CreatePost(ctx context.Context, userID string, req *domain.CreatePostRequest) (*domain.Post, error) {
	authorID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	post := &domain.Post{
		AuthorID:  authorID,
		Content:   req.Content,
		MediaURLs: req.MediaURLs,
	}

	if err := u.postRepo.Create(ctx, post); err != nil {
		return nil, err
	}

	return post, nil
}

func (u *postUseCase) GetPost(ctx context.Context, postID string) (*domain.Post, error) {
	post, err := u.postRepo.FindByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, ErrPostNotFound
	}
	return post, nil
}

func (u *postUseCase) DeletePost(ctx context.Context, postID string, userID string) error {
	return u.postRepo.Delete(ctx, postID, userID)
}
