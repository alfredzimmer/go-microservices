package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Bootstrap sets up the default structured logger and tracing for a service
// in one call. The returned shutdown function flushes pending spans and must
// run before the process exits — call it from a function that returns
// normally, not one terminated by os.Exit (which skips defers).
func Bootstrap(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	InitLogger(serviceName)
	return Init(ctx, serviceName)
}

// Init configures the global OpenTelemetry tracer provider for this process.
// Spans are exported over OTLP/gRPC to the endpoint in the
// OTEL_EXPORTER_OTLP_ENDPOINT environment variable (an http:// scheme selects
// an insecure connection; defaults to localhost:4317). The returned function
// flushes pending spans and must be called on shutdown.
func Init(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	res, err := sdkresource.Merge(
		sdkresource.Default(),
		sdkresource.NewSchemaless(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
