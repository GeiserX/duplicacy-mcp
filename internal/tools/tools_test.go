package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/geiserx/duplicacy-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// sampleMetrics is a realistic Prometheus text exposition payload.
const sampleMetrics = `# HELP duplicacy_backup_running Whether a backup is currently running
# TYPE duplicacy_backup_running gauge
duplicacy_backup_running{snapshot_id="photos",storage_target="b2",machine="nas"} 1
duplicacy_backup_running{snapshot_id="docs",storage_target="s3",machine="nas"} 0
duplicacy_backup_last_exit_code{snapshot_id="photos",storage_target="b2",machine="nas"} 0
duplicacy_backup_last_exit_code{snapshot_id="docs",storage_target="s3",machine="nas"} 1
duplicacy_backup_last_success_timestamp_seconds{snapshot_id="photos",storage_target="b2",machine="nas"} 1700000000
duplicacy_backup_last_duration_seconds{snapshot_id="photos",storage_target="b2",machine="nas"} 345.67
duplicacy_backup_last_files_total{snapshot_id="photos",storage_target="b2",machine="nas"} 5000
duplicacy_backup_last_files_new{snapshot_id="photos",storage_target="b2",machine="nas"} 50
duplicacy_backup_last_bytes_uploaded{snapshot_id="photos",storage_target="b2",machine="nas"} 1048576
duplicacy_backup_last_bytes_new{snapshot_id="photos",storage_target="b2",machine="nas"} 524288
duplicacy_backup_last_chunks_new{snapshot_id="photos",storage_target="b2",machine="nas"} 10
duplicacy_backup_last_revision{snapshot_id="photos",storage_target="b2",machine="nas"} 42
duplicacy_backup_bytes_uploaded_total{snapshot_id="photos",storage_target="b2",machine="nas"} 99999999
duplicacy_prune_running{storage_target="b2",machine="nas"} 0
duplicacy_prune_running{storage_target="s3",machine="nas"} 1
duplicacy_prune_last_success_timestamp_seconds{storage_target="b2",machine="nas"} 1699999000
`

func newMetricsServer(body string, status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}

func makeCallToolRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// --- NewBackupStatus ---

func TestNewBackupStatus_returns_tool_and_handler(t *testing.T) {
	srv := newMetricsServer(sampleMetrics, http.StatusOK)
	defer srv.Close()

	c := client.New(srv.URL)
	tool, handler := NewBackupStatus(c)

	if tool.Name != "get_backup_status" {
		t.Errorf("tool name = %q, want get_backup_status", tool.Name)
	}
	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestNewBackupStatus_handler_returns_status_for_known_snapshot(t *testing.T) {
	srv := newMetricsServer(sampleMetrics, http.StatusOK)
	defer srv.Close()

	c := client.New(srv.URL)
	_, handler := NewBackupStatus(c)

	req := makeCallToolRequest(map[string]any{"snapshot_id": "photos"})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	// Result should contain the snapshot data as text
	text := extractText(t, result)
	if !strings.Contains(text, "photos") {
		t.Errorf("result should contain 'photos': %s", text)
	}
}

func TestNewBackupStatus_handler_returns_error_for_unknown_snapshot(t *testing.T) {
	srv := newMetricsServer(sampleMetrics, http.StatusOK)
	defer srv.Close()

	c := client.New(srv.URL)
	_, handler := NewBackupStatus(c)

	req := makeCallToolRequest(map[string]any{"snapshot_id": "nonexistent"})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for unknown snapshot")
	}
}

func TestNewBackupStatus_handler_returns_error_when_missing_snapshot_id(t *testing.T) {
	srv := newMetricsServer(sampleMetrics, http.StatusOK)
	defer srv.Close()

	c := client.New(srv.URL)
	_, handler := NewBackupStatus(c)

	req := makeCallToolRequest(map[string]any{})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for missing snapshot_id")
	}
}

func TestNewBackupStatus_handler_returns_error_when_fetch_fails(t *testing.T) {
	c := client.New("http://127.0.0.1:1")
	_, handler := NewBackupStatus(c)

	req := makeCallToolRequest(map[string]any{"snapshot_id": "photos"})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for connection failure")
	}
}

// --- NewBackupHistory ---

func TestNewBackupHistory_returns_tool_and_handler(t *testing.T) {
	srv := newMetricsServer(sampleMetrics, http.StatusOK)
	defer srv.Close()

	c := client.New(srv.URL)
	tool, handler := NewBackupHistory(c)

	if tool.Name != "get_backup_history" {
		t.Errorf("tool name = %q, want get_backup_history", tool.Name)
	}
	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestNewBackupHistory_handler_returns_history_for_known_snapshot(t *testing.T) {
	srv := newMetricsServer(sampleMetrics, http.StatusOK)
	defer srv.Close()

	c := client.New(srv.URL)
	_, handler := NewBackupHistory(c)

	req := makeCallToolRequest(map[string]any{"snapshot_id": "photos"})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	text := extractText(t, result)
	if !strings.Contains(text, "photos") {
		t.Errorf("result should contain 'photos': %s", text)
	}
}

func TestNewBackupHistory_handler_returns_error_for_unknown_snapshot(t *testing.T) {
	srv := newMetricsServer(sampleMetrics, http.StatusOK)
	defer srv.Close()

	c := client.New(srv.URL)
	_, handler := NewBackupHistory(c)

	req := makeCallToolRequest(map[string]any{"snapshot_id": "nonexistent"})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for unknown snapshot")
	}
}

func TestNewBackupHistory_handler_returns_error_when_missing_snapshot_id(t *testing.T) {
	srv := newMetricsServer(sampleMetrics, http.StatusOK)
	defer srv.Close()

	c := client.New(srv.URL)
	_, handler := NewBackupHistory(c)

	req := makeCallToolRequest(map[string]any{})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for missing snapshot_id")
	}
}

func TestNewBackupHistory_handler_returns_error_when_fetch_fails(t *testing.T) {
	c := client.New("http://127.0.0.1:1")
	_, handler := NewBackupHistory(c)

	req := makeCallToolRequest(map[string]any{"snapshot_id": "photos"})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for connection failure")
	}
}

// --- NewListSnapshots ---

func TestNewListSnapshots_returns_tool_and_handler(t *testing.T) {
	srv := newMetricsServer(sampleMetrics, http.StatusOK)
	defer srv.Close()

	c := client.New(srv.URL)
	tool, handler := NewListSnapshots(c)

	if tool.Name != "list_snapshots" {
		t.Errorf("tool name = %q, want list_snapshots", tool.Name)
	}
	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestNewListSnapshots_handler_returns_snapshot_list(t *testing.T) {
	srv := newMetricsServer(sampleMetrics, http.StatusOK)
	defer srv.Close()

	c := client.New(srv.URL)
	_, handler := NewListSnapshots(c)

	req := makeCallToolRequest(map[string]any{})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	text := extractText(t, result)
	if !strings.Contains(text, "photos") {
		t.Errorf("result should contain 'photos': %s", text)
	}
	if !strings.Contains(text, "docs") {
		t.Errorf("result should contain 'docs': %s", text)
	}
}

func TestNewListSnapshots_handler_returns_error_when_fetch_fails(t *testing.T) {
	c := client.New("http://127.0.0.1:1")
	_, handler := NewListSnapshots(c)

	req := makeCallToolRequest(map[string]any{})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for connection failure")
	}
}

// --- NewPruneStatus ---

func TestNewPruneStatus_returns_tool_and_handler(t *testing.T) {
	srv := newMetricsServer(sampleMetrics, http.StatusOK)
	defer srv.Close()

	c := client.New(srv.URL)
	tool, handler := NewPruneStatus(c)

	if tool.Name != "get_prune_status" {
		t.Errorf("tool name = %q, want get_prune_status", tool.Name)
	}
	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestNewPruneStatus_handler_returns_all_prune_statuses(t *testing.T) {
	srv := newMetricsServer(sampleMetrics, http.StatusOK)
	defer srv.Close()

	c := client.New(srv.URL)
	_, handler := NewPruneStatus(c)

	req := makeCallToolRequest(map[string]any{})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	text := extractText(t, result)
	if !strings.Contains(text, "all targets") {
		t.Errorf("result should mention 'all targets': %s", text)
	}
}

func TestNewPruneStatus_handler_filters_by_storage_target(t *testing.T) {
	srv := newMetricsServer(sampleMetrics, http.StatusOK)
	defer srv.Close()

	c := client.New(srv.URL)
	_, handler := NewPruneStatus(c)

	req := makeCallToolRequest(map[string]any{"storage_target": "b2"})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	text := extractText(t, result)
	if !strings.Contains(text, "b2") {
		t.Errorf("result should mention 'b2': %s", text)
	}
}

func TestNewPruneStatus_handler_returns_error_when_fetch_fails(t *testing.T) {
	c := client.New("http://127.0.0.1:1")
	_, handler := NewPruneStatus(c)

	req := makeCallToolRequest(map[string]any{})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for connection failure")
	}
}

// --- helpers ---

func extractText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}
