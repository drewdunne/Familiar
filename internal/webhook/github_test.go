package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubHandler_ValidSignature(t *testing.T) {
	secret := "test-secret"
	payload := `{"action":"opened","number":1}`

	// Calculate expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	handler := NewGitHubHandler(secret, func(event *GitHubEvent) error {
		if event.Action != "opened" {
			t.Errorf("event.Action = %q, want %q", event.Action, "opened")
		}
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", "pull_request")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestGitHubHandler_InvalidSignature(t *testing.T) {
	secret := "test-secret"
	payload := `{"action":"opened","number":1}`

	handler := NewGitHubHandler(secret, func(event *GitHubEvent) error {
		t.Error("handler should not be called with invalid signature")
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	req.Header.Set("X-GitHub-Event", "pull_request")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGitHubHandler_MissingSignature(t *testing.T) {
	secret := "test-secret"
	payload := `{"action":"opened","number":1}`

	handler := NewGitHubHandler(secret, func(event *GitHubEvent) error {
		t.Error("handler should not be called with missing signature")
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "pull_request")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
