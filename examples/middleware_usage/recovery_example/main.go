package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/examples/middlewares"
)

// App is a simple application with a method that can panic.
type App struct{}

// Panic is a method that always panics and conforms to the toolHandler signature.
func (a *App) Panic(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	panic("This is a test panic!")
}

func main() {
	// Create a new MCP server.
	s := mcp.NewServer(
		"mcp-server",
		"1.0.0",
		mcp.WithStatelessMode(true),
		mcp.WithServerPath(""), // Respond to the root path.
	)

	// Create an instance of our app.
	app := &App{}

	// Create a new tool definition.
	panicTool := mcp.NewTool(
		"panic",
		mcp.WithDescription("A tool that always panics."),
	)

	// Register the tool with its handler.
	s.RegisterTool(panicTool, app.Panic)

	// Use the recovery middleware.
	s.Use(middlewares.Recovery())

	// Create a new HTTP server.
	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: s.Handler(), // Use the Handler() method to get the http.Handler.
	}

	// Start the server in a goroutine.
	go func() {
		fmt.Println("Server listening on :8080")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Could not listen on %s: %v\n", ":8080", err)
		}
	}()

	// Wait for a shutdown signal.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	// Shutdown the server gracefully.
	fmt.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		fmt.Printf("Server shutdown failed: %v\n", err)
	}
	fmt.Println("Server gracefully stopped")
}