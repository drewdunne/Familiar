package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubProvider_GetRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing or incorrect authorization header")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":             123,
			"name":           "repo",
			"full_name":      "owner/repo",
			"clone_url":      "https://github.com/owner/repo.git",
			"ssh_url":        "git@github.com:owner/repo.git",
			"default_branch": "main",
		})
	}))
	defer server.Close()

	p := New("test-token", WithBaseURL(server.URL))
	repo, err := p.GetRepository(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("GetRepository() error = %v", err)
	}

	if repo.FullName != "owner/repo" {
		t.Errorf("FullName = %q, want %q", repo.FullName, "owner/repo")
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "main")
	}
}

func TestGitHubProvider_GetMergeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/pulls/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     999,
			"number": 42,
			"title":  "Test PR",
			"body":   "Description",
			"state":  "open",
			"head":   map[string]string{"ref": "feature"},
			"base":   map[string]string{"ref": "main"},
			"user":   map[string]string{"login": "author"},
			"html_url": "https://github.com/owner/repo/pull/42",
		})
	}))
	defer server.Close()

	p := New("test-token", WithBaseURL(server.URL))
	mr, err := p.GetMergeRequest(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatalf("GetMergeRequest() error = %v", err)
	}

	if mr.Number != 42 {
		t.Errorf("Number = %d, want %d", mr.Number, 42)
	}
	if mr.Title != "Test PR" {
		t.Errorf("Title = %q, want %q", mr.Title, "Test PR")
	}
	if mr.SourceBranch != "feature" {
		t.Errorf("SourceBranch = %q, want %q", mr.SourceBranch, "feature")
	}
}

func TestGitHubProvider_Name(t *testing.T) {
	p := New("test-token")
	if p.Name() != "github" {
		t.Errorf("Name() = %q, want %q", p.Name(), "github")
	}
}

func TestGitHubProvider_GetChangedFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/pulls/42/files" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"filename": "file1.go", "status": "modified", "additions": 10, "deletions": 5},
			{"filename": "file2.go", "status": "added", "additions": 20, "deletions": 0},
		})
	}))
	defer server.Close()

	p := New("test-token", WithBaseURL(server.URL))
	files, err := p.GetChangedFiles(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatalf("GetChangedFiles() error = %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("GetChangedFiles() returned %d files, want 2", len(files))
	}
	if files[0].Path != "file1.go" {
		t.Errorf("files[0].Path = %q, want %q", files[0].Path, "file1.go")
	}
}

func TestGitHubProvider_PostComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/issues/42/comments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
	}))
	defer server.Close()

	p := New("test-token", WithBaseURL(server.URL))
	err := p.PostComment(context.Background(), "owner", "repo", 42, "test comment")
	if err != nil {
		t.Fatalf("PostComment() error = %v", err)
	}
}

func TestGitHubProvider_GetComments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/issues/42/comments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": 1, "body": "comment 1", "user": map[string]string{"login": "user1"}},
			{"id": 2, "body": "comment 2", "user": map[string]string{"login": "user2"}},
		})
	}))
	defer server.Close()

	p := New("test-token", WithBaseURL(server.URL))
	comments, err := p.GetComments(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatalf("GetComments() error = %v", err)
	}

	if len(comments) != 2 {
		t.Fatalf("GetComments() returned %d comments, want 2", len(comments))
	}
	if comments[0].Body != "comment 1" {
		t.Errorf("comments[0].Body = %q, want %q", comments[0].Body, "comment 1")
	}
}
