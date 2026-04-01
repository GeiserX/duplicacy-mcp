package resources

import (
	"context"

	"github.com/geiserx/duplicacy-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterHealth wires duplicacy://health into the server.
func RegisterHealth(s *server.MCPServer, c *client.Client) {
	res := mcp.NewResource(
		"duplicacy://health",
		"Exporter health check",
		mcp.WithResourceDescription("Health status of the duplicacy-exporter service"),
		mcp.WithMIMEType("application/json"),
	)

	s.AddResource(res, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		body, err := c.CheckHealth(ctx)
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "duplicacy://health",
				MIMEType: "application/json",
				Text:     string(body),
			},
		}, nil
	})
}
