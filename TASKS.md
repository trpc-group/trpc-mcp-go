# Experimental Tasks client

This client implements opt-in polling for the
[SEP-2663 Tasks extension](https://tasks.extensions.modelcontextprotocol.io/seps/2663-tasks-extension).
It supports explicit 2026-07-28 Streamable HTTP discovery and request metadata,
as well as legacy initialization with servers that explicitly support that
combination. The default protocol and ordinary client behavior are unchanged.
This is client-side support for the task workflow, not full MCP 2026-07-28
conformance: server-side protocol support, modern stdio, subscriptions and
automatic multi-round-trip input handling are outside this change.

## Usage

```go
client, err := mcp.NewClient(endpoint, info,
    mcp.WithProtocolVersion(mcp.ProtocolVersion_2026_07_28),
    mcp.WithTasks(true),
)
// Handle err, initialize client and defer Close as usual.
ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
defer cancel()
result, err := client.CallTool(ctx, request)
```

For the explicitly selected stateless protocol, `Initialize(ctx, nil)` performs
`server/discover`; it sends no legacy initialization, session or GET stream.
Nonempty `InitializeRequest.Params` are rejected; configure the client through
constructor options. `SendInitialized` and `TerminateSession` are unavailable in
this mode. Protocol rejection never silently falls back to legacy initialization.
Omit the version option only for an explicitly compatible legacy server.

Each stateless request carries protocol/client metadata and routing headers.
Tool schemas are cached when listed; `CallTool` lists an unknown tool before
submission to obtain its `x-mcp-header` annotations. Invalid annotations exclude
the definition, and invalid argument/header values fail before tool submission.
Call `ListTools` again to refresh definitions when the server changes them.
The HTTP request hook can still supply authorization; protocol/header values are
computed after that hook to keep them consistent with the request parameters.

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
outside this implementation.

Cancelling the context stops local waiting. Use `CancelTask` with a separate,
bounded live context to request remote cancellation. Its acknowledgement is not
a terminal result; `ResumeToolTask` observes the final state. RPC failures retain
their code/data in `TaskRPCError`; completed tool errors remain `IsError` results.

## Validation and integration

`go test -race . -run 'TestTasks|TestStateless' -count=1` runs local HTTP fixtures and needs no
model credentials. Tests cover ordinary responses, opt-out behavior, task
completion, retained results, polling interruption/resume, unknown submission,
input-required, failed/cancelled/malformed tasks, deadline and explicit actions.
Stateless tests also cover discovery errors, version rejection, envelope IDs,
parameter-header schemas and integer precision, pagination, metadata concurrency,
non-2xx RPC errors, and closing an SSE response after the final result.
The companion Agent branch tests legacy session expiry and stateless connection
loss, asserting two initialization/discovery requests, two polls and one tool
invocation.

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
go run . --serve --addr=127.0.0.1:18087 --wire=stateless
```

Then, from this SDK checkout:

```sh
MCP_TASKS_INTEROP_PROTOCOL=2026-07-28 \
MCP_TASKS_INTEROP_URL=http://127.0.0.1:18087/mcp \
  go test -race -tags mcptasksinterop . -run '^TestTasksMCPKitInterop$' -count=1
```

Stop the example server after testing. To check the legacy compatibility path,
start the example with `--wire=dual` and omit `MCP_TASKS_INTEROP_PROTOCOL`.
Both modes verify ordinary/completed results, tool-level errors, failed task
identity/status, explicit input response and continuation, deadline interruption,
and cancellation. A new stateless client resumes the existing task; a new legacy
session is rejected, while the original legacy session can still resume it.

The pinned example does not prove authorization isolation. A separate local
experiment used the same unmodified mcpkit task engine, an authenticated-principal
`WithTaskBucketKeyer`, and a real CSV export tool. Another identity was rejected
for get/update/cancel; the original identity resumed from a new client. Lost
polling and submission responses each left one tool invocation and one 10,000-row
CSV artifact. These bounded fault experiments do not prove exactly-once delivery
or durable restart recovery. See the evidence in
[SDK #114](https://github.com/trpc-group/trpc-mcp-go/issues/114).

## Review and release boundaries

Coordinate the client protocol scope with the broader base-protocol work in
[#111](https://github.com/trpc-group/trpc-mcp-go/issues/111). The supported server
version list is unchanged; this client feature does not enable the new protocol
on this library's server. Unsupported immediate `input_required` results produce
an error; task-level input requests are returned for explicit application handling.
No subscriptions, automatic approval, input deduplication or durable task store
are introduced. The dependent Agent change must consume a released SDK version
and enable its cross-repository tests in normal CI before merge.
