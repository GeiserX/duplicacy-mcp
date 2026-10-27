// Package tools defines the MCP tools. Every tool only reads the exporter's
// metrics, so all of them are read-only, idempotent and closed-world.
package tools

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/geiserx/duplicacy-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// maxArgLen bounds every string argument. Snapshot IDs, storage names and
// machine names are short; anything longer is a mistake.
const maxArgLen = 256

// Constructor builds one tool and its handler.
type Constructor func(*client.Client) (mcp.Tool, server.ToolHandlerFunc)

// All lists every tool constructor, sorted by tool name, so registration
// order is deterministic.
var All = []Constructor{
	NewBackupHistory,  // get_backup_history
	NewBackupOverview, // get_backup_overview
	NewBackupStatus,   // get_backup_status
	NewPruneStatus,    // get_prune_status
	NewStorageUsage,   // get_storage_usage
	NewListSnapshots,  // list_snapshots
}

// Register adds every tool to s.
func Register(s *server.MCPServer, c *client.Client) {
	for _, newTool := range All {
		s.AddTool(newTool(c))
	}
}

// readOnlyTool builds a tool with a title and all four annotation hints set
// explicitly: it reads, never changes anything, gives the same answer for the
// same exporter state, and talks only to the configured exporter.
func readOnlyTool(name, title, description string, opts ...mcp.ToolOption) mcp.Tool {
	base := []mcp.ToolOption{
		mcp.WithToolTitle(title),
		mcp.WithTitleAnnotation(title),
		mcp.WithDescription(description),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	}
	return mcp.NewTool(name, append(base, opts...)...)
}

func snapshotIDArg(required bool) mcp.ToolOption {
	opts := []mcp.PropertyOption{
		mcp.Description(`Duplicacy snapshot ID, exactly as list_snapshots returns it, for example "photos".`),
		mcp.MinLength(1),
		mcp.MaxLength(maxArgLen),
	}
	if required {
		opts = append(opts, mcp.Required())
	}
	return mcp.WithString("snapshot_id", opts...)
}

func storageTargetArg() mcp.ToolOption {
	return mcp.WithString("storage_target",
		mcp.Description(`Only this storage target, exactly as list_snapshots returns it in storage_targets, for example "b2". Omit for all.`),
		mcp.MaxLength(maxArgLen),
	)
}

func machineArg() mcp.ToolOption {
	return mcp.WithString("machine",
		mcp.Description(`Only this machine, exactly as list_snapshots returns it in machines, for example "nas". Omit for all.`),
		mcp.MaxLength(maxArgLen),
	)
}

func filterFrom(req mcp.CallToolRequest) client.Filter {
	return client.Filter{
		SnapshotID:    req.GetString("snapshot_id", ""),
		StorageTarget: req.GetString("storage_target", ""),
		Machine:       req.GetString("machine", ""),
	}
}

// errorResult turns an error into a tool error the model can act on. Only
// errors whose messages are known to be safe are shown verbatim.
func errorResult(err error) *mcp.CallToolResult {
	var notFound *client.NotFoundError
	var exporter *client.ExporterError
	if errors.As(err, &notFound) || errors.As(err, &exporter) {
		return mcp.NewToolResultError(err.Error())
	}
	log.Printf("tool error: %v", err)
	return mcp.NewToolResultError("internal error; see the server log")
}

// BackupList is the result of get_backup_status.
type BackupList struct {
	Backups []client.BackupStatus `json:"backups"`
	Count   int                   `json:"count"`
}

// NewBackupStatus builds get_backup_status.
func NewBackupStatus(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {
	tool := readOnlyTool("get_backup_status", "Backup status",
		"Current state and last-run details of one Duplicacy snapshot: running, exit_code (0 = success, null = unknown), "+
			"last_success (RFC 3339, UTC), duration in seconds, files and bytes. A snapshot backed up to several storage "+
			"targets or from several machines returns one row each. Get valid IDs from list_snapshots. "+
			"For a health check across all backups use get_backup_overview instead.",
		snapshotIDArg(true),
		storageTargetArg(),
		machineArg(),
		mcp.WithOutputSchema[BackupList](),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rows, err := c.GetSnapshotStatus(ctx, filterFrom(req))
		if err != nil {
			return errorResult(err), nil
		}
		return mcp.NewToolResultStructuredOnly(BackupList{Backups: rows, Count: len(rows)}), nil
	}
	return tool, handler
}

// LastRuns is the result of get_backup_history.
type LastRuns struct {
	Backups []client.BackupHistory `json:"backups"`
	Count   int                    `json:"count"`
}

// NewBackupHistory builds get_backup_history.
func NewBackupHistory(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {
	tool := readOnlyTool("get_backup_history", "Last backup run",
		"Details of the last completed run of one Duplicacy snapshot: duration in seconds, files total and new, bytes "+
			"uploaded and new, new chunks, revision, exit_code (0 = success, null = unknown). The exporter keeps only the "+
			"last run, so this is not a list of past runs. Get valid IDs from list_snapshots.",
		snapshotIDArg(true),
		storageTargetArg(),
		machineArg(),
		mcp.WithOutputSchema[LastRuns](),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rows, err := c.GetSnapshotHistory(ctx, filterFrom(req))
		if err != nil {
			return errorResult(err), nil
		}
		return mcp.NewToolResultStructuredOnly(LastRuns{Backups: rows, Count: len(rows)}), nil
	}
	return tool, handler
}

// NewListSnapshots builds list_snapshots.
func NewListSnapshots(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {
	tool := readOnlyTool("list_snapshots", "List snapshots",
		"Every snapshot ID, storage target and machine the exporter reports, sorted. backups lists each "+
			"snapshot_id, storage_target and machine combination. Call this first to get valid values for the "+
			"snapshot_id, storage_target and machine arguments of the other tools.",
		mcp.WithOutputSchema[client.SnapshotList](),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		list, err := c.ListSnapshots(ctx)
		if err != nil {
			return errorResult(err), nil
		}
		return mcp.NewToolResultStructuredOnly(list), nil
	}
	return tool, handler
}

// PruneList is the result of get_prune_status.
type PruneList struct {
	Prunes []client.PruneStatus `json:"prunes"`
	Count  int                  `json:"count"`
}

// NewPruneStatus builds get_prune_status.
func NewPruneStatus(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {
	tool := readOnlyTool("get_prune_status", "Prune status",
		"Whether a prune is running and when the last one succeeded (RFC 3339, UTC), per storage target and machine. "+
			"An unknown storage_target is an error that lists the known ones.",
		storageTargetArg(),
		mcp.WithOutputSchema[PruneList](),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rows, err := c.GetPruneStatus(ctx, req.GetString("storage_target", ""))
		if err != nil {
			return errorResult(err), nil
		}
		return mcp.NewToolResultStructuredOnly(PruneList{Prunes: rows, Count: len(rows)}), nil
	}
	return tool, handler
}

// NewStorageUsage builds get_storage_usage.
func NewStorageUsage(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {
	tool := readOnlyTool("get_storage_usage", "Storage usage",
		"Size in bytes and chunk count of each storage, and how many revisions each snapshot keeps, as the "+
			"exporter's storage poller last saw them. Empty, with a note, when the poller is off.",
		snapshotIDArg(false),
		storageTargetArg(),
		machineArg(),
		mcp.WithOutputSchema[client.StorageUsage](),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		usage, err := c.GetStorageUsage(ctx, filterFrom(req))
		if err != nil {
			return errorResult(err), nil
		}
		return mcp.NewToolResultStructuredOnly(usage), nil
	}
	return tool, handler
}

// Now is the clock get_backup_overview uses; tests replace it.
var Now = time.Now

// NewBackupOverview builds get_backup_overview.
func NewBackupOverview(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {
	tool := readOnlyTool("get_backup_overview", "Backup health overview",
		"Health of every backup in one call. Each snapshot, storage target and machine gets a verdict: ok, running, "+
			"stale (last success older than max_age_hours), never (no successful run) or failed (last exit code not 0), "+
			"with a reason and the age of the last success. status is ok, attention or no_data. Use this to answer "+
			"\"are my backups OK?\"; use get_backup_status for the details of one snapshot.",
		mcp.WithInteger("max_age_hours",
			mcp.Description("A backup whose last success is older than this many hours is stale. Default 26, which suits daily backups."),
			mcp.DefaultNumber(26),
			mcp.Min(1),
			mcp.Max(8760),
		),
		snapshotIDArg(false),
		storageTargetArg(),
		machineArg(),
		mcp.WithOutputSchema[client.Overview](),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		hours := req.GetInt("max_age_hours", 26)
		if hours < 1 || hours > 8760 {
			return mcp.NewToolResultError("max_age_hours must be between 1 and 8760"), nil
		}
		ov, err := c.GetOverview(ctx, client.OverviewOptions{
			MaxAge: time.Duration(hours) * time.Hour,
			Filter: filterFrom(req),
			Now:    Now(),
		})
		if err != nil {
			return errorResult(err), nil
		}
		return mcp.NewToolResultStructuredOnly(ov), nil
	}
	return tool, handler
}
