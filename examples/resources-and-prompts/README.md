# Resources and Prompts Example

This example demonstrates MCP's resource management and prompt template capabilities.

## Features

- **Resources**: Serve static and dynamic content
  - Text resources with plain text content
  - Binary resources (e.g., images, documents)
  - Dynamic resource generation

- **Prompts**: Template-based prompt management
  - Parameterized prompt templates
  - Argument validation
  - Dynamic prompt generation

- **Tools**: Basic tool integration with resource access

## Quick Start

**Start the server:**
```bash
cd server
go run main.go
```

**Run the client:**
```bash
cd client  
go run main.go
```

## What it demonstrates

1. **Resource Registration**: How to register and serve different types of resources
2. **Resource Handlers**: Dynamic content generation for resources
3. **Prompt Templates**: Creating reusable prompt templates with parameters
4. **Client Integration**: How clients can discover and use resources and prompts

The example shows practical patterns for serving content and managing prompt templates in MCP applications.
