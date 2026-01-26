package event

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/drewdunne/familiar/internal/webhook"
)

// gitHubPayload represents the common GitHub webhook payload structure.
type gitHubPayload struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		Head  struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"pull_request"`
	Issue struct {
		Number int `json:"number"`
	} `json:"issue"`
	Comment struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

// NormalizeGitHubEvent converts a GitHub webhook event to a normalized Event.
func NormalizeGitHubEvent(ghEvent *webhook.GitHubEvent) (*Event, error) {
	var payload gitHubPayload
	if err := json.Unmarshal(ghEvent.RawPayload, &payload); err != nil {
		return nil, fmt.Errorf("parsing payload: %w", err)
	}

	// Parse owner/repo from full_name
	parts := strings.SplitN(payload.Repository.FullName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository full_name: %s", payload.Repository.FullName)
	}

	event := &Event{
		Provider:   "github",
		RepoOwner:  parts[0],
		RepoName:   parts[1],
		RepoURL:    payload.Repository.CloneURL,
		Actor:      payload.Sender.Login,
		Timestamp:  time.Now(),
		RawPayload: ghEvent.RawPayload,
	}

	switch ghEvent.EventType {
	case "pull_request":
		event.MRNumber = payload.Number
		event.MRTitle = payload.PullRequest.Title
		event.MRDescription = payload.PullRequest.Body
		event.SourceBranch = payload.PullRequest.Head.Ref
		event.TargetBranch = payload.PullRequest.Base.Ref

		switch payload.Action {
		case "opened":
			event.Type = TypeMROpened
		case "synchronize":
			event.Type = TypeMRUpdated
		default:
			return nil, fmt.Errorf("unhandled pull_request action: %s", payload.Action)
		}

	case "issue_comment":
		event.MRNumber = payload.Issue.Number
		event.CommentID = payload.Comment.ID
		event.CommentBody = payload.Comment.Body
		event.CommentAuthor = payload.Comment.User.Login

		// Check for @mention
		if containsMention(payload.Comment.Body) {
			event.Type = TypeMention
		} else {
			event.Type = TypeMRComment
		}

	default:
		return nil, fmt.Errorf("unhandled event type: %s", ghEvent.EventType)
	}

	return event, nil
}

// containsMention checks if the text contains @familiar mention.
func containsMention(text string) bool {
	return strings.Contains(strings.ToLower(text), "@familiar")
}
