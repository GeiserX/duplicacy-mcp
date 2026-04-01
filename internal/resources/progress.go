package resources

import (
	"context"

	"github.com/geiserx/duplicacy-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterProgress wires duplicacy://progress into the server.
func RegisterProgress(s *server.MCPServer, c *client.Client) {
	res := mcp.NewResource(
		"duplicacy://progress",
		"Active backup progress",
		mcp.WithResourceDescription("Real-time progress of any running backups (speed, progress %, chunks uploaded/skipped)"),
		mcp.WithMIMEType("application/json"),
	)

	s.AddResource(res, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		body, err := c.GetProgress(ctx)
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "duplicacy://progress",
				MIMEType: "application/json",
				Text:     string(body),
			},
		}, nil
	})
}
