package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/drewdunne/familiar/internal/provider"
	"github.com/google/go-github/v60/github"
)

// GitHubProvider implements provider.Provider for GitHub.
type GitHubProvider struct {
	client *github.Client
	token  string
}

// Option configures the GitHub provider.
type Option func(*GitHubProvider)

// WithBaseURL sets a custom base URL (for testing).
func WithBaseURL(url string) Option {
	return func(p *GitHubProvider) {
		p.client.BaseURL, _ = p.client.BaseURL.Parse(url + "/")
	}
}

// New creates a new GitHub provider.
func New(token string, opts ...Option) *GitHubProvider {
	httpClient := &http.Client{
		Transport: &tokenTransport{token: token},
	}
	client := github.NewClient(httpClient)

	p := &GitHubProvider{
		client: client,
		token:  token,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// tokenTransport adds authorization header to requests.
type tokenTransport struct {
	token string
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(req)
}

// Name returns the provider name.
func (p *GitHubProvider) Name() string {
	return "github"
}

// GetRepository fetches repository metadata.
func (p *GitHubProvider) GetRepository(ctx context.Context, owner, repo string) (*provider.Repository, error) {
	r, _, err := p.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("fetching repository: %w", err)
	}

	return &provider.Repository{
		ID:            int(r.GetID()),
		Name:          r.GetName(),
		FullName:      r.GetFullName(),
		CloneURL:      r.GetCloneURL(),
		SSHURL:        r.GetSSHURL(),
		DefaultBranch: r.GetDefaultBranch(),
	}, nil
}

// GetMergeRequest fetches a pull request by number.
func (p *GitHubProvider) GetMergeRequest(ctx context.Context, owner, repo string, number int) (*provider.MergeRequest, error) {
	pr, _, err := p.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("fetching pull request: %w", err)
	}

	return &provider.MergeRequest{
		ID:           int(pr.GetID()),
		Number:       pr.GetNumber(),
		Title:        pr.GetTitle(),
		Description:  pr.GetBody(),
		SourceBranch: pr.GetHead().GetRef(),
		TargetBranch: pr.GetBase().GetRef(),
		State:        pr.GetState(),
		Author:       pr.GetUser().GetLogin(),
		URL:          pr.GetHTMLURL(),
		CreatedAt:    pr.GetCreatedAt().Time,
		UpdatedAt:    pr.GetUpdatedAt().Time,
	}, nil
}

// GetChangedFiles returns files changed in a pull request.
func (p *GitHubProvider) GetChangedFiles(ctx context.Context, owner, repo string, number int) ([]provider.ChangedFile, error) {
	files, _, err := p.client.PullRequests.ListFiles(ctx, owner, repo, number, nil)
	if err != nil {
		return nil, fmt.Errorf("listing changed files: %w", err)
	}

	result := make([]provider.ChangedFile, len(files))
	for i, f := range files {
		result[i] = provider.ChangedFile{
			Path:      f.GetFilename(),
			Status:    f.GetStatus(),
			Additions: f.GetAdditions(),
			Deletions: f.GetDeletions(),
		}
	}
	return result, nil
}

// PostComment posts a comment on a pull request.
func (p *GitHubProvider) PostComment(ctx context.Context, owner, repo string, number int, body string) error {
	_, _, err := p.client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{
		Body: &body,
	})
	if err != nil {
		return fmt.Errorf("posting comment: %w", err)
	}
	return nil
}

// GetComments fetches comments on a pull request.
func (p *GitHubProvider) GetComments(ctx context.Context, owner, repo string, number int) ([]provider.Comment, error) {
	comments, _, err := p.client.Issues.ListComments(ctx, owner, repo, number, nil)
	if err != nil {
		return nil, fmt.Errorf("listing comments: %w", err)
	}

	result := make([]provider.Comment, len(comments))
	for i, c := range comments {
		result[i] = provider.Comment{
			ID:        int(c.GetID()),
			Body:      c.GetBody(),
			Author:    c.GetUser().GetLogin(),
			CreatedAt: c.GetCreatedAt().Time,
		}
	}
	return result, nil
}

// AgentEnv returns environment variables for agent containers to authenticate
// with the GitHub API via gh CLI.
func (p *GitHubProvider) AgentEnv() map[string]string {
	return map[string]string{
		"GITHUB_TOKEN": p.token,
	}
}

// AuthenticatedCloneURL returns a clone URL with embedded GitHub token.
// Format: https://x-access-token:TOKEN@github.com/org/repo.git
func (p *GitHubProvider) AuthenticatedCloneURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing URL: %w", err)
	}

	// Embed token as password with x-access-token username (GitHub convention)
	parsed.User = url.UserPassword("x-access-token", p.token)
	return parsed.String(), nil
}
