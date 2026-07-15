package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"social-network-system/pkg/response"
	"social-network-system/services/post/internal/domain"
	"social-network-system/services/post/internal/usecase"
)

// PostHandler handles HTTP requests for post operations.
type PostHandler struct {
	useCase usecase.PostUseCase
}

// NewPostHandler creates a new PostHandler.
func NewPostHandler(useCase usecase.PostUseCase) *PostHandler {
	return &PostHandler{useCase: useCase}
}

func (h *PostHandler) CreatePost(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "User context not found")
		return
	}

	var req domain.CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	post, err := h.useCase.CreatePost(c.Request.Context(), userID.(string), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Post created successfully", gin.H{
		"id":         post.ID.Hex(),
		"author_id":  post.AuthorID.Hex(),
		"content":    post.Content,
		"media_urls": post.MediaURLs,
		"like_count": post.LikeCount,
		"created_at": post.CreatedAt,
	})
}

func (h *PostHandler) GetPost(c *gin.Context) {
	postID := c.Param("id")
	if postID == "" {
		response.Error(c, http.StatusBadRequest, "Post ID is required")
		return
	}

	post, err := h.useCase.GetPost(c.Request.Context(), postID)
	if err != nil {
		if err == usecase.ErrPostNotFound {
			response.Error(c, http.StatusNotFound, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Get post successful", gin.H{
		"id":         post.ID.Hex(),
		"author_id":  post.AuthorID.Hex(),
		"content":    post.Content,
		"media_urls": post.MediaURLs,
		"like_count": post.LikeCount,
		"created_at": post.CreatedAt,
	})
}

func (h *PostHandler) DeletePost(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "User context not found")
		return
	}

	postID := c.Param("id")
	if postID == "" {
		response.Error(c, http.StatusBadRequest, "Post ID is required")
		return
	}

	err := h.useCase.DeletePost(c.Request.Context(), postID, userID.(string))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Post deleted successfully", nil)
}
