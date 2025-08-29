// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// Weather tool demonstrates struct-first input AND output schemas
type WeatherInput struct {
	Location string `json:"location" jsonschema:"required,description=City name"`
	Units    string `json:"units,omitempty" jsonschema:"description=Temperature units,enum=celsius,enum=fahrenheit,default=celsius"`
}

type WeatherOutput struct {
	Location    string  `json:"location" jsonschema:"description=Requested location"`
	Temperature float64 `json:"temperature" jsonschema:"description=Current temperature"`
	Description string  `json:"description" jsonschema:"description=Weather description"`
	Units       string  `json:"units" jsonschema:"description=Temperature units"`
}

// Calculator tool demonstrates struct-first validation features
type CalculatorInput struct {
	Operation string  `json:"operation" jsonschema:"required,description=Math operation,enum=add,enum=subtract,enum=multiply,enum=divide"`
	A         float64 `json:"a" jsonschema:"required,description=First number"`
	B         float64 `json:"b" jsonschema:"required,description=Second number"`
}

type CalculatorOutput struct {
	Operation string  `json:"operation" jsonschema:"description=Performed operation"`
	A         float64 `json:"a" jsonschema:"description=First operand"`
	B         float64 `json:"b" jsonschema:"description=Second operand"`
	Result    float64 `json:"result" jsonschema:"description=Calculation result"`
	Message   string  `json:"message" jsonschema:"description=Status message"`
}

func main() {
	log.Println("Starting Struct-First MCP Server...")

	// Create MCP server
	mcpServer := mcp.NewServer(
		"Struct-First-Demo-Server",
		"1.0.0",
		mcp.WithServerAddress(":3002"),
		mcp.WithServerPath("/mcp"),
	)

	// 1. Weather Tool - Full struct-first approach
	weatherTool := mcp.NewTool(
		"get_weather",
		mcp.WithDescription("Get weather information using struct-first schemas"),
		mcp.WithInputStruct[WeatherInput](),   // ⭐ Generate input schema from struct
		mcp.WithOutputStruct[WeatherOutput](), // ⭐ Generate output schema from struct
	)

	weatherHandler := mcp.NewTypedToolHandler(func(ctx context.Context, req *mcp.CallToolRequest, input WeatherInput) (WeatherOutput, error) {
		log.Printf("Weather request: %+v", input)

		// Simulate weather API call
		return WeatherOutput{
			Location:    input.Location,
			Temperature: 22.5,
			Description: "Partly cloudy",
			Units:       input.Units,
		}, nil
	})

	mcpServer.RegisterTool(weatherTool, weatherHandler)

	// 2. Calculator Tool - Demonstrates validation and enums
	calculatorTool := mcp.NewTool(
		"calculator",
		mcp.WithDescription("Perform math operations with automatic validation"),
		mcp.WithInputStruct[CalculatorInput](),   // ⭐ Automatic enum validation
		mcp.WithOutputStruct[CalculatorOutput](), // ⭐ Structured response
	)

	calculatorHandler := mcp.NewTypedToolHandler(func(ctx context.Context, req *mcp.CallToolRequest, input CalculatorInput) (CalculatorOutput, error) {
		log.Printf("Calculator request: %+v", input)

		var result float64
		var message string

		switch input.Operation {
		case "add":
			result = input.A + input.B
			message = "Addition completed successfully"
		case "subtract":
			result = input.A - input.B
			message = "Subtraction completed successfully"
		case "multiply":
			result = input.A * input.B
			message = "Multiplication completed successfully"
		case "divide":
			if input.B == 0 {
				message = "Error: Division by zero"
			} else {
				result = input.A / input.B
				message = "Division completed successfully"
			}
		default:
			return CalculatorOutput{}, fmt.Errorf("invalid operation: %s (must be add, subtract, multiply, or divide)", input.Operation)
		}

		return CalculatorOutput{
			Operation: input.Operation,
			A:         input.A,
			B:         input.B,
			Result:    result,
			Message:   message,
		}, nil
	})

	mcpServer.RegisterTool(calculatorTool, calculatorHandler)

	log.Printf("Registered %d struct-first tools", 2)
	log.Println("Key features demonstrated:")
	log.Println("  ⭐ WithInputStruct[T]() - Generate input schemas from Go structs")
	log.Println("  ⭐ WithOutputStruct[T]() - Generate output schemas from Go structs")
	log.Println("  ⭐ NewTypedToolHandler() - Type-safe handlers with automatic validation")
	log.Println("  ⭐ Struct tags - description, required, enum, default values")

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		log.Println("Received shutdown signal, stopping server...")
		os.Exit(0)
	}()

	// Start the server
	log.Println("Server listening on http://localhost:3002/mcp")
	log.Println("Press Ctrl+C to stop...")

	if err := mcpServer.Start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
