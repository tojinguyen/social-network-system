package response

import (
	"github.com/gin-gonic/gin"
)

// APIResponse represents the standard API response structure.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSON sends a standard JSON response.
func JSON(c *gin.Context, statusCode int, success bool, message string, data interface{}) {
	c.JSON(statusCode, APIResponse{
		Success: success,
		Message: message,
		Data:    data,
	})
}

// Error sends an error JSON response.
func Error(c *gin.Context, statusCode int, message string) {
	JSON(c, statusCode, false, message, nil)
}

// Success sends a successful JSON response.
func Success(c *gin.Context, statusCode int, message string, data interface{}) {
	JSON(c, statusCode, true, message, data)
}
