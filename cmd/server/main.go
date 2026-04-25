package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/geiserx/duplicacy-mcp/client"
	"github.com/geiserx/duplicacy-mcp/config"
	"github.com/geiserx/duplicacy-mcp/internal/resources"
	"github.com/geiserx/duplicacy-mcp/internal/tools"
	"github.com/geiserx/duplicacy-mcp/version"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	log.Printf("Duplicacy MCP %s starting…", version.String())

	// Load config & initialise Duplicacy exporter client
	cfg := config.LoadDuplicacyConfig()
	dc := client.New(cfg.ExporterURL)

	// Create MCP server
	s := server.NewMCPServer(
		"Duplicacy MCP Bridge",
		version.Version,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithRecovery(),
	)

	//----------------------------------------------------
	// Resource: duplicacy://status
	//----------------------------------------------------
	resources.RegisterStatus(s, dc)

	//----------------------------------------------------
	// Resource: duplicacy://progress
	//----------------------------------------------------
	resources.RegisterProgress(s, dc)

	//----------------------------------------------------
	// Resource: duplicacy://health
	//----------------------------------------------------
	resources.RegisterHealth(s, dc)

	// -----------------------------------------------------------------
	// TOOL: get_backup_status
	// -----------------------------------------------------------------
	tool, handler := tools.NewBackupStatus(dc)
	s.AddTool(tool, handler)

	// -----------------------------------------------------------------
	// TOOL: get_backup_history
	// -----------------------------------------------------------------
	tool, handler = tools.NewBackupHistory(dc)
	s.AddTool(tool, handler)

	// -----------------------------------------------------------------
	// TOOL: list_snapshots
	// -----------------------------------------------------------------
	tool, handler = tools.NewListSnapshots(dc)
	s.AddTool(tool, handler)

	// -----------------------------------------------------------------
	// TOOL: get_prune_status
	// -----------------------------------------------------------------
	tool, handler = tools.NewPruneStatus(dc)
	s.AddTool(tool, handler)

	transport := strings.ToLower(os.Getenv("TRANSPORT"))
	if transport == "stdio" {
		stdioSrv := server.NewStdioServer(s)
		log.Println("Duplicacy MCP bridge running on stdio")
		if err := stdioSrv.Listen(context.Background(), os.Stdin, os.Stdout); err != nil {
			log.Fatalf("stdio server error: %v", err)
		}
	} else {
		httpSrv := server.NewStreamableHTTPServer(s)
		addr := os.Getenv("LISTEN_ADDR")
		if addr == "" {
			addr = "127.0.0.1:8080"
		}
		log.Printf("Duplicacy MCP bridge listening on %s", addr)
		if err := httpSrv.Start(addr); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}
}
