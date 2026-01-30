package registry

import (
	"testing"

	"github.com/drewdunne/familiar/internal/config"
)

func TestRegistry_Get(t *testing.T) {
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			GitHub: config.GitHubConfig{Token: "gh-token"},
			GitLab: config.GitLabConfig{Token: "gl-token"},
		},
	}

	reg := New(cfg)

	gh := reg.Get("github")
	if gh == nil {
		t.Error("Get(github) returned nil")
	}
	if gh.Name() != "github" {
		t.Errorf("github provider name = %q, want %q", gh.Name(), "github")
	}

	gl := reg.Get("gitlab")
	if gl == nil {
		t.Error("Get(gitlab) returned nil")
	}
	if gl.Name() != "gitlab" {
		t.Errorf("gitlab provider name = %q, want %q", gl.Name(), "gitlab")
	}

	unknown := reg.Get("unknown")
	if unknown != nil {
		t.Error("Get(unknown) should return nil")
	}
}

func TestRegistry_GitLabWithBaseURL(t *testing.T) {
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			GitLab: config.GitLabConfig{
				Token:   "gl-token",
				BaseURL: "https://gitlab.example.com",
			},
		},
	}

	reg := New(cfg)

	gl := reg.Get("gitlab")
	if gl == nil {
		t.Fatal("Get(gitlab) returned nil when base_url is configured")
	}
	if gl.Name() != "gitlab" {
		t.Errorf("gitlab provider name = %q, want %q", gl.Name(), "gitlab")
	}
}

func TestRegistry_List(t *testing.T) {
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			GitHub: config.GitHubConfig{Token: "gh-token"},
		},
	}

	reg := New(cfg)
	names := reg.List()

	if len(names) != 1 {
		t.Errorf("List() returned %d providers, want 1", len(names))
	}
	if names[0] != "github" {
		t.Errorf("List()[0] = %q, want %q", names[0], "github")
	}
}
