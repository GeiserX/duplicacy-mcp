package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func call(t *testing.T, newTool Constructor, body string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	srv := newMetricsServer(body, http.StatusOK)
	t.Cleanup(srv.Close)
	_, handler := newTool(client.New(srv.URL))
	res, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}})
	if err != nil {
		t.Fatalf("handler returned a protocol error: %v", err)
	}
	return res
}

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

// structured checks that a successful result carries structuredContent and
// that the text block is the same data as JSON, for older clients.
func structured[T any](t *testing.T, res *mcp.CallToolResult) T {
	t.Helper()
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", extractText(t, res))
	}
	v, ok := res.StructuredContent.(T)
	if !ok {
		t.Fatalf("structuredContent is %T", res.StructuredContent)
	}
	var fromText T
	if err := json.Unmarshal([]byte(extractText(t, res)), &fromText); err != nil {
		t.Fatalf("text block is not the JSON of the result: %v", err)
	}
	return v
}

// --- metadata ---

func TestAllTools_have_title_annotations_and_output_schema(t *testing.T) {
	c := client.New("http://localhost:9750")
	var names []string
	for _, newTool := range All {
		tool, handler := newTool(c)
		names = append(names, tool.Name)
		if handler == nil {
			t.Errorf("%s: nil handler", tool.Name)
		}
		if tool.Title == "" || tool.Annotations.Title == "" {
			t.Errorf("%s: missing title", tool.Name)
		}
		a := tool.Annotations
		if a.ReadOnlyHint == nil || !*a.ReadOnlyHint {
			t.Errorf("%s: readOnlyHint must be true", tool.Name)
		}
		if a.DestructiveHint == nil || *a.DestructiveHint {
			t.Errorf("%s: destructiveHint must be false", tool.Name)
		}
		if a.IdempotentHint == nil || !*a.IdempotentHint {
			t.Errorf("%s: idempotentHint must be true", tool.Name)
		}
		if a.OpenWorldHint == nil || *a.OpenWorldHint {
			t.Errorf("%s: openWorldHint must be false", tool.Name)
		}
		if tool.OutputSchema.Type != "object" || len(tool.OutputSchema.Properties) == 0 {
			t.Errorf("%s: missing outputSchema", tool.Name)
		}
		if len(tool.Description) < 80 {
			t.Errorf("%s: description too thin: %q", tool.Name, tool.Description)
		}
		for prop, schema := range tool.InputSchema.Properties {
			m, _ := schema.(map[string]any)
			if d, _ := m["description"].(string); d == "" {
				t.Errorf("%s.%s: missing description", tool.Name, prop)
			}
		}
	}
	want := "get_backup_history,get_backup_overview,get_backup_status,get_prune_status,get_storage_usage,list_snapshots"
	if got := strings.Join(names, ","); got != want {
		t.Errorf("tools = %s, want %s (sorted)", got, want)
	}
}

func TestOverview_max_age_is_a_bounded_integer(t *testing.T) {
	tool, _ := NewBackupOverview(client.New("http://localhost:9750"))
	p, _ := tool.InputSchema.Properties["max_age_hours"].(map[string]any)
	if p["type"] != "integer" || p["minimum"] == nil || p["maximum"] == nil {
		t.Errorf("max_age_hours schema = %v, want a bounded integer", p)
	}
}

// --- handlers ---

func TestBackupStatus_returns_structured_rows(t *testing.T) {
	got := structured[BackupList](t, call(t, NewBackupStatus, sampleMetrics, map[string]any{"snapshot_id": "photos"}))
	if got.Count != 1 || got.Backups[0].SnapshotID != "photos" || got.Backups[0].LastFiles != 5000 {
		t.Errorf("result = %+v", got)
	}
}

func TestBackupStatus_unknown_snapshot_lists_known_ids(t *testing.T) {
	res := call(t, NewBackupStatus, sampleMetrics, map[string]any{"snapshot_id": "nonexistent"})
	if !res.IsError || !strings.Contains(extractText(t, res), "docs, photos") {
		t.Errorf("want an error listing docs, photos; got %v %q", res.IsError, extractText(t, res))
	}
}

func TestBackupStatus_exporter_down_is_a_tool_error(t *testing.T) {
	srv := newMetricsServer("boom", http.StatusInternalServerError)
	defer srv.Close()
	_, handler := NewBackupStatus(client.New(srv.URL))
	res, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"snapshot_id": "x"}}})
	if err != nil || !res.IsError {
		t.Fatalf("want a tool error, got err=%v res=%+v", err, res)
	}
	if text := extractText(t, res); strings.Contains(text, "boom") || strings.Contains(text, srv.URL) {
		t.Errorf("error leaks upstream body or URL: %q", text)
	}
}

func TestBackupHistory_returns_structured_rows(t *testing.T) {
	got := structured[LastRuns](t, call(t, NewBackupHistory, sampleMetrics, map[string]any{"snapshot_id": "docs"}))
	if got.Count != 1 || got.Backups[0].ExitCode == nil || *got.Backups[0].ExitCode != 1 {
		t.Errorf("result = %+v", got)
	}
}

func TestListSnapshots_returns_structured_list(t *testing.T) {
	got := structured[client.SnapshotList](t, call(t, NewListSnapshots, sampleMetrics, nil))
	if strings.Join(got.SnapshotIDs, ",") != "docs,photos" || strings.Join(got.StorageTargets, ",") != "b2,s3" {
		t.Errorf("result = %+v", got)
	}
}

func TestPruneStatus_filter_and_unknown_target(t *testing.T) {
	got := structured[PruneList](t, call(t, NewPruneStatus, sampleMetrics, map[string]any{"storage_target": "b2"}))
	if got.Count != 1 || got.Prunes[0].StorageTarget != "b2" || got.Prunes[0].LastSuccess == "" {
		t.Errorf("result = %+v", got)
	}
	res := call(t, NewPruneStatus, sampleMetrics, map[string]any{"storage_target": "nope"})
	if !res.IsError || !strings.Contains(extractText(t, res), "b2, s3") {
		t.Errorf("want an error listing b2, s3; got %q", extractText(t, res))
	}
}

func TestStorageUsage_poller_off(t *testing.T) {
	got := structured[client.StorageUsage](t, call(t, NewStorageUsage, sampleMetrics, nil))
	if got.Note == "" || len(got.Storages) != 0 {
		t.Errorf("result = %+v", got)
	}
}

func TestBackupOverview_uses_max_age(t *testing.T) {
	old := Now
	Now = func() time.Time { return time.Unix(1700000000, 0).Add(5 * time.Hour) }
	t.Cleanup(func() { Now = old })

	body := `duplicacy_backup_last_exit_code{snapshot_id="a",storage_target="b2",machine="nas"} 0
duplicacy_backup_last_success_timestamp_seconds{snapshot_id="a",storage_target="b2",machine="nas"} 1700000000
`
	got := structured[client.Overview](t, call(t, NewBackupOverview, body, nil))
	if got.Status != "ok" || got.Backups[0].Verdict != client.VerdictOK {
		t.Errorf("default max age: %+v", got)
	}
	got = structured[client.Overview](t, call(t, NewBackupOverview, body, map[string]any{"max_age_hours": float64(4)}))
	if got.Status != "attention" || got.Backups[0].Verdict != client.VerdictStale || got.MaxAgeHours != 4 {
		t.Errorf("max_age_hours 4: %+v", got)
	}
	res := call(t, NewBackupOverview, body, map[string]any{"max_age_hours": float64(0)})
	if !res.IsError {
		t.Error("max_age_hours 0 should be rejected")
	}
}
