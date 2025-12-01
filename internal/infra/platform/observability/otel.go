package observability

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Instruments bundles the runtime-wide observability dependencies.
type Instruments struct {
	Logger         *slog.Logger
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
}

// Init configures slog, OpenTelemetry tracing, and meters for the process.
// It returns initialized instruments plus a shutdown function that should be
// invoked on exit to flush pending spans/metrics.
func Init(ctx context.Context, serviceName string) (*Instruments, func(context.Context) error, error) {
	logger := newLogger()

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("deployment.environment", envOrDefault("ENVIRONMENT", "local")),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	spanExporter, err := newSpanExporter(ctx, logger)
	if err != nil {
		return nil, nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(spanExporter),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	meterProvider := newMeterProvider(res)
	otel.SetMeterProvider(meterProvider)

	instruments := &Instruments{
		Logger:         logger,
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
	}

	shutdown := func(ctx context.Context) error {
		var shutdownErr error
		if meterProvider != nil {
			shutdownErr = errors.Join(shutdownErr, meterProvider.Shutdown(ctx))
		}
		if tracerProvider != nil {
			shutdownErr = errors.Join(shutdownErr, tracerProvider.Shutdown(ctx))
		}
		return shutdownErr
	}

	return instruments, shutdown, nil
}

// Tracer returns a named tracer from the configured provider.
func (i *Instruments) Tracer(name string) trace.Tracer {
	if i == nil || i.TracerProvider == nil {
		return otel.Tracer(name)
	}
	return i.TracerProvider.Tracer(name)
}

// Meter returns a named meter from the configured provider.
func (i *Instruments) Meter(name string) metric.Meter {
	if i == nil || i.MeterProvider == nil {
		return metricnoop.NewMeterProvider().Meter(name)
	}
	return i.MeterProvider.Meter(name)
}

func newLogger() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo, AddSource: true})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

func newSpanExporter(ctx context.Context, logger *slog.Logger) (sdktrace.SpanExporter, error) {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	opts := []otlptracehttp.Option{}
	if endpoint != "" {
		opts = append(opts, otlptracehttp.WithEndpoint(endpoint))
	}
	if os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") != "0" {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err == nil {
		return exporter, nil
	}
	if logger != nil {
		logger.Warn("failed to initialize OTLP trace exporter, falling back to stdout", slog.String("error", err.Error()))
	}
	return stdouttrace.New(stdouttrace.WithPrettyPrint())
}

func newMeterProvider(res *resource.Resource) *sdkmetric.MeterProvider {
	reader := sdkmetric.NewManualReader()
	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
