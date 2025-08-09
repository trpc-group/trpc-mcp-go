// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package metric

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	. "trpc.group/trpc-go/trpc-mcp-go"
)

// OtelExporterType declares supported exporter types for metrics output.
//
// ExporterStdout is suitable for local development/testing.
// ExporterOTLP exports metrics via OTLP over gRPC to a collector/observability backend.
//
// Note: In production, prefer OTLP with TLS and proper authN/Z.
type OtelExporterType string

const (
	// ExporterStdout writes metrics to stdout in a human-readable form.
	ExporterStdout OtelExporterType = "stdout"
	// ExporterOTLP exports metrics to an OTLP-compatible collector (default endpoint: localhost:4317).
	ExporterOTLP OtelExporterType = "otlp"
)

// MetricsRecorder is an abstraction over metric reporting used by the middleware.
// Implementations MUST be safe for concurrent use by multiple goroutines.
//
// The reference implementation in this package uses OpenTelemetry, but users can
// provide a custom recorder (e.g., in-memory, Prometheus, vendor SDK) by
// implementing this interface and passing it via WithRecorder.
type MetricsRecorder interface {
	// RecordRequest increments the total request counter for a given method.
	RecordRequest(ctx context.Context, method string)
	// RecordError increments the error counter for a given method and error code.
	RecordError(ctx context.Context, method string, code int)
	// RecordLatency records the observed request latency (milliseconds) for a method.
	RecordLatency(ctx context.Context, method string, latencyMs float64)
	// RecordInFlight adjusts the in-flight request gauge for a method by count (can be negative).
	RecordInFlight(ctx context.Context, method string, count int64)
}

// MetricsConfig controls which metrics are recorded and allows filtering of
// methods. The recorder is optional; if nil, the middleware becomes a no-op.
//
// Enable* flags allow fine-grained control to minimize overhead when needed.
// Filter can be used to explicitly include/exclude certain methods (low-cardinality only).
//
// This configuration is intentionally minimal. Extend as needed in your project
// (e.g., slow request thresholds, per-method overrides, environment tags, etc.).
type MetricsConfig struct {
	recorder MetricsRecorder

	// EnableRequests toggles recording of total request counts.
	EnableRequests bool
	// EnableErrors toggles recording of error counts (by method and code).
	EnableErrors bool
	// EnableLatency toggles recording of request latency histogram.
	EnableLatency bool
	// EnableInFlight toggles recording of in-flight (active) requests.
	EnableInFlight bool

	// Filter determines whether a given method should be recorded.
	// Return true to record, false to skip. If Filter is nil, all methods are recorded.
	Filter func(method string) bool
}

// DefaultMetricsConfig returns a minimally useful configuration suitable for
// most example scenarios: all metric families enabled and no filter.
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		EnableRequests: true,
		EnableErrors:   true,
		EnableLatency:  true,
		EnableInFlight: true,
		Filter:         nil,
	}
}

// MetricsOption applies functional options to MetricsConfig.
// This pattern makes it easy to evolve configuration without breaking callers.
type MetricsOption func(*MetricsConfig)

// WithRecorder sets a custom MetricsRecorder. If not provided, the middleware is a no-op.
func WithRecorder(recorder MetricsRecorder) MetricsOption {
	return func(o *MetricsConfig) {
		o.recorder = recorder
	}
}

// WithEnableRequests toggles request count recording.
func WithEnableRequests(enableRequests bool) MetricsOption {
	return func(o *MetricsConfig) {
		o.EnableRequests = enableRequests
	}
}

// WithEnableErrors toggles error count recording.
func WithEnableErrors(enableErrors bool) MetricsOption {
	return func(o *MetricsConfig) {
		o.EnableErrors = enableErrors
	}
}

// WithEnableLatency toggles latency histogram recording.
func WithEnableLatency(enableLatency bool) MetricsOption {
	return func(o *MetricsConfig) {
		o.EnableLatency = enableLatency
	}
}

// WithEnableInFlight toggles in-flight gauge recording.
func WithEnableInFlight(enableInFlight bool) MetricsOption {
	return func(o *MetricsConfig) {
		o.EnableInFlight = enableInFlight
	}
}

// WithFilter sets a method filter. Return true to record metrics for the method.
// Keep the cardinality of methods low to avoid explosive time series growth.
func WithFilter(filter func(method string) bool) MetricsOption {
	return func(o *MetricsConfig) {
		o.Filter = filter
	}
}

// initConn initializes a gRPC connection used by the OTLP metrics exporter.
// This example uses insecure credentials and a fixed endpoint, which is suitable
// for local development only. For production, use TLS and configure retries/auth.
func initConn(endpoint string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	return conn, err
}

// initMeterProvider configures the OpenTelemetry MeterProvider and exporter.
//
// The example uses a PeriodicReader with a short 5s interval to make local
// testing responsive. In production, consider 30–60s to reduce overhead.
func initMeterProvider(ctx context.Context, res *resource.Resource, exporterType OtelExporterType, endpoint string) (func(context.Context) error, error) {
	var metricExporter sdkmetric.Exporter
	var err error
	switch exporterType {
	case ExporterStdout:
		metricExporter, err = stdoutmetric.New(stdoutmetric.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout metric exporter: %w", err)
		}
	case ExporterOTLP:
		conn, err := initConn(endpoint)
		if err != nil {
			return nil, err
		}

		metricExporter, err = otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
		if err != nil {
			return nil, fmt.Errorf("failed to create metric exporter: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", exporterType)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	return meterProvider.Shutdown, nil
}

// RecorderConfig controls how the OpenTelemetry-based MetricsRecorder is built.
// This keeps recorder setup separate from the middleware wiring.
//
// serviceName should remain low-cardinality and stable across the service's
// lifetime in a given deployment environment.
// exporterType selects stdout vs OTLP exporter.
// endpoint is used when exporterType == ExporterOTLP.
type RecorderConfig struct {
	serviceName  string
	exporterType OtelExporterType
	endpoint     string
}

// OtelMetricsRecorderConfig returns a default RecorderConfig for examples.
// Adjust serviceName, exporterType, and endpoint as appropriate for your setup.
func OtelMetricsRecorderConfig() *RecorderConfig {
	return &RecorderConfig{
		serviceName:  "trpc-mcp-go/examples/middlewares/metric",
		exporterType: ExporterStdout,
		endpoint:     "localhost:4317",
	}
}

// RecorderOption applies functional options to RecorderConfig.
// This mirrors the pattern used by MetricsOption for consistency.
type RecorderOption func(*RecorderConfig)

// WithRecorderServiceName overrides the OTel resource service.name.
func WithRecorderServiceName(serviceName string) RecorderOption {
	return func(o *RecorderConfig) {
		o.serviceName = serviceName
	}
}

// WithRecorderExporterType selects the exporter implementation (stdout or OTLP).
func WithRecorderExporterType(exporterType OtelExporterType) RecorderOption {
	return func(o *RecorderConfig) {
		o.exporterType = exporterType
	}
}

// WithRecorderEndpoint sets the OTLP endpoint; ignored for stdout exporter.
func WithRecorderEndpoint(endpoint string) RecorderOption {
	return func(o *RecorderConfig) {
		o.endpoint = endpoint
	}
}

// OtelMetricsRecorder is a MetricsRecorder implemented using OpenTelemetry.
// It reports the following metric instruments with low-cardinality attributes:
//   - mcp_requests_total (counter): total requests by method
//   - mcp_errors_total (counter): total errors by method and code
//   - mcp_request_duration_ms (histogram): request latency in milliseconds by method
//   - mcp_requests_in_flight (updowncounter): active requests in flight by method
//
// Attribute guidance: keep method names normalized and low-cardinality. Avoid user IDs
// or request IDs as attributes. Prefer environment and version on the Resource.
type OtelMetricsRecorder struct {
	requestCounter metric.Int64Counter
	errorCounter   metric.Int64Counter
	latencyHist    metric.Float64Histogram
	inFlightGauge  metric.Int64UpDownCounter
}

// NewOtelMetricsRecorder constructs an OpenTelemetry-backed MetricsRecorder and
// returns a shutdown function. Call the returned shutdown function during
// graceful shutdown to flush metrics.
func NewOtelMetricsRecorder(option ...RecorderOption) (MetricsRecorder, func(ctx context.Context) error, error) {
	cfg := OtelMetricsRecorderConfig()
	for _, opt := range option {
		if opt != nil {
			opt(cfg)
		}
	}

	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.serviceName),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("resource.New: %w", err)
	}

	shutdownMeterProvider, err := initMeterProvider(ctx, res, cfg.exporterType, cfg.endpoint)
	if err != nil {
		return nil, nil, err
	}

	name := "github.com/trpc-group/trpc-mcp-go/examples/middlewares/metric"
	meter := otel.Meter(name)

	requestCounter, _ := meter.Int64Counter("mcp_requests_total", metric.WithDescription("Total number of MCP requests"))
	errorCounter, _ := meter.Int64Counter("mcp_errors_total", metric.WithDescription("Total number of MCP errors"))
	latencyHist, _ := meter.Float64Histogram("mcp_request_duration_ms", metric.WithDescription("MCP request latency in ms"), metric.WithUnit("ms"))
	inflightGauge, _ := meter.Int64UpDownCounter("mcp_requests_in_flight", metric.WithDescription("Number of MCP requests in flight"))

	recorder := &OtelMetricsRecorder{
		requestCounter: requestCounter,
		errorCounter:   errorCounter,
		latencyHist:    latencyHist,
		inFlightGauge:  inflightGauge,
	}

	return recorder, shutdownMeterProvider, nil
}

// RecordRequest increments the request counter for the given method.
func (r *OtelMetricsRecorder) RecordRequest(ctx context.Context, method string) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
	}
	r.requestCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordError increments the error counter for the given method and error code.
func (r *OtelMetricsRecorder) RecordError(ctx context.Context, method string, code int) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.Int("code", code),
	}
	r.errorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordLatency records the latency (in milliseconds) for the given method.
// The metric name encodes the unit (ms). Keep histogram bucket policy consistent
// across services to simplify fleet-wide dashboards.
func (r *OtelMetricsRecorder) RecordLatency(ctx context.Context, method string, latencyMs float64) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
	}
	r.latencyHist.Record(ctx, latencyMs, metric.WithAttributes(attrs...))
}

// RecordInFlight adjusts the in-flight gauge by count (use +1 on start, -1 on finish).
func (r *OtelMetricsRecorder) RecordInFlight(ctx context.Context, method string, count int64) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
	}
	r.inFlightGauge.Add(ctx, count, metric.WithAttributes(attrs...))
}

// NewMetricsMiddleware wraps a JSON-RPC handler with metrics instrumentation.
//
// Behavior:
//   - Optionally increments in-flight gauge on entry and decrements on exit
//   - Optionally counts requests
//   - Optionally records latency around the downstream handler call
//   - Optionally counts errors, attributing internal errors vs JSON-RPC errors by code
//
// Thread-safety: The middleware is stateless and safe for concurrent use. The
// provided MetricsRecorder implementation must be concurrency-safe.
func NewMetricsMiddleware(opts ...MetricsOption) MiddlewareFunc {
	cfg := DefaultMetricsConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	return func(ctx context.Context, req *JSONRPCRequest, session Session, next HandleFunc) (JSONRPCMessage, error) {
		method := req.Method

		// No-op if no recorder is provided.
		if cfg.recorder == nil {
			return next(ctx, req, session)
		}

		// Optional method-level filter.
		if cfg.Filter != nil && !cfg.Filter(method) {
			return next(ctx, req, session)
		}

		if cfg.EnableInFlight {
			cfg.recorder.RecordInFlight(ctx, method, 1)
			defer cfg.recorder.RecordInFlight(ctx, method, -1)
		}

		var startTime time.Time
		if cfg.EnableLatency {
			startTime = time.Now()
		}

		if cfg.EnableRequests {
			cfg.recorder.RecordRequest(ctx, method)
		}

		resp, err := next(ctx, req, session)

		success := err == nil
		errorCode := 0
		if err != nil {
			errorCode = ErrCodeInternal
		} else {
			if rpcError, ok := resp.(*JSONRPCError); ok {
				success = false
				errorCode = rpcError.Error.Code
			}
		}

		if !success && cfg.EnableErrors {
			cfg.recorder.RecordError(ctx, method, errorCode)
		}
		if cfg.EnableLatency {
			latencyMs := float64(time.Since(startTime).Milliseconds())
			cfg.recorder.RecordLatency(ctx, method, latencyMs)
		}

		return resp, err
	}
}
