package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Config represents the server configuration.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Logging   LoggingConfig   `yaml:"logging"`
	Providers ProvidersConfig `yaml:"providers"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Dir           string `yaml:"dir"`
	RetentionDays int    `yaml:"retention_days"`
}

// ProvidersConfig holds git provider configurations.
type ProvidersConfig struct {
	GitHub GitHubConfig `yaml:"github"`
	GitLab GitLabConfig `yaml:"gitlab"`
}

// GitHubConfig holds GitHub-specific settings.
type GitHubConfig struct {
	AuthMethod    string `yaml:"auth_method"`
	Token         string `yaml:"token"`
	WebhookSecret string `yaml:"webhook_secret"`
}

// GitLabConfig holds GitLab-specific settings.
type GitLabConfig struct {
	AuthMethod    string `yaml:"auth_method"`
	Token         string `yaml:"token"`
	WebhookSecret string `yaml:"webhook_secret"`
}

// envVarPattern matches ${VAR_NAME} patterns.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// Load reads and parses the config file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Substitute environment variables
	data = envVarPattern.ReplaceAllFunc(data, func(match []byte) []byte {
		varName := envVarPattern.FindSubmatch(match)[1]
		return []byte(os.Getenv(string(varName)))
	})

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}
