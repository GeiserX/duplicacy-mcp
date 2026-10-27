package config

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func init() {
	// Skip .env loading in stdio mode to avoid picking up unrelated project
	// .env files when running via npx or as a global binary.
	if strings.ToLower(os.Getenv("TRANSPORT")) != "stdio" {
		_ = godotenv.Load()
	}
}

// DuplicacyConfig is the configuration shared by both transports.
type DuplicacyConfig struct {
	ExporterURL string
	// Transport is "stdio" or "http" (the default).
	Transport string
}

// LoadDuplicacyConfig reads the configuration from the environment.
func LoadDuplicacyConfig() DuplicacyConfig {
	transport := "http"
	if strings.ToLower(os.Getenv("TRANSPORT")) == "stdio" {
		transport = "stdio"
	}
	return DuplicacyConfig{
		ExporterURL: getEnv("DUPLICACY_EXPORTER_URL", "http://localhost:9750"),
		Transport:   transport,
	}
}

// Validate checks that the exporter URL is an absolute http or https URL.
func (c DuplicacyConfig) Validate() error {
	u, err := url.Parse(c.ExporterURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("DUPLICACY_EXPORTER_URL must be an absolute http:// or https:// URL, for example http://localhost:9750")
	}
	return nil
}

// MinAuthTokenLength is the shortest MCP_AUTH_TOKEN accepted.
const MinAuthTokenLength = 16

// HTTPConfig configures the Streamable HTTP transport.
type HTTPConfig struct {
	ListenAddr string
	// AuthToken, when set, is required as "Authorization: Bearer <token>" on
	// every request. It is mandatory when ListenAddr is not loopback.
	AuthToken string
	// AllowedOrigins are extra browser origins (scheme://host[:port]) allowed
	// to call the endpoint cross-origin. Same-origin and non-browser clients
	// need no entry.
	AllowedOrigins []string
	// AllowUnauthenticated lets a non-loopback bind start without a token, for
	// deployments where a reverse proxy in front does the authentication.
	AllowUnauthenticated bool
}

// LoadHTTPConfig reads the HTTP settings and refuses an unsafe combination:
// a listen address reachable from other hosts with no token.
func LoadHTTPConfig() (HTTPConfig, error) {
	cfg := HTTPConfig{
		ListenAddr:           getEnv("LISTEN_ADDR", "127.0.0.1:8080"),
		AuthToken:            strings.TrimSpace(os.Getenv("MCP_AUTH_TOKEN")),
		AllowUnauthenticated: strings.EqualFold(os.Getenv("MCP_ALLOW_UNAUTHENTICATED"), "true"),
	}
	for _, o := range strings.Split(os.Getenv("MCP_ALLOWED_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, o)
		}
	}
	if cfg.AuthToken != "" && len(cfg.AuthToken) < MinAuthTokenLength {
		return cfg, fmt.Errorf("MCP_AUTH_TOKEN must be at least %d characters; generate one with: openssl rand -hex 32", MinAuthTokenLength)
	}
	if cfg.AuthToken == "" && !cfg.AllowUnauthenticated && !IsLoopback(cfg.ListenAddr) {
		return cfg, fmt.Errorf("LISTEN_ADDR %q is reachable from other hosts, so MCP_AUTH_TOKEN is required "+
			"(generate one with: openssl rand -hex 32). Set MCP_ALLOW_UNAUTHENTICATED=true only if a reverse proxy "+
			"in front authenticates every request", cfg.ListenAddr)
	}
	return cfg, nil
}

// IsLoopback reports whether a listen address only accepts connections from
// this host. An empty host (":8080") listens on every interface.
func IsLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = strings.Trim(addr, "[]")
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
