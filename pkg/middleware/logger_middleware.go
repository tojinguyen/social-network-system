package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// LoggerMiddleware returns a Gin middleware that logs HTTP requests using slog.
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()

		// Log context will include trace_id and span_id automatically if trace context is set in Request.Context
		ctx := c.Request.Context()

		attributes := []any{
			slog.Int("status", status),
			slog.String("method", method),
			slog.String("path", path),
			slog.String("query", query),
			slog.String("ip", clientIP),
			slog.Duration("latency", latency),
		}

		// Log error if status is 5xx
		if status >= 500 {
			slog.ErrorContext(ctx, "HTTP Request Failed", attributes...)
		} else {
			slog.InfoContext(ctx, "HTTP Request Completed", attributes...)
		}
	}
}
