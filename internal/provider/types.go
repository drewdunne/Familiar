package provider

import "time"

// MergeRequest represents a merge request/pull request.
type MergeRequest struct {
	ID           int
	Number       int       // PR number (GitHub) or MR IID (GitLab)
	Title        string
	Description  string
	SourceBranch string
	TargetBranch string
	State        string    // open, closed, merged
	Author       string
	URL          string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Comment represents a comment on a merge request.
type Comment struct {
	ID        int
	Body      string
	Author    string
	CreatedAt time.Time
}

// ChangedFile represents a file changed in a merge request.
type ChangedFile struct {
	Path      string
	Status    string // added, modified, deleted, renamed
	Additions int
	Deletions int
}

// Repository represents a git repository.
type Repository struct {
	ID        int
	Name      string
	FullName  string // owner/repo
	CloneURL  string
	SSHURL    string
	DefaultBranch string
}
