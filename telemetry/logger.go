package telemetry

import (
	"context"
	"io"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

// traceHandler adds the active trace and span IDs to every record logged
// with a context, so log lines can be matched to traces in Jaeger.
type traceHandler struct {
	slog.Handler
}

func (h traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", span.TraceID().String()),
			slog.String("span_id", span.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{h.Handler.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{h.Handler.WithGroup(name)}
}

// NewLogger returns a JSON logger writing to w, tagged with the service
// name and trace-aware (see traceHandler).
func NewLogger(w io.Writer, serviceName string) *slog.Logger {
	return slog.New(traceHandler{slog.NewJSONHandler(w, nil)}).
		With("service", serviceName)
}

// InitLogger makes the process-wide default slog logger emit JSON tagged
// with the service name. Use slog.ErrorContext / slog.InfoContext inside
// request handlers so the trace IDs are included.
func InitLogger(serviceName string) *slog.Logger {
	logger := NewLogger(os.Stdout, serviceName)
	slog.SetDefault(logger)
	return logger
}
