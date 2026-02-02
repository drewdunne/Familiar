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

	// AgentEnv returns environment variables for agent containers to authenticate
	// with the provider's API via CLI tools (gh, glab).
	AgentEnv() map[string]string

	// AuthenticatedCloneURL returns a clone URL with embedded credentials.
	// The rawURL is the original clone URL from the webhook payload.
	AuthenticatedCloneURL(rawURL string) (string, error)
}
