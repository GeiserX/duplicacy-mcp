package config

import (
	"testing"
)

func TestLoadDuplicacyConfig_Defaults(t *testing.T) {
	t.Setenv("DUPLICACY_EXPORTER_URL", "")
	cfg := LoadDuplicacyConfig()
	if cfg.ExporterURL != "http://localhost:9750" {
		t.Errorf("ExporterURL default: got %q, want %q", cfg.ExporterURL, "http://localhost:9750")
	}
}

func TestLoadDuplicacyConfig_EnvOverride(t *testing.T) {
	t.Setenv("DUPLICACY_EXPORTER_URL", "http://myhost:1234")
	cfg := LoadDuplicacyConfig()
	if cfg.ExporterURL != "http://myhost:1234" {
		t.Errorf("ExporterURL: got %q, want %q", cfg.ExporterURL, "http://myhost:1234")
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		envVal   string
		fallback string
		want     string
	}{
		{
			name:     "returns env value when set",
			key:      "TEST_DUPLICACY_VAR",
			envVal:   "custom",
			fallback: "default",
			want:     "custom",
		},
		{
			name:     "returns default when env empty",
			key:      "TEST_DUPLICACY_EMPTY",
			envVal:   "",
			fallback: "fallback",
			want:     "fallback",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.key, tc.envVal)
			got := getEnv(tc.key, tc.fallback)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
