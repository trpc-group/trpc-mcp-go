# Struct-First Example

This example demonstrates the **struct-first approach** for defining MCP tools using Go structs, eliminating the need for manual schema building.

## Key Features

- 🏗️ **Automatic Schema Generation** - Generate OpenAPI schemas from Go structs
- 🛡️ **Type Safety** - Compile-time type checking and runtime type conversion  
- 📋 **Rich Metadata** - Support for descriptions, enums, required fields, defaults
- 🔄 **Structured Output** - Type-safe responses with backward compatibility

## Quick Start

```bash
# Start server
go run server/main.go

# In another terminal, run client
go run client/main.go
```

## Code Overview

### Struct Definitions
```go
type WeatherInput struct {
    Location string `json:"location" jsonschema:"required,description=City name"`
    Units    string `json:"units,omitempty" jsonschema:"description=Temperature units,enum=celsius,enum=fahrenheit,default=celsius"`
}

type WeatherOutput struct {
    Location    string  `json:"location" jsonschema:"description=Requested location"`
    Temperature float64 `json:"temperature" jsonschema:"description=Current temperature"`
    // ...
}
```

### Tool Registration
```go
// Generate schemas from structs
weatherTool := mcp.NewTool(
    "get_weather",
    mcp.WithDescription("Get weather information"),
    mcp.WithInputStruct[WeatherInput](),   // ⭐ Auto-generate input schema
    mcp.WithOutputStruct[WeatherOutput](), // ⭐ Auto-generate output schema
)

// Type-safe handler
weatherHandler := mcp.NewTypedToolHandler(func(ctx context.Context, req *mcp.CallToolRequest, input WeatherInput) (WeatherOutput, error) {
    // Handler receives typed input and returns typed output
    return WeatherOutput{
        Location:    input.Location,
        Temperature: 22.5,
        // ...
    }, nil
})
```

## Supported Tags

| Tag | Description | Example |
|-----|-------------|---------|
| `required` | Mark field as required | `jsonschema:"required"` |
| `description` | Field description | `jsonschema:"description=City name"` |
| `enum` | Allowed values | `jsonschema:"enum=celsius,enum=fahrenheit"` |
| `default` | Default value | `jsonschema:"default=celsius"` |
| `minimum` | Numeric minimum | `jsonschema:"minimum=0"` |
| `maximum` | Numeric maximum | `jsonschema:"maximum=150"` |
| `format` | String format validation | `jsonschema:"format=uri"` |

## Generated Schema Example

The struct above automatically generates this OpenAPI schema:

```json
{
  "type": "object",
  "properties": {
    "location": {
      "type": "string",
      "description": "City name"
    },
    "units": {
      "type": "string", 
      "description": "Temperature units",
      "enum": ["celsius", "fahrenheit"],
      "default": "celsius"
    }
  },
  "required": ["location"]
}
```

## vs Builder Pattern

| Aspect | Struct-First | Builder Pattern |
|--------|--------------|-----------------|
| **Schema Definition** | Go structs with tags | Manual `WithString()`, `WithNumber()` calls |
| **Type Safety** | Compile-time checked | Runtime only |
| **Code Volume** | Concise | Verbose |
| **Maintainability** | High (single source of truth) | Medium (separate schema + handler) |
| **IDE Support** | Full autocompletion | Limited |

Choose struct-first for new tools, builder pattern for complex dynamic schemas.
