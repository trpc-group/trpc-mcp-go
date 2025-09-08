# Annotations Example

Demonstrates MCP ToolAnnotations feature for providing behavioral hints about tools to clients.

## Features

- **ToolAnnotations Support**: Complete implementation of MCP tool behavior hints
- **Multiple Tool Types**: Four tools showcasing different annotation patterns
- **Client Display**: Annotations properly received and displayed by clients
- **Best Practices**: Examples of proper annotation usage patterns

## Tools Demonstrated

1. **greet** - Simple Greeter
   - `ReadOnlyHint: true` (safe, no side effects)
   - `IdempotentHint: true` (same input → same output)
   - `OpenWorldHint: false` (self-contained)

2. **advanced-greet** - Advanced Formatter  
   - Similar to greet but with multiple output formats
   - Demonstrates read-only tools with formatting capabilities

3. **delete_file** - File Deleter
   - `ReadOnlyHint: false` (modifies filesystem)
   - `DestructiveHint: true` (permanently deletes data)
   - `IdempotentHint: false` (multiple calls have different effects)
   - `OpenWorldHint: true` (interacts with external filesystem)

4. **calculate** - Math Calculator
   - `ReadOnlyHint: true` (pure computation)
   - `IdempotentHint: true` (deterministic)
   - `OpenWorldHint: false` (no external dependencies)

## Quick Start

**Start the server:**
```bash
cd server
go run main.go
```
Server will start on `localhost:3000/mcp`

**Run the client:**
```bash
cd client
go run main.go  
```

## What it demonstrates

1. **ToolAnnotations Definition**: Using `WithToolAnnotations()` to add behavior hints
2. **BoolPtr Utility**: Helper function for setting optional boolean hints
3. **Client Parsing**: How clients receive and display annotation information
4. **Annotation Patterns**: Different configurations for various tool types
5. **JSON Serialization**: Proper omitempty behavior for optional fields

## API Usage Example

```go
// Define a tool with annotations
weatherTool := mcp.NewTool(
    "get_weather",
    mcp.WithDescription("Get weather information for a location"),
    mcp.WithString("location", mcp.Description("City name"), mcp.Required()),
    mcp.WithToolAnnotations(&mcp.ToolAnnotations{
        Title:           "Weather Information Tool",
        ReadOnlyHint:    mcp.BoolPtr(true),  // Safe, no side effects
        DestructiveHint: mcp.BoolPtr(false), // Non-destructive operation
        IdempotentHint:  mcp.BoolPtr(true),  // Same input → same output
        OpenWorldHint:   mcp.BoolPtr(true),  // Interacts with external APIs
    }),
)
```

## Key Concepts

- **Hints, not Security**: Annotations are hints for UX optimization, not security guarantees
- **Optional Fields**: Use `BoolPtr()` for optional boolean annotations
- **Semantic Categories**: ReadOnly vs Destructive, Idempotent vs Non-idempotent
- **Interaction Scope**: OpenWorld (external) vs ClosedWorld (internal) tools

Perfect for understanding how to enhance tool definitions with behavioral metadata!
