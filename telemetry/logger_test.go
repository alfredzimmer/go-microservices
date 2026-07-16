package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestTraceHandlerAddsTraceIds(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(traceHandler{slog.NewJSONHandler(&buf, nil)}).With("service", "test")

	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		SpanID:  trace.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

	logger.ErrorContext(ctx, "something failed", "error", "boom")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("log output is not JSON: %v", err)
	}
	if record["service"] != "test" {
		t.Errorf("expected service=test, got %v", record["service"])
	}
	if record["trace_id"] != spanCtx.TraceID().String() {
		t.Errorf("expected trace_id=%s, got %v", spanCtx.TraceID(), record["trace_id"])
	}
	if record["span_id"] != spanCtx.SpanID().String() {
		t.Errorf("expected span_id=%s, got %v", spanCtx.SpanID(), record["span_id"])
	}
}

func TestTraceHandlerWithoutSpan(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(traceHandler{slog.NewJSONHandler(&buf, nil)})

	logger.InfoContext(context.Background(), "no trace")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("log output is not JSON: %v", err)
	}
	if _, ok := record["trace_id"]; ok {
		t.Error("expected no trace_id when there is no active span")
	}
}
