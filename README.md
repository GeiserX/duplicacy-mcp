<p align="center">
  <img src="https://raw.githubusercontent.com/GeiserX/duplicacy-mcp/main/docs/images/banner.svg" alt="Duplicacy MCP" width="900"/>
</p>

<h1 align="center">Duplicacy MCP</h1>

<p align="center">
  <a href="https://www.npmjs.com/package/duplicacy-mcp"><img src="https://img.shields.io/npm/v/duplicacy-mcp?style=flat-square&logo=npm" alt="npm"/></a>
  <a href="https://github.com/GeiserX/duplicacy-mcp/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/GeiserX/duplicacy-mcp/ci.yml?style=flat-square&label=CI" alt="CI"/></a>
  <a href="https://github.com/GeiserX/duplicacy-mcp/blob/main/LICENSE"><img src="https://img.shields.io/github/license/GeiserX/duplicacy-mcp?style=flat-square" alt="License"/></a>
  <a href="https://hub.docker.com/r/drumsergio/duplicacy-mcp"><img src="https://img.shields.io/docker/pulls/drumsergio/duplicacy-mcp?style=flat-square&logo=docker" alt="Docker Pulls"/></a>
  <a href="https://github.com/GeiserX/duplicacy-mcp/stargazers"><img src="https://img.shields.io/github/stars/GeiserX/duplicacy-mcp?style=flat-square&logo=github" alt="GitHub Stars"/></a>
</p>

<p align="center"><strong>A tiny bridge that reads Duplicacy backup metrics from a Prometheus exporter and exposes them as an MCP server, enabling LLMs to monitor backup status, progress, and health.</strong></p>

## Features

- Reads the metrics of [duplicacy-exporter](https://github.com/GeiserX/duplicacy-exporter) and serves them over MCP.
- Resources `duplicacy://status`, `duplicacy://progress` and `duplicacy://health` for read-only browsing.
- Tools `get_backup_status`, `get_backup_history`, `list_snapshots` and `get_prune_status`.
- One JSON-RPC endpoint (`/mcp`) over HTTP, or stdio through `npx duplicacy-mcp`.
- Listens on `127.0.0.1:8080` by default. Put a reverse proxy with authentication in front before exposing it.
- Ships as Go binaries for several platforms (GoReleaser), a Docker image and an npm wrapper.

## Quick start

```bash
docker run -d -p 127.0.0.1:8080:8080 -e DUPLICACY_EXPORTER_URL=http://duplicacy-exporter:9750 drumsergio/duplicacy-mcp:v0.1.0
```

That serves `http://127.0.0.1:8080/mcp`. For a client that starts the server itself over stdio, use the npm package in an `mcpServers` block:

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

Compose, a global npm install and a local build are in [Getting started](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/getting-started.md).

## Documentation

- [Getting started](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/getting-started.md): Docker Compose, npm, local build, first run
- [Configuration](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/configuration.md): environment variables, client config for stdio and HTTP
- [Usage](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/usage.md): resources and tools
- [Development](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/development.md): testing with Inspector, credits, contributing
- [Related projects](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/related.md): the Duplicacy family, other MCP servers, registry listings

## Related projects

Part of the Duplicacy family: [duplicacy-exporter](https://github.com/GeiserX/duplicacy-exporter), [duplicacy-cli-cron](https://github.com/GeiserX/duplicacy-cli-cron), [duplicacy-ha](https://github.com/GeiserX/duplicacy-ha), [duplicacy-container](https://github.com/GeiserX/duplicacy-container).

## License

[GPL-3.0-or-later](https://github.com/GeiserX/duplicacy-mcp/blob/main/LICENSE)
