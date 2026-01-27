package agent

import (
	"strings"
	"testing"
)

func TestBuildClaudeCommand(t *testing.T) {
	cmd := BuildClaudeCommand(ClaudeCommandConfig{
		Prompt:     "Review this PR",
		Autonomous: true,
	})

	// Should include claude command
	if cmd == "" {
		t.Error("Command should not be empty")
	}

	// Should include the prompt
	if !strings.Contains(cmd, "Review this PR") {
		t.Error("Command should contain the prompt")
	}
}

func TestBuildClaudeCommand_AutonomousMode(t *testing.T) {
	// Test with autonomous mode enabled
	cmd := BuildClaudeCommand(ClaudeCommandConfig{
		Prompt:     "Fix the bug",
		Autonomous: true,
	})

	if !strings.Contains(cmd, "--dangerously-skip-permissions") {
		t.Error("Autonomous mode should include --dangerously-skip-permissions flag")
	}

	// Test with autonomous mode disabled
	cmdNonAuto := BuildClaudeCommand(ClaudeCommandConfig{
		Prompt:     "Fix the bug",
		Autonomous: false,
	})

	if strings.Contains(cmdNonAuto, "--dangerously-skip-permissions") {
		t.Error("Non-autonomous mode should not include --dangerously-skip-permissions flag")
	}
}

func TestBuildClaudeCommand_PromptEscaping(t *testing.T) {
	// Test that single quotes in prompts are properly escaped
	cmd := BuildClaudeCommand(ClaudeCommandConfig{
		Prompt:     "Review the user's code",
		Autonomous: false,
	})

	// The prompt should be present (escaped form)
	if cmd == "" {
		t.Error("Command should not be empty")
	}

	// Should have the escaped form: 'Review the user'\''s code'
	if !strings.Contains(cmd, "'\\''") {
		t.Error("Single quotes should be escaped using '\\'' pattern for shell safety")
	}
}

func TestBuildClaudeCommand_StartsWithClaude(t *testing.T) {
	cmd := BuildClaudeCommand(ClaudeCommandConfig{
		Prompt:     "Hello",
		Autonomous: false,
	})

	if !strings.HasPrefix(cmd, "claude") {
		t.Error("Command should start with 'claude'")
	}
}

func TestBuildTmuxCommand(t *testing.T) {
	innerCmd := "claude 'Review this PR'"
	sessionName := "agent-123"

	cmd := BuildTmuxCommand(sessionName, innerCmd)

	// Should contain tmux new-session
	if !strings.Contains(cmd, "tmux new-session") {
		t.Error("Should create new tmux session")
	}

	// Should contain the session name
	if !strings.Contains(cmd, sessionName) {
		t.Error("Should contain session name")
	}

	// Should contain the inner command
	if !strings.Contains(cmd, innerCmd) {
		t.Error("Should contain inner command")
	}

	// Should contain tmux wait-for
	if !strings.Contains(cmd, "tmux wait-for") {
		t.Error("Should wait for session to complete")
	}
}

func TestBuildTmuxCommand_DetachedSession(t *testing.T) {
	cmd := BuildTmuxCommand("test-session", "echo hello")

	// Should use -d flag for detached session
	if !strings.Contains(cmd, "-d") {
		t.Error("Should create detached session with -d flag")
	}

	// Should use -s flag for session name
	if !strings.Contains(cmd, "-s") {
		t.Error("Should specify session name with -s flag")
	}
}
