# Getting started

You need a running [duplicacy-exporter](https://github.com/GeiserX/duplicacy-exporter) that the server can
reach; it reads the exporter's `/metrics`. Pick one way to run the server; the settings are on
[Configuration](configuration.md).

## Docker Compose

```yaml
services:
  duplicacy-mcp:
    image: drumsergio/duplicacy-mcp:v0.2.0
    ports:
      - "127.0.0.1:8080:8080"
    environment:
      - DUPLICACY_EXPORTER_URL=http://duplicacy-exporter:9750
      # Required: the container listens on 0.0.0.0. Generate with: openssl rand -hex 32
      - MCP_AUTH_TOKEN=${MCP_AUTH_TOKEN:?set MCP_AUTH_TOKEN}
    read_only: true
    cap_drop: [ALL]
    security_opt: [no-new-privileges:true]
```

The image runs as uid 65532 and needs no writable filesystem. Clients send the token as
`Authorization: Bearer <token>`; see [HTTP security](configuration.md#http-security).

## Install via npm (stdio transport)

```sh
npx -y duplicacy-mcp
```

Or install globally:

```sh
npm install -g duplicacy-mcp
duplicacy-mcp
```

This downloads the pre-built Go binary for your platform from
[GitHub Releases](https://github.com/GeiserX/duplicacy-mcp/releases), checks its SHA-256 against the
checksums shipped inside the npm package, and runs it with the stdio transport. The download happens at
install time, or on first start when the package manager skipped install scripts (npm 12, pnpm, Bun).
The client config block is on [Configuration](configuration.md#client-configuration).

## Local build

```sh
git clone https://github.com/GeiserX/duplicacy-mcp
cd duplicacy-mcp

# (optional) create .env from the sample
cp .env.example .env && $EDITOR .env

go run ./cmd/server
```

## First run

Add the server to your client, then ask it whether your backups are OK; it calls `get_backup_overview`.
Over stdio a legacy client's exchange looks like this (one JSON-RPC message per line):

```text
-> {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}
<- {"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25","capabilities":{"resources":{},"tools":{}},"serverInfo":{"name":"Duplicacy MCP Bridge","title":"Duplicacy MCP",...},"instructions":"..."}}
-> {"jsonrpc":"2.0","method":"notifications/initialized"}
-> {"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_backup_overview","arguments":{}}}
<- {"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{\"status\":\"ok\",...}"}],"structuredContent":{"status":"ok","summary":"Healthy: 1 of 1 backups (1 ok, 0 running).",...}}}
```

A 2026-07-28 client skips `initialize` and sends `server/discover`, then each request with its version in
`_meta`:

```text
-> {"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}
<- {"jsonrpc":"2.0","id":1,"result":{"supportedVersions":["2026-07-28","2025-11-25","2025-06-18","2025-03-26","2024-11-05"],...,"resultType":"complete","ttlMs":3600000,"cacheScope":"public"}}
```

When `DUPLICACY_EXPORTER_URL` is wrong, the tool call fails with a message that says what kind of failure
it was, without the URL:

```text
<- {"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"exporter unreachable (DNS lookup failed); check DUPLICACY_EXPORTER_URL and that duplicacy-exporter is running"}],"isError":true}}
```
