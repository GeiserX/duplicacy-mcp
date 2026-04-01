package config

import (
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

type DuplicacyConfig struct {
	ExporterURL string
}

func LoadDuplicacyConfig() DuplicacyConfig {
	return DuplicacyConfig{
		ExporterURL: getEnv("DUPLICACY_EXPORTER_URL", "http://localhost:9750"),
	}
}

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
