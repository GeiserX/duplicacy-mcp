# Configuration

| Variable                    | Default                 | Secret | Description |
|-----------------------------|-------------------------|--------|-------------|
| `DUPLICACY_EXPORTER_URL`    | `http://localhost:9750` | no     | Base URL of duplicacy-exporter (`http://` or `https://`). The server reads `<url>/metrics` and `<url>/health`. |
| `TRANSPORT`                 | _(empty = HTTP)_        | no     | Set to `stdio` for the stdio transport. The npm package always sets it. |
| `LISTEN_ADDR`               | `127.0.0.1:8080`        | no     | HTTP listen address. The Docker image sets `0.0.0.0:8080`. |
| `MCP_AUTH_TOKEN`            | _(none)_                | yes    | Bearer token every HTTP request must send as `Authorization: Bearer <token>`. At least 16 characters; `openssl rand -hex 32` makes a good one. Required when `LISTEN_ADDR` is not loopback. |
| `MCP_ALLOW_UNAUTHENTICATED` | `false`                 | no     | `true` lets a non-loopback `LISTEN_ADDR` start without a token. Only for a reverse proxy in front that authenticates every request. |
| `MCP_ALLOWED_ORIGINS`       | _(none)_                | no     | Comma-separated browser origins (`https://app.example`) allowed to call the endpoint cross-origin. Non-browser clients need no entry. |

Put them in a `.env` file (from `.env.example`) or set them in the environment. Over stdio the `.env`
file is not read, so an unrelated project's `.env` cannot leak in.

## HTTP security

The Streamable HTTP endpoint is `/mcp`. It is stateless: it never issues a session ID.

- **Bind and token.** On `127.0.0.1` (the default) no token is needed. On any other address, such as
  `0.0.0.0` in Docker, the server refuses to start until `MCP_AUTH_TOKEN` is set. A request without
  `Authorization: Bearer <token>`, or with the wrong token, gets `401`. The compare runs in constant time.
- **Origin.** A cross-origin browser request (`Sec-Fetch-Site: cross-site`, or an `Origin` that does not
  match the host) gets `403`, so a web page cannot drive the server through your browser. Add trusted
  origins with `MCP_ALLOWED_ORIGINS`. A loopback bind also rejects a non-localhost `Host` header (DNS
  rebinding).
- **Limits.** Request bodies over 1 MiB get `413`. The server drops connections that take more than 10 s
  to send headers or 30 s to send a request, and idle connections after 120 s.
- **Exporter output.** Label values (snapshot IDs, storage names, machine names) are stripped of control
  characters and capped at 128 characters. Errors never include the exporter URL or its response body;
  the details go to the server log on stderr.

## Protocol versions

The server answers MCP 2026-07-28 requests (`server/discover`, per-request `_meta`, no handshake) and the
`initialize` handshake of 2025-11-25, 2025-06-18, 2025-03-26 and 2024-11-05, from the same binary, over
stdio and HTTP. List results carry a one-hour cache hint for 2026-07-28 clients; resource reads are never
cached.

## Client configuration

Over stdio, which the npm package always uses, a client that reads an `mcpServers` block starts the
server itself:

```json
{
  "mcpServers": {
    "duplicacy": {
      "command": "npx",
      "args": ["-y", "duplicacy-mcp"],
      "env": { "DUPLICACY_EXPORTER_URL": "http://localhost:9750" }
    }
  }
}
```

The README has the same block for Claude Code, Claude Desktop, Cursor, VS Code and Codex.

Over HTTP, for a server you already run (the Docker image, or the binary without `TRANSPORT=stdio`), a
client that supports remote servers connects to the `/mcp` endpoint with the token as a header:

```bash
claude mcp add --transport http duplicacy http://127.0.0.1:8080/mcp --header "Authorization: Bearer $MCP_AUTH_TOKEN"
```

```json
{
  "mcpServers": {
    "duplicacy": {
      "type": "http",
      "url": "http://127.0.0.1:8080/mcp",
      "headers": { "Authorization": "Bearer ${MCP_AUTH_TOKEN}" }
    }
  }
}
```
