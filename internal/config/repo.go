package config

import (
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ErrConfigNotFound indicates the repo config file doesn't exist.
var ErrConfigNotFound = errors.New("config not found")

// RepoConfig represents repository-level configuration.
type RepoConfig struct {
	Events      EventsConfig      `yaml:"events"`
	Permissions PermissionsConfig `yaml:"permissions"`
	Prompts     PromptsConfig     `yaml:"prompts"`
	AgentImage  string            `yaml:"agent_image"`
}

// EventsConfig controls which events are enabled.
type EventsConfig struct {
	MROpened  bool `yaml:"mr_opened"`
	MRComment bool `yaml:"mr_comment"`
	MRUpdated bool `yaml:"mr_updated"`
	Mention   bool `yaml:"mention"`
}

// PermissionsConfig controls agent permissions.
type PermissionsConfig struct {
	Merge          string `yaml:"merge"`
	Approve        string `yaml:"approve"`
	PushCommits    string `yaml:"push_commits"`
	DismissReviews string `yaml:"dismiss_reviews"`
}

// PromptsConfig holds custom prompts per event type.
type PromptsConfig struct {
	MROpened  string `yaml:"mr_opened"`
	MRComment string `yaml:"mr_comment"`
	MRUpdated string `yaml:"mr_updated"`
	Mention   string `yaml:"mention"`
}

// FileReader reads files from a repository.
type FileReader interface {
	ReadFile(ctx context.Context, owner, repo, path, ref string) ([]byte, error)
}

// LoadRepoConfig loads the repo config from .familiar/config.yaml.
func LoadRepoConfig(ctx context.Context, reader FileReader, owner, repo, ref string) (*RepoConfig, error) {
	data, err := reader.ReadFile(ctx, owner, repo, ".familiar/config.yaml", ref)
	if errors.Is(err, ErrConfigNotFound) {
		return &RepoConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading repo config: %w", err)
	}

	var cfg RepoConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing repo config: %w", err)
	}

	return &cfg, nil
}
