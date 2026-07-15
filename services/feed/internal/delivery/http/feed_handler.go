package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"social-network-system/pkg/response"
	"social-network-system/services/feed/internal/usecase"
)

// FeedHandler handles HTTP requests for feed operations.
type FeedHandler struct {
	useCase usecase.FeedUseCase
}

// NewFeedHandler creates a new FeedHandler.
func NewFeedHandler(useCase usecase.FeedUseCase) *FeedHandler {
	return &FeedHandler{useCase: useCase}
}

func (h *FeedHandler) GetFeed(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "User context not found")
		return
	}

	cursor := c.Query("cursor")
	sizeStr := c.DefaultQuery("size", "20")

	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid size parameter")
		return
	}

	feed, err := h.useCase.GetFeed(c.Request.Context(), userID.(string), cursor, size)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Get feed successful", feed)
}
