package webhook

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
)

// GitLabEvent represents a parsed GitLab webhook event.
type GitLabEvent struct {
	EventType        string
	ObjectKind       string `json:"object_kind"`
	ObjectAttributes struct {
		Action string `json:"action"`
	} `json:"object_attributes"`
	RawPayload []byte
}

// GitLabEventHandler is called when a valid GitLab webhook is received.
type GitLabEventHandler func(event *GitLabEvent) error

// GitLabHandler handles GitLab webhook requests.
type GitLabHandler struct {
	secret  string
	handler GitLabEventHandler
}

// NewGitLabHandler creates a new GitLab webhook handler.
func NewGitLabHandler(secret string, handler GitLabEventHandler) *GitLabHandler {
	return &GitLabHandler{
		secret:  secret,
		handler: handler,
	}
}

// ServeHTTP implements http.Handler.
func (h *GitLabHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Verify token
	token := r.Header.Get("X-Gitlab-Token")
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	if subtle.ConstantTimeCompare([]byte(token), []byte(h.secret)) != 1 {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// Parse event
	event := &GitLabEvent{
		EventType:  r.Header.Get("X-Gitlab-Event"),
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
