# Usage

## What you get

| Type          | What for                                                   | MCP URI / Tool id                |
|---------------|------------------------------------------------------------|----------------------------------|
| **Resources** | Browse backup status, progress, and health read-only       | `duplicacy://status`<br>`duplicacy://progress`<br>`duplicacy://health` |
| **Tools**     | Query backup history, list snapshots, and check prune status | `get_backup_status`<br>`get_backup_history`<br>`list_snapshots`<br>`get_prune_status` |

Everything is exposed over a single JSON-RPC endpoint (`/mcp`).
LLMs / Agents can: `initialize` -> `readResource` -> `listTools` -> `callTool` ... and so on.

## Example configuration for client LLMs

```json
{
  "schema_version": "v1",
  "name_for_human": "Duplicacy-MCP",
  "name_for_model": "duplicacy_mcp",
  "description_for_human": "Monitor Duplicacy backup status, progress, and health via Prometheus metrics.",
  "description_for_model": "Interact with a Duplicacy backup monitoring server that reads metrics from a Prometheus exporter. First call initialize, then reuse the returned session id in header \"Mcp-Session-Id\" for every other call. Use readResource to fetch URIs that begin with duplicacy://. Use listTools to discover available actions and callTool to execute them.",
  "auth": { "type": "none" },
  "api": {
    "type": "jsonrpc-mcp",
    "url":  "http://localhost:8080/mcp",
    "init_method": "initialize",
    "session_header": "Mcp-Session-Id"
  },
  "contact_email": "acsdesk@protonmail.com",
  "legal_info_url": "https://github.com/GeiserX/duplicacy-mcp/blob/main/LICENSE"
}
```

