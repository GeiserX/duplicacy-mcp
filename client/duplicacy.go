package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// MetricSample represents a single Prometheus metric line with its labels and value.
type MetricSample struct {
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

// BackupStatus holds the structured status for a single snapshot.
type BackupStatus struct {
	SnapshotID    string  `json:"snapshot_id"`
	StorageTarget string  `json:"storage_target"`
	Machine       string  `json:"machine"`
	Running       bool    `json:"running"`
	ExitCode      float64 `json:"exit_code"`
	LastSuccess   string  `json:"last_success,omitempty"`
	LastDuration  float64 `json:"last_duration_seconds,omitempty"`
	LastFiles     float64 `json:"last_files_total,omitempty"`
	LastFilesNew  float64 `json:"last_files_new,omitempty"`
	LastBytesUp   float64 `json:"last_bytes_uploaded,omitempty"`
	LastBytesNew  float64 `json:"last_bytes_new,omitempty"`
	LastChunksNew float64 `json:"last_chunks_new,omitempty"`
	LastRevision  float64 `json:"last_revision,omitempty"`
	TotalBytesUp  float64 `json:"total_bytes_uploaded,omitempty"`
}

// BackupProgress holds real-time progress for a running backup.
type BackupProgress struct {
	SnapshotID     string  `json:"snapshot_id"`
	StorageTarget  string  `json:"storage_target"`
	Machine        string  `json:"machine"`
	Running        bool    `json:"running"`
	SpeedBPS       float64 `json:"speed_bytes_per_second"`
	ProgressRatio  float64 `json:"progress_ratio"`
	ChunksUploaded float64 `json:"chunks_uploaded"`
	ChunksSkipped  float64 `json:"chunks_skipped"`
}

// PruneStatus holds the structured status for pruning operations.
type PruneStatus struct {
	StorageTarget string `json:"storage_target"`
	Machine       string `json:"machine"`
	Running       bool   `json:"running"`
	LastSuccess   string `json:"last_success,omitempty"`
}

// Client talks to the duplicacy-exporter Prometheus metrics endpoint.
type Client struct {
	base string
	hc   *http.Client
}

// New creates a new Duplicacy exporter client.
func New(base string) *Client {
	return &Client{
		base: strings.TrimRight(base, "/"),
		hc:   &http.Client{Timeout: 10 * time.Second},
	}
}

// FetchMetrics retrieves and parses the /metrics endpoint into structured data.
func (c *Client) FetchMetrics() (map[string][]MetricSample, error) {
	resp, err := c.hc.Get(c.base + "/metrics")
	if err != nil {
		return nil, fmt.Errorf("fetch metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("exporter error %d: %s", resp.StatusCode, string(b))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read metrics body: %w", err)
	}

	return parsePrometheusText(string(body)), nil
}

// CheckHealth hits the exporter's health endpoint and returns the result.
func (c *Client) CheckHealth() ([]byte, error) {
	// Try /healthz first, fall back to /health
	for _, path := range []string{"/healthz", "/health"} {
		resp, err := c.hc.Get(c.base + path)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode < 400 {
			b, _ := io.ReadAll(resp.Body)
			result := map[string]any{
				"status":   "ok",
				"endpoint": path,
				"code":     resp.StatusCode,
				"body":     string(b),
			}
			return json.Marshal(result)
		}
	}

	// If neither health endpoint works, try /metrics as a liveness check
	resp, err := c.hc.Get(c.base + "/metrics")
	if err != nil {
		result := map[string]any{
			"status": "error",
			"error":  err.Error(),
		}
		return json.Marshal(result)
	}
	defer resp.Body.Close()

	status := "ok"
	if resp.StatusCode >= 400 {
		status = "degraded"
	}
	result := map[string]any{
		"status":   status,
		"endpoint": "/metrics",
		"code":     resp.StatusCode,
	}
	return json.Marshal(result)
}

// GetStatus parses metrics into a structured JSON of all backup statuses.
func (c *Client) GetStatus() ([]byte, error) {
	metrics, err := c.FetchMetrics()
	if err != nil {
		return nil, err
	}

	// Build status map keyed by snapshot_id+storage_target+machine
	type statusKey struct {
		SnapshotID    string
		StorageTarget string
		Machine       string
	}
	statusMap := make(map[statusKey]*BackupStatus)

	getOrCreate := func(s MetricSample) *BackupStatus {
		k := statusKey{
			SnapshotID:    s.Labels["snapshot_id"],
			StorageTarget: s.Labels["storage_target"],
			Machine:       s.Labels["machine"],
		}
		if st, ok := statusMap[k]; ok {
			return st
		}
		st := &BackupStatus{
			SnapshotID:    k.SnapshotID,
			StorageTarget: k.StorageTarget,
			Machine:       k.Machine,
		}
		statusMap[k] = st
		return st
	}

	for name, samples := range metrics {
		for _, s := range samples {
			st := getOrCreate(s)
			switch name {
			case "duplicacy_backup_running":
				st.Running = s.Value == 1
			case "duplicacy_backup_last_exit_code":
				st.ExitCode = s.Value
			case "duplicacy_backup_last_success_timestamp_seconds":
				if s.Value > 0 {
					st.LastSuccess = time.Unix(int64(s.Value), 0).UTC().Format(time.RFC3339)
				}
			case "duplicacy_backup_last_duration_seconds":
				st.LastDuration = s.Value
			case "duplicacy_backup_last_files_total":
				st.LastFiles = s.Value
			case "duplicacy_backup_last_files_new":
				st.LastFilesNew = s.Value
			case "duplicacy_backup_last_bytes_uploaded":
				st.LastBytesUp = s.Value
			case "duplicacy_backup_last_bytes_new":
				st.LastBytesNew = s.Value
			case "duplicacy_backup_last_chunks_new":
				st.LastChunksNew = s.Value
			case "duplicacy_backup_last_revision":
				st.LastRevision = s.Value
			case "duplicacy_backup_bytes_uploaded_total":
				st.TotalBytesUp = s.Value
			}
		}
	}

	statuses := make([]BackupStatus, 0, len(statusMap))
	for _, st := range statusMap {
		statuses = append(statuses, *st)
	}

	return json.Marshal(statuses)
}

// GetProgress parses metrics for real-time progress of any running backups.
func (c *Client) GetProgress() ([]byte, error) {
	metrics, err := c.FetchMetrics()
	if err != nil {
		return nil, err
	}

	type progressKey struct {
		SnapshotID    string
		StorageTarget string
		Machine       string
	}
	progressMap := make(map[progressKey]*BackupProgress)

	getOrCreate := func(s MetricSample) *BackupProgress {
		k := progressKey{
			SnapshotID:    s.Labels["snapshot_id"],
			StorageTarget: s.Labels["storage_target"],
			Machine:       s.Labels["machine"],
		}
		if p, ok := progressMap[k]; ok {
			return p
		}
		p := &BackupProgress{
			SnapshotID:    k.SnapshotID,
			StorageTarget: k.StorageTarget,
			Machine:       k.Machine,
		}
		progressMap[k] = p
		return p
	}

	for name, samples := range metrics {
		for _, s := range samples {
			switch name {
			case "duplicacy_backup_running":
				p := getOrCreate(s)
				p.Running = s.Value == 1
			case "duplicacy_backup_speed_bytes_per_second":
				p := getOrCreate(s)
				p.SpeedBPS = s.Value
			case "duplicacy_backup_progress_ratio":
				p := getOrCreate(s)
				p.ProgressRatio = s.Value
			case "duplicacy_backup_chunks_uploaded":
				p := getOrCreate(s)
				p.ChunksUploaded = s.Value
			case "duplicacy_backup_chunks_skipped":
				p := getOrCreate(s)
				p.ChunksSkipped = s.Value
			}
		}
	}

	progresses := make([]BackupProgress, 0, len(progressMap))
	for _, p := range progressMap {
		progresses = append(progresses, *p)
	}

	return json.Marshal(progresses)
}

// GetSnapshotStatus returns status for a specific snapshot_id.
func (c *Client) GetSnapshotStatus(snapshotID string) ([]byte, error) {
	metrics, err := c.FetchMetrics()
	if err != nil {
		return nil, err
	}

	type statusKey struct {
		StorageTarget string
		Machine       string
	}
	statusMap := make(map[statusKey]*BackupStatus)

	getOrCreate := func(s MetricSample) *BackupStatus {
		k := statusKey{
			StorageTarget: s.Labels["storage_target"],
			Machine:       s.Labels["machine"],
		}
		if st, ok := statusMap[k]; ok {
			return st
		}
		st := &BackupStatus{
			SnapshotID:    snapshotID,
			StorageTarget: k.StorageTarget,
			Machine:       k.Machine,
		}
		statusMap[k] = st
		return st
	}

	for name, samples := range metrics {
		for _, s := range samples {
			if s.Labels["snapshot_id"] != snapshotID {
				continue
			}
			st := getOrCreate(s)
			switch name {
			case "duplicacy_backup_running":
				st.Running = s.Value == 1
			case "duplicacy_backup_last_exit_code":
				st.ExitCode = s.Value
			case "duplicacy_backup_last_success_timestamp_seconds":
				if s.Value > 0 {
					st.LastSuccess = time.Unix(int64(s.Value), 0).UTC().Format(time.RFC3339)
				}
			case "duplicacy_backup_last_duration_seconds":
				st.LastDuration = s.Value
			case "duplicacy_backup_last_files_total":
				st.LastFiles = s.Value
			case "duplicacy_backup_last_files_new":
				st.LastFilesNew = s.Value
			case "duplicacy_backup_last_bytes_uploaded":
				st.LastBytesUp = s.Value
			case "duplicacy_backup_last_bytes_new":
				st.LastBytesNew = s.Value
			case "duplicacy_backup_last_chunks_new":
				st.LastChunksNew = s.Value
			case "duplicacy_backup_last_revision":
				st.LastRevision = s.Value
			case "duplicacy_backup_bytes_uploaded_total":
				st.TotalBytesUp = s.Value
			}
		}
	}

	if len(statusMap) == 0 {
		return json.Marshal(map[string]string{
			"error":       "snapshot not found",
			"snapshot_id": snapshotID,
		})
	}

	statuses := make([]BackupStatus, 0, len(statusMap))
	for _, st := range statusMap {
		statuses = append(statuses, *st)
	}

	return json.Marshal(statuses)
}

// GetSnapshotHistory returns last backup details for a specific snapshot_id.
func (c *Client) GetSnapshotHistory(snapshotID string) ([]byte, error) {
	metrics, err := c.FetchMetrics()
	if err != nil {
		return nil, err
	}

	type historyKey struct {
		StorageTarget string
		Machine       string
	}

	type BackupHistory struct {
		SnapshotID    string  `json:"snapshot_id"`
		StorageTarget string  `json:"storage_target"`
		Machine       string  `json:"machine"`
		LastSuccess   string  `json:"last_success,omitempty"`
		LastDuration  float64 `json:"last_duration_seconds"`
		LastFiles     float64 `json:"last_files_total"`
		LastFilesNew  float64 `json:"last_files_new"`
		LastBytesUp   float64 `json:"last_bytes_uploaded"`
		LastBytesNew  float64 `json:"last_bytes_new"`
		LastChunksNew float64 `json:"last_chunks_new"`
		ExitCode      float64 `json:"exit_code"`
		LastRevision  float64 `json:"last_revision"`
		TotalBytesUp  float64 `json:"total_bytes_uploaded"`
	}

	historyMap := make(map[historyKey]*BackupHistory)

	getOrCreate := func(s MetricSample) *BackupHistory {
		k := historyKey{
			StorageTarget: s.Labels["storage_target"],
			Machine:       s.Labels["machine"],
		}
		if h, ok := historyMap[k]; ok {
			return h
		}
		h := &BackupHistory{
			SnapshotID:    snapshotID,
			StorageTarget: k.StorageTarget,
			Machine:       k.Machine,
		}
		historyMap[k] = h
		return h
	}

	for name, samples := range metrics {
		for _, s := range samples {
			if s.Labels["snapshot_id"] != snapshotID {
				continue
			}
			h := getOrCreate(s)
			switch name {
			case "duplicacy_backup_last_success_timestamp_seconds":
				if s.Value > 0 {
					h.LastSuccess = time.Unix(int64(s.Value), 0).UTC().Format(time.RFC3339)
				}
			case "duplicacy_backup_last_duration_seconds":
				h.LastDuration = s.Value
			case "duplicacy_backup_last_files_total":
				h.LastFiles = s.Value
			case "duplicacy_backup_last_files_new":
				h.LastFilesNew = s.Value
			case "duplicacy_backup_last_bytes_uploaded":
				h.LastBytesUp = s.Value
			case "duplicacy_backup_last_bytes_new":
				h.LastBytesNew = s.Value
			case "duplicacy_backup_last_chunks_new":
				h.LastChunksNew = s.Value
			case "duplicacy_backup_last_exit_code":
				h.ExitCode = s.Value
			case "duplicacy_backup_last_revision":
				h.LastRevision = s.Value
			case "duplicacy_backup_bytes_uploaded_total":
				h.TotalBytesUp = s.Value
			}
		}
	}

	if len(historyMap) == 0 {
		return json.Marshal(map[string]string{
			"error":       "snapshot not found",
			"snapshot_id": snapshotID,
		})
	}

	histories := make([]BackupHistory, 0, len(historyMap))
	for _, h := range historyMap {
		histories = append(histories, *h)
	}

	return json.Marshal(histories)
}

// ListSnapshots extracts all unique snapshot_id values from metrics.
func (c *Client) ListSnapshots() ([]byte, error) {
	metrics, err := c.FetchMetrics()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	for _, samples := range metrics {
		for _, s := range samples {
			if id, ok := s.Labels["snapshot_id"]; ok && id != "" {
				seen[id] = true
			}
		}
	}

	snapshots := make([]string, 0, len(seen))
	for id := range seen {
		snapshots = append(snapshots, id)
	}

	return json.Marshal(map[string]any{
		"snapshot_ids": snapshots,
		"count":        len(snapshots),
	})
}

// GetPruneStatus returns prune status, optionally filtered by storage_target.
func (c *Client) GetPruneStatus(storageTarget string) ([]byte, error) {
	metrics, err := c.FetchMetrics()
	if err != nil {
		return nil, err
	}

	type pruneKey struct {
		StorageTarget string
		Machine       string
	}
	pruneMap := make(map[pruneKey]*PruneStatus)

	getOrCreate := func(s MetricSample) *PruneStatus {
		k := pruneKey{
			StorageTarget: s.Labels["storage_target"],
			Machine:       s.Labels["machine"],
		}
		if p, ok := pruneMap[k]; ok {
			return p
		}
		p := &PruneStatus{
			StorageTarget: k.StorageTarget,
			Machine:       k.Machine,
		}
		pruneMap[k] = p
		return p
	}

	for name, samples := range metrics {
		if name != "duplicacy_prune_running" && name != "duplicacy_prune_last_success_timestamp_seconds" {
			continue
		}
		for _, s := range samples {
			if storageTarget != "" && s.Labels["storage_target"] != storageTarget {
				continue
			}
			p := getOrCreate(s)
			switch name {
			case "duplicacy_prune_running":
				p.Running = s.Value == 1
			case "duplicacy_prune_last_success_timestamp_seconds":
				if s.Value > 0 {
					p.LastSuccess = time.Unix(int64(s.Value), 0).UTC().Format(time.RFC3339)
				}
			}
		}
	}

	prunes := make([]PruneStatus, 0, len(pruneMap))
	for _, p := range pruneMap {
		prunes = append(prunes, *p)
	}

	return json.Marshal(prunes)
}

// parsePrometheusText parses Prometheus text exposition format into a map
// of metric name to slice of MetricSample.
func parsePrometheusText(text string) map[string][]MetricSample {
	result := make(map[string][]MetricSample)

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, sample, ok := parseMetricLine(line)
		if !ok {
			continue
		}
		result[name] = append(result[name], sample)
	}

	return result
}

// parseMetricLine parses a single Prometheus metric line.
// Format: metric_name{label1="val1",label2="val2"} value [timestamp]
func parseMetricLine(line string) (string, MetricSample, bool) {
	var name string
	var labels map[string]string
	var rest string

	if idx := strings.IndexByte(line, '{'); idx >= 0 {
		name = line[:idx]
		endIdx := strings.IndexByte(line[idx:], '}')
		if endIdx < 0 {
			return "", MetricSample{}, false
		}
		labelStr := line[idx+1 : idx+endIdx]
		labels = parseLabels(labelStr)
		rest = strings.TrimSpace(line[idx+endIdx+1:])
	} else {
		// No labels: metric_name value [timestamp]
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return "", MetricSample{}, false
		}
		name = parts[0]
		labels = make(map[string]string)
		rest = strings.Join(parts[1:], " ")
	}

	// Parse value (first field of rest)
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "", MetricSample{}, false
	}

	val, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "", MetricSample{}, false
	}

	return name, MetricSample{Labels: labels, Value: val}, true
}

// parseLabels parses the label portion: label1="val1",label2="val2"
func parseLabels(s string) map[string]string {
	labels := make(map[string]string)
	if s == "" {
		return labels
	}

	// Simple state-machine parser for quoted values that may contain commas
	i := 0
	for i < len(s) {
		// Find the key
		eqIdx := strings.IndexByte(s[i:], '=')
		if eqIdx < 0 {
			break
		}
		key := strings.TrimSpace(s[i : i+eqIdx])
		i += eqIdx + 1

		// Skip opening quote
		if i >= len(s) || s[i] != '"' {
			break
		}
		i++

		// Find closing quote (handle escaped quotes)
		var val strings.Builder
		for i < len(s) {
			if s[i] == '\\' && i+1 < len(s) {
				val.WriteByte(s[i+1])
				i += 2
				continue
			}
			if s[i] == '"' {
				i++
				break
			}
			val.WriteByte(s[i])
			i++
		}

		labels[key] = val.String()

		// Skip comma separator
		if i < len(s) && s[i] == ',' {
			i++
		}
	}

	return labels
}
