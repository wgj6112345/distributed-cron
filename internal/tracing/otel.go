// internal/tracing/otel.go
package tracing

import (
	"context"
	"io"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// InitTracer initializes the OpenTelemetry tracer provider.
// It returns a function that should be called on application shutdown.
func InitTracer(serviceName string) (func(context.Context) error, error) {
	// For this example, we will use a simple stdout exporter.
	// In a production environment, you would use an exporter for Jaeger, Zipkin, Datadog, etc.
	exporter, err := newExporter(log.Writer()) // You can also use os.Stdout
	if err != nil {
		return nil, err
	}

	// The service.name resource attribute is highly recommended.
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create a new TracerProvider with the exporter and resource.
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)

	// Set the global TracerProvider.
	otel.SetTracerProvider(tp)

	log.Println("OpenTelemetry tracer initialized.")

	// Return the shutdown function.
	return tp.Shutdown, nil
}

// newExporter creates a new stdout trace exporter.
func newExporter(w io.Writer) (trace.SpanExporter, error) {
	return stdouttrace.New(
		stdouttrace.WithWriter(w),
		// Use human-readable output.
		stdouttrace.WithPrettyPrint(),
		// Do not print timestamps for this example.
		stdouttrace.WithoutTimestamps(),
	)
}
