package config

import (
	"os"

	"github.com/joho/godotenv"
)

func init() {
	// Load .env in the working directory; ignore error if the file is absent.
	_ = godotenv.Load()
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
