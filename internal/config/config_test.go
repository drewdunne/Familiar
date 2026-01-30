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

func TestLoadConfig_EnvVarSubstitution(t *testing.T) {
	// Set env var for test
	os.Setenv("TEST_SECRET_TOKEN", "my-secret-value")
	defer os.Unsetenv("TEST_SECRET_TOKEN")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080

providers:
  github:
    token: "${TEST_SECRET_TOKEN}"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Providers.GitHub.Token != "my-secret-value" {
		t.Errorf("Providers.GitHub.Token = %q, want %q", cfg.Providers.GitHub.Token, "my-secret-value")
	}
}

func TestLoadConfig_EnvVarNotSet(t *testing.T) {
	// Ensure env var is not set
	os.Unsetenv("NONEXISTENT_VAR")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080

providers:
  github:
    token: "${NONEXISTENT_VAR}"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should be empty string when env var not set
	if cfg.Providers.GitHub.Token != "" {
		t.Errorf("Providers.GitHub.Token = %q, want empty string", cfg.Providers.GitHub.Token)
	}
}

func TestLoadConfig_GitLabBaseURL(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
providers:
  gitlab:
    token: "gl-token"
    base_url: "https://gitlab.example.com"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Providers.GitLab.BaseURL != "https://gitlab.example.com" {
		t.Errorf("Providers.GitLab.BaseURL = %q, want %q", cfg.Providers.GitLab.BaseURL, "https://gitlab.example.com")
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Minimal config - should fill in defaults
	configContent := `
server:
  port: 9000
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Explicit value should be preserved
	if cfg.Server.Port != 9000 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 9000)
	}

	// Default values should be applied
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want default %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Logging.RetentionDays != 30 {
		t.Errorf("Logging.RetentionDays = %d, want default %d", cfg.Logging.RetentionDays, 30)
	}
}
