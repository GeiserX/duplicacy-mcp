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
- One call answers "are my backups OK?": `get_backup_overview` gives every backup a verdict (ok, running, stale, never, failed) with a reason.
- Speaks MCP 2026-07-28 (`server/discover`, stateless requests) and every earlier revision down to 2024-11-05 (the `initialize` handshake) from the same binary, over stdio and Streamable HTTP. Clients that still use the old handshake keep working.
- Read-only by design: every tool and resource only reads the exporter, so there is no read-only flag to set. Every tool carries `readOnlyHint`, a title, an output schema and structured results.
- HTTP listens on `127.0.0.1:8080` by default. On any other address it refuses to start without a bearer token, rejects cross-origin browser requests and caps request bodies.
- Ships as Go binaries for several platforms (GoReleaser), a Docker image and an npm wrapper.

## Tools and resources

| Tool | What it answers |
|------|-----------------|
| `get_backup_overview` | Health of every backup in one call: verdict, reason, age of the last success. `max_age_hours` (default 26) sets when a backup is stale. |
| `list_snapshots` | Every snapshot ID, storage target and machine, for use as filter values. |
| `get_backup_status` | State and last-run details of one snapshot, per storage target and machine. |
| `get_backup_history` | The last completed run of one snapshot. The exporter keeps only the last run. |
| `get_prune_status` | Whether a prune is running and when the last one succeeded, per storage target. |
| `get_storage_usage` | Storage size and chunks, and revisions per snapshot, from the exporter's storage poller. |

Resources: `duplicacy://status`, `duplicacy://progress` and `duplicacy://health`. Parameters and output fields are on [Usage](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/usage.md).

## Install

The npm package runs the server over stdio; set `DUPLICACY_EXPORTER_URL` to your exporter.

**Claude Code**

```bash
claude mcp add --transport stdio --env DUPLICACY_EXPORTER_URL=http://localhost:9750 duplicacy -- npx -y duplicacy-mcp
```

**Claude Desktop** (`claude_desktop_config.json`) and **Cursor** (`~/.cursor/mcp.json`)

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

**VS Code** (`.vscode/mcp.json`)

```json
{
  "servers": {
    "duplicacy": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "duplicacy-mcp"],
      "env": { "DUPLICACY_EXPORTER_URL": "http://localhost:9750" }
    }
  }
}
```

**Codex** (`~/.codex/config.toml`)

```toml
[mcp_servers.duplicacy]
command = "npx"
args = ["-y", "duplicacy-mcp"]
env = { DUPLICACY_EXPORTER_URL = "http://localhost:9750" }
```

npm 12, pnpm and Bun skip install scripts, so the wrapper downloads and verifies the binary on first start. Versions up to 0.1.2 relied on the install script alone; with npm 12 run them as `npx -y --allow-scripts=duplicacy-mcp duplicacy-mcp`.

**Docker (Streamable HTTP)**

```bash
export MCP_AUTH_TOKEN=$(openssl rand -hex 32)
docker run -d -p 127.0.0.1:8080:8080 -e MCP_AUTH_TOKEN \
  -e DUPLICACY_EXPORTER_URL=http://duplicacy-exporter:9750 drumsergio/duplicacy-mcp:v0.1.2
claude mcp add --transport http duplicacy http://127.0.0.1:8080/mcp --header "Authorization: Bearer $MCP_AUTH_TOKEN"
```

The image listens on `0.0.0.0:8080` inside the container, so it will not start without `MCP_AUTH_TOKEN` (or `MCP_ALLOW_UNAUTHENTICATED=true` behind a reverse proxy that authenticates). Compose, a global npm install and a local build are in [Getting started](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/getting-started.md); every variable is in [Configuration](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/configuration.md).

## Documentation

- [Getting started](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/getting-started.md): Docker Compose, npm, local build, first run
- [Configuration](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/configuration.md): environment variables, HTTP security, client config for stdio and HTTP
- [Usage](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/usage.md): tools, parameters, output fields and resources
- [Development](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/development.md): testing with Inspector, credits, contributing
- [Related projects](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/related.md): the Duplicacy family, other MCP servers, registry listings

## Related projects

Part of the Duplicacy family: [duplicacy-exporter](https://github.com/GeiserX/duplicacy-exporter), [duplicacy-cli-cron](https://github.com/GeiserX/duplicacy-cli-cron), [duplicacy-ha](https://github.com/GeiserX/duplicacy-ha), [duplicacy-container](https://github.com/GeiserX/duplicacy-container).

## License

[GPL-3.0-or-later](https://github.com/GeiserX/duplicacy-mcp/blob/main/LICENSE)
