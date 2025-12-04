package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const (
	defaultOTLPEndpoint = "localhost:4318"
	serviceName         = "llamastack-proxy"
	serviceVersion      = "1.0.0"
)

// initTelemetry initializes OpenTelemetry tracing
func initTelemetry() (func(context.Context) error, error) {
	ctx := context.Background()

	// Check if telemetry is disabled
	if os.Getenv("OTEL_SDK_DISABLED") == "true" {
		log.Println("OpenTelemetry is disabled via OTEL_SDK_DISABLED")
		return func(context.Context) error { return nil }, nil
	}

	// Get OTLP endpoint from environment, default to localhost
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		otlpEndpoint = defaultOTLPEndpoint
	}

	// Strip http:// or https:// prefix if present
	// The otlptracehttp.WithEndpoint expects just host:port
	otlpEndpoint = strings.TrimPrefix(otlpEndpoint, "http://")
	otlpEndpoint = strings.TrimPrefix(otlpEndpoint, "https://")

	// Remove trailing path if present (e.g., /v1/traces)
	if idx := strings.Index(otlpEndpoint, "/"); idx != -1 {
		otlpEndpoint = otlpEndpoint[:idx]
	}

	// Port 4317 is for gRPC, port 4318 is for HTTP
	// Replace 4317 with 4318 for HTTP exporter
	if strings.HasSuffix(otlpEndpoint, ":4317") {
		otlpEndpoint = strings.TrimSuffix(otlpEndpoint, ":4317") + ":4318"
		log.Printf("Changed gRPC port 4317 to HTTP port 4318: %s", otlpEndpoint)
	}

	// Configure OTLP HTTP exporter
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(otlpEndpoint),
		otlptracehttp.WithInsecure(), // Use TLS in production
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	// Set global propagator to support trace context propagation
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	log.Printf("OpenTelemetry tracing initialized (endpoint: %s)", otlpEndpoint)

	// Return shutdown function
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(ctx)
	}, nil
}
