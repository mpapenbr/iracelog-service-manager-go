package config

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/version"
)

func SetupStdOutMetrics() (metric.Exporter, error) {
	return stdoutmetric.New()
}

func SetupStdOutTracing() (trace.SpanExporter, error) {
	return stdouttrace.New()
}

type Telemetry struct {
	ctx     context.Context
	metrics *metric.MeterProvider
	traces  *trace.TracerProvider
}

func (t Telemetry) Shutdown() {
	log.Info("Shutdown telemetry")
	if err := t.metrics.Shutdown(context.Background()); err != nil {
		fmt.Printf("shutdown metrics error:%+v\n", err)
	}
	if err := t.traces.Shutdown(context.Background()); err != nil {
		fmt.Printf("shutdown traces error:%+v\n", err)
	}
}

func SetupTelemetry(ctx context.Context) (*Telemetry, error) {
	res, err := resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("iracelog-service-manager"),
			semconv.ServiceVersionKey.String(version.Version),
		),
		resource.WithFromEnv())
	if err != nil {
		log.Error("Failed to create resource", log.ErrorField(err))
		return nil, err
	}

	ret := Telemetry{ctx: ctx}

	if m, err := setupMetrics(res); err != nil {
		return nil, err
	} else {
		ret.metrics = m
	}
	if t, err := setupTraces(res); err != nil {
		return nil, err
	} else {
		ret.traces = t
	}
	return &ret, nil
}

func setupMetrics(r *resource.Resource) (*metric.MeterProvider, error) {
	exporter, err := otlpmetricgrpc.New(
		context.Background(),
		otlpmetricgrpc.WithEndpoint(TelemetryEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	provider := metric.NewMeterProvider(
		metric.WithResource(r),
		metric.WithReader(metric.NewPeriodicReader(exporter,
			metric.WithInterval(15*time.Second))), // TODO: configure?
	)

	otel.SetMeterProvider(provider)
	return provider, nil
}

func setupTraces(r *resource.Resource) (*trace.TracerProvider, error) {
	exporter, err := otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithEndpoint(TelemetryEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	provider := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(r),
		// set the sampling rate based on the parent span to 60%
		// trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(0.6))),
		trace.WithSampler(trace.AlwaysSample()), // TODO: confiure?
	)

	otel.SetTracerProvider(provider)

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			// W3C Trace Context format; https://www.w3.org/TR/trace-context/
			propagation.TraceContext{},
		),
	)
	return provider, nil
}
