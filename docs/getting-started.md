# Getting started

You need a running [duplicacy-exporter](https://github.com/GeiserX/duplicacy-exporter) that the server can
reach; it reads the exporter's `/metrics`. Pick one way to run the server; the settings are on
[Configuration](configuration.md).

## Docker Compose

```yaml
services:
  duplicacy-mcp:
    image: drumsergio/duplicacy-mcp:v0.1.0
    ports:
      - "127.0.0.1:8080:8080"
    environment:
      - DUPLICACY_EXPORTER_URL=http://duplicacy-exporter:9750
```

> **Security note:** The HTTP transport listens on `127.0.0.1:8080` by default. If you need to expose it on a network, place it behind a reverse proxy with authentication.

## Install via npm (stdio transport)

```sh
npx -y duplicacy-mcp
```

Or install globally:

```sh
npm install -g duplicacy-mcp
duplicacy-mcp
```

This downloads the pre-built Go binary from [GitHub Releases](https://github.com/GeiserX/duplicacy-mcp/releases) for your platform and runs it with stdio transport. The client config block is on [Configuration](configuration.md#client-configuration).

## Local build

```sh
git clone https://github.com/GeiserX/duplicacy-mcp
cd duplicacy-mcp

# (optional) create .env from the sample
cp .env.example .env && $EDITOR .env

go run ./cmd/server
```

## First run

Add the server to your client, then ask it for the backup status, or read the `duplicacy://status`
resource yourself. Over stdio the exchange looks like this (one JSON-RPC message per line):

```text
-> {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}
<- {"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{"resources":{},"tools":{"listChanged":true}},"serverInfo":{"name":"Duplicacy MCP Bridge",...}}}
-> {"jsonrpc":"2.0","method":"notifications/initialized"}
-> {"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"duplicacy://status"}}
<- {"jsonrpc":"2.0","id":2,"result":{"contents":[{"uri":"duplicacy://status","mimeType":"application/json","text":"[{\"snapshot_id\":\"photos\",\"storage_target\":\"nas\",\"machine\":\"mac-mini\",\"running\":false,\"exit_code\":0,\"last_success\":\"2026-09-29T21:33:13Z\",\"last_duration_seconds\":600,...}]"}]}}
```

`text` holds one entry per backup the exporter has seen. When `DUPLICACY_EXPORTER_URL` is wrong,
`initialize` still succeeds and the read fails with the connection error, for example:

```text
<- {"jsonrpc":"2.0","id":2,"error":{"code":-32603,"message":"fetch metrics: Get \"http://nothere:9750/metrics\": dial tcp: lookup nothere ...: no such host"}}
```
