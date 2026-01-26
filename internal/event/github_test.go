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
