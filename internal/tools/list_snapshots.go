// internal/tools/list_snapshots.go
package tools

import (
	"context"
	"fmt"

	"github.com/geiserx/duplicacy-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewListSnapshots builds the Tool definition plus its handler.
func NewListSnapshots(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("list_snapshots",
		mcp.WithDescription("List all unique Duplicacy snapshot IDs known to the exporter"),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		resp, err := c.ListSnapshots(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(
			fmt.Sprintf("Snapshots: %s", string(resp)),
		), nil
	}

	return tool, handler
}
