package logger

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

// TraceHandler is a slog.Handler wrapper that injects trace_id and span_id from context.
type TraceHandler struct {
	slog.Handler
}

// NewTraceHandler wraps an existing slog.Handler.
func NewTraceHandler(next slog.Handler) *TraceHandler {
	return &TraceHandler{Handler: next}
}

// Handle extracts OpenTelemetry trace information and adds it to the log record attributes.
func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx == nil {
		return h.Handler.Handle(ctx, r)
	}

	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		)
	}

	return h.Handler.Handle(ctx, r)
}

// WithAttrs returns a new TraceHandler with the given attributes.
func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup returns a new TraceHandler with the given group name.
func (h *TraceHandler) WithGroup(name string) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithGroup(name)}
}

// Init initializes the default slog logger with JSON formatting,
// automatic trace context propagation, and a service name tag.
func Init(serviceName string) {
	// JSON handler for easy parsing by Promtail/Loki
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	traceHandler := NewTraceHandler(jsonHandler)

	// Set default logger with default attributes
	logger := slog.New(traceHandler).With(slog.String("service", serviceName))
	slog.SetDefault(logger)
}
