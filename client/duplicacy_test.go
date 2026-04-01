package client

import (
	"testing"
)

func TestParseLabels(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect map[string]string
	}{
		{
			name:   "empty string",
			input:  "",
			expect: map[string]string{},
		},
		{
			name:  "single label",
			input: `snapshot_id="photos"`,
			expect: map[string]string{
				"snapshot_id": "photos",
			},
		},
		{
			name:  "multiple labels",
			input: `snapshot_id="docs",storage_target="b2",machine="server1"`,
			expect: map[string]string{
				"snapshot_id":    "docs",
				"storage_target": "b2",
				"machine":        "server1",
			},
		},
		{
			name:  "value with escaped quote",
			input: `snapshot_id="my\"snap"`,
			expect: map[string]string{
				"snapshot_id": `my"snap`,
			},
		},
		{
			name:  "value with comma inside quotes",
			input: `path="/a,b",id="test"`,
			expect: map[string]string{
				"path": "/a,b",
				"id":   "test",
			},
		},
		{
			name:  "empty value",
			input: `snapshot_id=""`,
			expect: map[string]string{
				"snapshot_id": "",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseLabels(tc.input)
			if len(got) != len(tc.expect) {
				t.Fatalf("len mismatch: got %d, want %d\ngot:  %v\nwant: %v", len(got), len(tc.expect), got, tc.expect)
			}
			for k, v := range tc.expect {
				if got[k] != v {
					t.Errorf("label %q: got %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestParseMetricLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantName   string
		wantValue  float64
		wantLabels map[string]string
		wantOK     bool
	}{
		{
			name:       "metric with labels",
			line:       `duplicacy_backup_running{snapshot_id="photos",storage_target="b2",machine="nas"} 1`,
			wantName:   "duplicacy_backup_running",
			wantValue:  1,
			wantLabels: map[string]string{"snapshot_id": "photos", "storage_target": "b2", "machine": "nas"},
			wantOK:     true,
		},
		{
			name:       "metric without labels",
			line:       `go_goroutines 42`,
			wantName:   "go_goroutines",
			wantValue:  42,
			wantLabels: map[string]string{},
			wantOK:     true,
		},
		{
			name:       "float value",
			line:       `duplicacy_backup_last_duration_seconds{snapshot_id="docs",storage_target="s3",machine="srv"} 123.456`,
			wantName:   "duplicacy_backup_last_duration_seconds",
			wantValue:  123.456,
			wantLabels: map[string]string{"snapshot_id": "docs", "storage_target": "s3", "machine": "srv"},
			wantOK:     true,
		},
		{
			name:       "value with timestamp (ignored)",
			line:       `duplicacy_backup_running{snapshot_id="x",storage_target="y",machine="z"} 0 1609459200000`,
			wantName:   "duplicacy_backup_running",
			wantValue:  0,
			wantLabels: map[string]string{"snapshot_id": "x", "storage_target": "y", "machine": "z"},
			wantOK:     true,
		},
		{
			name:   "comment line",
			line:   `# HELP duplicacy_backup_running Whether a backup is currently running`,
			wantOK: false,
		},
		{
			name:   "empty line",
			line:   ``,
			wantOK: false,
		},
		{
			name:   "malformed - no value",
			line:   `metric_name`,
			wantOK: false,
		},
		{
			name:   "malformed - unclosed brace",
			line:   `metric{label="val" 1`,
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name, sample, ok := parseMetricLine(tc.line)
			if ok != tc.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if name != tc.wantName {
				t.Errorf("name: got %q, want %q", name, tc.wantName)
			}
			if sample.Value != tc.wantValue {
				t.Errorf("value: got %f, want %f", sample.Value, tc.wantValue)
			}
			if len(sample.Labels) != len(tc.wantLabels) {
				t.Fatalf("labels len: got %d, want %d", len(sample.Labels), len(tc.wantLabels))
			}
			for k, v := range tc.wantLabels {
				if sample.Labels[k] != v {
					t.Errorf("label %q: got %q, want %q", k, sample.Labels[k], v)
				}
			}
		})
	}
}

func TestParsePrometheusText(t *testing.T) {
	input := `# HELP duplicacy_backup_running Whether a backup is currently running
# TYPE duplicacy_backup_running gauge
duplicacy_backup_running{snapshot_id="photos",storage_target="b2",machine="nas"} 1
duplicacy_backup_running{snapshot_id="docs",storage_target="s3",machine="nas"} 0
# HELP duplicacy_backup_last_exit_code Exit code of the last backup
# TYPE duplicacy_backup_last_exit_code gauge
duplicacy_backup_last_exit_code{snapshot_id="photos",storage_target="b2",machine="nas"} 0
duplicacy_backup_last_duration_seconds{snapshot_id="photos",storage_target="b2",machine="nas"} 345.67
`

	result := parsePrometheusText(input)

	// Check duplicacy_backup_running has 2 samples
	running, ok := result["duplicacy_backup_running"]
	if !ok {
		t.Fatal("missing duplicacy_backup_running")
	}
	if len(running) != 2 {
		t.Fatalf("duplicacy_backup_running: got %d samples, want 2", len(running))
	}

	// Check one running sample
	found := false
	for _, s := range running {
		if s.Labels["snapshot_id"] == "photos" && s.Value == 1 {
			found = true
		}
	}
	if !found {
		t.Error("missing photos running=1 sample")
	}

	// Check exit code
	exitCode, ok := result["duplicacy_backup_last_exit_code"]
	if !ok {
		t.Fatal("missing duplicacy_backup_last_exit_code")
	}
	if len(exitCode) != 1 {
		t.Fatalf("exit_code: got %d samples, want 1", len(exitCode))
	}
	if exitCode[0].Value != 0 {
		t.Errorf("exit_code value: got %f, want 0", exitCode[0].Value)
	}

	// Check duration
	duration, ok := result["duplicacy_backup_last_duration_seconds"]
	if !ok {
		t.Fatal("missing duplicacy_backup_last_duration_seconds")
	}
	if len(duration) != 1 || duration[0].Value != 345.67 {
		t.Errorf("duration: got %v, want [345.67]", duration)
	}

	// Comments and blank lines should not produce entries
	if _, ok := result["#"]; ok {
		t.Error("comments should not appear as metric names")
	}
}

func TestParsePrometheusText_Empty(t *testing.T) {
	result := parsePrometheusText("")
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}
