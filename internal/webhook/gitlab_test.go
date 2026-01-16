package webhook

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitLabHandler_ValidToken(t *testing.T) {
	secret := "test-secret-token"
	payload := `{"object_kind":"merge_request","object_attributes":{"action":"open"}}`

	handler := NewGitLabHandler(secret, func(event *GitLabEvent) error {
		if event.ObjectKind != "merge_request" {
			t.Errorf("event.ObjectKind = %q, want %q", event.ObjectKind, "merge_request")
		}
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(payload))
	req.Header.Set("X-Gitlab-Token", secret)
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestGitLabHandler_InvalidToken(t *testing.T) {
	secret := "test-secret-token"
	payload := `{"object_kind":"merge_request"}`

	handler := NewGitLabHandler(secret, func(event *GitLabEvent) error {
		t.Error("handler should not be called with invalid token")
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(payload))
	req.Header.Set("X-Gitlab-Token", "wrong-token")
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGitLabHandler_MissingToken(t *testing.T) {
	secret := "test-secret-token"
	payload := `{"object_kind":"merge_request"}`

	handler := NewGitLabHandler(secret, func(event *GitLabEvent) error {
		t.Error("handler should not be called with missing token")
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(payload))
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGitLabHandler_InvalidJSON(t *testing.T) {
	secret := "test-secret-token"
	payload := `{invalid json`

	handler := NewGitLabHandler(secret, func(event *GitLabEvent) error {
		t.Error("handler should not be called with invalid JSON")
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(payload))
	req.Header.Set("X-Gitlab-Token", secret)
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGitLabHandler_HandlerError(t *testing.T) {
	secret := "test-secret-token"
	payload := `{"object_kind":"merge_request"}`

	handler := NewGitLabHandler(secret, func(event *GitLabEvent) error {
		return fmt.Errorf("processing error")
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(payload))
	req.Header.Set("X-Gitlab-Token", secret)
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
