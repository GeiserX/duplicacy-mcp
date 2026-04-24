package resources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/geiserx/duplicacy-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const sampleMetrics = `# HELP duplicacy_backup_running Whether a backup is currently running
# TYPE duplicacy_backup_running gauge
duplicacy_backup_running{snapshot_id="photos",storage_target="b2",machine="nas"} 1
duplicacy_backup_last_exit_code{snapshot_id="photos",storage_target="b2",machine="nas"} 0
duplicacy_backup_last_duration_seconds{snapshot_id="photos",storage_target="b2",machine="nas"} 345.67
duplicacy_backup_speed_bytes_per_second{snapshot_id="photos",storage_target="b2",machine="nas"} 5242880
duplicacy_backup_progress_ratio{snapshot_id="photos",storage_target="b2",machine="nas"} 0.75
`

func newOKServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/metrics":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(sampleMetrics))
		case "/healthz":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"healthy"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func newFailServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
}

// initAndReadResource sends an initialize request followed by a resources/read
// request through the MCP server's HandleMessage, returning the raw response.
func initAndReadResource(s *server.MCPServer, uri string) mcp.JSONRPCMessage {
	// The server must be initialized before handling resources/read.
	initMsg, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":   map[string]any{},
			"clientInfo":     map[string]any{"name": "test", "version": "0.0.0"},
		},
	})
	s.HandleMessage(context.Background(), initMsg)

	readMsg, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "resources/read",
		"params":  map[string]any{"uri": uri},
	})
	return s.HandleMessage(context.Background(), readMsg)
}

// --- RegisterStatus ---

func TestRegisterStatus_handler_returns_status_json(t *testing.T) {
	srv := newOKServer()
	defer srv.Close()

	c := client.New(srv.URL)
	s := server.NewMCPServer("test", "0.0.0", server.WithResourceCapabilities(true, true))
	RegisterStatus(s, c)

	resp := initAndReadResource(s, "duplicacy://status")
	raw, _ := json.Marshal(resp)
	respStr := string(raw)

	if respStr == "" {
		t.Fatal("empty response")
	}

	// Parse the JSON-RPC response
	var rpcResp map[string]any
	if err := json.Unmarshal(raw, &rpcResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Should not have an error
	if _, hasErr := rpcResp["error"]; hasErr {
		t.Fatalf("unexpected error in response: %v", rpcResp["error"])
	}

	// Should have a result with contents
	result, ok := rpcResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result map, got %T", rpcResp["result"])
	}
	contents, ok := result["contents"].([]any)
	if !ok {
		t.Fatalf("expected contents array, got %T", result["contents"])
	}
	if len(contents) == 0 {
		t.Fatal("expected at least one content entry")
	}

	first := contents[0].(map[string]any)
	if first["uri"] != "duplicacy://status" {
		t.Errorf("uri = %v, want duplicacy://status", first["uri"])
	}
	text, ok := first["text"].(string)
	if !ok || text == "" {
		t.Error("expected non-empty text content")
	}
}

func TestRegisterStatus_handler_returns_error_when_fetch_fails(t *testing.T) {
	srv := newFailServer()
	defer srv.Close()

	c := client.New(srv.URL)
	s := server.NewMCPServer("test", "0.0.0", server.WithResourceCapabilities(true, true))
	RegisterStatus(s, c)

	resp := initAndReadResource(s, "duplicacy://status")
	raw, _ := json.Marshal(resp)

	var rpcResp map[string]any
	json.Unmarshal(raw, &rpcResp)
	if _, hasErr := rpcResp["error"]; !hasErr {
		t.Error("expected error in response when fetch fails")
	}
}

// --- RegisterProgress ---

func TestRegisterProgress_handler_returns_progress_json(t *testing.T) {
	srv := newOKServer()
	defer srv.Close()

	c := client.New(srv.URL)
	s := server.NewMCPServer("test", "0.0.0", server.WithResourceCapabilities(true, true))
	RegisterProgress(s, c)

	resp := initAndReadResource(s, "duplicacy://progress")
	raw, _ := json.Marshal(resp)

	var rpcResp map[string]any
	if err := json.Unmarshal(raw, &rpcResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if _, hasErr := rpcResp["error"]; hasErr {
		t.Fatalf("unexpected error: %v", rpcResp["error"])
	}

	result := rpcResp["result"].(map[string]any)
	contents := result["contents"].([]any)
	if len(contents) == 0 {
		t.Fatal("expected at least one content entry")
	}
	first := contents[0].(map[string]any)
	if first["uri"] != "duplicacy://progress" {
		t.Errorf("uri = %v, want duplicacy://progress", first["uri"])
	}
}

func TestRegisterProgress_handler_returns_error_when_fetch_fails(t *testing.T) {
	srv := newFailServer()
	defer srv.Close()

	c := client.New(srv.URL)
	s := server.NewMCPServer("test", "0.0.0", server.WithResourceCapabilities(true, true))
	RegisterProgress(s, c)

	resp := initAndReadResource(s, "duplicacy://progress")
	raw, _ := json.Marshal(resp)

	var rpcResp map[string]any
	json.Unmarshal(raw, &rpcResp)
	if _, hasErr := rpcResp["error"]; !hasErr {
		t.Error("expected error in response when fetch fails")
	}
}

// --- RegisterHealth ---

func TestRegisterHealth_handler_returns_health_json(t *testing.T) {
	srv := newOKServer()
	defer srv.Close()

	c := client.New(srv.URL)
	s := server.NewMCPServer("test", "0.0.0", server.WithResourceCapabilities(true, true))
	RegisterHealth(s, c)

	resp := initAndReadResource(s, "duplicacy://health")
	raw, _ := json.Marshal(resp)

	var rpcResp map[string]any
	if err := json.Unmarshal(raw, &rpcResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if _, hasErr := rpcResp["error"]; hasErr {
		t.Fatalf("unexpected error: %v", rpcResp["error"])
	}

	result := rpcResp["result"].(map[string]any)
	contents := result["contents"].([]any)
	if len(contents) == 0 {
		t.Fatal("expected at least one content entry")
	}
	first := contents[0].(map[string]any)
	if first["uri"] != "duplicacy://health" {
		t.Errorf("uri = %v, want duplicacy://health", first["uri"])
	}
	text := first["text"].(string)
	if text == "" {
		t.Error("expected non-empty health text")
	}
}

func TestRegisterHealth_handler_returns_error_when_all_endpoints_fail(t *testing.T) {
	// Use a server that's completely unreachable to trigger the error path
	c := client.New("http://127.0.0.1:1")
	s := server.NewMCPServer("test", "0.0.0", server.WithResourceCapabilities(true, true))
	RegisterHealth(s, c)

	resp := initAndReadResource(s, "duplicacy://health")
	raw, _ := json.Marshal(resp)

	var rpcResp map[string]any
	json.Unmarshal(raw, &rpcResp)

	// CheckHealth does NOT return an error for unreachable hosts --
	// it returns a JSON body with status:"error". So the resource handler
	// should succeed and return that JSON.
	if _, hasErr := rpcResp["error"]; hasErr {
		t.Fatalf("did not expect JSON-RPC error for health check (it returns error status in body)")
	}

	result := rpcResp["result"].(map[string]any)
	contents := result["contents"].([]any)
	first := contents[0].(map[string]any)
	text := first["text"].(string)
	if text == "" {
		t.Error("expected non-empty health text with error status")
	}
}
