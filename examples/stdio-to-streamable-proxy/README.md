# STDIO to Streamable HTTP Proxy

This example starts a local stdio MCP child server, exposes it as a Streamable HTTP MCP server through `NewStreamableServerWithStdio`, and verifies the proxy with a Streamable HTTP client.

Run it from the repository root:

```bash
go run ./examples/stdio-to-streamable-proxy
```

Expected output includes the discovered `echo` and `add` tools plus successful tool call results.
