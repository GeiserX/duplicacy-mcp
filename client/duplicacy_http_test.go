package client

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

// pollerMetrics adds the storage poller and exporter diagnostics series.
const pollerMetrics = `duplicacy_storage_total_size_bytes{storage_target="s3",machine="nas"} 2000
duplicacy_storage_total_chunks{storage_target="s3",machine="nas"} 20
duplicacy_storage_total_size_bytes{storage_target="b2",machine="nas"} 1000
duplicacy_storage_total_chunks{storage_target="b2",machine="nas"} 10
duplicacy_snapshot_revisions{snapshot_id="photos",storage_target="b2",machine="nas"} 7
duplicacy_snapshot_last_revision{snapshot_id="photos",storage_target="b2",machine="nas"} 42
duplicacy_poller_last_success_timestamp_seconds 1700000500
duplicacy_poller_errors_total 3
duplicacy_exporter_last_activity_timestamp_seconds 1700000600
duplicacy_exporter_backups_seen_total 12
`

func newClient(t *testing.T, body string) *Client {
	t.Helper()
	srv := newTestServer(body, http.StatusOK, `{"status":"healthy"}`, http.StatusOK)
	t.Cleanup(srv.Close)
	return New(srv.URL)
}

// --- New() ---

func TestNew_trims_trailing_slash(t *testing.T) {
	c := New("http://example.com/")
	if c.base != "http://example.com" {
		t.Errorf("base = %q, want http://example.com", c.base)
	}
}

// --- FetchMetrics ---

func TestFetchMetrics_requests_metrics_with_GET(t *testing.T) {
	var gotMethod, gotPath, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAccept = r.Method, r.URL.Path, r.Header.Get("Accept")
		w.Write([]byte(sampleMetrics))
	}))
	defer srv.Close()

	metrics, err := New(srv.URL).FetchMetrics(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/metrics" || gotAccept != "text/plain" {
		t.Errorf("request = %s %s Accept=%q, want GET /metrics Accept=text/plain", gotMethod, gotPath, gotAccept)
	}
	if len(metrics["duplicacy_backup_running"]) != 2 {
		t.Errorf("duplicacy_backup_running samples = %d, want 2", len(metrics["duplicacy_backup_running"]))
	}
}

func TestFetchMetrics_http_error_never_echoes_body(t *testing.T) {
	srv := newTestServer("secret upstream text", http.StatusBadGateway, "", http.StatusOK)
	defer srv.Close()

	_, err := New(srv.URL).FetchMetrics(context.Background())
	var ee *ExporterError
	if !errors.As(err, &ee) || ee.Kind != "http_status" || ee.Status != http.StatusBadGateway {
		t.Fatalf("err = %v, want ExporterError http_status 502", err)
	}
	if strings.Contains(err.Error(), "secret upstream text") {
		t.Errorf("error echoes the upstream body: %v", err)
	}
}

func TestFetchMetrics_connection_refused_hides_url(t *testing.T) {
	// Grab a free port, then close it so the connection is refused.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()

	_, err = New("http://" + addr).FetchMetrics(context.Background())
	var ee *ExporterError
	if !errors.As(err, &ee) || ee.Kind != "refused" {
		t.Fatalf("err = %v, want ExporterError refused", err)
	}
	if strings.Contains(err.Error(), addr) || strings.Contains(err.Error(), "127.0.0.1") {
		t.Errorf("error leaks the exporter address: %v", err)
	}
	if !strings.Contains(err.Error(), "DUPLICACY_EXPORTER_URL") {
		t.Errorf("error should point at DUPLICACY_EXPORTER_URL: %v", err)
	}
}

func TestFetchMetrics_cancelled_context_is_an_error(t *testing.T) {
	srv := newTestServer(sampleMetrics, http.StatusOK, "", http.StatusOK)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(srv.URL).FetchMetrics(ctx); err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestFetchMetrics_rejects_oversized_body(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chunk := []byte(strings.Repeat("#", 1<<20) + "\n")
		for i := 0; i <= maxMetricsBytes>>20; i++ {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	_, err := New(srv.URL).FetchMetrics(context.Background())
	var ee *ExporterError
	if !errors.As(err, &ee) || ee.Kind != "too_large" {
		t.Fatalf("err = %v, want ExporterError too_large", err)
	}
}

func TestParse_skips_non_finite_values(t *testing.T) {
	m := parsePrometheusText("a NaN\nb +Inf\nc -Inf\nd 1\n")
	if len(m) != 1 || len(m["d"]) != 1 {
		t.Errorf("parsed = %v, want only d", m)
	}
}

func TestParse_strips_control_characters_from_labels(t *testing.T) {
	m := parsePrometheusText("x{snapshot_id=\"ph\u202eotos\x07\"} 1\n")
	if got := m["x"][0].Labels["snapshot_id"]; got != "photos" {
		t.Errorf("label = %q, want photos", got)
	}
	long := strings.Repeat("a", 300)
	m = parsePrometheusText("x{snapshot_id=\"" + long + "\"} 1\n")
	if got := []rune(m["x"][0].Labels["snapshot_id"]); len(got) != maxLabelRunes+1 {
		t.Errorf("long label has %d runes, want %d (truncated plus ellipsis)", len(got), maxLabelRunes+1)
	}
}

// --- CheckHealth ---

func TestCheckHealth_probes_health_first(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Write([]byte("{\"status\":\"healthy\"}\n\x1b[31m"))
	}))
	defer srv.Close()

	got := New(srv.URL).CheckHealth(context.Background())
	if got.Status != "ok" || got.Endpoint != "/health" || got.Code != 200 {
		t.Errorf("report = %+v, want ok /health 200", got)
	}
	if got.Body != `{"status":"healthy"}[31m` {
		t.Errorf("body = %q, want control characters stripped", got.Body)
	}
	if len(paths) != 1 || paths[0] != "/health" {
		t.Errorf("probed %v, want only /health", paths)
	}
}

func TestCheckHealth_falls_back_to_metrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			w.Write([]byte("go_goroutines 42\n"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	got := New(srv.URL).CheckHealth(context.Background())
	if got.Status != "ok" || got.Endpoint != "/metrics" {
		t.Errorf("report = %+v, want ok /metrics", got)
	}
}

func TestCheckHealth_degraded_when_metrics_fails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if got := New(srv.URL).CheckHealth(context.Background()); got.Status != "degraded" || got.Code != 500 {
		t.Errorf("report = %+v, want degraded 500", got)
	}
}

func TestCheckHealth_unreachable(t *testing.T) {
	got := New("http://127.0.0.1:1").CheckHealth(context.Background())
	if got.Status != "unreachable" || got.Error == "" {
		t.Errorf("report = %+v, want unreachable with an error", got)
	}
	if strings.Contains(got.Error, "127.0.0.1") {
		t.Errorf("error leaks the exporter address: %q", got.Error)
	}
}

// --- GetStatus ---

func TestGetStatus_has_no_phantom_rows(t *testing.T) {
	// The fixture also carries prune series with no snapshot_id; they must
	// not show up as backups.
	rows, err := newClient(t, sampleMetrics+pollerMetrics).GetStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(rows), rows)
	}
	for _, r := range rows {
		if r.SnapshotID == "" {
			t.Errorf("phantom row with empty snapshot_id: %+v", r)
		}
	}
}

func TestGetStatus_is_sorted_and_complete(t *testing.T) {
	rows, err := newClient(t, sampleMetrics).GetStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].SnapshotID != "docs" || rows[1].SnapshotID != "photos" {
		t.Fatalf("order = %s, %s; want docs, photos", rows[0].SnapshotID, rows[1].SnapshotID)
	}
	p := rows[1]
	if !p.Running || p.ExitCode == nil || *p.ExitCode != 0 || p.LastSuccess != "2023-11-14T22:13:20Z" ||
		p.LastDuration != 345.67 || p.LastFiles != 5000 || p.LastBytesUp != 1048576 || p.LastRevision != 42 ||
		p.TotalBytesUp != 99999999 {
		t.Errorf("photos row = %+v", p)
	}
	d := rows[0]
	if d.Running || d.ExitCode == nil || *d.ExitCode != 1 || d.LastSuccess != "" {
		t.Errorf("docs row = %+v", d)
	}
}

func TestGetStatus_deterministic_across_calls(t *testing.T) {
	c := newClient(t, sampleMetrics+`duplicacy_backup_running{snapshot_id="a",storage_target="z",machine="m"} 0
duplicacy_backup_running{snapshot_id="a",storage_target="b",machine="m"} 0
duplicacy_backup_running{snapshot_id="a",storage_target="b",machine="a"} 0
`)
	first, _ := c.GetStatus(context.Background())
	firstJSON, _ := json.Marshal(first)
	for i := 0; i < 20; i++ {
		again, _ := c.GetStatus(context.Background())
		againJSON, _ := json.Marshal(again)
		if string(againJSON) != string(firstJSON) {
			t.Fatalf("call %d differs:\n%s\n%s", i, firstJSON, againJSON)
		}
	}
	if first[0].StorageTarget != "b" || first[0].Machine != "a" || first[2].StorageTarget != "z" {
		t.Errorf("rows not sorted by snapshot, target, machine: %+v", first[:3])
	}
}

func TestGetStatus_missing_exit_code_is_null_not_success(t *testing.T) {
	rows, err := newClient(t, `duplicacy_backup_running{snapshot_id="x",storage_target="b2",machine="nas"} 0
`).GetStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].ExitCode != nil {
		t.Errorf("exit code = %d, want nil", *rows[0].ExitCode)
	}
	b, _ := json.Marshal(rows[0])
	if !strings.Contains(string(b), `"exit_code":null`) {
		t.Errorf("JSON should carry exit_code null: %s", b)
	}
}

func TestGetStatus_includes_size_metrics(t *testing.T) {
	rows, _ := newClient(t, `duplicacy_backup_last_files_size_bytes{snapshot_id="x",storage_target="b2",machine="nas"} 123
duplicacy_backup_last_chunks_size_bytes{snapshot_id="x",storage_target="b2",machine="nas"} 45
`).GetStatus(context.Background())
	if len(rows) != 1 || rows[0].LastFilesSize != 123 || rows[0].LastChunksSize != 45 {
		t.Errorf("rows = %+v", rows)
	}
}

func TestGetStatus_empty_metrics(t *testing.T) {
	rows, err := newClient(t, "").GetStatus(context.Background())
	if err != nil || rows == nil || len(rows) != 0 {
		t.Errorf("rows = %#v, err = %v; want empty non-nil slice", rows, err)
	}
}

func TestGetStatus_fetch_error(t *testing.T) {
	if _, err := New("http://127.0.0.1:1").GetStatus(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

// --- GetSnapshotStatus / History ---

func TestGetSnapshotStatus_filters(t *testing.T) {
	body := sampleMetrics + `duplicacy_backup_running{snapshot_id="photos",storage_target="s3",machine="nas"} 0
`
	c := newClient(t, body)
	rows, err := c.GetSnapshotStatus(context.Background(), Filter{SnapshotID: "photos"})
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows = %+v, err = %v; want 2", rows, err)
	}
	rows, err = c.GetSnapshotStatus(context.Background(), Filter{SnapshotID: "photos", StorageTarget: "s3"})
	if err != nil || len(rows) != 1 || rows[0].StorageTarget != "s3" {
		t.Fatalf("rows = %+v, err = %v; want the s3 row", rows, err)
	}
}

func TestGetSnapshotStatus_not_found_lists_known_values(t *testing.T) {
	c := newClient(t, sampleMetrics)
	cases := []struct {
		f     Filter
		field string
		known []string
	}{
		{Filter{SnapshotID: "nope"}, "snapshot_id", []string{"docs", "photos"}},
		{Filter{SnapshotID: "photos", StorageTarget: "nope"}, "storage_target", []string{"b2"}},
		{Filter{SnapshotID: "photos", Machine: "nope"}, "machine", []string{"nas"}},
	}
	for _, tc := range cases {
		_, err := c.GetSnapshotStatus(context.Background(), tc.f)
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("%+v: err = %v, want NotFoundError", tc.f, err)
		}
		if nf.Field != tc.field || strings.Join(nf.Known, ",") != strings.Join(tc.known, ",") {
			t.Errorf("%+v: got %s %v, want %s %v", tc.f, nf.Field, nf.Known, tc.field, tc.known)
		}
	}
}

func TestGetSnapshotHistory_last_run(t *testing.T) {
	rows, err := newClient(t, sampleMetrics).GetSnapshotHistory(context.Background(), Filter{SnapshotID: "docs"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	if rows[0].ExitCode == nil || *rows[0].ExitCode != 1 || rows[0].LastDuration != 120.5 {
		t.Errorf("docs history = %+v", rows[0])
	}
	b, _ := json.Marshal(rows[0])
	if !strings.Contains(string(b), `"last_files_total":0`) {
		t.Errorf("history fields should not be omitted when zero: %s", b)
	}
}

func TestGetSnapshotHistory_not_found(t *testing.T) {
	_, err := newClient(t, sampleMetrics).GetSnapshotHistory(context.Background(), Filter{SnapshotID: "nope"})
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err = %v, want NotFoundError", err)
	}
}

// --- GetProgress ---

func TestGetProgress(t *testing.T) {
	rows, err := newClient(t, sampleMetrics).GetProgress(context.Background())
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	p := rows[1]
	if p.SnapshotID != "photos" || !p.Running || p.ProgressRatio != 0.75 || p.ProgressPercent != 75 ||
		p.SpeedBPS != 5242880 || p.ChunksUploaded != 300 || p.ChunksSkipped != 100 {
		t.Errorf("photos progress = %+v", p)
	}
}

// --- ListSnapshots ---

func TestListSnapshots(t *testing.T) {
	list, err := newClient(t, sampleMetrics+pollerMetrics).ListSnapshots(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Join(list.SnapshotIDs, ",") != "docs,photos" || list.Count != 2 {
		t.Errorf("ids = %v count = %d", list.SnapshotIDs, list.Count)
	}
	if strings.Join(list.StorageTargets, ",") != "b2,s3" || strings.Join(list.Machines, ",") != "nas" {
		t.Errorf("targets = %v machines = %v", list.StorageTargets, list.Machines)
	}
	if len(list.Backups) != 2 || list.Backups[0] != (BackupRef{"docs", "s3", "nas"}) {
		t.Errorf("backups = %+v", list.Backups)
	}
}

func TestListSnapshots_empty(t *testing.T) {
	list, err := newClient(t, "").ListSnapshots(context.Background())
	if err != nil || list.Count != 0 || list.SnapshotIDs == nil || list.Backups == nil {
		t.Errorf("list = %#v, err = %v; want empty non-nil slices", list, err)
	}
}

// --- GetPruneStatus ---

func TestGetPruneStatus(t *testing.T) {
	c := newClient(t, sampleMetrics)
	rows, err := c.GetPruneStatus(context.Background(), "")
	if err != nil || len(rows) != 2 || rows[0].StorageTarget != "b2" || rows[1].StorageTarget != "s3" {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	if rows[0].LastSuccess != "2023-11-14T21:56:40Z" || rows[0].Running || !rows[1].Running || rows[1].LastSuccess != "" {
		t.Errorf("rows = %+v", rows)
	}
	rows, err = c.GetPruneStatus(context.Background(), "s3")
	if err != nil || len(rows) != 1 || rows[0].StorageTarget != "s3" {
		t.Errorf("filtered rows = %+v, err = %v", rows, err)
	}
}

func TestGetPruneStatus_unknown_target_lists_known(t *testing.T) {
	_, err := newClient(t, sampleMetrics).GetPruneStatus(context.Background(), "nope")
	var nf *NotFoundError
	if !errors.As(err, &nf) || strings.Join(nf.Known, ",") != "b2,s3" {
		t.Fatalf("err = %v, want NotFoundError listing b2,s3", err)
	}
}

// --- GetStorageUsage ---

func TestGetStorageUsage(t *testing.T) {
	u, err := newClient(t, sampleMetrics+pollerMetrics).GetStorageUsage(context.Background(), Filter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(u.Storages) != 2 || u.Storages[0].StorageTarget != "b2" || u.Storages[0].TotalSizeBytes != 1000 ||
		u.Storages[1].TotalChunks != 20 {
		t.Errorf("storages = %+v", u.Storages)
	}
	if len(u.Snapshots) != 1 || u.Snapshots[0].Revisions != 7 || u.Snapshots[0].LastRevision != 42 {
		t.Errorf("snapshots = %+v", u.Snapshots)
	}
	if u.PollerLastSuccess != "2023-11-14T22:21:40Z" || u.PollerErrorsTotal != 3 || u.Note != "" {
		t.Errorf("usage = %+v", u)
	}
}

func TestGetStorageUsage_poller_off_has_note(t *testing.T) {
	u, err := newClient(t, sampleMetrics).GetStorageUsage(context.Background(), Filter{})
	if err != nil || u.Note == "" || u.Storages == nil || len(u.Storages) != 0 {
		t.Errorf("usage = %#v, err = %v", u, err)
	}
}

// --- GetOverview ---

func TestGetOverview_verdicts(t *testing.T) {
	now := time.Unix(1700000000, 0).Add(2 * time.Hour)
	body := sampleMetrics + pollerMetrics + `duplicacy_backup_running{snapshot_id="old",storage_target="b2",machine="nas"} 0
duplicacy_backup_last_exit_code{snapshot_id="old",storage_target="b2",machine="nas"} 0
duplicacy_backup_last_success_timestamp_seconds{snapshot_id="old",storage_target="b2",machine="nas"} 1600000000
duplicacy_backup_running{snapshot_id="new",storage_target="b2",machine="nas"} 0
duplicacy_backup_last_success_timestamp_seconds{snapshot_id="new",storage_target="b2",machine="nas"} 1700000000
duplicacy_backup_running{snapshot_id="unknown",storage_target="b2",machine="nas"} 0
`
	ov, err := newClient(t, body).GetOverview(context.Background(), OverviewOptions{MaxAge: 26 * time.Hour, Now: now})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{
		"docs":    VerdictFailed,  // exit code 1
		"new":     VerdictOK,      // 2 h old, no exit code reported
		"old":     VerdictStale,   // years old
		"photos":  VerdictRunning, // running
		"unknown": VerdictNever,   // no success recorded
	}
	if len(ov.Backups) != len(want) {
		t.Fatalf("got %d rows, want %d", len(ov.Backups), len(want))
	}
	for _, r := range ov.Backups {
		if r.Verdict != want[r.SnapshotID] {
			t.Errorf("%s: verdict %s (%s), want %s", r.SnapshotID, r.Verdict, r.Reason, want[r.SnapshotID])
		}
	}
	if ov.Status != "attention" || ov.Counts != (OverviewCounts{Total: 5, OK: 1, Running: 1, Stale: 1, Never: 1, Failed: 1}) {
		t.Errorf("status = %s counts = %+v", ov.Status, ov.Counts)
	}
	if ov.Exporter.LastActivity != "2023-11-14T22:23:20Z" || ov.Exporter.BackupsSeenTotal != 12 {
		t.Errorf("exporter = %+v", ov.Exporter)
	}
	for _, r := range ov.Backups {
		if r.SnapshotID == "new" && (r.AgeSeconds == nil || *r.AgeSeconds != 7200 || r.Age != "2h0m0s") {
			t.Errorf("new age = %v %q, want 7200 2h0m0s", r.AgeSeconds, r.Age)
		}
		if r.SnapshotID == "unknown" && r.AgeSeconds != nil {
			t.Errorf("unknown age = %d, want nil", *r.AgeSeconds)
		}
	}
}

func TestGetOverview_stale_boundary(t *testing.T) {
	body := `duplicacy_backup_last_exit_code{snapshot_id="a",storage_target="b2",machine="nas"} 0
duplicacy_backup_last_success_timestamp_seconds{snapshot_id="a",storage_target="b2",machine="nas"} 1700000000
`
	c := newClient(t, body)
	at := func(age time.Duration) string {
		ov, err := c.GetOverview(context.Background(), OverviewOptions{MaxAge: 26 * time.Hour, Now: time.Unix(1700000000, 0).Add(age)})
		if err != nil {
			t.Fatal(err)
		}
		return ov.Backups[0].Verdict
	}
	if v := at(26 * time.Hour); v != VerdictOK {
		t.Errorf("exactly max age: %s, want ok", v)
	}
	if v := at(26*time.Hour + time.Second); v != VerdictStale {
		t.Errorf("one second past max age: %s, want stale", v)
	}
}

func TestGetOverview_all_ok_and_no_data(t *testing.T) {
	body := `duplicacy_backup_last_exit_code{snapshot_id="a",storage_target="b2",machine="nas"} 0
duplicacy_backup_last_success_timestamp_seconds{snapshot_id="a",storage_target="b2",machine="nas"} 1700000000
`
	ov, _ := newClient(t, body).GetOverview(context.Background(), OverviewOptions{Now: time.Unix(1700000000, 0)})
	if ov.Status != "ok" || ov.MaxAgeHours != 26 {
		t.Errorf("status = %s max = %d, want ok 26", ov.Status, ov.MaxAgeHours)
	}
	ov, _ = newClient(t, "").GetOverview(context.Background(), OverviewOptions{})
	if ov.Status != "no_data" || ov.Backups == nil {
		t.Errorf("status = %s backups = %#v, want no_data and an empty slice", ov.Status, ov.Backups)
	}
}

func TestGetOverview_filter_not_found(t *testing.T) {
	_, err := newClient(t, sampleMetrics).GetOverview(context.Background(), OverviewOptions{Filter: Filter{Machine: "nope"}})
	var nf *NotFoundError
	if !errors.As(err, &nf) || nf.Field != "machine" {
		t.Fatalf("err = %v, want NotFoundError for machine", err)
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
			if got := findClosingBrace(tc.in); got != tc.want {
				t.Errorf("findClosingBrace(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
