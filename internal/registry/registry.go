package registry

import (
	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/provider"
	"github.com/drewdunne/familiar/internal/provider/github"
	"github.com/drewdunne/familiar/internal/provider/gitlab"
)

// Registry manages provider instances.
type Registry struct {
	providers map[string]provider.Provider
}

// New creates a new provider registry from config.
func New(cfg *config.Config) *Registry {
	r := &Registry{
		providers: make(map[string]provider.Provider),
	}

	if cfg.Providers.GitHub.Token != "" {
		r.providers["github"] = github.New(cfg.Providers.GitHub.Token)
	}

	if cfg.Providers.GitLab.Token != "" {
		r.providers["gitlab"] = gitlab.New(cfg.Providers.GitLab.Token)
	}

	return r
}

// Get returns the provider for the given name, or nil if not configured.
func (r *Registry) Get(name string) provider.Provider {
	return r.providers[name]
}

// List returns all configured provider names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
