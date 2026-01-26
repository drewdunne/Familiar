package config

import "testing"

func TestMergeConfigs(t *testing.T) {
	server := &Config{
		Prompts: ServerPromptsConfig{
			MROpened: "Server default prompt",
		},
		Permissions: ServerPermissionsConfig{
			Merge:       "never",
			PushCommits: "on_request",
		},
		Events: ServerEventsConfig{
			MROpened:  true,
			MRComment: true,
			MRUpdated: true,
			Mention:   true,
		},
	}

	repo := &RepoConfig{
		Prompts: PromptsConfig{
			MROpened: "Repo custom prompt",
		},
		Permissions: PermissionsConfig{
			Merge: "on_request", // Override
		},
		Events: EventsConfig{
			MRUpdated: false, // Disable
		},
	}

	merged := MergeConfigs(server, repo)

	// Repo prompt should override
	if merged.Prompts.MROpened != "Repo custom prompt" {
		t.Errorf("Prompts.MROpened = %q, want repo override", merged.Prompts.MROpened)
	}

	// Repo permission should override
	if merged.Permissions.Merge != "on_request" {
		t.Errorf("Permissions.Merge = %q, want %q", merged.Permissions.Merge, "on_request")
	}

	// Server default should remain where repo doesn't override
	if merged.Permissions.PushCommits != "on_request" {
		t.Errorf("Permissions.PushCommits = %q, want server default", merged.Permissions.PushCommits)
	}
}

func TestMergeConfigs_EmptyRepo(t *testing.T) {
	server := &Config{
		Prompts: ServerPromptsConfig{
			MROpened: "Server prompt",
		},
		Permissions: ServerPermissionsConfig{
			Merge: "never",
		},
		Events: ServerEventsConfig{
			MROpened: true,
		},
	}

	repo := &RepoConfig{} // Empty repo config

	merged := MergeConfigs(server, repo)

	// Should use server defaults
	if merged.Prompts.MROpened != "Server prompt" {
		t.Errorf("Prompts.MROpened = %q, want server default", merged.Prompts.MROpened)
	}
	if merged.Permissions.Merge != "never" {
		t.Errorf("Permissions.Merge = %q, want server default", merged.Permissions.Merge)
	}
}
