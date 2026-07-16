package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"social-network-system/pkg/response"
	"social-network-system/services/chat/internal/domain"
)

// ChatHandler handles HTTP requests for Chat history.
type ChatHandler struct {
	chatUseCase domain.ChatUseCase
}

// NewChatHandler creates a new ChatHandler.
func NewChatHandler(chatUseCase domain.ChatUseCase) *ChatHandler {
	return &ChatHandler{chatUseCase: chatUseCase}
}

// GetHistory retrieves the chat history between the current user and a recipient.
func (h *ChatHandler) GetHistory(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "User unauthorized")
		return
	}

	recipientID := c.Query("recipient_id")
	if recipientID == "" {
		response.Error(c, http.StatusBadRequest, "recipient_id is required")
		return
	}

	cursor := c.Query("cursor")
	sizeStr := c.Query("size")
	size := 20
	if sizeStr != "" {
		if val, err := strconv.Atoi(sizeStr); err == nil {
			size = val
		}
	}

	messages, nextCursor, err := h.chatUseCase.GetChatHistory(c.Request.Context(), userID.(string), recipientID, cursor, size)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Chat history retrieved successfully", gin.H{
		"messages":    messages,
		"next_cursor": nextCursor,
	})
}
