package config

import (
	"context"
	"testing"
)

type mockFileReader struct {
	content string
	err     error
}

func (m *mockFileReader) ReadFile(ctx context.Context, owner, repo, path, ref string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []byte(m.content), nil
}

func TestLoadRepoConfig(t *testing.T) {
	reader := &mockFileReader{
		content: `
events:
  mr_opened: true
  mr_updated: false
permissions:
  merge: "on_request"
`,
	}

	cfg, err := LoadRepoConfig(context.Background(), reader, "owner", "repo", "main")
	if err != nil {
		t.Fatalf("LoadRepoConfig() error = %v", err)
	}

	if cfg.Events.MROpened != true {
		t.Error("Events.MROpened should be true")
	}
	if cfg.Events.MRUpdated != false {
		t.Error("Events.MRUpdated should be false")
	}
	if cfg.Permissions.Merge != "on_request" {
		t.Errorf("Permissions.Merge = %q, want %q", cfg.Permissions.Merge, "on_request")
	}
}

func TestLoadRepoConfig_NotFound(t *testing.T) {
	reader := &mockFileReader{
		err: ErrConfigNotFound,
	}

	cfg, err := LoadRepoConfig(context.Background(), reader, "owner", "repo", "main")
	if err != nil {
		t.Fatalf("LoadRepoConfig() should not error for missing config, got: %v", err)
	}

	// Should return empty config
	if cfg == nil {
		t.Error("Should return empty config, not nil")
	}
}
