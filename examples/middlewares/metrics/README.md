# MCP Metrics Middleware

This directory contains a minimal, production-minded example of a metrics middleware for MCP (JSON-RPC) servers implemented with `trpc-mcp-go`. It demonstrates how to collect core performance signals with OpenTelemetry and attach them to the request lifecycle via a middleware.

What you get
- Request count, error count (by method and error code), in-flight gauge, and latency histogram (milliseconds)
- Pluggable recorder via a small `MetricsRecorder` interface (OpenTelemetry-based implementation provided)
- Toggleable metric families and an optional method filter to keep cardinality under control
- Exporters for local stdout (development) and OTLP/gRPC (collector/backends)

Files
- metric.go: The middleware, config options, and the OpenTelemetry-backed recorder
- docker-compose.yaml: Optional local stack to run an OTLP collector and Prometheus (for exploration)
- otel-collector.yaml: Collector configuration referenced by docker-compose
- prometheus.yaml: Prometheus configuration referenced by docker-compose

Quick start (stdout exporter)
1) Create a recorder and middleware

```go
package main

import (
    "context"

    mcp "trpc.group/trpc-go/trpc-mcp-go"
    metricmw "trpc.group/trpc-go/trpc-mcp-go/examples/middlewares/metric"
)

func main() {
    // 1) Build an OTel-backed recorder (stdout exporter by default)
    rec, shutdown, err := metricmw.NewOtelMetricsRecorder(
        metricmw.WithRecorderServiceName("my-mcp-service"),
        // metricmw.WithRecorderExporterType(metricmw.ExporterStdout), // default
    )
    if err != nil { panic(err) }
    defer func() { _ = shutdown(context.Background()) }()

    // 2) Build the metrics middleware
    mw := metricmw.NewMetricsMiddleware(
        metricmw.WithRecorder(rec),
        // Optional toggles:
        // metricmw.WithEnableRequests(true),
        // metricmw.WithEnableErrors(true),
        // metricmw.WithEnableLatency(true),
        // metricmw.WithEnableInFlight(true),
        // metricmw.WithFilter(func(method string) bool { return true }),
    )

    // 3) Chain it into your MCP server
    chain := mcp.NewMiddlewareChain(mw)
    handler := chain.Then(func(ctx context.Context, req *mcp.JSONRPCRequest, s mcp.Session) (mcp.JSONRPCMessage, error) {
        // ... handle request ...
        return &mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: "ok"}, nil
    })

    // Pass `handler` into your server wiring (omitted for brevity)
    _ = handler
}
```

Quick start (OTLP exporter)
- Optional: run a local collector and Prometheus via docker compose (see the files in this folder).
- Point the recorder to the collector endpoint.

```go
rec, shutdown, err := metricmw.NewOtelMetricsRecorder(
    metricmw.WithRecorderServiceName("my-mcp-service"),
    metricmw.WithRecorderExporterType(metricmw.ExporterOTLP),
    metricmw.WithRecorderEndpoint("localhost:4317"),
)
```

Note: The example uses an insecure gRPC connection for development. For production, use TLS and proper authentication/authorization at your collector or observability backend.

Metrics emitted
- mcp_requests_total (Counter)
  - Labels: method
  - Meaning: total requests by method
- mcp_errors_total (Counter)
  - Labels: method, code
  - Meaning: total errors by method and error code. Internal handler errors are reported with a predefined internal code.
- mcp_requests_in_flight (UpDownCounter)
  - Labels: method
  - Meaning: active requests in flight by method (increment on entry, decrement on exit)
- mcp_request_duration_ms (Histogram)
  - Labels: method
  - Unit: milliseconds (encoded in the metric name)
  - Meaning: request latency distribution by method

Behavior and error handling
- The middleware increments in-flight at the beginning of a call and decrements on return (via `defer`).
- Requests are counted as soon as the call enters the handler.
- Latency is measured around the downstream handler invocation.
- Errors: if the handler returns a Go error, it is reported as an internal error. If the handler returns a JSON-RPC error (`*JSONRPCError`), its code is used as the error code label.

Configuration
- Metrics middleware
  - WithRecorder(recorder): supply a `MetricsRecorder`. If nil, the middleware is a no-op.
  - WithEnableRequests(bool): toggle request counts
  - WithEnableErrors(bool): toggle error counts
  - WithEnableLatency(bool): toggle latency histogram
  - WithEnableInFlight(bool): toggle in-flight gauge
  - WithFilter(func(string) bool): selectively include methods (return true to record)
- Recorder (OpenTelemetry)
  - WithRecorderServiceName(string): set `service.name` resource
  - WithRecorderExporterType(ExporterStdout|ExporterOTLP)
  - WithRecorderEndpoint(string): OTLP gRPC endpoint (ExporterOTLP only)

Cardinality and performance guidance
- Keep `method` values normalized and low-cardinality. Avoid embedding dynamic IDs.
- Do not attach PII (user IDs, emails, tokens) as metric attributes.
- The example uses a short 5s export interval for responsiveness; in production, consider 30–60s to reduce overhead and network load.
- The provided recorder is concurrency-safe; the middleware itself is stateless and safe for concurrent use.

Local stack (optional)
- `docker-compose.yaml` + `otel-collector.yaml` + `prometheus.yaml` can run a basic local observability stack. Adjust ports and endpoints as needed. Start with:

```
docker compose up -d
```

Then switch the recorder to `ExporterOTLP` and verify metrics flow through the collector.

License
- This example is part of the `trpc-mcp-go` project and is licensed under the Apache License 2.0. See the repository’s LICENSE for details.

