// Package resources defines the read-only MCP resources.
package resources

import (
	"context"
	"encoding/json"

	"github.com/geiserx/duplicacy-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// register adds one JSON resource whose body is produced by read.
func register(s *server.MCPServer, uri, name, description string, read func(context.Context) (any, error)) {
	res := mcp.NewResource(uri, name,
		mcp.WithResourceDescription(description),
		mcp.WithMIMEType("application/json"),
	)
	s.AddResource(res, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		v, err := read(ctx)
		if err != nil {
			return nil, err
		}
		body, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(body)},
		}, nil
	})
}

// RegisterStatus wires duplicacy://status into the server.
func RegisterStatus(s *server.MCPServer, c *client.Client) {
	register(s, "duplicacy://status", "All backup statuses",
		"Every backup the exporter reports, one entry per snapshot_id, storage_target and machine, sorted: running, "+
			"exit_code (0 = success, null = unknown), last success (RFC 3339, UTC) and last-run sizes.",
		func(ctx context.Context) (any, error) { return c.GetStatus(ctx) })
}

// RegisterProgress wires duplicacy://progress into the server.
func RegisterProgress(s *server.MCPServer, c *client.Client) {
	register(s, "duplicacy://progress", "Backup progress",
		"Live progress of every backup: running, speed in bytes per second, progress_ratio from 0 to 1, "+
			"progress_percent from 0 to 100, chunks uploaded and skipped.",
		func(ctx context.Context) (any, error) { return c.GetProgress(ctx) })
}

// RegisterHealth wires duplicacy://health into the server.
func RegisterHealth(s *server.MCPServer, c *client.Client) {
	register(s, "duplicacy://health", "Exporter health",
		"Whether the duplicacy-exporter answers: status ok, degraded or unreachable, with the endpoint and HTTP code.",
		func(ctx context.Context) (any, error) { return c.CheckHealth(ctx), nil })
}

// Register adds every resource to s, in URI order.
func Register(s *server.MCPServer, c *client.Client) {
	RegisterHealth(s, c)
	RegisterProgress(s, c)
	RegisterStatus(s, c)
}
