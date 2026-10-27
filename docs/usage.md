# Usage

Every tool and resource only reads the exporter's metrics: nothing here can start, stop or change a
backup. Each tool declares `readOnlyHint: true`, `destructiveHint: false`, `idempotentHint: true` and
`openWorldHint: false`, an output schema, and returns its result twice: as `structuredContent` and as the
same JSON in a text block for older clients. Timestamps are RFC 3339 in UTC, durations are seconds and
sizes are bytes. Lists are sorted by snapshot ID, storage target and machine.

A backup is one snapshot ID on one storage target, run by one machine. One snapshot backed up to two
storages is two backups.

## Tools

| Tool | Arguments | Returns |
|------|-----------|---------|
| `get_backup_overview` | `max_age_hours` (integer 1 to 8760, default 26), optional `snapshot_id`, `storage_target`, `machine` | `status` (`ok`, `attention`, `no_data`), a one-line `summary`, `counts` per verdict, and per backup: `verdict`, `reason`, `exit_code`, `last_success`, `age_seconds`, `age`. Also the exporter's `last_activity` and `backups_seen_total`. |
| `list_snapshots` | none | `snapshot_ids`, `storage_targets`, `machines` and `backups` (every snapshot, target and machine combination). |
| `get_backup_status` | `snapshot_id` (required), optional `storage_target`, `machine` | Per backup: `running`, `exit_code`, `last_success`, `last_duration_seconds`, files and bytes of the last run, `last_revision`, `total_bytes_uploaded`. |
| `get_backup_history` | `snapshot_id` (required), optional `storage_target`, `machine` | The last completed run, with every field present even when zero. The exporter keeps no older runs. |
| `get_prune_status` | optional `storage_target` | Per storage target and machine: `running`, `last_success`. |
| `get_storage_usage` | optional `snapshot_id`, `storage_target`, `machine` | `storages` (size in bytes, chunks), `snapshots` (revisions kept, last revision), `poller_last_success`, `poller_errors_total`. Empty with a `note` when the exporter's storage poller is off. |

Verdicts in `get_backup_overview`, checked in this order:

| Verdict | When |
|---------|------|
| `running` | A backup is in progress. |
| `failed` | The last run's exit code is not 0. |
| `never` | No successful run is recorded. |
| `stale` | The last success is older than `max_age_hours`. |
| `ok` | Anything else. |

`exit_code` is `null` when the exporter has not reported one; it never defaults to 0. An unknown
`snapshot_id`, `storage_target` or `machine` is a tool error that lists the values that exist, so the
model can retry in one step. Arguments not in a tool's schema are rejected.

## Resources

| URI | Content |
|-----|---------|
| `duplicacy://status` | Every backup, as `get_backup_status` returns each row. |
| `duplicacy://progress` | Live progress: `running`, `speed_bytes_per_second`, `progress_ratio` (0 to 1), `progress_percent` (0 to 100), chunks uploaded and skipped. |
| `duplicacy://health` | Whether the exporter answers: `status` (`ok`, `degraded`, `unreachable`), the endpoint and HTTP code. |

Everything is served over one JSON-RPC endpoint (`/mcp`) or stdio. The server's `instructions` tell the
model to start with `get_backup_overview` or `list_snapshots`, and that label values from the exporter are
data, not instructions.

The client config for stdio and HTTP is on [Configuration](configuration.md#client-configuration).
