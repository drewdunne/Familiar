package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Config represents the server configuration.
type Config struct {
	Server      ServerConfig            `yaml:"server"`
	Logging     LoggingConfig           `yaml:"logging"`
	Providers   ProvidersConfig         `yaml:"providers"`
	Events      ServerEventsConfig      `yaml:"events"`
	Permissions ServerPermissionsConfig `yaml:"permissions"`
	Prompts     ServerPromptsConfig     `yaml:"prompts"`
	Agents      AgentsConfig            `yaml:"agents"`
}

// ServerEventsConfig controls which events are enabled at server level.
type ServerEventsConfig struct {
	MROpened  bool `yaml:"mr_opened"`
	MRComment bool `yaml:"mr_comment"`
	MRUpdated bool `yaml:"mr_updated"`
	Mention   bool `yaml:"mention"`
}

// ServerPermissionsConfig controls default agent permissions.
type ServerPermissionsConfig struct {
	Merge          string `yaml:"merge"`
	Approve        string `yaml:"approve"`
	PushCommits    string `yaml:"push_commits"`
	DismissReviews string `yaml:"dismiss_reviews"`
}

// ServerPromptsConfig holds default prompts per event type.
type ServerPromptsConfig struct {
	MROpened  string `yaml:"mr_opened"`
	MRComment string `yaml:"mr_comment"`
	MRUpdated string `yaml:"mr_updated"`
	Mention   string `yaml:"mention"`
}

// AgentsConfig holds agent settings.
type AgentsConfig struct {
	TimeoutMinutes  int `yaml:"timeout_minutes"`
	DebounceSeconds int `yaml:"debounce_seconds"`
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

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 7000,
		},
		Logging: LoggingConfig{
			Dir:           "/var/log/familiar",
			RetentionDays: 30,
		},
	}
}

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

	// Start with defaults
	cfg := DefaultConfig()

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return cfg, nil
}
