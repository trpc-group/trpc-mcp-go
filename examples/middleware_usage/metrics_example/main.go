package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"trpc.group/trpc-go/trpc-mcp-go"
	metricmw "trpc.group/trpc-go/trpc-mcp-go/examples/middlewares/metrics"
)

func main() {
	// 1. Create a new server instance.
	s := mcp.NewServer(
		"mcp-server",
		"1.0.0",
		mcp.WithStatelessMode(true),
		mcp.WithServerPath(""),
	)

	// 2. Setup the metrics recorder and middleware.
	rec, shutdown, err := metricmw.NewOtelMetricsRecorder(
		metricmw.WithRecorderServiceName("my-mcp-service"),
	)
	if err != nil {
		log.Fatalf("Failed to create metrics recorder: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	mw := metricmw.NewMetricsMiddleware(
		metricmw.WithRecorder(rec),
	)
	s.Use(mw)

	// 3. Register a tool handler with the correct function signature and return type.
	echoTool := &mcp.Tool{
		Name:        "echo",
		Description: "A simple tool that echoes back its input arguments as a JSON string.",
	}
	s.RegisterTool(echoTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Echo back the arguments as a JSON string.
		resultBytes, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return mcp.NewErrorResult("failed to serialize arguments"), nil
		}
		return mcp.NewTextResult(string(resultBytes)), nil
	})

	// 4. Start the server.
	log.Println("Server is starting on :8080...")
	if err := http.ListenAndServe(":8080", s.Handler()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
