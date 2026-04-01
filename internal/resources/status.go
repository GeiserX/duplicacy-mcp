package resources

import (
	"context"

	"github.com/geiserx/duplicacy-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterStatus wires duplicacy://status into the server.
func RegisterStatus(s *server.MCPServer, c *client.Client) {
	res := mcp.NewResource(
		"duplicacy://status",
		"All backup statuses",
		mcp.WithResourceDescription("Parsed Prometheus metrics showing all backup statuses (running/idle, last success, exit code per snapshot)"),
		mcp.WithMIMEType("application/json"),
	)

	s.AddResource(res, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		body, err := c.GetStatus()
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "duplicacy://status",
				MIMEType: "application/json",
				Text:     string(body),
			},
		}, nil
	})
}
