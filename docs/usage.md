# Usage

## What you get

| Type          | What for                                                   | MCP URI / Tool id                |
|---------------|------------------------------------------------------------|----------------------------------|
| **Resources** | Browse backup status, progress, and health read-only       | `duplicacy://status`<br>`duplicacy://progress`<br>`duplicacy://health` |
| **Tools**     | Query backup history, list snapshots, and check prune status | `get_backup_status`<br>`get_backup_history`<br>`list_snapshots`<br>`get_prune_status` |

Everything is exposed over a single JSON-RPC endpoint (`/mcp`).
LLMs / Agents can: `initialize` -> `readResource` -> `listTools` -> `callTool` ... and so on.

The client config for stdio and HTTP is on [Configuration](configuration.md#client-configuration).
