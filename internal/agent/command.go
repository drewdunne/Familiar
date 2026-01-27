package agent

import (
	"fmt"
	"strings"
)

// ClaudeCommandConfig holds configuration for the Claude command.
type ClaudeCommandConfig struct {
	Prompt     string
	WorkDir    string
	Autonomous bool
}

// BuildClaudeCommand builds the command to run Claude Code in the container.
func BuildClaudeCommand(cfg ClaudeCommandConfig) string {
	// Escape the prompt for shell
	// Pattern: replace ' with '"'"' (end quote, double-quoted quote, start quote)
	escapedPrompt := strings.ReplaceAll(cfg.Prompt, "'", "'\"'\"'")

	var args []string
	args = append(args, "claude")

	if cfg.Autonomous {
		// Run in autonomous mode without permission prompts
		args = append(args, "--dangerously-skip-permissions")
	}

	// Add the prompt
	args = append(args, fmt.Sprintf("'%s'", escapedPrompt))

	return strings.Join(args, " ")
}

// BuildTmuxCommand wraps a command in a tmux session.
func BuildTmuxCommand(sessionName, innerCmd string) string {
	return fmt.Sprintf(
		"tmux new-session -d -s %s '%s' && tmux wait-for %s",
		sessionName,
		innerCmd,
		sessionName,
	)
}
