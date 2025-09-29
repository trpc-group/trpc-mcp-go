# Changelog

## 0.0.6 (2025-09-29)

### Features
- feat: enhance jsonschema tag support and add SSE server path method (#84)
- feat: align prompt/resource list filtering with tools (#77)
- feat(sse): add custom session ID generator support (#83)

### Fixes
- fix: Resource arguments not passed to handler (#82)
- fix: fix double-wrapped JSON-RPC error in HTTP transports (#79)

## 0.0.5 (2025-09-05)

### Features
- feat: downgrade Go version from 1.22.5 to 1.20 for broader compatibility (#70)
- feat: add get tools methods for server-side (#67)
- feat: implement MCP ToolAnnotations support (#66)
- refactor: reorganize examples with transport-based structure (#65)
- feat: add output schema and structured content support (#62)

### Fixes
- fix: ensure client options properly configure SSE transport (#71)
- fix: static check warning (#49)
- fix: example (#50)
- fix: default value for WithClientGetSSEEnabled (#51)

## 0.0.4 (2025-08-29)

- sse: remove SSE response size limit (#63)

## 0.0.3 (2025-08-27)

- fix: remove syscall.Fsync to support Windows builds (#60)
- feat: add MCP send request retry mechanism (#59)
- fix: fix SSE client headers and server session context management (#58)

## 0.0.2 (2025-08-18)

- feat: support register multiple resources (#55)
- feat: add server-initiated requests and Roots capability support (#45)
- fix: support multiple options in WithHTTPReqHandlerOption (#53)
- enhance: ease the 202 check on notifications for compatibility reasons (#47)

## 0.0.1 (2025-07-24)

- Initial release

### Features

#### Core MCP Protocol Implementation
- **JSON-RPC 2.0 Foundation**: Robust JSON-RPC 2.0 message handling with comprehensive error handling and validation
- **Protocol Version Negotiation**: Automatic protocol version selection and client compatibility support
- **Lifecycle Management**: Complete session initialization, management, and termination with state tracking

#### Transport Layer Support
- **Streamable HTTP**: HTTP-based transport with optional Server-Sent Events (SSE) streaming for real-time communication
- **STDIO Transport**: Full stdio-based communication for process-to-process MCP integration
- **SSE Server**: Dedicated Server-Sent Events implementation for event streaming and real-time updates
- **Multi-language Compatibility**: STDIO client can connect to TypeScript (npx), Python (uvx), and Go MCP servers

#### Connection Modes
- **Stateful Connections**: Persistent sessions with session ID management and activity tracking
- **Stateless Mode**: Temporary sessions for simple request-response patterns without persistence
- **GET SSE Support**: Optional GET-based SSE connections for enhanced client compatibility

#### Tool Framework
- **Dynamic Tool Registration**: Runtime tool registration with structured parameter schemas using OpenAPI 3.0
- **Type-Safe Parameters**: Built-in support for string, number, boolean, array, and object parameters
- **Parameter Validation**: Comprehensive validation including required fields, constraints, and type checking
- **Tool Filtering**: Context-based dynamic tool filtering for role-based access control
- **Progress Notifications**: Real-time progress updates for long-running tool operations
- **Error Handling**: Structured error responses with detailed error codes and messages

#### Resource Management
- **Text and Binary Resources**: Serve both text and binary resources with MIME type support
- **Resource Templates**: URI template-based dynamic resource generation
- **Resource Subscriptions**: Real-time resource update notifications

#### Prompt Templates
- **Dynamic Prompt Creation**: Runtime prompt template registration and management
- **Parameterized Prompts**: Support for prompt arguments with validation
- **Message Composition**: Multi-message prompt support with role-based content
