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
	if event.RepoOwner != "owner" {
		t.Errorf("RepoOwner = %q, want %q", event.RepoOwner, "owner")
	}
	if event.SourceBranch != "feature" {
		t.Errorf("SourceBranch = %q, want %q", event.SourceBranch, "feature")
	}
}

func TestNormalizeGitLabEvent_MRUpdated(t *testing.T) {
	raw := []byte(`{
		"object_kind": "merge_request",
		"object_attributes": {
			"iid": 42,
			"title": "Test MR",
			"description": "Description",
			"source_branch": "feature",
			"target_branch": "main",
			"action": "update"
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

	if event.Type != TypeMRUpdated {
		t.Errorf("Type = %q, want %q", event.Type, TypeMRUpdated)
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
			"iid": 42,
			"source_branch": "feature",
			"target_branch": "main"
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
	if event.CommentBody != "Please fix this" {
		t.Errorf("CommentBody = %q, want %q", event.CommentBody, "Please fix this")
	}
	if event.SourceBranch != "feature" {
		t.Errorf("SourceBranch = %q, want %q", event.SourceBranch, "feature")
	}
	if event.TargetBranch != "main" {
		t.Errorf("TargetBranch = %q, want %q", event.TargetBranch, "main")
	}
}

func TestNormalizeGitLabEvent_NoteWithPosition(t *testing.T) {
	raw := []byte(`{
		"object_kind": "note",
		"object_attributes": {
			"id": 456,
			"note": "Add an exclamation mark here",
			"noteable_type": "MergeRequest",
			"discussion_id": "abc123def456",
			"position": {
				"new_path": "services/api/src/index.ts",
				"new_line": 8,
				"position_type": "text"
			}
		},
		"merge_request": {
			"iid": 10,
			"source_branch": "feature/greeting",
			"target_branch": "main"
		},
		"project": {
			"path_with_namespace": "owner/repo",
			"git_http_url": "https://gitlab.com/owner/repo.git"
		},
		"user": {"username": "reviewer"}
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

	if event.CommentFilePath != "services/api/src/index.ts" {
		t.Errorf("CommentFilePath = %q, want %q", event.CommentFilePath, "services/api/src/index.ts")
	}
	if event.CommentLine != 8 {
		t.Errorf("CommentLine = %d, want %d", event.CommentLine, 8)
	}
	if event.CommentDiscussionID != "abc123def456" {
		t.Errorf("CommentDiscussionID = %q, want %q", event.CommentDiscussionID, "abc123def456")
	}
}

func TestNormalizeGitLabEvent_Mention(t *testing.T) {
	raw := []byte(`{
		"object_kind": "note",
		"object_attributes": {
			"id": 123,
			"note": "@familiar please review",
			"noteable_type": "MergeRequest"
		},
		"merge_request": {
			"iid": 42,
			"source_branch": "feature",
			"target_branch": "main"
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

	// Should be TypeMention because it contains @familiar
	if event.Type != TypeMention {
		t.Errorf("Type = %q, want %q", event.Type, TypeMention)
	}
	if event.SourceBranch != "feature" {
		t.Errorf("SourceBranch = %q, want %q", event.SourceBranch, "feature")
	}
}

func TestNormalizeGitLabEvent_UnhandledAction(t *testing.T) {
	raw := []byte(`{
		"object_kind": "merge_request",
		"object_attributes": {"action": "close"},
		"project": {"path_with_namespace": "owner/repo", "git_http_url": "https://gitlab.com/owner/repo.git"},
		"user": {"username": "actor"}
	}`)

	glEvent := &webhook.GitLabEvent{
		EventType:  "Merge Request Hook",
		ObjectKind: "merge_request",
		RawPayload: raw,
	}

	_, err := NormalizeGitLabEvent(glEvent)
	if err == nil {
		t.Error("Expected error for unhandled action")
	}
}

func TestNormalizeGitLabEvent_UnhandledObjectKind(t *testing.T) {
	raw := []byte(`{
		"object_kind": "push",
		"project": {"path_with_namespace": "owner/repo", "git_http_url": "https://gitlab.com/owner/repo.git"},
		"user": {"username": "actor"}
	}`)

	glEvent := &webhook.GitLabEvent{
		EventType:  "Push Hook",
		ObjectKind: "push",
		RawPayload: raw,
	}

	_, err := NormalizeGitLabEvent(glEvent)
	if err == nil {
		t.Error("Expected error for unhandled object_kind")
	}
}

func TestNormalizeGitLabEvent_NonMRNote(t *testing.T) {
	raw := []byte(`{
		"object_kind": "note",
		"object_attributes": {"noteable_type": "Issue"},
		"project": {"path_with_namespace": "owner/repo", "git_http_url": "https://gitlab.com/owner/repo.git"},
		"user": {"username": "actor"}
	}`)

	glEvent := &webhook.GitLabEvent{
		EventType:  "Note Hook",
		ObjectKind: "note",
		RawPayload: raw,
	}

	_, err := NormalizeGitLabEvent(glEvent)
	if err == nil {
		t.Error("Expected error for non-MR note")
	}
}
