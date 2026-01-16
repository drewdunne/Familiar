package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ValidFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080

logging:
  dir: "/var/log/familiar"
  retention_days: 30
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 8080)
	}
	if cfg.Logging.Dir != "/var/log/familiar" {
		t.Errorf("Logging.Dir = %q, want %q", cfg.Logging.Dir, "/var/log/familiar")
	}
	if cfg.Logging.RetentionDays != 30 {
		t.Errorf("Logging.RetentionDays = %d, want %d", cfg.Logging.RetentionDays, 30)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Load() expected error for nonexistent file, got nil")
	}
}
