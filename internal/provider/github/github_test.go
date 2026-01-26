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
