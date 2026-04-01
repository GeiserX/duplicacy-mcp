// internal/tools/prune_status.go
package tools

import (
	"context"
	"fmt"

	"github.com/geiserx/duplicacy-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewPruneStatus builds the Tool definition plus its handler.
func NewPruneStatus(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_prune_status",
		mcp.WithDescription("Get the prune operation status, optionally filtered by storage target"),
		mcp.WithString("storage_target",
			mcp.Description("(optional) Filter prune status to a specific storage target"),
		),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		storageTarget, _ := req.GetArguments()["storage_target"].(string)

		resp, err := c.GetPruneStatus(storageTarget)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		msg := "Prune status (all targets)"
		if storageTarget != "" {
			msg = fmt.Sprintf("Prune status for target '%s'", storageTarget)
		}

		return mcp.NewToolResultText(
			fmt.Sprintf("%s: %s", msg, string(resp)),
		), nil
	}

	return tool, handler
}
