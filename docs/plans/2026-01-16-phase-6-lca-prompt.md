# Phase 6: LCA Algorithm + Prompt Construction Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

> **Note:** This plan may need adjustment based on patterns established in Phases 1-5.

**Goal:** Calculate Least Common Ancestor directory from changed files, construct full agent prompts with context, permissions, and safety reminders.

**Tech Stack:** Go 1.25, existing packages

**Prerequisites:** Phases 1-5 complete

---

## Task 1: LCA Algorithm (TDD)

**Files:**
- Create: `internal/lca/lca.go`
- Create: `internal/lca/lca_test.go`

**Step 1: Write failing tests**

```go
package lca

import "testing"

func TestFindLCA_SingleFile(t *testing.T) {
	files := []string{"services/auth/handler.go"}
	result := FindLCA(files)
	if result != "services/auth" {
		t.Errorf("LCA = %q, want %q", result, "services/auth")
	}
}

func TestFindLCA_SameDirectory(t *testing.T) {
	files := []string{
		"services/auth/handler.go",
		"services/auth/utils.go",
	}
	result := FindLCA(files)
	if result != "services/auth" {
		t.Errorf("LCA = %q, want %q", result, "services/auth")
	}
}

func TestFindLCA_SiblingDirectories(t *testing.T) {
	files := []string{
		"services/auth/handler.go",
		"services/billing/handler.go",
	}
	result := FindLCA(files)
	if result != "services" {
		t.Errorf("LCA = %q, want %q", result, "services")
	}
}

func TestFindLCA_DifferentTrees(t *testing.T) {
	files := []string{
		"services/auth/handler.go",
		"lib/utils.go",
	}
	result := FindLCA(files)
	if result != "." {
		t.Errorf("LCA = %q, want %q", result, ".")
	}
}

func TestFindLCA_RootFiles(t *testing.T) {
	files := []string{"README.md", "go.mod"}
	result := FindLCA(files)
	if result != "." {
		t.Errorf("LCA = %q, want %q", result, ".")
	}
}

func TestFindLCA_Empty(t *testing.T) {
	result := FindLCA([]string{})
	if result != "." {
		t.Errorf("LCA = %q, want %q", result, ".")
	}
}
```

**Step 2: Implement LCA**

```go
package lca

import (
	"path/filepath"
	"strings"
)

// FindLCA finds the least common ancestor directory of the given file paths.
func FindLCA(files []string) string {
	if len(files) == 0 {
		return "."
	}

	// Get directory of first file
	dirs := make([][]string, len(files))
	for i, f := range files {
		dir := filepath.Dir(f)
		if dir == "." {
			dirs[i] = []string{}
		} else {
			dirs[i] = strings.Split(filepath.ToSlash(dir), "/")
		}
	}

	if len(dirs[0]) == 0 {
		return "."
	}

	// Find common prefix
	result := []string{}
	for i := 0; i < len(dirs[0]); i++ {
		component := dirs[0][i]
		allMatch := true
		for _, d := range dirs[1:] {
			if i >= len(d) || d[i] != component {
				allMatch = false
				break
			}
		}
		if !allMatch {
			break
		}
		result = append(result, component)
	}

	if len(result) == 0 {
		return "."
	}
	return strings.Join(result, "/")
}
```

**Step 3: Run tests, commit**

---

## Task 2: Prompt Builder (TDD)

**Files:**
- Create: `internal/prompt/builder.go`
- Create: `internal/prompt/builder_test.go`

**Step 1: Write failing test**

```go
package prompt

import (
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
```

**Step 2: Implement builder**

```go
package prompt

import (
	"fmt"
	"strings"

	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/event"
	"github.com/drewdunne/familiar/internal/intent"
)

type Builder struct{}

func NewBuilder() *Builder { return &Builder{} }

func (b *Builder) Build(evt *event.Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) string {
	var parts []string

	// System context
	parts = append(parts, b.buildContext(evt))

	// Base prompt
	parts = append(parts, b.getBasePrompt(evt.Type, cfg))

	// User instructions
	if parsedIntent != nil && parsedIntent.Instructions != "" {
		parts = append(parts, fmt.Sprintf("\n## User Instructions\n%s", parsedIntent.Instructions))
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
```

**Step 3: Run tests, commit**

---

## Task 3: Integrate LCA into Agent Handler

Update `internal/handler/agent.go` to:
1. Fetch changed files via provider
2. Calculate LCA
3. Set WorkDir to LCA path inside container

---

## Task 4: Run Full Test Suite

Verify coverage >= 80%

---

## Summary

| Task | Component | Tests |
|------|-----------|-------|
| 1 | LCA algorithm | 6 |
| 2 | Prompt builder | 1 |
| 3 | Handler integration | - |
| 4 | Coverage | - |

**Total: 4 tasks, ~7 tests**
