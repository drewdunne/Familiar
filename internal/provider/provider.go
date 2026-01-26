package provider

import "context"

// Provider defines the interface for git provider operations.
type Provider interface {
	// Name returns the provider name (github, gitlab).
	Name() string

	// GetRepository fetches repository metadata.
	GetRepository(ctx context.Context, owner, repo string) (*Repository, error)

	// GetMergeRequest fetches a merge request by number.
	GetMergeRequest(ctx context.Context, owner, repo string, number int) (*MergeRequest, error)

	// GetChangedFiles returns files changed in a merge request.
	GetChangedFiles(ctx context.Context, owner, repo string, number int) ([]ChangedFile, error)

	// PostComment posts a comment on a merge request.
	PostComment(ctx context.Context, owner, repo string, number int, body string) error

	// GetComments fetches comments on a merge request.
	GetComments(ctx context.Context, owner, repo string, number int) ([]Comment, error)
}
