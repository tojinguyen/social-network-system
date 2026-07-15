package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"social-network-system/pkg/response"
	"social-network-system/services/post/internal/domain"
	"social-network-system/services/post/internal/usecase"
)

// FollowHandler handles HTTP requests for follow operations.
type FollowHandler struct {
	useCase usecase.FollowUseCase
}

// NewFollowHandler creates a new FollowHandler.
func NewFollowHandler(useCase usecase.FollowUseCase) *FollowHandler {
	return &FollowHandler{useCase: useCase}
}

func (h *FollowHandler) Follow(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "User context not found")
		return
	}

	var req domain.FollowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	err := h.useCase.Follow(c.Request.Context(), userID.(string), req.TargetID)
	if err != nil {
		if err == usecase.ErrSelfFollow {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Followed successfully", nil)
}

func (h *FollowHandler) Unfollow(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "User context not found")
		return
	}

	targetID := c.Param("target_id")
	if targetID == "" {
		response.Error(c, http.StatusBadRequest, "Target user ID is required")
		return
	}

	err := h.useCase.Unfollow(c.Request.Context(), userID.(string), targetID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Unfollowed successfully", nil)
}

func (h *FollowHandler) GetFollowing(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "User context not found")
		return
	}

	ids, err := h.useCase.GetFollowing(c.Request.Context(), userID.(string))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert ObjectIDs to string slice for JSON response
	following := make([]string, len(ids))
	for i, id := range ids {
		following[i] = id.Hex()
	}

	response.Success(c, http.StatusOK, "Get following list successful", gin.H{
		"following": following,
		"count":     len(following),
	})
}

func (h *FollowHandler) GetFollowers(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "User context not found")
		return
	}

	ids, err := h.useCase.GetFollowers(c.Request.Context(), userID.(string))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert ObjectIDs to string slice for JSON response
	followers := make([]string, len(ids))
	for i, id := range ids {
		followers[i] = id.Hex()
	}

	response.Success(c, http.StatusOK, "Get followers list successful", gin.H{
		"followers": followers,
		"count":     len(followers),
	})
}
