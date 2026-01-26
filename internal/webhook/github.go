package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// GitHubEvent represents a parsed GitHub webhook event.
type GitHubEvent struct {
	EventType  string
	Action     string `json:"action"`
	Number     int    `json:"number"`
	RawPayload []byte
}

// GitHubEventHandler is called when a valid GitHub webhook is received.
type GitHubEventHandler func(event *GitHubEvent) error

// GitHubHandler handles GitHub webhook requests.
type GitHubHandler struct {
	secret  string
	handler GitHubEventHandler
}

// NewGitHubHandler creates a new GitHub webhook handler.
func NewGitHubHandler(secret string, handler GitHubEventHandler) *GitHubHandler {
	return &GitHubHandler{
		secret:  secret,
		handler: handler,
	}
}

// ServeHTTP implements http.Handler.
func (h *GitHubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Verify signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		http.Error(w, "missing signature", http.StatusUnauthorized)
		return
	}

	if !h.verifySignature(body, signature) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse event
	event := &GitHubEvent{
		EventType:  r.Header.Get("X-GitHub-Event"),
		RawPayload: body,
	}
	if err := json.Unmarshal(body, event); err != nil {
		http.Error(w, "failed to parse payload", http.StatusBadRequest)
		return
	}

	// Call handler
	if err := h.handler(event); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// verifySignature verifies the GitHub webhook signature.
func (h *GitHubHandler) verifySignature(payload []byte, signature string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	sig, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(payload)
	expected := mac.Sum(nil)

	return hmac.Equal(sig, expected)
}
