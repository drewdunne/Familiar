# Phase 3: Event Routing + Config Merging Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

> **Note:** This plan may need adjustment based on patterns established in Phases 1-2. Review previous implementations before starting.

**Goal:** Route webhook events to appropriate handlers, normalize events across providers, fetch and merge repo-level config with server defaults, implement debouncing.

**Architecture:** Event router receives normalized events from webhook handlers, fetches repo config via provider, merges with server config, applies debouncing, and dispatches to agent spawner (Phase 5).

**Tech Stack:** Go 1.25, existing config and provider packages

**Prerequisites:** Phases 1-2 complete

---

## Task 1: Define Normalized Event Types

**Files:**
- Create: `internal/event/event.go`

**Step 1: Create event types**

Create `internal/event/event.go`:
```go
package event

import "time"

// Type represents the type of webhook event.
type Type string

const (
	TypeMROpened  Type = "mr_opened"
	TypeMRComment Type = "mr_comment"
	TypeMRUpdated Type = "mr_updated"
	TypeMention   Type = "mention"
)

// Event represents a normalized webhook event.
type Event struct {
	// Type is the event type.
	Type Type

	// Provider is the git provider (github, gitlab).
	Provider string

	// Repository information.
	RepoOwner string
	RepoName  string
	RepoURL   string

	// Merge request information.
	MRNumber       int
	MRTitle        string
	MRDescription  string
	SourceBranch   string
	TargetBranch   string

	// Comment information (for TypeMRComment and TypeMention).
	CommentID     int
	CommentBody   string
	CommentAuthor string

	// Actor who triggered the event.
	Actor string

	// Timestamp of the event.
	Timestamp time.Time

	// RawPayload is the original webhook payload.
	RawPayload []byte
}

// Key returns a unique key for this event (used for debouncing).
func (e *Event) Key() string {
	return e.Provider + "/" + e.RepoOwner + "/" + e.RepoName + "/" + string(e.Type) + "/" + fmt.Sprint(e.MRNumber)
}
```

**Step 2: Commit**

```bash
git add internal/event/
git commit -m "feat(event): define normalized event types"
```

---

## Task 2: GitHub Event Normalizer (TDD)

**Files:**
- Create: `internal/event/github.go`
- Create: `internal/event/github_test.go`

**Step 1: Write failing test**

Create `internal/event/github_test.go`:
```go
package event

import (
	"testing"

	"github.com/drewdunne/familiar/internal/webhook"
)

func TestNormalizeGitHubEvent_PROpened(t *testing.T) {
	raw := []byte(`{
		"action": "opened",
		"number": 42,
		"pull_request": {
			"title": "Test PR",
			"body": "Description",
			"head": {"ref": "feature"},
			"base": {"ref": "main"},
			"user": {"login": "author"}
		},
		"repository": {
			"full_name": "owner/repo",
			"clone_url": "https://github.com/owner/repo.git"
		},
		"sender": {"login": "actor"}
	}`)

	ghEvent := &webhook.GitHubEvent{
		EventType:  "pull_request",
		Action:     "opened",
		RawPayload: raw,
	}

	event, err := NormalizeGitHubEvent(ghEvent)
	if err != nil {
		t.Fatalf("NormalizeGitHubEvent() error = %v", err)
	}

	if event.Type != TypeMROpened {
		t.Errorf("Type = %q, want %q", event.Type, TypeMROpened)
	}
	if event.MRNumber != 42 {
		t.Errorf("MRNumber = %d, want %d", event.MRNumber, 42)
	}
	if event.RepoOwner != "owner" {
		t.Errorf("RepoOwner = %q, want %q", event.RepoOwner, "owner")
	}
	if event.SourceBranch != "feature" {
		t.Errorf("SourceBranch = %q, want %q", event.SourceBranch, "feature")
	}
}

func TestNormalizeGitHubEvent_PRComment(t *testing.T) {
	raw := []byte(`{
		"action": "created",
		"issue": {"number": 42},
		"comment": {
			"id": 123,
			"body": "Please fix this",
			"user": {"login": "commenter"}
		},
		"repository": {
			"full_name": "owner/repo",
			"clone_url": "https://github.com/owner/repo.git"
		},
		"sender": {"login": "commenter"}
	}`)

	ghEvent := &webhook.GitHubEvent{
		EventType:  "issue_comment",
		Action:     "created",
		RawPayload: raw,
	}

	event, err := NormalizeGitHubEvent(ghEvent)
	if err != nil {
		t.Fatalf("NormalizeGitHubEvent() error = %v", err)
	}

	if event.Type != TypeMRComment {
		t.Errorf("Type = %q, want %q", event.Type, TypeMRComment)
	}
	if event.CommentBody != "Please fix this" {
		t.Errorf("CommentBody = %q, want %q", event.CommentBody, "Please fix this")
	}
}

func TestNormalizeGitHubEvent_Mention(t *testing.T) {
	raw := []byte(`{
		"action": "created",
		"issue": {"number": 42},
		"comment": {
			"id": 123,
			"body": "@familiar please review",
			"user": {"login": "commenter"}
		},
		"repository": {
			"full_name": "owner/repo",
			"clone_url": "https://github.com/owner/repo.git"
		},
		"sender": {"login": "commenter"}
	}`)

	ghEvent := &webhook.GitHubEvent{
		EventType:  "issue_comment",
		Action:     "created",
		RawPayload: raw,
	}

	event, err := NormalizeGitHubEvent(ghEvent)
	if err != nil {
		t.Fatalf("NormalizeGitHubEvent() error = %v", err)
	}

	// Should be TypeMention because it contains @familiar
	if event.Type != TypeMention {
		t.Errorf("Type = %q, want %q", event.Type, TypeMention)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/event/... -v
```

**Step 3: Implement normalizer**

Create `internal/event/github.go`:
```go
package event

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/drewdunne/familiar/internal/webhook"
)

// gitHubPayload represents the common GitHub webhook payload structure.
type gitHubPayload struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		Head  struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"pull_request"`
	Issue struct {
		Number int `json:"number"`
	} `json:"issue"`
	Comment struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

// NormalizeGitHubEvent converts a GitHub webhook event to a normalized Event.
func NormalizeGitHubEvent(ghEvent *webhook.GitHubEvent) (*Event, error) {
	var payload gitHubPayload
	if err := json.Unmarshal(ghEvent.RawPayload, &payload); err != nil {
		return nil, fmt.Errorf("parsing payload: %w", err)
	}

	// Parse owner/repo from full_name
	parts := strings.SplitN(payload.Repository.FullName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository full_name: %s", payload.Repository.FullName)
	}

	event := &Event{
		Provider:   "github",
		RepoOwner:  parts[0],
		RepoName:   parts[1],
		RepoURL:    payload.Repository.CloneURL,
		Actor:      payload.Sender.Login,
		Timestamp:  time.Now(),
		RawPayload: ghEvent.RawPayload,
	}

	switch ghEvent.EventType {
	case "pull_request":
		event.MRNumber = payload.Number
		event.MRTitle = payload.PullRequest.Title
		event.MRDescription = payload.PullRequest.Body
		event.SourceBranch = payload.PullRequest.Head.Ref
		event.TargetBranch = payload.PullRequest.Base.Ref

		switch payload.Action {
		case "opened":
			event.Type = TypeMROpened
		case "synchronize":
			event.Type = TypeMRUpdated
		default:
			return nil, fmt.Errorf("unhandled pull_request action: %s", payload.Action)
		}

	case "issue_comment":
		event.MRNumber = payload.Issue.Number
		event.CommentID = payload.Comment.ID
		event.CommentBody = payload.Comment.Body
		event.CommentAuthor = payload.Comment.User.Login

		// Check for @mention
		if containsMention(payload.Comment.Body) {
			event.Type = TypeMention
		} else {
			event.Type = TypeMRComment
		}

	default:
		return nil, fmt.Errorf("unhandled event type: %s", ghEvent.EventType)
	}

	return event, nil
}

// containsMention checks if the text contains @familiar mention.
func containsMention(text string) bool {
	// TODO: Make mention pattern configurable
	return strings.Contains(strings.ToLower(text), "@familiar")
}
```

**Step 4: Run tests**

```bash
go test ./internal/event/... -v
```

**Step 5: Commit**

```bash
git add internal/event/
git commit -m "feat(event): add GitHub event normalizer"
```

---

## Task 3: GitLab Event Normalizer (TDD)

**Files:**
- Create: `internal/event/gitlab.go`
- Create: `internal/event/gitlab_test.go`

**Step 1: Write failing test**

Create `internal/event/gitlab_test.go`:
```go
package event

import (
	"testing"

	"github.com/drewdunne/familiar/internal/webhook"
)

func TestNormalizeGitLabEvent_MROpened(t *testing.T) {
	raw := []byte(`{
		"object_kind": "merge_request",
		"object_attributes": {
			"iid": 42,
			"title": "Test MR",
			"description": "Description",
			"source_branch": "feature",
			"target_branch": "main",
			"action": "open"
		},
		"project": {
			"path_with_namespace": "owner/repo",
			"git_http_url": "https://gitlab.com/owner/repo.git"
		},
		"user": {"username": "actor"}
	}`)

	glEvent := &webhook.GitLabEvent{
		EventType:  "Merge Request Hook",
		ObjectKind: "merge_request",
		RawPayload: raw,
	}

	event, err := NormalizeGitLabEvent(glEvent)
	if err != nil {
		t.Fatalf("NormalizeGitLabEvent() error = %v", err)
	}

	if event.Type != TypeMROpened {
		t.Errorf("Type = %q, want %q", event.Type, TypeMROpened)
	}
	if event.MRNumber != 42 {
		t.Errorf("MRNumber = %d, want %d", event.MRNumber, 42)
	}
}

func TestNormalizeGitLabEvent_Note(t *testing.T) {
	raw := []byte(`{
		"object_kind": "note",
		"object_attributes": {
			"id": 123,
			"note": "Please fix this",
			"noteable_type": "MergeRequest"
		},
		"merge_request": {
			"iid": 42
		},
		"project": {
			"path_with_namespace": "owner/repo",
			"git_http_url": "https://gitlab.com/owner/repo.git"
		},
		"user": {"username": "commenter"}
	}`)

	glEvent := &webhook.GitLabEvent{
		EventType:  "Note Hook",
		ObjectKind: "note",
		RawPayload: raw,
	}

	event, err := NormalizeGitLabEvent(glEvent)
	if err != nil {
		t.Fatalf("NormalizeGitLabEvent() error = %v", err)
	}

	if event.Type != TypeMRComment {
		t.Errorf("Type = %q, want %q", event.Type, TypeMRComment)
	}
}
```

**Step 2: Implement normalizer**

Create `internal/event/gitlab.go`:
```go
package event

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/drewdunne/familiar/internal/webhook"
)

type gitLabPayload struct {
	ObjectKind       string `json:"object_kind"`
	ObjectAttributes struct {
		IID          int    `json:"iid"`
		ID           int    `json:"id"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		Note         string `json:"note"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		Action       string `json:"action"`
		NoteableType string `json:"noteable_type"`
	} `json:"object_attributes"`
	MergeRequest struct {
		IID int `json:"iid"`
	} `json:"merge_request"`
	Project struct {
		PathWithNamespace string `json:"path_with_namespace"`
		GitHTTPURL        string `json:"git_http_url"`
	} `json:"project"`
	User struct {
		Username string `json:"username"`
	} `json:"user"`
}

// NormalizeGitLabEvent converts a GitLab webhook event to a normalized Event.
func NormalizeGitLabEvent(glEvent *webhook.GitLabEvent) (*Event, error) {
	var payload gitLabPayload
	if err := json.Unmarshal(glEvent.RawPayload, &payload); err != nil {
		return nil, fmt.Errorf("parsing payload: %w", err)
	}

	parts := strings.SplitN(payload.Project.PathWithNamespace, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid project path: %s", payload.Project.PathWithNamespace)
	}

	event := &Event{
		Provider:   "gitlab",
		RepoOwner:  parts[0],
		RepoName:   parts[1],
		RepoURL:    payload.Project.GitHTTPURL,
		Actor:      payload.User.Username,
		Timestamp:  time.Now(),
		RawPayload: glEvent.RawPayload,
	}

	switch payload.ObjectKind {
	case "merge_request":
		event.MRNumber = payload.ObjectAttributes.IID
		event.MRTitle = payload.ObjectAttributes.Title
		event.MRDescription = payload.ObjectAttributes.Description
		event.SourceBranch = payload.ObjectAttributes.SourceBranch
		event.TargetBranch = payload.ObjectAttributes.TargetBranch

		switch payload.ObjectAttributes.Action {
		case "open":
			event.Type = TypeMROpened
		case "update":
			event.Type = TypeMRUpdated
		default:
			return nil, fmt.Errorf("unhandled merge_request action: %s", payload.ObjectAttributes.Action)
		}

	case "note":
		if payload.ObjectAttributes.NoteableType != "MergeRequest" {
			return nil, fmt.Errorf("note on non-MR not supported")
		}
		event.MRNumber = payload.MergeRequest.IID
		event.CommentID = payload.ObjectAttributes.ID
		event.CommentBody = payload.ObjectAttributes.Note
		event.CommentAuthor = payload.User.Username

		if containsMention(payload.ObjectAttributes.Note) {
			event.Type = TypeMention
		} else {
			event.Type = TypeMRComment
		}

	default:
		return nil, fmt.Errorf("unhandled object_kind: %s", payload.ObjectKind)
	}

	return event, nil
}
```

**Step 3: Run tests**

```bash
go test ./internal/event/... -v
```

**Step 4: Commit**

```bash
git add internal/event/
git commit -m "feat(event): add GitLab event normalizer"
```

---

## Task 4: Repo Config Fetching (TDD)

**Files:**
- Create: `internal/config/repo.go`
- Create: `internal/config/repo_test.go`

**Step 1: Write failing test**

Create `internal/config/repo_test.go`:
```go
package config

import (
	"context"
	"testing"
)

type mockFileReader struct {
	content string
	err     error
}

func (m *mockFileReader) ReadFile(ctx context.Context, owner, repo, path, ref string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []byte(m.content), nil
}

func TestLoadRepoConfig(t *testing.T) {
	reader := &mockFileReader{
		content: `
events:
  mr_opened: true
  mr_updated: false
permissions:
  merge: "on_request"
`,
	}

	cfg, err := LoadRepoConfig(context.Background(), reader, "owner", "repo", "main")
	if err != nil {
		t.Fatalf("LoadRepoConfig() error = %v", err)
	}

	if cfg.Events.MROpened != true {
		t.Error("Events.MROpened should be true")
	}
	if cfg.Events.MRUpdated != false {
		t.Error("Events.MRUpdated should be false")
	}
	if cfg.Permissions.Merge != "on_request" {
		t.Errorf("Permissions.Merge = %q, want %q", cfg.Permissions.Merge, "on_request")
	}
}

func TestLoadRepoConfig_NotFound(t *testing.T) {
	reader := &mockFileReader{
		err: ErrConfigNotFound,
	}

	cfg, err := LoadRepoConfig(context.Background(), reader, "owner", "repo", "main")
	if err != nil {
		t.Fatalf("LoadRepoConfig() should not error for missing config, got: %v", err)
	}

	// Should return empty config
	if cfg == nil {
		t.Error("Should return empty config, not nil")
	}
}
```

**Step 2: Implement repo config loading**

Create `internal/config/repo.go`:
```go
package config

import (
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ErrConfigNotFound indicates the repo config file doesn't exist.
var ErrConfigNotFound = errors.New("config not found")

// RepoConfig represents repository-level configuration.
type RepoConfig struct {
	Events      EventsConfig      `yaml:"events"`
	Permissions PermissionsConfig `yaml:"permissions"`
	Prompts     PromptsConfig     `yaml:"prompts"`
	AgentImage  string            `yaml:"agent_image"`
}

// EventsConfig controls which events are enabled.
type EventsConfig struct {
	MROpened  bool `yaml:"mr_opened"`
	MRComment bool `yaml:"mr_comment"`
	MRUpdated bool `yaml:"mr_updated"`
	Mention   bool `yaml:"mention"`
}

// PermissionsConfig controls agent permissions.
type PermissionsConfig struct {
	Merge          string `yaml:"merge"`
	Approve        string `yaml:"approve"`
	PushCommits    string `yaml:"push_commits"`
	DismissReviews string `yaml:"dismiss_reviews"`
}

// PromptsConfig holds custom prompts per event type.
type PromptsConfig struct {
	MROpened  string `yaml:"mr_opened"`
	MRComment string `yaml:"mr_comment"`
	MRUpdated string `yaml:"mr_updated"`
	Mention   string `yaml:"mention"`
}

// FileReader reads files from a repository.
type FileReader interface {
	ReadFile(ctx context.Context, owner, repo, path, ref string) ([]byte, error)
}

// LoadRepoConfig loads the repo config from .familiar/config.yaml.
func LoadRepoConfig(ctx context.Context, reader FileReader, owner, repo, ref string) (*RepoConfig, error) {
	data, err := reader.ReadFile(ctx, owner, repo, ".familiar/config.yaml", ref)
	if errors.Is(err, ErrConfigNotFound) {
		return &RepoConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading repo config: %w", err)
	}

	var cfg RepoConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing repo config: %w", err)
	}

	return &cfg, nil
}
```

**Step 3: Run tests**

```bash
go test ./internal/config/... -v
```

**Step 4: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add repo config loading"
```

---

## Task 5: Config Merging (TDD)

**Files:**
- Create: `internal/config/merge.go`
- Create: `internal/config/merge_test.go`

**Step 1: Write failing test**

Create `internal/config/merge_test.go`:
```go
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

	// Repo event disable should override
	if merged.Events.MRUpdated != false {
		t.Error("Events.MRUpdated should be false (repo override)")
	}
}
```

**Step 2: Implement config merging**

Create `internal/config/merge.go`:
```go
package config

// MergedConfig represents the final merged configuration.
type MergedConfig struct {
	Prompts     PromptsConfig
	Permissions PermissionsConfig
	Events      EventsConfig
	AgentImage  string
}

// MergeConfigs merges server config with repo config.
// Repo config values take precedence over server defaults.
func MergeConfigs(server *Config, repo *RepoConfig) *MergedConfig {
	merged := &MergedConfig{}

	// Merge prompts (repo overrides if non-empty)
	merged.Prompts.MROpened = coalesce(repo.Prompts.MROpened, server.Prompts.MROpened)
	merged.Prompts.MRComment = coalesce(repo.Prompts.MRComment, server.Prompts.MRComment)
	merged.Prompts.MRUpdated = coalesce(repo.Prompts.MRUpdated, server.Prompts.MRUpdated)
	merged.Prompts.Mention = coalesce(repo.Prompts.Mention, server.Prompts.Mention)

	// Merge permissions (repo overrides if non-empty)
	merged.Permissions.Merge = coalesce(repo.Permissions.Merge, server.Permissions.Merge)
	merged.Permissions.Approve = coalesce(repo.Permissions.Approve, server.Permissions.Approve)
	merged.Permissions.PushCommits = coalesce(repo.Permissions.PushCommits, server.Permissions.PushCommits)
	merged.Permissions.DismissReviews = coalesce(repo.Permissions.DismissReviews, server.Permissions.DismissReviews)

	// Merge events - repo can disable but not enable if server has it disabled
	// For simplicity, repo values override server values
	merged.Events.MROpened = repo.Events.MROpened || server.Events.MROpened
	merged.Events.MRComment = repo.Events.MRComment || server.Events.MRComment
	merged.Events.MRUpdated = repo.Events.MRUpdated && server.Events.MRUpdated // Both must be true
	merged.Events.Mention = repo.Events.Mention || server.Events.Mention

	// Agent image
	merged.AgentImage = coalesce(repo.AgentImage, "")

	return merged
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
```

Note: This task will require adding `ServerPromptsConfig`, `ServerPermissionsConfig`, `ServerEventsConfig` types to the existing config package.

**Step 3: Run tests**

```bash
go test ./internal/config/... -v
```

**Step 4: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add config merging"
```

---

## Task 6: Debouncer (TDD)

**Files:**
- Create: `internal/event/debouncer.go`
- Create: `internal/event/debouncer_test.go`

**Step 1: Write failing test**

Create `internal/event/debouncer_test.go`:
```go
package event

import (
	"testing"
	"time"
)

func TestDebouncer(t *testing.T) {
	debounceWindow := 100 * time.Millisecond
	d := NewDebouncer(debounceWindow)

	event1 := &Event{
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		Type:      TypeMRUpdated,
		MRNumber:  42,
	}

	// First event should be accepted
	if !d.ShouldProcess(event1) {
		t.Error("First event should be accepted")
	}

	// Same event immediately after should be debounced
	if d.ShouldProcess(event1) {
		t.Error("Duplicate event should be debounced")
	}

	// Wait for debounce window
	time.Sleep(debounceWindow + 10*time.Millisecond)

	// Now it should be accepted again
	if !d.ShouldProcess(event1) {
		t.Error("Event after debounce window should be accepted")
	}
}

func TestDebouncer_DifferentEvents(t *testing.T) {
	d := NewDebouncer(100 * time.Millisecond)

	event1 := &Event{
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		Type:      TypeMRUpdated,
		MRNumber:  42,
	}

	event2 := &Event{
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		Type:      TypeMRUpdated,
		MRNumber:  43, // Different MR
	}

	d.ShouldProcess(event1)

	// Different event should be accepted
	if !d.ShouldProcess(event2) {
		t.Error("Different event should be accepted")
	}
}
```

**Step 2: Implement debouncer**

Create `internal/event/debouncer.go`:
```go
package event

import (
	"sync"
	"time"
)

// Debouncer prevents duplicate events within a time window.
type Debouncer struct {
	window time.Duration
	seen   map[string]time.Time
	mu     sync.Mutex
}

// NewDebouncer creates a new debouncer with the given window.
func NewDebouncer(window time.Duration) *Debouncer {
	return &Debouncer{
		window: window,
		seen:   make(map[string]time.Time),
	}
}

// ShouldProcess returns true if the event should be processed.
// Returns false if a similar event was processed recently.
func (d *Debouncer) ShouldProcess(e *Event) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := e.Key()
	now := time.Now()

	if lastSeen, ok := d.seen[key]; ok {
		if now.Sub(lastSeen) < d.window {
			return false
		}
	}

	d.seen[key] = now
	return true
}

// Cleanup removes old entries from the seen map.
func (d *Debouncer) Cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()

	threshold := time.Now().Add(-d.window * 2)
	for key, t := range d.seen {
		if t.Before(threshold) {
			delete(d.seen, key)
		}
	}
}
```

**Step 3: Run tests**

```bash
go test ./internal/event/... -v
```

**Step 4: Commit**

```bash
git add internal/event/
git commit -m "feat(event): add event debouncer"
```

---

## Task 7: Event Router

**Files:**
- Create: `internal/event/router.go`
- Create: `internal/event/router_test.go`

**Step 1: Write failing test**

Create `internal/event/router_test.go`:
```go
package event

import (
	"context"
	"testing"

	"github.com/drewdunne/familiar/internal/config"
)

func TestRouter_Route(t *testing.T) {
	var handledEvent *Event
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig) error {
		handledEvent = e
		return nil
	}

	serverCfg := &config.Config{
		Events: config.ServerEventsConfig{
			MROpened: true,
		},
	}

	router := NewRouter(serverCfg, handler)

	event := &Event{
		Type:      TypeMROpened,
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  42,
	}

	err := router.Route(context.Background(), event)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if handledEvent == nil {
		t.Error("Handler was not called")
	}
}

func TestRouter_EventDisabled(t *testing.T) {
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig) error {
		t.Error("Handler should not be called for disabled event")
		return nil
	}

	serverCfg := &config.Config{
		Events: config.ServerEventsConfig{
			MROpened: false, // Disabled
		},
	}

	router := NewRouter(serverCfg, handler)

	event := &Event{
		Type: TypeMROpened,
	}

	// Should not error, but also not call handler
	router.Route(context.Background(), event)
}
```

**Step 2: Implement router**

Create `internal/event/router.go`:
```go
package event

import (
	"context"
	"log"

	"github.com/drewdunne/familiar/internal/config"
)

// Handler processes a normalized event with merged config.
type Handler func(ctx context.Context, event *Event, cfg *config.MergedConfig) error

// Router routes events to handlers after config merging and validation.
type Router struct {
	serverCfg *config.Config
	handler   Handler
	debouncer *Debouncer
}

// NewRouter creates a new event router.
func NewRouter(serverCfg *config.Config, handler Handler) *Router {
	return &Router{
		serverCfg: serverCfg,
		handler:   handler,
		debouncer: NewDebouncer(time.Duration(serverCfg.Agents.DebounceSeconds) * time.Second),
	}
}

// Route processes an event through the routing pipeline.
func (r *Router) Route(ctx context.Context, event *Event) error {
	// Check debounce
	if !r.debouncer.ShouldProcess(event) {
		log.Printf("Event debounced: %s", event.Key())
		return nil
	}

	// Check if event type is enabled at server level
	if !r.isEventEnabled(event.Type) {
		log.Printf("Event type disabled: %s", event.Type)
		return nil
	}

	// TODO: Fetch repo config and merge
	// For now, use server config only
	merged := config.MergeConfigs(r.serverCfg, &config.RepoConfig{})

	// Check if event type is enabled after merge
	if !r.isEventEnabledMerged(event.Type, merged) {
		log.Printf("Event type disabled by repo config: %s", event.Type)
		return nil
	}

	// Call handler
	return r.handler(ctx, event, merged)
}

func (r *Router) isEventEnabled(t Type) bool {
	switch t {
	case TypeMROpened:
		return r.serverCfg.Events.MROpened
	case TypeMRComment:
		return r.serverCfg.Events.MRComment
	case TypeMRUpdated:
		return r.serverCfg.Events.MRUpdated
	case TypeMention:
		return r.serverCfg.Events.Mention
	default:
		return false
	}
}

func (r *Router) isEventEnabledMerged(t Type, cfg *config.MergedConfig) bool {
	switch t {
	case TypeMROpened:
		return cfg.Events.MROpened
	case TypeMRComment:
		return cfg.Events.MRComment
	case TypeMRUpdated:
		return cfg.Events.MRUpdated
	case TypeMention:
		return cfg.Events.Mention
	default:
		return false
	}
}
```

**Step 3: Run tests**

```bash
go test ./internal/event/... -v
```

**Step 4: Commit**

```bash
git add internal/event/
git commit -m "feat(event): add event router"
```

---

## Task 8: Run Full Test Suite

**Step 1: Run all tests with coverage**

```bash
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

**Step 2: Verify coverage >= 80%**

**Step 3: Commit if needed**

```bash
git add .
git commit -m "test: ensure 80% coverage for Phase 3"
```

---

## Summary

| Task | Component | Tests Added |
|------|-----------|-------------|
| 1 | Event types | - |
| 2 | GitHub normalizer | 3 |
| 3 | GitLab normalizer | 2 |
| 4 | Repo config fetching | 2 |
| 5 | Config merging | 1 |
| 6 | Debouncer | 2 |
| 7 | Event router | 2 |
| 8 | Coverage verification | - |

**Total: 8 tasks, ~12 tests**
