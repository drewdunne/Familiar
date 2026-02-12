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
	parts = append(parts, b.getBasePrompt(evt.Type, cfg, evt))

	// User instructions
	if parsedIntent != nil && parsedIntent.Instructions != "" {
		parts = append(parts, fmt.Sprintf("## User Instructions\n%s", parsedIntent.Instructions))
	}

	// Permissions
	parts = append(parts, b.buildPermissions(evt, cfg, parsedIntent))

	// Safety reminders
	parts = append(parts, b.buildSafetyReminders())

	return strings.Join(parts, "\n\n")
}

func (b *Builder) buildContext(evt *event.Event) string {
	ctx := fmt.Sprintf(`## Context
- Repository: %s/%s
- MR #%d: %s → %s
- Provider: %s`,
		evt.RepoOwner, evt.RepoName,
		evt.MRNumber, evt.SourceBranch, evt.TargetBranch,
		evt.Provider)

	if evt.MRTitle != "" {
		ctx += fmt.Sprintf("\n- Title: %s", evt.MRTitle)
	}
	if evt.MRDescription != "" {
		ctx += fmt.Sprintf("\n\n## MR Description\n%s", evt.MRDescription)
	}
	if evt.CommentBody != "" {
		ctx += fmt.Sprintf("\n\n## Comment\n**@%s:**\n%s", evt.CommentAuthor, evt.CommentBody)
		if evt.CommentFilePath != "" {
			ctx += fmt.Sprintf("\n\n**File:** `%s`", evt.CommentFilePath)
			if evt.CommentLine > 0 {
				ctx += fmt.Sprintf(" (line %d)", evt.CommentLine)
			}
			ctx += "\n\nThis comment was left on a specific line of code. The reviewer is referencing this exact location."
		}
		if evt.CommentDiscussionID != "" {
			ctx += fmt.Sprintf("\n\n**Discussion ID:** %s\nReply to this discussion thread, not as a top-level MR comment.", evt.CommentDiscussionID)
		}
	}

	return ctx
}

func (b *Builder) getBasePrompt(t event.Type, cfg *config.MergedConfig, evt *event.Event) string {
	var prompt string
	switch t {
	case event.TypeMROpened:
		prompt = cfg.Prompts.MROpened
	case event.TypeMRComment:
		prompt = cfg.Prompts.MRComment
	case event.TypeMRUpdated:
		prompt = cfg.Prompts.MRUpdated
	case event.TypeMention:
		prompt = cfg.Prompts.Mention
	default:
		prompt = ""
	}
	// Substitute placeholders
	prompt = strings.ReplaceAll(prompt, "{MR_NUMBER}", fmt.Sprintf("%d", evt.MRNumber))
	prompt = strings.ReplaceAll(prompt, "{REPO_OWNER}", evt.RepoOwner)
	prompt = strings.ReplaceAll(prompt, "{REPO_NAME}", evt.RepoName)
	return prompt
}

func (b *Builder) buildPermissions(evt *event.Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) string {
	var perms []string
	perms = append(perms, "## Permissions")

	// Push commits
	switch cfg.Permissions.PushCommits {
	case "always":
		perms = append(perms, "- You SHOULD push commits when needed")
	case "on_request":
		explicitPush := parsedIntent != nil && (parsedIntent.HasAction(intent.ActionMerge) || parsedIntent.HasAction(intent.ActionPush))
		// Any MR-related event may require code changes — grant push.
		// Comments and mentions may ask for changes (or the thread may contain
		// prior requests); review events (opened/updated) imply the agent may
		// need to fix issues it finds.
		mrEvent := evt != nil && (evt.Type == event.TypeMROpened || evt.Type == event.TypeMRUpdated ||
			evt.Type == event.TypeMRComment || evt.Type == event.TypeMention)
		if explicitPush || mrEvent {
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
