package handler

import (
	"testing"
	"time"

	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/event"
	"github.com/drewdunne/familiar/internal/intent"
)

func TestNewAgentHandler(t *testing.T) {
	// Test that NewAgentHandler returns a properly initialized handler
	handler := NewAgentHandler(nil, nil)

	if handler == nil {
		t.Fatal("NewAgentHandler() returned nil")
	}

	if handler.spawner != nil {
		t.Error("handler.spawner should be nil when nil is passed")
	}

	if handler.repoCache != nil {
		t.Error("handler.repoCache should be nil when nil is passed")
	}
}

func TestBuildPrompt_MROpened(t *testing.T) {
	evt := &event.Event{
		Type:      event.TypeMROpened,
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  42,
		Timestamp: time.Now(),
	}

	cfg := &config.MergedConfig{
		Prompts: config.PromptsConfig{
			MROpened: "Review this MR",
		},
	}

	prompt := buildPrompt(evt, cfg, nil)

	if prompt != "Review this MR" {
		t.Errorf("buildPrompt() = %q, want %q", prompt, "Review this MR")
	}
}

func TestBuildPrompt_MRComment(t *testing.T) {
	evt := &event.Event{
		Type:        event.TypeMRComment,
		Provider:    "github",
		RepoOwner:   "owner",
		RepoName:    "repo",
		MRNumber:    42,
		CommentBody: "@familiar help me",
		Timestamp:   time.Now(),
	}

	cfg := &config.MergedConfig{
		Prompts: config.PromptsConfig{
			MRComment: "Handle comment",
		},
	}

	prompt := buildPrompt(evt, cfg, nil)

	if prompt != "Handle comment" {
		t.Errorf("buildPrompt() = %q, want %q", prompt, "Handle comment")
	}
}

func TestBuildPrompt_MRUpdated(t *testing.T) {
	evt := &event.Event{
		Type:      event.TypeMRUpdated,
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  42,
		Timestamp: time.Now(),
	}

	cfg := &config.MergedConfig{
		Prompts: config.PromptsConfig{
			MRUpdated: "Review updated MR",
		},
	}

	prompt := buildPrompt(evt, cfg, nil)

	if prompt != "Review updated MR" {
		t.Errorf("buildPrompt() = %q, want %q", prompt, "Review updated MR")
	}
}

func TestBuildPrompt_Mention(t *testing.T) {
	evt := &event.Event{
		Type:        event.TypeMention,
		Provider:    "github",
		RepoOwner:   "owner",
		RepoName:    "repo",
		MRNumber:    42,
		CommentBody: "@familiar assist",
		Timestamp:   time.Now(),
	}

	cfg := &config.MergedConfig{
		Prompts: config.PromptsConfig{
			Mention: "You were mentioned",
		},
	}

	prompt := buildPrompt(evt, cfg, nil)

	if prompt != "You were mentioned" {
		t.Errorf("buildPrompt() = %q, want %q", prompt, "You were mentioned")
	}
}

func TestBuildPrompt_WithIntent(t *testing.T) {
	evt := &event.Event{
		Type:        event.TypeMRComment,
		Provider:    "github",
		RepoOwner:   "owner",
		RepoName:    "repo",
		MRNumber:    42,
		CommentBody: "@familiar review and approve",
		Timestamp:   time.Now(),
	}

	cfg := &config.MergedConfig{
		Prompts: config.PromptsConfig{
			MRComment: "Base prompt",
		},
	}

	parsedIntent := &intent.ParsedIntent{
		Instructions:     "Please review and approve this PR",
		RequestedActions: []intent.Action{intent.ActionApprove},
		Confidence:       0.95,
		Raw:              "@familiar review and approve",
	}

	prompt := buildPrompt(evt, cfg, parsedIntent)

	expected := "Base prompt\n\nUser instructions: Please review and approve this PR"
	if prompt != expected {
		t.Errorf("buildPrompt() = %q, want %q", prompt, expected)
	}
}

func TestBuildPrompt_WithEmptyIntent(t *testing.T) {
	evt := &event.Event{
		Type:      event.TypeMRComment,
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  42,
		Timestamp: time.Now(),
	}

	cfg := &config.MergedConfig{
		Prompts: config.PromptsConfig{
			MRComment: "Base prompt",
		},
	}

	// Intent with empty instructions
	parsedIntent := &intent.ParsedIntent{
		Instructions: "",
		Confidence:   0.5,
	}

	prompt := buildPrompt(evt, cfg, parsedIntent)

	// Should not append empty instructions
	if prompt != "Base prompt" {
		t.Errorf("buildPrompt() = %q, want %q", prompt, "Base prompt")
	}
}

func TestBuildPrompt_UnknownEventType(t *testing.T) {
	evt := &event.Event{
		Type:      event.Type("unknown"),
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  42,
		Timestamp: time.Now(),
	}

	cfg := &config.MergedConfig{
		Prompts: config.PromptsConfig{
			MROpened:  "MR opened",
			MRComment: "MR comment",
			MRUpdated: "MR updated",
			Mention:   "Mention",
		},
	}

	prompt := buildPrompt(evt, cfg, nil)

	// Unknown event type should result in empty prompt
	if prompt != "" {
		t.Errorf("buildPrompt() = %q, want empty string for unknown event type", prompt)
	}
}

func TestBuildPrompt_AllEventTypesWithIntent(t *testing.T) {
	tests := []struct {
		name        string
		eventType   event.Type
		basePrompt  string
		promptField func(cfg *config.MergedConfig) string
	}{
		{
			name:       "MROpened",
			eventType:  event.TypeMROpened,
			basePrompt: "MR opened prompt",
			promptField: func(cfg *config.MergedConfig) string {
				cfg.Prompts.MROpened = "MR opened prompt"
				return cfg.Prompts.MROpened
			},
		},
		{
			name:       "MRComment",
			eventType:  event.TypeMRComment,
			basePrompt: "MR comment prompt",
			promptField: func(cfg *config.MergedConfig) string {
				cfg.Prompts.MRComment = "MR comment prompt"
				return cfg.Prompts.MRComment
			},
		},
		{
			name:       "MRUpdated",
			eventType:  event.TypeMRUpdated,
			basePrompt: "MR updated prompt",
			promptField: func(cfg *config.MergedConfig) string {
				cfg.Prompts.MRUpdated = "MR updated prompt"
				return cfg.Prompts.MRUpdated
			},
		},
		{
			name:       "Mention",
			eventType:  event.TypeMention,
			basePrompt: "Mention prompt",
			promptField: func(cfg *config.MergedConfig) string {
				cfg.Prompts.Mention = "Mention prompt"
				return cfg.Prompts.Mention
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			evt := &event.Event{
				Type:      tc.eventType,
				Provider:  "github",
				RepoOwner: "owner",
				RepoName:  "repo",
				MRNumber:  42,
				Timestamp: time.Now(),
			}

			cfg := &config.MergedConfig{}
			tc.promptField(cfg)

			parsedIntent := &intent.ParsedIntent{
				Instructions: "User request",
				Confidence:   0.9,
			}

			prompt := buildPrompt(evt, cfg, parsedIntent)

			expected := tc.basePrompt + "\n\nUser instructions: User request"
			if prompt != expected {
				t.Errorf("buildPrompt() = %q, want %q", prompt, expected)
			}
		})
	}
}

func TestBuildPrompt_NilConfig(t *testing.T) {
	evt := &event.Event{
		Type:      event.TypeMROpened,
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  42,
		Timestamp: time.Now(),
	}

	// This should not panic with empty config
	cfg := &config.MergedConfig{}

	prompt := buildPrompt(evt, cfg, nil)

	if prompt != "" {
		t.Errorf("buildPrompt() = %q, want empty string with empty prompts", prompt)
	}
}
