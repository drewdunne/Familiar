package handler

import (
	"context"
	"fmt"
	"log"

	"github.com/drewdunne/familiar/internal/agent"
	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/event"
	"github.com/drewdunne/familiar/internal/intent"
	"github.com/drewdunne/familiar/internal/lca"
	"github.com/drewdunne/familiar/internal/prompt"
	"github.com/drewdunne/familiar/internal/registry"
	"github.com/drewdunne/familiar/internal/repocache"
)

// AgentHandler handles events by spawning agents.
type AgentHandler struct {
	spawner       *agent.Spawner
	repoCache     *repocache.Cache
	registry      *registry.Registry
	promptBuilder *prompt.Builder
}

// NewAgentHandler creates a new agent handler.
func NewAgentHandler(spawner *agent.Spawner, repoCache *repocache.Cache, reg *registry.Registry) *AgentHandler {
	return &AgentHandler{
		spawner:       spawner,
		repoCache:     repoCache,
		registry:      reg,
		promptBuilder: prompt.NewBuilder(),
	}
}

// Handle processes an event by spawning an agent.
func (h *AgentHandler) Handle(ctx context.Context, evt *event.Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
	// Generate unique agent ID
	agentID := fmt.Sprintf("%s-%s-%d-%d", evt.Provider, evt.RepoName, evt.MRNumber, evt.Timestamp.Unix())

	// Get authenticated clone URL from provider
	cloneURL := evt.RepoURL
	provider := h.registry.Get(evt.Provider)
	if provider != nil {
		authURL, err := provider.AuthenticatedCloneURL(evt.RepoURL)
		if err != nil {
			log.Printf("warning: failed to get authenticated URL, using raw URL: %v", err)
		} else {
			cloneURL = authURL
		}
	}

	// Ensure repo is cached and create worktree
	_, err := h.repoCache.EnsureRepo(ctx, cloneURL, evt.RepoOwner, evt.RepoName)
	if err != nil {
		return fmt.Errorf("ensuring repo: %w", err)
	}

	worktreePath, err := h.repoCache.CreateWorktree(ctx, evt.RepoOwner, evt.RepoName, evt.SourceBranch, agentID)
	if err != nil {
		return fmt.Errorf("creating worktree: %w", err)
	}

	// Get changed files and calculate LCA for working directory
	workDir := "/workspace"
	if provider != nil {
		changedFiles, err := provider.GetChangedFiles(ctx, evt.RepoOwner, evt.RepoName, evt.MRNumber)
		if err != nil {
			log.Printf("warning: failed to get changed files: %v", err)
		} else if len(changedFiles) > 0 {
			// Extract file paths
			filePaths := make([]string, len(changedFiles))
			for i, f := range changedFiles {
				filePaths[i] = f.Path
			}
			// Calculate LCA
			lcaDir := lca.FindLCA(filePaths)
			if lcaDir != "." {
				workDir = "/workspace/" + lcaDir
			}
		}
	}

	// Build prompt using the prompt builder
	agentPrompt := h.promptBuilder.Build(evt, cfg, parsedIntent)

	// Spawn agent - use host path for Docker bind mount
	hostWorktreePath := h.repoCache.HostPath(worktreePath)
	_, err = h.spawner.Spawn(ctx, agent.SpawnRequest{
		ID:           agentID,
		WorktreePath: hostWorktreePath,
		WorkDir:      workDir,
		Prompt:       agentPrompt,
	})
	if err != nil {
		// Cleanup worktree on failure
		if cleanupErr := h.repoCache.RemoveWorktree(ctx, evt.RepoOwner, evt.RepoName, agentID); cleanupErr != nil {
			log.Printf("warning: failed to cleanup worktree %s: %v", agentID, cleanupErr)
		}
		return fmt.Errorf("spawning agent: %w", err)
	}

	log.Printf("Spawned agent %s for %s/%s MR #%d (workDir: %s)", agentID, evt.RepoOwner, evt.RepoName, evt.MRNumber, workDir)
	return nil
}
