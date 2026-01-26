package config

// MergedConfig represents the final merged configuration.
type MergedConfig struct {
	Prompts     PromptsConfig
	Permissions PermissionsConfig
	Events      EventsConfig
	AgentImage  string
}

// MergeConfigs merges server config with repo config.
// Repo config values take precedence over server defaults.
func MergeConfigs(server *Config, repo *RepoConfig) *MergedConfig {
	merged := &MergedConfig{}

	// Merge prompts (repo overrides if non-empty)
	merged.Prompts.MROpened = coalesce(repo.Prompts.MROpened, server.Prompts.MROpened)
	merged.Prompts.MRComment = coalesce(repo.Prompts.MRComment, server.Prompts.MRComment)
	merged.Prompts.MRUpdated = coalesce(repo.Prompts.MRUpdated, server.Prompts.MRUpdated)
	merged.Prompts.Mention = coalesce(repo.Prompts.Mention, server.Prompts.Mention)

	// Merge permissions (repo overrides if non-empty)
	merged.Permissions.Merge = coalesce(repo.Permissions.Merge, server.Permissions.Merge)
	merged.Permissions.Approve = coalesce(repo.Permissions.Approve, server.Permissions.Approve)
	merged.Permissions.PushCommits = coalesce(repo.Permissions.PushCommits, server.Permissions.PushCommits)
	merged.Permissions.DismissReviews = coalesce(repo.Permissions.DismissReviews, server.Permissions.DismissReviews)

	// Merge events - use repo value if explicitly set, otherwise use server
	merged.Events.MROpened = repo.Events.MROpened || server.Events.MROpened
	merged.Events.MRComment = repo.Events.MRComment || server.Events.MRComment
	merged.Events.MRUpdated = repo.Events.MRUpdated || server.Events.MRUpdated
	merged.Events.Mention = repo.Events.Mention || server.Events.Mention

	// Agent image
	merged.AgentImage = coalesce(repo.AgentImage, "")

	return merged
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
