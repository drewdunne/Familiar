package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitLabProvider_GetRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/owner%2Frepo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			t.Errorf("missing or incorrect token header")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":                  123,
			"name":                "repo",
			"path_with_namespace": "owner/repo",
			"http_url_to_repo":    "https://gitlab.com/owner/repo.git",
			"ssh_url_to_repo":     "git@gitlab.com:owner/repo.git",
			"default_branch":      "main",
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
}

func TestGitLabProvider_GetMergeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/owner%2Frepo/merge_requests/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":            999,
			"iid":           42,
			"title":         "Test MR",
			"description":   "Description",
			"state":         "opened",
			"source_branch": "feature",
			"target_branch": "main",
			"author":        map[string]string{"username": "author"},
			"web_url":       "https://gitlab.com/owner/repo/-/merge_requests/42",
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
	if mr.Title != "Test MR" {
		t.Errorf("Title = %q, want %q", mr.Title, "Test MR")
	}
}
