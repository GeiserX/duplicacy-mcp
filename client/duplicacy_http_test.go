package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sampleMetrics is a realistic Prometheus text exposition payload used across tests.
const sampleMetrics = `# HELP duplicacy_backup_running Whether a backup is currently running
# TYPE duplicacy_backup_running gauge
duplicacy_backup_running{snapshot_id="photos",storage_target="b2",machine="nas"} 1
duplicacy_backup_running{snapshot_id="docs",storage_target="s3",machine="nas"} 0
# HELP duplicacy_backup_last_exit_code Exit code of the last backup
# TYPE duplicacy_backup_last_exit_code gauge
duplicacy_backup_last_exit_code{snapshot_id="photos",storage_target="b2",machine="nas"} 0
duplicacy_backup_last_exit_code{snapshot_id="docs",storage_target="s3",machine="nas"} 1
# HELP duplicacy_backup_last_success_timestamp_seconds Unix timestamp of last success
# TYPE duplicacy_backup_last_success_timestamp_seconds gauge
duplicacy_backup_last_success_timestamp_seconds{snapshot_id="photos",storage_target="b2",machine="nas"} 1700000000
duplicacy_backup_last_success_timestamp_seconds{snapshot_id="docs",storage_target="s3",machine="nas"} 0
# HELP duplicacy_backup_last_duration_seconds Duration of last backup
# TYPE duplicacy_backup_last_duration_seconds gauge
duplicacy_backup_last_duration_seconds{snapshot_id="photos",storage_target="b2",machine="nas"} 345.67
duplicacy_backup_last_duration_seconds{snapshot_id="docs",storage_target="s3",machine="nas"} 120.5
# HELP duplicacy_backup_last_files_total Total files in last backup
# TYPE duplicacy_backup_last_files_total gauge
duplicacy_backup_last_files_total{snapshot_id="photos",storage_target="b2",machine="nas"} 5000
# HELP duplicacy_backup_last_files_new New files in last backup
# TYPE duplicacy_backup_last_files_new gauge
duplicacy_backup_last_files_new{snapshot_id="photos",storage_target="b2",machine="nas"} 50
# HELP duplicacy_backup_last_bytes_uploaded Bytes uploaded in last backup
# TYPE duplicacy_backup_last_bytes_uploaded gauge
duplicacy_backup_last_bytes_uploaded{snapshot_id="photos",storage_target="b2",machine="nas"} 1048576
# HELP duplicacy_backup_last_bytes_new New bytes in last backup
# TYPE duplicacy_backup_last_bytes_new gauge
duplicacy_backup_last_bytes_new{snapshot_id="photos",storage_target="b2",machine="nas"} 524288
# HELP duplicacy_backup_last_chunks_new New chunks in last backup
# TYPE duplicacy_backup_last_chunks_new gauge
duplicacy_backup_last_chunks_new{snapshot_id="photos",storage_target="b2",machine="nas"} 10
# HELP duplicacy_backup_last_revision Last revision number
# TYPE duplicacy_backup_last_revision gauge
duplicacy_backup_last_revision{snapshot_id="photos",storage_target="b2",machine="nas"} 42
# HELP duplicacy_backup_bytes_uploaded_total Total bytes uploaded across all backups
# TYPE duplicacy_backup_bytes_uploaded_total counter
duplicacy_backup_bytes_uploaded_total{snapshot_id="photos",storage_target="b2",machine="nas"} 99999999
# HELP duplicacy_backup_speed_bytes_per_second Current upload speed
# TYPE duplicacy_backup_speed_bytes_per_second gauge
duplicacy_backup_speed_bytes_per_second{snapshot_id="photos",storage_target="b2",machine="nas"} 5242880
# HELP duplicacy_backup_progress_ratio Backup progress 0-1
# TYPE duplicacy_backup_progress_ratio gauge
duplicacy_backup_progress_ratio{snapshot_id="photos",storage_target="b2",machine="nas"} 0.75
# HELP duplicacy_backup_chunks_uploaded Chunks uploaded so far
# TYPE duplicacy_backup_chunks_uploaded gauge
duplicacy_backup_chunks_uploaded{snapshot_id="photos",storage_target="b2",machine="nas"} 300
# HELP duplicacy_backup_chunks_skipped Chunks skipped (already exist)
# TYPE duplicacy_backup_chunks_skipped gauge
duplicacy_backup_chunks_skipped{snapshot_id="photos",storage_target="b2",machine="nas"} 100
# HELP duplicacy_prune_running Whether a prune is running
# TYPE duplicacy_prune_running gauge
duplicacy_prune_running{storage_target="b2",machine="nas"} 0
duplicacy_prune_running{storage_target="s3",machine="nas"} 1
# HELP duplicacy_prune_last_success_timestamp_seconds Unix timestamp of last prune success
# TYPE duplicacy_prune_last_success_timestamp_seconds gauge
duplicacy_prune_last_success_timestamp_seconds{storage_target="b2",machine="nas"} 1699999000
duplicacy_prune_last_success_timestamp_seconds{storage_target="s3",machine="nas"} 0
`

// newTestServer returns an httptest.Server that serves the given body on /metrics
// and the given health body/status on /healthz.
func newTestServer(metricsBody string, metricsStatus int, healthBody string, healthStatus int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/metrics":
			w.WriteHeader(metricsStatus)
			w.Write([]byte(metricsBody))
		case "/healthz":
			w.WriteHeader(healthStatus)
			w.Write([]byte(healthBody))
		case "/health":
			w.WriteHeader(healthStatus)
			w.Write([]byte(healthBody))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// --- New() ---

func TestNew_trims_trailing_slash(t *testing.T) {
	c := New("http://example.com/")
	if c.base != "http://example.com" {
		t.Errorf("base = %q, want trailing slash trimmed", c.base)
	}
}

func TestNew_no_trailing_slash(t *testing.T) {
	c := New("http://example.com")
	if c.base != "http://example.com" {
		t.Errorf("base = %q, want %q", c.base, "http://example.com")
	}
}

// --- FetchMetrics ---

func TestFetchMetrics_returns_parsed_metrics(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	metrics, err := c.FetchMetrics(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := metrics["duplicacy_backup_running"]; !ok {
		t.Error("expected duplicacy_backup_running in metrics")
	}
	running := metrics["duplicacy_backup_running"]
	if len(running) != 2 {
		t.Errorf("duplicacy_backup_running samples: got %d, want 2", len(running))
	}
}

func TestFetchMetrics_returns_error_on_http_error(t *testing.T) {
	srv := newTestServer("internal error", http.StatusInternalServerError, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.FetchMetrics(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status code: %v", err)
	}
}

func TestFetchMetrics_returns_error_on_connection_failure(t *testing.T) {
	c := New("http://127.0.0.1:1") // nothing listening
	_, err := c.FetchMetrics(context.Background())
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestFetchMetrics_returns_error_on_cancelled_context(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.FetchMetrics(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// --- CheckHealth ---

func TestCheckHealth_returns_ok_on_healthz_success(t *testing.T) {
	srv := newTestServer("", http.StatusOK, `{"status":"healthy"}`, http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.CheckHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("status = %v, want ok", result["status"])
	}
	if result["endpoint"] != "/healthz" {
		t.Errorf("endpoint = %v, want /healthz", result["endpoint"])
	}
}

func TestCheckHealth_falls_back_to_metrics_when_health_endpoints_fail(t *testing.T) {
	// Server that fails health but serves metrics
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/health":
			w.WriteHeader(http.StatusNotFound)
		case "/metrics":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("go_goroutines 42\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.CheckHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("status = %v, want ok", result["status"])
	}
	if result["endpoint"] != "/metrics" {
		t.Errorf("endpoint = %v, want /metrics", result["endpoint"])
	}
}

func TestCheckHealth_returns_degraded_when_metrics_returns_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/health":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "/metrics":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.CheckHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if result["status"] != "degraded" {
		t.Errorf("status = %v, want degraded", result["status"])
	}
}

func TestCheckHealth_returns_error_when_all_endpoints_unreachable(t *testing.T) {
	c := New("http://127.0.0.1:1")
	body, err := c.CheckHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if result["status"] != "error" {
		t.Errorf("status = %v, want error", result["status"])
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error field in response")
	}
}

// --- GetStatus ---

func TestGetStatus_returns_all_backup_statuses(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.GetStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var statuses []BackupStatus
	if err := json.Unmarshal(body, &statuses); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(statuses) < 2 {
		t.Fatalf("expected at least 2 statuses, got %d", len(statuses))
	}

	// Find the photos snapshot
	var photos *BackupStatus
	for i := range statuses {
		if statuses[i].SnapshotID == "photos" {
			photos = &statuses[i]
			break
		}
	}
	if photos == nil {
		t.Fatal("photos snapshot not found in statuses")
	}
	if !photos.Running {
		t.Error("photos should be running")
	}
	if photos.ExitCode != 0 {
		t.Errorf("photos exit code = %f, want 0", photos.ExitCode)
	}
	if photos.LastDuration != 345.67 {
		t.Errorf("photos duration = %f, want 345.67", photos.LastDuration)
	}
	if photos.LastFiles != 5000 {
		t.Errorf("photos files = %f, want 5000", photos.LastFiles)
	}
	if photos.LastFilesNew != 50 {
		t.Errorf("photos files new = %f, want 50", photos.LastFilesNew)
	}
	if photos.LastBytesUp != 1048576 {
		t.Errorf("photos bytes up = %f, want 1048576", photos.LastBytesUp)
	}
	if photos.LastBytesNew != 524288 {
		t.Errorf("photos bytes new = %f, want 524288", photos.LastBytesNew)
	}
	if photos.LastChunksNew != 10 {
		t.Errorf("photos chunks new = %f, want 10", photos.LastChunksNew)
	}
	if photos.LastRevision != 42 {
		t.Errorf("photos revision = %f, want 42", photos.LastRevision)
	}
	if photos.TotalBytesUp != 99999999 {
		t.Errorf("photos total bytes = %f, want 99999999", photos.TotalBytesUp)
	}
	if photos.LastSuccess == "" {
		t.Error("photos should have a last success timestamp")
	}
}

func TestGetStatus_returns_error_when_fetch_fails(t *testing.T) {
	c := New("http://127.0.0.1:1")
	_, err := c.GetStatus(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetStatus_handles_empty_metrics(t *testing.T) {
	srv := newTestServer("", http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.GetStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var statuses []BackupStatus
	if err := json.Unmarshal(body, &statuses); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("expected 0 statuses, got %d", len(statuses))
	}
}

// --- GetProgress ---

func TestGetProgress_returns_progress_for_running_backups(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.GetProgress(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var progresses []BackupProgress
	if err := json.Unmarshal(body, &progresses); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	// Find the photos progress entry
	var photos *BackupProgress
	for i := range progresses {
		if progresses[i].SnapshotID == "photos" {
			photos = &progresses[i]
			break
		}
	}
	if photos == nil {
		t.Fatal("photos progress not found")
	}
	if !photos.Running {
		t.Error("photos should be running")
	}
	if photos.SpeedBPS != 5242880 {
		t.Errorf("speed = %f, want 5242880", photos.SpeedBPS)
	}
	if photos.ProgressRatio != 0.75 {
		t.Errorf("progress ratio = %f, want 0.75", photos.ProgressRatio)
	}
	if photos.ChunksUploaded != 300 {
		t.Errorf("chunks uploaded = %f, want 300", photos.ChunksUploaded)
	}
	if photos.ChunksSkipped != 100 {
		t.Errorf("chunks skipped = %f, want 100", photos.ChunksSkipped)
	}
}

func TestGetProgress_returns_error_when_fetch_fails(t *testing.T) {
	c := New("http://127.0.0.1:1")
	_, err := c.GetProgress(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetProgress_handles_empty_metrics(t *testing.T) {
	srv := newTestServer("", http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.GetProgress(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var progresses []BackupProgress
	if err := json.Unmarshal(body, &progresses); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(progresses) != 0 {
		t.Errorf("expected 0 progresses, got %d", len(progresses))
	}
}

// --- GetSnapshotStatus ---

func TestGetSnapshotStatus_returns_status_for_known_snapshot(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.GetSnapshotStatus(context.Background(), "photos")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var statuses []BackupStatus
	if err := json.Unmarshal(body, &statuses); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].SnapshotID != "photos" {
		t.Errorf("snapshot id = %q, want photos", statuses[0].SnapshotID)
	}
	if !statuses[0].Running {
		t.Error("photos should be running")
	}
	if statuses[0].ExitCode != 0 {
		t.Errorf("exit code = %f, want 0", statuses[0].ExitCode)
	}
	if statuses[0].LastDuration != 345.67 {
		t.Errorf("duration = %f, want 345.67", statuses[0].LastDuration)
	}
	if statuses[0].LastSuccess == "" {
		t.Error("expected last success timestamp")
	}
}

func TestGetSnapshotStatus_returns_not_found_for_unknown_snapshot(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.GetSnapshotStatus(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if result["error"] != "snapshot not found" {
		t.Errorf("error = %q, want 'snapshot not found'", result["error"])
	}
	if result["snapshot_id"] != "nonexistent" {
		t.Errorf("snapshot_id = %q, want nonexistent", result["snapshot_id"])
	}
}

func TestGetSnapshotStatus_returns_error_when_fetch_fails(t *testing.T) {
	c := New("http://127.0.0.1:1")
	_, err := c.GetSnapshotStatus(context.Background(), "photos")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetSnapshotStatus_docs_snapshot_has_zero_timestamp(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.GetSnapshotStatus(context.Background(), "docs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var statuses []BackupStatus
	if err := json.Unmarshal(body, &statuses); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	// timestamp 0 means no success yet, field should be empty
	if statuses[0].LastSuccess != "" {
		t.Errorf("expected empty last success for zero timestamp, got %q", statuses[0].LastSuccess)
	}
	if statuses[0].Running {
		t.Error("docs should not be running")
	}
	if statuses[0].ExitCode != 1 {
		t.Errorf("docs exit code = %f, want 1", statuses[0].ExitCode)
	}
}

// --- GetSnapshotHistory ---

func TestGetSnapshotHistory_returns_history_for_known_snapshot(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.GetSnapshotHistory(context.Background(), "photos")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var histories []json.RawMessage
	if err := json.Unmarshal(body, &histories); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(histories) != 1 {
		t.Fatalf("expected 1 history, got %d", len(histories))
	}

	var h map[string]any
	json.Unmarshal(histories[0], &h)

	if h["snapshot_id"] != "photos" {
		t.Errorf("snapshot_id = %v, want photos", h["snapshot_id"])
	}
	if h["last_duration_seconds"] != 345.67 {
		t.Errorf("duration = %v, want 345.67", h["last_duration_seconds"])
	}
	if h["last_files_total"] != float64(5000) {
		t.Errorf("files = %v, want 5000", h["last_files_total"])
	}
	if h["last_files_new"] != float64(50) {
		t.Errorf("files new = %v, want 50", h["last_files_new"])
	}
	if h["exit_code"] != float64(0) {
		t.Errorf("exit code = %v, want 0", h["exit_code"])
	}
	if h["last_revision"] != float64(42) {
		t.Errorf("revision = %v, want 42", h["last_revision"])
	}
	if h["total_bytes_uploaded"] != float64(99999999) {
		t.Errorf("total bytes = %v, want 99999999", h["total_bytes_uploaded"])
	}
}

func TestGetSnapshotHistory_returns_not_found_for_unknown_snapshot(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.GetSnapshotHistory(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if result["error"] != "snapshot not found" {
		t.Errorf("error = %q, want 'snapshot not found'", result["error"])
	}
}

func TestGetSnapshotHistory_returns_error_when_fetch_fails(t *testing.T) {
	c := New("http://127.0.0.1:1")
	_, err := c.GetSnapshotHistory(context.Background(), "photos")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- ListSnapshots ---

func TestListSnapshots_returns_all_unique_ids(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.ListSnapshots(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	count, ok := result["count"].(float64)
	if !ok {
		t.Fatal("count not found or not a number")
	}
	if count != 2 {
		t.Errorf("count = %f, want 2", count)
	}

	ids, ok := result["snapshot_ids"].([]any)
	if !ok {
		t.Fatal("snapshot_ids not found or not an array")
	}
	found := map[string]bool{}
	for _, id := range ids {
		found[id.(string)] = true
	}
	if !found["photos"] {
		t.Error("expected photos in snapshot ids")
	}
	if !found["docs"] {
		t.Error("expected docs in snapshot ids")
	}
}

func TestListSnapshots_returns_error_when_fetch_fails(t *testing.T) {
	c := New("http://127.0.0.1:1")
	_, err := c.ListSnapshots(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListSnapshots_handles_empty_metrics(t *testing.T) {
	srv := newTestServer("", http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.ListSnapshots(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if result["count"] != float64(0) {
		t.Errorf("count = %v, want 0", result["count"])
	}
}

// --- GetPruneStatus ---

func TestGetPruneStatus_returns_all_prune_statuses(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.GetPruneStatus(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var prunes []PruneStatus
	if err := json.Unmarshal(body, &prunes); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(prunes) != 2 {
		t.Fatalf("expected 2 prune statuses, got %d", len(prunes))
	}
}

func TestGetPruneStatus_filters_by_storage_target(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.GetPruneStatus(context.Background(), "b2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var prunes []PruneStatus
	if err := json.Unmarshal(body, &prunes); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(prunes) != 1 {
		t.Fatalf("expected 1 prune status for b2, got %d", len(prunes))
	}
	if prunes[0].StorageTarget != "b2" {
		t.Errorf("storage target = %q, want b2", prunes[0].StorageTarget)
	}
	if prunes[0].Running {
		t.Error("b2 prune should not be running")
	}
	if prunes[0].LastSuccess == "" {
		t.Error("b2 prune should have a last success timestamp")
	}
}

func TestGetPruneStatus_running_prune_with_no_success(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.GetPruneStatus(context.Background(), "s3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var prunes []PruneStatus
	if err := json.Unmarshal(body, &prunes); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(prunes) != 1 {
		t.Fatalf("expected 1 prune status for s3, got %d", len(prunes))
	}
	if !prunes[0].Running {
		t.Error("s3 prune should be running")
	}
	if prunes[0].LastSuccess != "" {
		t.Errorf("s3 prune last success should be empty, got %q", prunes[0].LastSuccess)
	}
}

func TestGetPruneStatus_returns_empty_for_unknown_target(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	body, err := c.GetPruneStatus(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var prunes []PruneStatus
	if err := json.Unmarshal(body, &prunes); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(prunes) != 0 {
		t.Errorf("expected 0 prune statuses, got %d", len(prunes))
	}
}

func TestGetPruneStatus_returns_error_when_fetch_fails(t *testing.T) {
	c := New("http://127.0.0.1:1")
	_, err := c.GetPruneStatus(context.Background(), "")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- findClosingBrace ---

func TestFindClosingBrace_simple(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"immediate close", "}", 0},
		{"with content", `{key="val"}`, 10},
		{"escaped quote inside", `{k="a\"b"}`, 9},
		{"brace in quoted value", `{k="a}b"}`, 8},
		{"no closing brace", `{k="val"`, -1},
		{"empty string", "", -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := findClosingBrace(tc.in)
			if got != tc.want {
				t.Errorf("findClosingBrace(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// --- FetchMetrics with 4xx error body ---

func TestFetchMetrics_includes_error_body_on_4xx(t *testing.T) {
	srv := newTestServer("bad request body", http.StatusBadRequest, "", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.FetchMetrics(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 400")
	}
	if !strings.Contains(err.Error(), "bad request body") {
		t.Errorf("error should include response body: %v", err)
	}
}
