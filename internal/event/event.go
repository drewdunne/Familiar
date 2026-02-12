package event

import (
	"fmt"
	"time"
)

// Type represents the type of webhook event.
type Type string

const (
	TypeMROpened  Type = "mr_opened"
	TypeMRComment Type = "mr_comment"
	TypeMRUpdated Type = "mr_updated"
	TypeMention   Type = "mention"
)

// Event represents a normalized webhook event.
type Event struct {
	// Type is the event type.
	Type Type

	// Provider is the git provider (github, gitlab).
	Provider string

	// Repository information.
	RepoOwner string
	RepoName  string
	RepoURL   string

	// Merge request information.
	MRNumber      int
	MRTitle       string
	MRDescription string
	SourceBranch  string
	TargetBranch  string

	// Comment information (for TypeMRComment and TypeMention).
	CommentID           int
	CommentBody         string
	CommentAuthor       string
	CommentFilePath     string // File path for line-level comments (diff notes)
	CommentLine         int    // Line number for line-level comments
	CommentDiscussionID string // Discussion thread ID for threaded replies

	// Actor who triggered the event.
	Actor string

	// Timestamp of the event.
	Timestamp time.Time

	// RawPayload is the original webhook payload.
	RawPayload []byte
}

// Key returns a unique key for this event (used for debouncing).
func (e *Event) Key() string {
	return e.Provider + "/" + e.RepoOwner + "/" + e.RepoName + "/" + string(e.Type) + "/" + fmt.Sprint(e.MRNumber)
}
