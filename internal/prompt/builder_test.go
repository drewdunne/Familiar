package prompt

import (
	"strings"
	"testing"

	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/event"
	"github.com/drewdunne/familiar/internal/intent"
)

func TestBuilder_Build(t *testing.T) {
	builder := NewBuilder()

	evt := &event.Event{
		Type:         event.TypeMROpened,
		Provider:     "github",
		RepoOwner:    "owner",
		RepoName:     "repo",
		MRNumber:     42,
		SourceBranch: "feature",
		TargetBranch: "main",
	}

	cfg := &config.MergedConfig{
		Prompts: config.PromptsConfig{
			MROpened: "Review this MR",
		},
		Permissions: config.PermissionsConfig{
			Merge:       "on_request",
			PushCommits: "always",
		},
	}

	parsedIntent := &intent.ParsedIntent{
		Instructions:     "Focus on security",
		RequestedActions: []intent.Action{intent.ActionMerge},
	}

	prompt := builder.Build(evt, cfg, parsedIntent)

	if !strings.Contains(prompt, "MR #42") {
		t.Error("Prompt should contain MR number")
	}
	if !strings.Contains(prompt, "Review this MR") {
		t.Error("Prompt should contain base prompt")
	}
	if !strings.Contains(prompt, "Focus on security") {
		t.Error("Prompt should contain user instructions")
	}
	if !strings.Contains(prompt, "MAY merge") {
		t.Error("Prompt should contain merge permission (on_request + requested)")
	}
}

func TestBuilder_Build_NoIntent(t *testing.T) {
	builder := NewBuilder()

	evt := &event.Event{
		Type:         event.TypeMRComment,
		Provider:     "gitlab",
		RepoOwner:    "owner",
		RepoName:     "repo",
		MRNumber:     123,
		SourceBranch: "fix",
		TargetBranch: "main",
	}

	cfg := &config.MergedConfig{
		Prompts: config.PromptsConfig{
			MRComment: "Respond to the comment",
		},
		Permissions: config.PermissionsConfig{
			Merge:       "never",
			PushCommits: "on_request",
		},
	}

	prompt := builder.Build(evt, cfg, nil)

	if !strings.Contains(prompt, "MR #123") {
		t.Error("Prompt should contain MR number")
	}
	if !strings.Contains(prompt, "Respond to the comment") {
		t.Error("Prompt should contain base prompt")
	}
	if !strings.Contains(prompt, "must NOT merge") {
		t.Error("Prompt should indicate merge is not allowed")
	}
}

func TestBuilder_Build_AllEventTypes(t *testing.T) {
	builder := NewBuilder()

	tests := []struct {
		eventType event.Type
		prompt    string
		want      string
	}{
		{event.TypeMROpened, "opened prompt", "opened prompt"},
		{event.TypeMRComment, "comment prompt", "comment prompt"},
		{event.TypeMRUpdated, "updated prompt", "updated prompt"},
		{event.TypeMention, "mention prompt", "mention prompt"},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			evt := &event.Event{
				Type:         tt.eventType,
				MRNumber:     1,
				SourceBranch: "feature",
				TargetBranch: "main",
			}

			cfg := &config.MergedConfig{
				Prompts: config.PromptsConfig{
					MROpened:  "opened prompt",
					MRComment: "comment prompt",
					MRUpdated: "updated prompt",
					Mention:   "mention prompt",
				},
				Permissions: config.PermissionsConfig{
					Merge:       "never",
					PushCommits: "never",
				},
			}

			prompt := builder.Build(evt, cfg, nil)

			if !strings.Contains(prompt, tt.want) {
				t.Errorf("Prompt should contain %q", tt.want)
			}
		})
	}
}
