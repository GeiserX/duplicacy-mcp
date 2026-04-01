// internal/tools/backup_status.go
package tools // same package for every tool file

import (
	"context"
	"fmt"

	"github.com/geiserx/duplicacy-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewBackupStatus builds the Tool definition plus its handler.
func NewBackupStatus(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_backup_status",
		mcp.WithDescription("Get the current backup status for a specific Duplicacy snapshot"),
		mcp.WithString("snapshot_id",
			mcp.Required(),
			mcp.Description("The snapshot ID to query status for"),
		),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		snapshotID, err := req.RequireString("snapshot_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resp, err := c.GetSnapshotStatus(snapshotID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(
			fmt.Sprintf("Backup status for snapshot '%s': %s", snapshotID, string(resp)),
		), nil
	}

	return tool, handler
}
