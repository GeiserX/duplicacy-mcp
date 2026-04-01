// internal/tools/backup_history.go
package tools

import (
	"context"
	"fmt"

	"github.com/geiserx/duplicacy-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewBackupHistory builds the Tool definition plus its handler.
func NewBackupHistory(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_backup_history",
		mcp.WithDescription("Get the last backup details for a specific Duplicacy snapshot (files, bytes, duration, exit code)"),
		mcp.WithString("snapshot_id",
			mcp.Required(),
			mcp.Description("The snapshot ID to query history for"),
		),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		snapshotID, err := req.RequireString("snapshot_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resp, err := c.GetSnapshotHistory(snapshotID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(
			fmt.Sprintf("Backup history for snapshot '%s': %s", snapshotID, string(resp)),
		), nil
	}

	return tool, handler
}
