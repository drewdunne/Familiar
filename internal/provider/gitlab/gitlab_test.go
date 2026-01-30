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
		if r.URL.Path != "/api/v4/projects/owner/repo" {
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
		if r.URL.Path != "/api/v4/projects/owner/repo/merge_requests/42" {
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

func TestGitLabProvider_Name(t *testing.T) {
	p := New("test-token")
	if p.Name() != "gitlab" {
		t.Errorf("Name() = %q, want %q", p.Name(), "gitlab")
	}
}

func TestGitLabProvider_GetChangedFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/owner/repo/merge_requests/42/changes" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"changes": []map[string]interface{}{
				{"new_path": "file1.go", "new_file": false, "deleted_file": false, "renamed_file": false},
				{"new_path": "file2.go", "new_file": true, "deleted_file": false, "renamed_file": false},
			},
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
	if files[1].Status != "added" {
		t.Errorf("files[1].Status = %q, want %q", files[1].Status, "added")
	}
}

func TestGitLabProvider_PostComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/owner/repo/merge_requests/42/notes" {
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

func TestGitLabProvider_GetComments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/owner/repo/merge_requests/42/notes" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": 1, "body": "comment 1", "author": map[string]string{"username": "user1"}},
			{"id": 2, "body": "comment 2", "author": map[string]string{"username": "user2"}},
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

func TestGitLabProvider_GetRepository_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := New("test-token", WithBaseURL(server.URL))
	_, err := p.GetRepository(context.Background(), "owner", "nonexistent")
	if err == nil {
		t.Error("GetRepository() expected error for non-existent repo")
	}
}

func TestGitLabProvider_GetMergeRequest_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := New("test-token", WithBaseURL(server.URL))
	_, err := p.GetMergeRequest(context.Background(), "owner", "repo", 999)
	if err == nil {
		t.Error("GetMergeRequest() expected error for non-existent MR")
	}
}

func TestGitLabProvider_GetChangedFiles_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	p := New("test-token", WithBaseURL(server.URL))
	_, err := p.GetChangedFiles(context.Background(), "owner", "repo", 42)
	if err == nil {
		t.Error("GetChangedFiles() expected error for server error")
	}
}

func TestGitLabProvider_PostComment_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	p := New("test-token", WithBaseURL(server.URL))
	err := p.PostComment(context.Background(), "owner", "repo", 42, "test")
	if err == nil {
		t.Error("PostComment() expected error for forbidden")
	}
}

func TestGitLabProvider_GetComments_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	p := New("test-token", WithBaseURL(server.URL))
	_, err := p.GetComments(context.Background(), "owner", "repo", 42)
	if err == nil {
		t.Error("GetComments() expected error for server unavailable")
	}
}
