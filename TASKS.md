# Experimental Tasks client

This branch implements a **compatibility prototype** of the
[SEP-2663 Tasks extension](https://tasks.extensions.modelcontextprotocol.io/seps/2663-tasks-extension).
It is not full MCP 2026-07-28 support: initialization, discovery and transport
lifecycle still use the existing SDK protocol. Use only with a server that
explicitly supports this combination. Do not change the protocol version string
to claim stateless-protocol compliance.

## Usage

```go
client, err := mcp.NewClient(endpoint, info, mcp.WithTasks(true))
// Handle err, initialize client and defer Close as usual.
ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
defer cancel()
result, err := client.CallTool(ctx, request)
```

`Connector.CallTool` keeps its signature. Tasks are disabled by default. When
enabled, a tool request advertises the extension in per-request metadata. An
ordinary result returns immediately; a task handle is polled with `tasks/get`.
Polling honors the server interval, uses a one-second fallback, clamps zero to
10 milliseconds and can be interrupted by context cancellation. The completed
result retains content, structured content, metadata and `isError`.

Transport retries are suppressed for the initial tool call and task mutations.
If submission fails without a handle, the SDK returns a `TaskError` with an empty
`TaskID()`: **do not replay the original call**. The remote operation may already
have executed. This is an unknown outcome, not exactly-once delivery.

After a polling interruption, `errors.As(err, &taskError)` recovers the handle.
Call `ResumeToolTask(ctx, taskError.TaskID())` on an initialized client for the
same endpoint and authorization identity. It sends only `tasks/get`, never
`tools/call`. Callers must persist handles themselves if crash recovery is
needed. Legacy servers may scope handles to the original session: the same
endpoint and identity do not guarantee cross-session recovery. Reinitializing a
stdio process is not guaranteed to preserve tasks. A missing task is an error,
never a reason to submit the original tool again.

`input_required` returns a `TaskError` containing the outstanding request map.
The application must validate the method, authorize the action and deduplicate
request IDs before explicitly calling `UpdateTask`. No sampling, elicitation or
human approval is performed automatically. After acknowledgement, resume polling.
If acknowledgement is lost, reconcile with the server before repeating a side
effecting input handler. Automatic input handling and durable deduplication are
outside this prototype.

Cancelling the context stops local waiting. Use `CancelTask` with a separate,
bounded live context to request remote cancellation. Its acknowledgement is not
a terminal result; `ResumeToolTask` observes the final state. RPC failures retain
their code/data in `TaskRPCError`; completed tool errors remain `IsError` results.

## Validation and integration

`go test -race . -run TestTasks -count=1` runs local HTTP fixtures and needs no
model credentials. Tests cover ordinary responses, opt-out behavior, task
completion, retained results, polling interruption/resume, unknown submission,
input-required, failed/cancelled/malformed tasks, deadline and explicit actions.
The companion Agent branch has an HTTP reconnect test asserting two initialized
connections, two polls and exactly one tool invocation.

The optional `TasksEnabled()` capability lets adapters apply one overall task
deadline without changing timeout behavior for clients that have not opted in.

## Independent interoperability

The build-tagged test runs against the unmodified
[mcpkit Tasks v2 example](https://github.com/panyam/mcpkit/tree/4bfb36c0a2687aa9f9ad8ce7e770a94311e21643/examples/tasks-v2).
It needs Go 1.26.9 for that server, no model credentials, and an unused local port.
In a separate terminal, start the pinned server:

```sh
scratch=$(mktemp -d)
git clone https://github.com/panyam/mcpkit.git "$scratch/mcpkit"
git -C "$scratch/mcpkit" checkout --detach 4bfb36c0a2687aa9f9ad8ce7e770a94311e21643
cd "$scratch/mcpkit/examples/tasks-v2"
go run . --serve --addr=127.0.0.1:18086 --wire=dual
```

Then, from this SDK checkout:

```sh
MCP_TASKS_INTEROP_URL=http://127.0.0.1:18086/mcp \
  go test -race -tags mcptasksinterop . -run '^TestTasksMCPKitInterop$' -count=1
```

Stop the example server after testing. The test verifies ordinary/completed
results, tool-level errors, failed task identity/status, deadline interruption,
same-session resume, cross-session rejection, and explicit cancellation.
This establishes interoperability with that pinned dual-mode example only.
It does not establish authorization isolation or stateless compatibility.

## Draft release gates

Track the full plan in [SDK #114](https://github.com/trpc-group/trpc-mcp-go/issues/114).
The base-protocol upgrade in [#111](https://github.com/trpc-group/trpc-mcp-go/issues/111)
is separate and unresolved: `server/discover`, stateless transport and the newer
protocol negotiation are not implemented here. The pinned server's stateless-only
endpoint rejects this SDK's initialization with HTTP 400. Keep this PR in draft
until the protocol/API decisions and supported-transport validation are resolved.
Automatic policy-routed input handling and durable task storage are follow-ups.
