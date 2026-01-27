package handler

import (
	"context"
	"fmt"
	"log"

	"github.com/drewdunne/familiar/internal/agent"
	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/event"
	"github.com/drewdunne/familiar/internal/intent"
	"github.com/drewdunne/familiar/internal/repocache"
)

// AgentHandler handles events by spawning agents.
type AgentHandler struct {
	spawner   *agent.Spawner
	repoCache *repocache.Cache
}

// NewAgentHandler creates a new agent handler.
func NewAgentHandler(spawner *agent.Spawner, repoCache *repocache.Cache) *AgentHandler {
	return &AgentHandler{
		spawner:   spawner,
		repoCache: repoCache,
	}
}

// Handle processes an event by spawning an agent.
func (h *AgentHandler) Handle(ctx context.Context, evt *event.Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
	// Generate unique agent ID
	agentID := fmt.Sprintf("%s-%s-%d-%d", evt.Provider, evt.RepoName, evt.MRNumber, evt.Timestamp.Unix())

	// Ensure repo is cached and create worktree
	_, err := h.repoCache.EnsureRepo(ctx, evt.RepoURL, evt.RepoOwner, evt.RepoName)
	if err != nil {
		return fmt.Errorf("ensuring repo: %w", err)
	}

	worktreePath, err := h.repoCache.CreateWorktree(ctx, evt.RepoOwner, evt.RepoName, evt.SourceBranch, agentID)
	if err != nil {
		return fmt.Errorf("creating worktree: %w", err)
	}

	// Build prompt (Phase 6 will enhance this)
	prompt := buildPrompt(evt, cfg, parsedIntent)

	// Spawn agent
	_, err = h.spawner.Spawn(ctx, agent.SpawnRequest{
		ID:           agentID,
		WorktreePath: worktreePath,
		WorkDir:      "/workspace",
		Prompt:       prompt,
	})
	if err != nil {
		// Cleanup worktree on failure
		h.repoCache.RemoveWorktree(ctx, evt.RepoOwner, evt.RepoName, agentID)
		return fmt.Errorf("spawning agent: %w", err)
	}

	log.Printf("Spawned agent %s for %s/%s MR #%d", agentID, evt.RepoOwner, evt.RepoName, evt.MRNumber)
	return nil
}

func buildPrompt(evt *event.Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) string {
	// Simple prompt for now - Phase 6 will build full prompt
	var prompt string

	switch evt.Type {
	case event.TypeMROpened:
		prompt = cfg.Prompts.MROpened
	case event.TypeMRComment:
		prompt = cfg.Prompts.MRComment
	case event.TypeMRUpdated:
		prompt = cfg.Prompts.MRUpdated
	case event.TypeMention:
		prompt = cfg.Prompts.Mention
	}

	if parsedIntent != nil && parsedIntent.Instructions != "" {
		prompt += "\n\nUser instructions: " + parsedIntent.Instructions
	}

	return prompt
}
