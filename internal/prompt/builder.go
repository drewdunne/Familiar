package prompt

import (
	"fmt"
	"strings"

	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/event"
	"github.com/drewdunne/familiar/internal/intent"
)

// Builder constructs prompts for Claude agents.
type Builder struct{}

// NewBuilder creates a new prompt builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// Build constructs a full prompt for the given event and configuration.
func (b *Builder) Build(evt *event.Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) string {
	var parts []string

	// System context
	parts = append(parts, b.buildContext(evt))

	// Base prompt
	parts = append(parts, b.getBasePrompt(evt.Type, cfg))

	// User instructions
	if parsedIntent != nil && parsedIntent.Instructions != "" {
		parts = append(parts, fmt.Sprintf("## User Instructions\n%s", parsedIntent.Instructions))
	}

	// Permissions
	parts = append(parts, b.buildPermissions(cfg, parsedIntent))

	// Safety reminders
	parts = append(parts, b.buildSafetyReminders())

	return strings.Join(parts, "\n\n")
}

func (b *Builder) buildContext(evt *event.Event) string {
	return fmt.Sprintf(`## Context
- Repository: %s/%s
- MR #%d: %s → %s
- Provider: %s`,
		evt.RepoOwner, evt.RepoName,
		evt.MRNumber, evt.SourceBranch, evt.TargetBranch,
		evt.Provider)
}

func (b *Builder) getBasePrompt(t event.Type, cfg *config.MergedConfig) string {
	switch t {
	case event.TypeMROpened:
		return cfg.Prompts.MROpened
	case event.TypeMRComment:
		return cfg.Prompts.MRComment
	case event.TypeMRUpdated:
		return cfg.Prompts.MRUpdated
	case event.TypeMention:
		return cfg.Prompts.Mention
	default:
		return ""
	}
}

func (b *Builder) buildPermissions(cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) string {
	var perms []string
	perms = append(perms, "## Permissions")

	// Push commits
	switch cfg.Permissions.PushCommits {
	case "always":
		perms = append(perms, "- You SHOULD push commits when needed")
	case "on_request":
		if parsedIntent != nil && parsedIntent.HasAction(intent.ActionMerge) {
			perms = append(perms, "- You MAY push commits")
		} else {
			perms = append(perms, "- You must NOT push commits (not requested)")
		}
	case "never":
		perms = append(perms, "- You must NOT push commits")
	}

	// Merge
	switch cfg.Permissions.Merge {
	case "always":
		perms = append(perms, "- You SHOULD merge when appropriate")
	case "on_request":
		if parsedIntent != nil && parsedIntent.HasAction(intent.ActionMerge) {
			perms = append(perms, "- You MAY merge this MR")
		} else {
			perms = append(perms, "- You must NOT merge (not requested)")
		}
	case "never":
		perms = append(perms, "- You must NOT merge")
	}

	return strings.Join(perms, "\n")
}

func (b *Builder) buildSafetyReminders() string {
	return `## Safety
- Branch protection is enabled; destructive actions will be rejected
- Never force push or push to protected branches
- If uncertain, ask via comment rather than taking action`
}
