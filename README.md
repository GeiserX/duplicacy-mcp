<p align="center">
  <img src="https://raw.githubusercontent.com/GeiserX/duplicacy-mcp/main/docs/images/banner.svg" alt="Duplicacy MCP banner" width="900"/>
</p>

<h1 align="center">Duplicacy-MCP</h1>

<p align="center">
  <a href="https://www.npmjs.com/package/duplicacy-mcp"><img src="https://img.shields.io/npm/v/duplicacy-mcp?style=flat-square&logo=npm" alt="npm"/></a>
  <a href="https://github.com/GeiserX/duplicacy-mcp/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/GeiserX/duplicacy-mcp/ci.yml?style=flat-square&label=CI" alt="CI"/></a>
  <a href="https://hub.docker.com/r/drumsergio/duplicacy-mcp"><img src="https://img.shields.io/docker/pulls/drumsergio/duplicacy-mcp?style=flat-square&logo=docker" alt="Docker Pulls"/></a>
  <a href="https://github.com/GeiserX/duplicacy-mcp/stargazers"><img src="https://img.shields.io/github/stars/GeiserX/duplicacy-mcp?style=flat-square&logo=github" alt="GitHub Stars"/></a>
  <a href="https://github.com/GeiserX/duplicacy-mcp/blob/main/LICENSE"><img src="https://img.shields.io/github/license/GeiserX/duplicacy-mcp?style=flat-square" alt="License"/></a>
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
docker run -d -p 127.0.0.1:8080:8080 -e DUPLICACY_EXPORTER_URL=http://duplicacy-exporter:9750 drumsergio/duplicacy-mcp:0.1.0
```

Or run it over stdio with `npx duplicacy-mcp`.

## Documentation

- [Installation](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/installation.md): Docker Compose, npm, local build
- [Configuration](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/configuration.md): environment variables
- [Usage](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/usage.md): resources and tools, example client configuration
- [Development](https://github.com/GeiserX/duplicacy-mcp/blob/main/docs/development.md): testing with Inspector, credits, contributing

Listed on the [Official MCP Registry](https://registry.modelcontextprotocol.io), [Glama](https://glama.ai/mcp/servers/GeiserX/duplicacy-mcp), [MCPServers.org](https://mcpservers.org/servers/geiserx/duplicacy-mcp), [mcp.so](https://mcp.so/server/duplicacy-mcp), [ToolSDK Registry](https://github.com/toolsdk-ai/toolsdk-mcp-registry) and [awesome-mcp-servers](https://github.com/punkpeye/awesome-mcp-servers#readme).

## Related projects

| Project | Description |
|---------|-------------|
| [duplicacy-cli-cron](https://github.com/GeiserX/duplicacy-cli-cron) | Docker-based encrypted dual-storage backup automation using Duplicacy CLI |
| [duplicacy-exporter](https://github.com/GeiserX/duplicacy-exporter) | Real-time Prometheus exporter for Duplicacy backups |
| [duplicacy-ha](https://github.com/GeiserX/duplicacy-ha) | Home Assistant custom integration for monitoring Duplicacy backups |
| [duplicacy-container](https://github.com/GeiserX/duplicacy-container) | Container image and Helm chart for running Duplicacy on Kubernetes |

Other MCP servers by GeiserX:

- [cashpilot-mcp](https://github.com/GeiserX/cashpilot-mcp) — Passive income monitoring
- [genieacs-mcp](https://github.com/GeiserX/genieacs-mcp) — TR-069 device management
- [lynxprompt-mcp](https://github.com/GeiserX/lynxprompt-mcp) — AI configuration blueprints
- [pumperly-mcp](https://github.com/GeiserX/pumperly-mcp) — Fuel and EV charging prices
- [telegram-archive-mcp](https://github.com/GeiserX/telegram-archive-mcp) — Telegram message archive

## License

GPL-3.0, see [LICENSE](https://github.com/GeiserX/duplicacy-mcp/blob/main/LICENSE).
