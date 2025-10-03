package api

import (
	"context"
	"fmt"
	"log"
	"os"

	promClient "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

func newResource() (*resource.Resource, error) {
	return resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName("api"),
			semconv.ServiceVersion("0.1.0"),
		))
}

func newMeterProvider(ctx context.Context) (*metric.MeterProvider, promClient.Gatherer, error) {
	res, err := newResource()
	if err != nil {
		return nil, nil, fmt.Errorf("creating resource: %w", err)
	}

	reg := promClient.NewRegistry()
	metricExporter, err := prometheus.New(
		prometheus.WithRegisterer(reg),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating prometheus exporter: %w", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metricExporter),
	)
	otel.SetMeterProvider(meterProvider)

	// Only create a trace provider if the environment variable is set
	if _, ok := os.LookupEnv("OTEL_EXPORTER_OTLP_ENDPOINT"); ok {
		client := otlptracegrpc.NewClient()
		exp, err := otlptrace.New(ctx, client)
		if err != nil {
			log.Fatalf("failed to initialize exporter: %e", err)
		}

		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(
				resource.NewWithAttributes(
					semconv.SchemaURL,
					semconv.ServiceNameKey.String("api"),
					semconv.ServiceNamespace("nais.io"),
				)),
		)
		otel.SetTracerProvider(tp)
	}

	tc := propagation.TraceContext{}
	// Register the TraceContext propagator globally.
	otel.SetTextMapPropagator(tc)

	return meterProvider, reg, nil
}
