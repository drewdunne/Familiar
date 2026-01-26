package gitlab

import (
	"context"
	"fmt"
	"net/url"

	"github.com/drewdunne/familiar/internal/provider"
	"github.com/xanzy/go-gitlab"
)

// GitLabProvider implements provider.Provider for GitLab.
type GitLabProvider struct {
	client *gitlab.Client
	token  string
}

// Option configures the GitLab provider.
type Option func(*GitLabProvider)

// WithBaseURL sets a custom base URL (for testing).
func WithBaseURL(baseURL string) Option {
	return func(p *GitLabProvider) {
		p.client, _ = gitlab.NewClient(p.token, gitlab.WithBaseURL(baseURL+"/api/v4"))
	}
}

// New creates a new GitLab provider.
func New(token string, opts ...Option) *GitLabProvider {
	client, _ := gitlab.NewClient(token)
	p := &GitLabProvider{client: client, token: token}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Name returns the provider name.
func (p *GitLabProvider) Name() string {
	return "gitlab"
}

// projectPath encodes owner/repo for GitLab API.
func projectPath(owner, repo string) string {
	return url.PathEscape(owner + "/" + repo)
}

// GetRepository fetches repository metadata.
func (p *GitLabProvider) GetRepository(ctx context.Context, owner, repo string) (*provider.Repository, error) {
	project, _, err := p.client.Projects.GetProject(projectPath(owner, repo), nil)
	if err != nil {
		return nil, fmt.Errorf("fetching project: %w", err)
	}

	return &provider.Repository{
		ID:            project.ID,
		Name:          project.Name,
		FullName:      project.PathWithNamespace,
		CloneURL:      project.HTTPURLToRepo,
		SSHURL:        project.SSHURLToRepo,
		DefaultBranch: project.DefaultBranch,
	}, nil
}

// GetMergeRequest fetches a merge request by IID.
func (p *GitLabProvider) GetMergeRequest(ctx context.Context, owner, repo string, number int) (*provider.MergeRequest, error) {
	mr, _, err := p.client.MergeRequests.GetMergeRequest(projectPath(owner, repo), number, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching merge request: %w", err)
	}

	result := &provider.MergeRequest{
		ID:           mr.ID,
		Number:       mr.IID,
		Title:        mr.Title,
		Description:  mr.Description,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		State:        mr.State,
		URL:          mr.WebURL,
	}

	if mr.Author != nil {
		result.Author = mr.Author.Username
	}
	if mr.CreatedAt != nil {
		result.CreatedAt = *mr.CreatedAt
	}
	if mr.UpdatedAt != nil {
		result.UpdatedAt = *mr.UpdatedAt
	}

	return result, nil
}

// GetChangedFiles returns files changed in a merge request.
func (p *GitLabProvider) GetChangedFiles(ctx context.Context, owner, repo string, number int) ([]provider.ChangedFile, error) {
	changes, _, err := p.client.MergeRequests.GetMergeRequestChanges(projectPath(owner, repo), number, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching merge request changes: %w", err)
	}

	result := make([]provider.ChangedFile, len(changes.Changes))
	for i, c := range changes.Changes {
		status := "modified"
		if c.NewFile {
			status = "added"
		} else if c.DeletedFile {
			status = "deleted"
		} else if c.RenamedFile {
			status = "renamed"
		}
		result[i] = provider.ChangedFile{
			Path:   c.NewPath,
			Status: status,
		}
	}
	return result, nil
}

// PostComment posts a comment on a merge request.
func (p *GitLabProvider) PostComment(ctx context.Context, owner, repo string, number int, body string) error {
	_, _, err := p.client.Notes.CreateMergeRequestNote(projectPath(owner, repo), number, &gitlab.CreateMergeRequestNoteOptions{
		Body: &body,
	})
	if err != nil {
		return fmt.Errorf("posting comment: %w", err)
	}
	return nil
}

// GetComments fetches comments on a merge request.
func (p *GitLabProvider) GetComments(ctx context.Context, owner, repo string, number int) ([]provider.Comment, error) {
	notes, _, err := p.client.Notes.ListMergeRequestNotes(projectPath(owner, repo), number, nil)
	if err != nil {
		return nil, fmt.Errorf("listing comments: %w", err)
	}

	result := make([]provider.Comment, len(notes))
	for i, n := range notes {
		result[i] = provider.Comment{
			ID:     n.ID,
			Body:   n.Body,
			Author: n.Author.Username,
		}
		if n.CreatedAt != nil {
			result[i].CreatedAt = *n.CreatedAt
		}
	}
	return result, nil
}
