package event

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/drewdunne/familiar/internal/webhook"
)

type gitLabPayload struct {
	ObjectKind       string `json:"object_kind"`
	ObjectAttributes struct {
		IID          int    `json:"iid"`
		ID           int    `json:"id"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		Note         string `json:"note"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		Action       string `json:"action"`
		NoteableType string `json:"noteable_type"`
		DiscussionID string `json:"discussion_id"`
		Position     struct {
			NewPath string `json:"new_path"`
			NewLine int    `json:"new_line"`
		} `json:"position"`
	} `json:"object_attributes"`
	MergeRequest struct {
		IID          int    `json:"iid"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
	} `json:"merge_request"`
	Project struct {
		PathWithNamespace string `json:"path_with_namespace"`
		GitHTTPURL        string `json:"git_http_url"`
	} `json:"project"`
	User struct {
		Username string `json:"username"`
	} `json:"user"`
}

// NormalizeGitLabEvent converts a GitLab webhook event to a normalized Event.
func NormalizeGitLabEvent(glEvent *webhook.GitLabEvent) (*Event, error) {
	var payload gitLabPayload
	if err := json.Unmarshal(glEvent.RawPayload, &payload); err != nil {
		return nil, fmt.Errorf("parsing payload: %w", err)
	}

	parts := strings.SplitN(payload.Project.PathWithNamespace, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid project path: %s", payload.Project.PathWithNamespace)
	}

	event := &Event{
		Provider:   "gitlab",
		RepoOwner:  parts[0],
		RepoName:   parts[1],
		RepoURL:    payload.Project.GitHTTPURL,
		Actor:      payload.User.Username,
		Timestamp:  time.Now(),
		RawPayload: glEvent.RawPayload,
	}

	switch payload.ObjectKind {
	case "merge_request":
		event.MRNumber = payload.ObjectAttributes.IID
		event.MRTitle = payload.ObjectAttributes.Title
		event.MRDescription = payload.ObjectAttributes.Description
		event.SourceBranch = payload.ObjectAttributes.SourceBranch
		event.TargetBranch = payload.ObjectAttributes.TargetBranch

		switch payload.ObjectAttributes.Action {
		case "open":
			event.Type = TypeMROpened
		case "update":
			event.Type = TypeMRUpdated
		default:
			return nil, fmt.Errorf("unhandled merge_request action: %s", payload.ObjectAttributes.Action)
		}

	case "note":
		if payload.ObjectAttributes.NoteableType != "MergeRequest" {
			return nil, fmt.Errorf("note on non-MR not supported")
		}
		event.MRNumber = payload.MergeRequest.IID
		event.SourceBranch = payload.MergeRequest.SourceBranch
		event.TargetBranch = payload.MergeRequest.TargetBranch
		event.CommentID = payload.ObjectAttributes.ID
		event.CommentBody = payload.ObjectAttributes.Note
		event.CommentAuthor = payload.User.Username
		event.CommentFilePath = payload.ObjectAttributes.Position.NewPath
		event.CommentLine = payload.ObjectAttributes.Position.NewLine
		event.CommentDiscussionID = payload.ObjectAttributes.DiscussionID

		if containsMention(payload.ObjectAttributes.Note) {
			event.Type = TypeMention
		} else {
			event.Type = TypeMRComment
		}

	default:
		return nil, fmt.Errorf("unhandled object_kind: %s", payload.ObjectKind)
	}

	return event, nil
}
