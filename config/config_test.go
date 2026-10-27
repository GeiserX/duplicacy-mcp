package config

import (
	"strings"
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

func TestValidate(t *testing.T) {
	for url, ok := range map[string]bool{
		"http://localhost:9750":  true,
		"https://exporter:9750/": true,
		"localhost:9750":         false,
		"ftp://x":                false,
		"http://":                false,
	} {
		if err := (DuplicacyConfig{ExporterURL: url}).Validate(); (err == nil) != ok {
			t.Errorf("%q: err = %v, want ok=%v", url, err, ok)
		}
	}
}

func TestLoadDuplicacyConfig_Transport(t *testing.T) {
	t.Setenv("TRANSPORT", "STDIO")
	if got := LoadDuplicacyConfig().Transport; got != "stdio" {
		t.Errorf("transport = %q, want stdio", got)
	}
	t.Setenv("TRANSPORT", "")
	if got := LoadDuplicacyConfig().Transport; got != "http" {
		t.Errorf("transport = %q, want http", got)
	}
}

func TestIsLoopback(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:8080": true,
		"[::1]:8080":     true,
		"localhost:8080": true,
		"0.0.0.0:8080":   false,
		":8080":          false,
		"10.0.0.5:8080":  false,
		"[::]:8080":      false,
		"example.com:80": false,
	} {
		if got := IsLoopback(addr); got != want {
			t.Errorf("IsLoopback(%q) = %v, want %v", addr, got, want)
		}
	}
}

func TestLoadHTTPConfig(t *testing.T) {
	const token = "0123456789abcdef"
	cases := []struct {
		name, addr, token, unauth, origins string
		wantErr                            bool
	}{
		{name: "default loopback, no token", wantErr: false},
		{name: "all interfaces without token is refused", addr: "0.0.0.0:8080", wantErr: true},
		{name: "empty host without token is refused", addr: ":8080", wantErr: true},
		{name: "all interfaces with token", addr: "0.0.0.0:8080", token: token},
		{name: "explicit opt-out", addr: "0.0.0.0:8080", unauth: "true"},
		{name: "short token is refused", addr: "127.0.0.1:8080", token: "short", wantErr: true},
		{name: "origins are split and trimmed", origins: " https://a.example , ,https://b.example"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LISTEN_ADDR", tc.addr)
			t.Setenv("MCP_AUTH_TOKEN", tc.token)
			t.Setenv("MCP_ALLOW_UNAUTHENTICATED", tc.unauth)
			t.Setenv("MCP_ALLOWED_ORIGINS", tc.origins)
			cfg, err := LoadHTTPConfig()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && strings.Contains(err.Error(), token) {
				t.Error("error message echoes the token")
			}
			if tc.addr == "" && cfg.ListenAddr != "127.0.0.1:8080" {
				t.Errorf("default ListenAddr = %q", cfg.ListenAddr)
			}
			if tc.origins != "" && strings.Join(cfg.AllowedOrigins, "|") != "https://a.example|https://b.example" {
				t.Errorf("origins = %v", cfg.AllowedOrigins)
			}
		})
	}
}
