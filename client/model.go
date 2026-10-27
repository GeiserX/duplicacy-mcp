package client

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Filter narrows results to one snapshot, storage target or machine. An empty
// field matches everything.
type Filter struct {
	SnapshotID    string
	StorageTarget string
	Machine       string
}

func (f Filter) match(snapshotID, storageTarget, machine string) bool {
	return (f.SnapshotID == "" || f.SnapshotID == snapshotID) &&
		(f.StorageTarget == "" || f.StorageTarget == storageTarget) &&
		(f.Machine == "" || f.Machine == machine)
}

// NotFoundError reports a filter value that matches nothing, with the values
// that do exist so the caller can correct itself in one step.
type NotFoundError struct {
	// Field is the argument that matched nothing: snapshot_id, storage_target or machine.
	Field string
	Value string
	Known []string
}

func (e *NotFoundError) Error() string {
	known := "none"
	if len(e.Known) > 0 {
		known = strings.Join(e.Known, ", ")
	}
	return fmt.Sprintf("no data for %s %q; known %s values: %s",
		e.Field, sanitizeText(e.Value, maxLabelRunes), e.Field, known)
}

// BackupStatus is the state of one backup, identified by snapshot_id,
// storage_target and machine, as of its last run.
type BackupStatus struct {
	SnapshotID    string `json:"snapshot_id" jsonschema:"Duplicacy snapshot ID"`
	StorageTarget string `json:"storage_target" jsonschema:"Storage the snapshot is backed up to"`
	Machine       string `json:"machine" jsonschema:"Machine that runs the backup"`
	Running       bool   `json:"running" jsonschema:"True while a backup is in progress"`
	// ExitCode is nil when the exporter has not reported one, so a missing
	// metric never reads as success.
	ExitCode       *int    `json:"exit_code" jsonschema:"Exit code of the last run: 0 is success, anything else is a failure, null means the exporter has not reported one"`
	LastSuccess    string  `json:"last_success,omitempty" jsonschema:"Time of the last successful run, RFC 3339 in UTC"`
	LastDuration   float64 `json:"last_duration_seconds,omitempty" jsonschema:"Duration of the last run, in seconds"`
	LastFiles      float64 `json:"last_files_total,omitempty" jsonschema:"Files in the last snapshot"`
	LastFilesNew   float64 `json:"last_files_new,omitempty" jsonschema:"Files new in the last run"`
	LastFilesSize  float64 `json:"last_files_size_bytes,omitempty" jsonschema:"Logical size of all files in the last snapshot, in bytes"`
	LastBytesUp    float64 `json:"last_bytes_uploaded,omitempty" jsonschema:"Bytes uploaded by the last run"`
	LastBytesNew   float64 `json:"last_bytes_new,omitempty" jsonschema:"New bytes in the last run"`
	LastChunksNew  float64 `json:"last_chunks_new,omitempty" jsonschema:"New chunks in the last run"`
	LastChunksSize float64 `json:"last_chunks_size_bytes,omitempty" jsonschema:"Compressed size of the chunks the last snapshot references, in bytes"`
	LastRevision   float64 `json:"last_revision,omitempty" jsonschema:"Revision number created by the last run"`
	TotalBytesUp   float64 `json:"total_bytes_uploaded,omitempty" jsonschema:"Bytes uploaded since the exporter started counting"`

	lastSuccessUnix int64
}

// BackupHistory is the last completed run of one backup. The exporter keeps
// only the last run, not a history of runs.
type BackupHistory struct {
	SnapshotID    string  `json:"snapshot_id"`
	StorageTarget string  `json:"storage_target"`
	Machine       string  `json:"machine"`
	LastSuccess   string  `json:"last_success,omitempty" jsonschema:"Time of the last successful run, RFC 3339 in UTC"`
	LastDuration  float64 `json:"last_duration_seconds" jsonschema:"Duration of the last run, in seconds"`
	LastFiles     float64 `json:"last_files_total"`
	LastFilesNew  float64 `json:"last_files_new"`
	LastBytesUp   float64 `json:"last_bytes_uploaded"`
	LastBytesNew  float64 `json:"last_bytes_new"`
	LastChunksNew float64 `json:"last_chunks_new"`
	ExitCode      *int    `json:"exit_code" jsonschema:"Exit code of the last run: 0 is success, null means unknown"`
	LastRevision  float64 `json:"last_revision"`
	TotalBytesUp  float64 `json:"total_bytes_uploaded"`
}

// BackupProgress holds real-time progress for a backup.
type BackupProgress struct {
	SnapshotID      string  `json:"snapshot_id"`
	StorageTarget   string  `json:"storage_target"`
	Machine         string  `json:"machine"`
	Running         bool    `json:"running"`
	SpeedBPS        float64 `json:"speed_bytes_per_second"`
	ProgressRatio   float64 `json:"progress_ratio"`
	ProgressPercent float64 `json:"progress_percent"`
	ChunksUploaded  float64 `json:"chunks_uploaded"`
	ChunksSkipped   float64 `json:"chunks_skipped"`
}

// PruneStatus holds the status of pruning for one storage target and machine.
type PruneStatus struct {
	StorageTarget string `json:"storage_target"`
	Machine       string `json:"machine"`
	Running       bool   `json:"running" jsonschema:"True while a prune is in progress"`
	LastSuccess   string `json:"last_success,omitempty" jsonschema:"Time of the last successful prune, RFC 3339 in UTC"`
}

// BackupRef names one backup: a snapshot ID on a storage target, run by a machine.
type BackupRef struct {
	SnapshotID    string `json:"snapshot_id"`
	StorageTarget string `json:"storage_target"`
	Machine       string `json:"machine"`
}

// SnapshotList is what the exporter knows about, for use as filter values.
type SnapshotList struct {
	SnapshotIDs    []string    `json:"snapshot_ids" jsonschema:"Every snapshot ID the exporter has seen, sorted"`
	Count          int         `json:"count" jsonschema:"Number of snapshot IDs"`
	Backups        []BackupRef `json:"backups" jsonschema:"Every snapshot_id, storage_target and machine combination with backup metrics"`
	StorageTargets []string    `json:"storage_targets" jsonschema:"Every storage target seen, sorted"`
	Machines       []string    `json:"machines" jsonschema:"Every machine seen, sorted"`
}

// StorageStat is the size of one storage, as the exporter's poller last saw it.
type StorageStat struct {
	StorageTarget  string  `json:"storage_target"`
	Machine        string  `json:"machine"`
	TotalSizeBytes float64 `json:"total_size_bytes" jsonschema:"Total size of all chunks in the storage, in bytes (approximate)"`
	TotalChunks    float64 `json:"total_chunks" jsonschema:"Number of chunks in the storage"`
}

// SnapshotRevisions is how many revisions one snapshot keeps on a storage.
type SnapshotRevisions struct {
	SnapshotID    string  `json:"snapshot_id"`
	StorageTarget string  `json:"storage_target"`
	Machine       string  `json:"machine"`
	Revisions     float64 `json:"revisions" jsonschema:"Number of revisions kept"`
	LastRevision  float64 `json:"last_revision" jsonschema:"Highest revision number"`
}

// StorageUsage is the storage poller's view of every storage and snapshot.
type StorageUsage struct {
	Storages          []StorageStat       `json:"storages"`
	Snapshots         []SnapshotRevisions `json:"snapshots"`
	PollerLastSuccess string              `json:"poller_last_success,omitempty" jsonschema:"Time of the poller's last fully successful cycle, RFC 3339 in UTC"`
	PollerErrorsTotal float64             `json:"poller_errors_total" jsonschema:"Poller errors since the exporter started"`
	Note              string              `json:"note,omitempty"`
}

// Verdicts, from best to worst.
const (
	VerdictOK      = "ok"
	VerdictRunning = "running"
	VerdictStale   = "stale"
	VerdictNever   = "never"
	VerdictFailed  = "failed"
)

// OverviewRow is the health verdict for one backup.
type OverviewRow struct {
	SnapshotID    string `json:"snapshot_id"`
	StorageTarget string `json:"storage_target"`
	Machine       string `json:"machine"`
	Verdict       string `json:"verdict" jsonschema:"One of: ok, running, stale, never, failed"`
	Reason        string `json:"reason" jsonschema:"Why the backup got this verdict"`
	Running       bool   `json:"running"`
	ExitCode      *int   `json:"exit_code" jsonschema:"Exit code of the last run, null when unknown"`
	LastSuccess   string `json:"last_success,omitempty" jsonschema:"Time of the last successful run, RFC 3339 in UTC"`
	AgeSeconds    *int64 `json:"age_seconds" jsonschema:"Seconds since the last successful run, null when there was none"`
	Age           string `json:"age,omitempty" jsonschema:"The same age, human readable, for example 3h12m"`
}

// OverviewCounts counts backups per verdict.
type OverviewCounts struct {
	Total   int `json:"total"`
	OK      int `json:"ok"`
	Running int `json:"running"`
	Stale   int `json:"stale"`
	Never   int `json:"never"`
	Failed  int `json:"failed"`
}

// ExporterActivity tells whether the exporter itself is still seeing logs.
type ExporterActivity struct {
	LastActivity     string  `json:"last_activity,omitempty" jsonschema:"Time of the last log line the exporter parsed, RFC 3339 in UTC"`
	BackupsSeenTotal float64 `json:"backups_seen_total" jsonschema:"Completed backups the exporter detected since it started"`
}

// Overview is the health of every backup in one call.
type Overview struct {
	Status      string           `json:"status" jsonschema:"ok when every backup is ok or running, attention when any is stale, never ran or failed, no_data when the exporter reports no backups"`
	Summary     string           `json:"summary"`
	CheckedAt   string           `json:"checked_at" jsonschema:"When the overview was computed, RFC 3339 in UTC"`
	MaxAgeHours int              `json:"max_age_hours" jsonschema:"A backup whose last success is older than this is stale"`
	Counts      OverviewCounts   `json:"counts"`
	Backups     []OverviewRow    `json:"backups"`
	Exporter    ExporterActivity `json:"exporter"`
}

// OverviewOptions configures GetOverview.
type OverviewOptions struct {
	MaxAge time.Duration
	Filter Filter
	// Now is the reference time; zero means time.Now().
	Now time.Time
}

func formatTS(v float64) string {
	return time.Unix(int64(v), 0).UTC().Format(time.RFC3339)
}

// backupFields maps each per-backup metric to the field it sets. Metrics not
// listed here (prune, storage, poller, exporter) never create a backup row.
var backupFields = map[string]func(*BackupStatus, float64){
	"duplicacy_backup_running": func(b *BackupStatus, v float64) { b.Running = v == 1 },
	"duplicacy_backup_last_exit_code": func(b *BackupStatus, v float64) {
		code := int(math.Round(v))
		b.ExitCode = &code
	},
	"duplicacy_backup_last_success_timestamp_seconds": func(b *BackupStatus, v float64) {
		if v > 0 {
			b.LastSuccess = formatTS(v)
			b.lastSuccessUnix = int64(v)
		}
	},
	"duplicacy_backup_last_duration_seconds":  func(b *BackupStatus, v float64) { b.LastDuration = v },
	"duplicacy_backup_last_files_total":       func(b *BackupStatus, v float64) { b.LastFiles = v },
	"duplicacy_backup_last_files_new":         func(b *BackupStatus, v float64) { b.LastFilesNew = v },
	"duplicacy_backup_last_files_size_bytes":  func(b *BackupStatus, v float64) { b.LastFilesSize = v },
	"duplicacy_backup_last_bytes_uploaded":    func(b *BackupStatus, v float64) { b.LastBytesUp = v },
	"duplicacy_backup_last_bytes_new":         func(b *BackupStatus, v float64) { b.LastBytesNew = v },
	"duplicacy_backup_last_chunks_new":        func(b *BackupStatus, v float64) { b.LastChunksNew = v },
	"duplicacy_backup_last_chunks_size_bytes": func(b *BackupStatus, v float64) { b.LastChunksSize = v },
	"duplicacy_backup_last_revision":          func(b *BackupStatus, v float64) { b.LastRevision = v },
	"duplicacy_backup_bytes_uploaded_total":   func(b *BackupStatus, v float64) { b.TotalBytesUp = v },
	"duplicacy_backup_speed_bytes_per_second": nil, // progress only
	"duplicacy_backup_progress_ratio":         nil,
	"duplicacy_backup_chunks_uploaded":        nil,
	"duplicacy_backup_chunks_skipped":         nil,
}

type backupKey struct{ snapshotID, storageTarget, machine string }

func keyOf(s MetricSample) backupKey {
	return backupKey{s.Labels["snapshot_id"], s.Labels["storage_target"], s.Labels["machine"]}
}

func lessKey(a, b backupKey) bool {
	if a.snapshotID != b.snapshotID {
		return a.snapshotID < b.snapshotID
	}
	if a.storageTarget != b.storageTarget {
		return a.storageTarget < b.storageTarget
	}
	return a.machine < b.machine
}

// collectBackups builds one row per snapshot_id, storage_target and machine
// from the per-backup metrics, sorted by those three labels. Samples without
// a snapshot_id are skipped, so prune and storage series never show up as
// phantom backups.
func collectBackups(metrics map[string][]MetricSample) []BackupStatus {
	rows := make(map[backupKey]*BackupStatus)
	for name, samples := range metrics {
		set, ok := backupFields[name]
		if !ok {
			continue
		}
		for _, s := range samples {
			k := keyOf(s)
			if k.snapshotID == "" {
				continue
			}
			row, ok := rows[k]
			if !ok {
				row = &BackupStatus{SnapshotID: k.snapshotID, StorageTarget: k.storageTarget, Machine: k.machine}
				rows[k] = row
			}
			if set != nil {
				set(row, s.Value)
			}
		}
	}
	keys := make([]backupKey, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return lessKey(keys[i], keys[j]) })
	out := make([]BackupStatus, 0, len(keys))
	for _, k := range keys {
		out = append(out, *rows[k])
	}
	return out
}

// sortedSet returns the distinct non-empty values, sorted.
func sortedSet(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// filterBackups applies f, and when nothing matches says which filter value
// matched nothing and lists the values that would have.
func filterBackups(all []BackupStatus, f Filter) ([]BackupStatus, error) {
	out := []BackupStatus{}
	for _, b := range all {
		if f.match(b.SnapshotID, b.StorageTarget, b.Machine) {
			out = append(out, b)
		}
	}
	if len(out) > 0 || (f == Filter{}) {
		return out, nil
	}
	var ids, targets, machines []string
	for _, b := range all {
		ids = append(ids, b.SnapshotID)
		if (Filter{SnapshotID: f.SnapshotID}).match(b.SnapshotID, "", "") {
			targets = append(targets, b.StorageTarget)
			if (Filter{SnapshotID: f.SnapshotID, StorageTarget: f.StorageTarget}).match(b.SnapshotID, b.StorageTarget, "") {
				machines = append(machines, b.Machine)
			}
		}
	}
	switch {
	case f.SnapshotID != "" && len(targets) == 0:
		return nil, &NotFoundError{Field: "snapshot_id", Value: f.SnapshotID, Known: sortedSet(ids)}
	case f.StorageTarget != "" && len(machines) == 0:
		return nil, &NotFoundError{Field: "storage_target", Value: f.StorageTarget, Known: sortedSet(targets)}
	default:
		return nil, &NotFoundError{Field: "machine", Value: f.Machine, Known: sortedSet(machines)}
	}
}

// GetStatus returns the status of every backup, sorted.
func (c *Client) GetStatus(ctx context.Context) ([]BackupStatus, error) {
	metrics, err := c.FetchMetrics(ctx)
	if err != nil {
		return nil, err
	}
	return collectBackups(metrics), nil
}

// GetSnapshotStatus returns the status of the backups that match f.
func (c *Client) GetSnapshotStatus(ctx context.Context, f Filter) ([]BackupStatus, error) {
	all, err := c.GetStatus(ctx)
	if err != nil {
		return nil, err
	}
	return filterBackups(all, f)
}

// GetSnapshotHistory returns the last completed run of the backups that match f.
func (c *Client) GetSnapshotHistory(ctx context.Context, f Filter) ([]BackupHistory, error) {
	rows, err := c.GetSnapshotStatus(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]BackupHistory, 0, len(rows))
	for _, b := range rows {
		out = append(out, BackupHistory{
			SnapshotID:    b.SnapshotID,
			StorageTarget: b.StorageTarget,
			Machine:       b.Machine,
			LastSuccess:   b.LastSuccess,
			LastDuration:  b.LastDuration,
			LastFiles:     b.LastFiles,
			LastFilesNew:  b.LastFilesNew,
			LastBytesUp:   b.LastBytesUp,
			LastBytesNew:  b.LastBytesNew,
			LastChunksNew: b.LastChunksNew,
			ExitCode:      b.ExitCode,
			LastRevision:  b.LastRevision,
			TotalBytesUp:  b.TotalBytesUp,
		})
	}
	return out, nil
}

// GetProgress returns the live progress of every backup, sorted.
func (c *Client) GetProgress(ctx context.Context) ([]BackupProgress, error) {
	metrics, err := c.FetchMetrics(ctx)
	if err != nil {
		return nil, err
	}
	rows := make(map[backupKey]*BackupProgress)
	set := map[string]func(*BackupProgress, float64){
		"duplicacy_backup_running":                func(p *BackupProgress, v float64) { p.Running = v == 1 },
		"duplicacy_backup_speed_bytes_per_second": func(p *BackupProgress, v float64) { p.SpeedBPS = v },
		"duplicacy_backup_progress_ratio": func(p *BackupProgress, v float64) {
			p.ProgressRatio = v
			p.ProgressPercent = math.Round(v*1000) / 10
		},
		"duplicacy_backup_chunks_uploaded": func(p *BackupProgress, v float64) { p.ChunksUploaded = v },
		"duplicacy_backup_chunks_skipped":  func(p *BackupProgress, v float64) { p.ChunksSkipped = v },
	}
	for name, samples := range metrics {
		fn, ok := set[name]
		if !ok {
			continue
		}
		for _, s := range samples {
			k := keyOf(s)
			if k.snapshotID == "" {
				continue
			}
			p, ok := rows[k]
			if !ok {
				p = &BackupProgress{SnapshotID: k.snapshotID, StorageTarget: k.storageTarget, Machine: k.machine}
				rows[k] = p
			}
			fn(p, s.Value)
		}
	}
	keys := make([]backupKey, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return lessKey(keys[i], keys[j]) })
	out := make([]BackupProgress, 0, len(keys))
	for _, k := range keys {
		out = append(out, *rows[k])
	}
	return out, nil
}

// ListSnapshots returns every snapshot ID, storage target and machine the
// exporter reports, sorted.
func (c *Client) ListSnapshots(ctx context.Context) (SnapshotList, error) {
	metrics, err := c.FetchMetrics(ctx)
	if err != nil {
		return SnapshotList{}, err
	}
	var ids, targets, machines []string
	for _, samples := range metrics {
		for _, s := range samples {
			ids = append(ids, s.Labels["snapshot_id"])
			targets = append(targets, s.Labels["storage_target"])
			machines = append(machines, s.Labels["machine"])
		}
	}
	backups := collectBackups(metrics)
	refs := make([]BackupRef, 0, len(backups))
	for _, b := range backups {
		refs = append(refs, BackupRef{SnapshotID: b.SnapshotID, StorageTarget: b.StorageTarget, Machine: b.Machine})
	}
	list := SnapshotList{
		SnapshotIDs:    sortedSet(ids),
		Backups:        refs,
		StorageTargets: sortedSet(targets),
		Machines:       sortedSet(machines),
	}
	list.Count = len(list.SnapshotIDs)
	return list, nil
}

// GetPruneStatus returns prune status, optionally for one storage target.
func (c *Client) GetPruneStatus(ctx context.Context, storageTarget string) ([]PruneStatus, error) {
	metrics, err := c.FetchMetrics(ctx)
	if err != nil {
		return nil, err
	}
	type pruneKey struct{ storageTarget, machine string }
	rows := make(map[pruneKey]*PruneStatus)
	var known []string
	for _, name := range []string{"duplicacy_prune_running", "duplicacy_prune_last_success_timestamp_seconds"} {
		for _, s := range metrics[name] {
			k := pruneKey{s.Labels["storage_target"], s.Labels["machine"]}
			known = append(known, k.storageTarget)
			if storageTarget != "" && k.storageTarget != storageTarget {
				continue
			}
			p, ok := rows[k]
			if !ok {
				p = &PruneStatus{StorageTarget: k.storageTarget, Machine: k.machine}
				rows[k] = p
			}
			if name == "duplicacy_prune_running" {
				p.Running = s.Value == 1
			} else if s.Value > 0 {
				p.LastSuccess = formatTS(s.Value)
			}
		}
	}
	if storageTarget != "" && len(rows) == 0 {
		return nil, &NotFoundError{Field: "storage_target", Value: storageTarget, Known: sortedSet(known)}
	}
	keys := make([]pruneKey, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].storageTarget != keys[j].storageTarget {
			return keys[i].storageTarget < keys[j].storageTarget
		}
		return keys[i].machine < keys[j].machine
	})
	out := make([]PruneStatus, 0, len(keys))
	for _, k := range keys {
		out = append(out, *rows[k])
	}
	return out, nil
}

// GetStorageUsage returns the storage poller's view: storage sizes and
// revisions per snapshot. Both are empty when the poller is disabled.
func (c *Client) GetStorageUsage(ctx context.Context, f Filter) (StorageUsage, error) {
	metrics, err := c.FetchMetrics(ctx)
	if err != nil {
		return StorageUsage{}, err
	}
	type storageKey struct{ storageTarget, machine string }
	storages := make(map[storageKey]*StorageStat)
	for _, name := range []string{"duplicacy_storage_total_size_bytes", "duplicacy_storage_total_chunks"} {
		for _, s := range metrics[name] {
			k := storageKey{s.Labels["storage_target"], s.Labels["machine"]}
			if !f.match("", k.storageTarget, k.machine) || f.SnapshotID != "" {
				continue
			}
			st, ok := storages[k]
			if !ok {
				st = &StorageStat{StorageTarget: k.storageTarget, Machine: k.machine}
				storages[k] = st
			}
			if name == "duplicacy_storage_total_size_bytes" {
				st.TotalSizeBytes = s.Value
			} else {
				st.TotalChunks = s.Value
			}
		}
	}
	snaps := make(map[backupKey]*SnapshotRevisions)
	for _, name := range []string{"duplicacy_snapshot_revisions", "duplicacy_snapshot_last_revision"} {
		for _, s := range metrics[name] {
			k := keyOf(s)
			if k.snapshotID == "" || !f.match(k.snapshotID, k.storageTarget, k.machine) {
				continue
			}
			sr, ok := snaps[k]
			if !ok {
				sr = &SnapshotRevisions{SnapshotID: k.snapshotID, StorageTarget: k.storageTarget, Machine: k.machine}
				snaps[k] = sr
			}
			if name == "duplicacy_snapshot_revisions" {
				sr.Revisions = s.Value
			} else {
				sr.LastRevision = s.Value
			}
		}
	}

	usage := StorageUsage{Storages: []StorageStat{}, Snapshots: []SnapshotRevisions{}}
	skeys := make([]storageKey, 0, len(storages))
	for k := range storages {
		skeys = append(skeys, k)
	}
	sort.Slice(skeys, func(i, j int) bool {
		if skeys[i].storageTarget != skeys[j].storageTarget {
			return skeys[i].storageTarget < skeys[j].storageTarget
		}
		return skeys[i].machine < skeys[j].machine
	})
	for _, k := range skeys {
		usage.Storages = append(usage.Storages, *storages[k])
	}
	bkeys := make([]backupKey, 0, len(snaps))
	for k := range snaps {
		bkeys = append(bkeys, k)
	}
	sort.Slice(bkeys, func(i, j int) bool { return lessKey(bkeys[i], bkeys[j]) })
	for _, k := range bkeys {
		usage.Snapshots = append(usage.Snapshots, *snaps[k])
	}
	if s := metrics["duplicacy_poller_last_success_timestamp_seconds"]; len(s) > 0 && s[0].Value > 0 {
		usage.PollerLastSuccess = formatTS(s[0].Value)
	}
	if s := metrics["duplicacy_poller_errors_total"]; len(s) > 0 {
		usage.PollerErrorsTotal = s[0].Value
	}
	if len(usage.Storages) == 0 && len(usage.Snapshots) == 0 {
		usage.Note = "No storage metrics match. The exporter's storage poller is off, has not finished a cycle yet, or the filters match nothing."
	}
	return usage, nil
}

// GetOverview classifies every backup as ok, running, stale, never or failed.
func (c *Client) GetOverview(ctx context.Context, opts OverviewOptions) (Overview, error) {
	metrics, err := c.FetchMetrics(ctx)
	if err != nil {
		return Overview{}, err
	}
	rows, err := filterBackups(collectBackups(metrics), opts.Filter)
	if err != nil {
		return Overview{}, err
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	maxAge := opts.MaxAge
	if maxAge <= 0 {
		maxAge = 26 * time.Hour
	}

	ov := Overview{
		CheckedAt:   now.UTC().Format(time.RFC3339),
		MaxAgeHours: int(maxAge / time.Hour),
		Backups:     make([]OverviewRow, 0, len(rows)),
	}
	for _, b := range rows {
		ov.Backups = append(ov.Backups, classifyBackup(b, now, maxAge))
	}
	for _, r := range ov.Backups {
		ov.Counts.Total++
		switch r.Verdict {
		case VerdictOK:
			ov.Counts.OK++
		case VerdictRunning:
			ov.Counts.Running++
		case VerdictStale:
			ov.Counts.Stale++
		case VerdictNever:
			ov.Counts.Never++
		case VerdictFailed:
			ov.Counts.Failed++
		}
	}
	switch {
	case ov.Counts.Total == 0:
		ov.Status = "no_data"
		ov.Summary = "The exporter reports no backups yet."
	case ov.Counts.Stale+ov.Counts.Never+ov.Counts.Failed == 0:
		ov.Status = "ok"
		ov.Summary = fmt.Sprintf("Healthy: %d of %d backups (%d ok, %d running).", ov.Counts.Total, ov.Counts.Total, ov.Counts.OK, ov.Counts.Running)
	default:
		ov.Status = "attention"
		ov.Summary = fmt.Sprintf("Needs attention: %d of %d backups (%d failed, %d stale, %d never succeeded).",
			ov.Counts.Stale+ov.Counts.Never+ov.Counts.Failed, ov.Counts.Total, ov.Counts.Failed, ov.Counts.Stale, ov.Counts.Never)
	}
	if s := metrics["duplicacy_exporter_last_activity_timestamp_seconds"]; len(s) > 0 && s[0].Value > 0 {
		ov.Exporter.LastActivity = formatTS(s[0].Value)
	}
	if s := metrics["duplicacy_exporter_backups_seen_total"]; len(s) > 0 {
		ov.Exporter.BackupsSeenTotal = s[0].Value
	}
	return ov, nil
}

func classifyBackup(b BackupStatus, now time.Time, maxAge time.Duration) OverviewRow {
	r := OverviewRow{
		SnapshotID:    b.SnapshotID,
		StorageTarget: b.StorageTarget,
		Machine:       b.Machine,
		Running:       b.Running,
		ExitCode:      b.ExitCode,
		LastSuccess:   b.LastSuccess,
	}
	var age time.Duration
	if b.lastSuccessUnix > 0 {
		age = now.Sub(time.Unix(b.lastSuccessUnix, 0))
		if age < 0 {
			age = 0
		}
		secs := int64(age / time.Second)
		r.AgeSeconds = &secs
		r.Age = age.Truncate(time.Minute).String()
	}
	switch {
	case b.Running:
		r.Verdict, r.Reason = VerdictRunning, "a backup is in progress"
	case b.ExitCode != nil && *b.ExitCode != 0:
		r.Verdict, r.Reason = VerdictFailed, fmt.Sprintf("the last run exited with code %d", *b.ExitCode)
	case b.lastSuccessUnix == 0:
		r.Verdict, r.Reason = VerdictNever, "no successful run recorded"
	case age > maxAge:
		r.Verdict, r.Reason = VerdictStale, fmt.Sprintf("last success is older than %d hours", int(maxAge/time.Hour))
	default:
		r.Verdict, r.Reason = VerdictOK, "last run succeeded within the allowed age"
	}
	return r
}
