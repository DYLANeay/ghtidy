// Package github wraps the github api behind interfaces the rest of the app depends on
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	gh "github.com/google/go-github/v76/github"
)

// visibility values used by the github api
const (
	VisibilityPublic   = "public"
	VisibilityPrivate  = "private"
	VisibilityInternal = "internal"
)

// Repo is the subset of repository fields ghtidy cares about,
// kept independent from the go-github types so the api client stays an implementation detail
type Repo struct {
	Name        string
	FullName    string
	Owner       string
	Visibility  string
	Archived    bool
	Fork        bool
	Description string
	HTMLURL     string
	PushedAt    time.Time
	CreatedAt   time.Time
}

// RepoLister lists the repositories the authenticated user can act on
type RepoLister interface {
	ListOwned(ctx context.Context) ([]Repo, error)
}

// RepoEditor changes or removes one repository
type RepoEditor interface {
	Archive(ctx context.Context, owner, name string) error
	SetVisibility(ctx context.Context, owner, name, visibility string) error
	Delete(ctx context.Context, owner, name string) error
}

// RepoService is everything the app needs from github
type RepoService interface {
	RepoLister
	RepoEditor
}

// Client talks to the real github api through go-github
type Client struct {
	api *gh.Client
}

// NewClient builds a Client authenticated with the given token
func NewClient(token string) *Client {
	return &Client{api: gh.NewClient(nil).WithAuthToken(token)}
}

// newClientWithBaseURL points the api client at a custom base url, used in tests
func newClientWithBaseURL(token string, baseURL string) (*Client, error) {
	api, err := gh.NewClient(nil).WithAuthToken(token).WithEnterpriseURLs(baseURL, baseURL)
	if err != nil {
		return nil, fmt.Errorf("build client with base url: %w", err)
	}
	return &Client{api: api}, nil
}

// ListOwned fetches every repository owned by the authenticated user, following pagination
func (c *Client) ListOwned(ctx context.Context) ([]Repo, error) {
	// affiliation=owner excludes repos the user only collaborates on,
	// visibility=all makes sure private repos are listed too
	opts := &gh.RepositoryListByAuthenticatedUserOptions{
		Affiliation: "owner",
		Visibility:  "all",
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	var repos []Repo
	for {
		page, resp, err := c.api.Repositories.ListByAuthenticatedUser(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list repositories: %w", err)
		}
		for _, r := range page {
			repos = append(repos, repoFromAPI(r))
		}
		// NextPage is 0 once we have reached the last page
		if resp.NextPage == 0 {
			return repos, nil
		}
		opts.Page = resp.NextPage
	}
}

// Archive marks a repository as archived (read only)
func (c *Client) Archive(ctx context.Context, owner, name string) error {
	update := &gh.Repository{Archived: gh.Ptr(true)}
	if _, _, err := c.api.Repositories.Edit(ctx, owner, name, update); err != nil {
		return fmt.Errorf("archive %s/%s: %w", owner, name, err)
	}
	return nil
}

// SetVisibility makes a repository public or private
func (c *Client) SetVisibility(ctx context.Context, owner, name, visibility string) error {
	if visibility != VisibilityPublic && visibility != VisibilityPrivate {
		return fmt.Errorf("set visibility %s/%s: unsupported visibility %q", owner, name, visibility)
	}
	update := &gh.Repository{Visibility: gh.Ptr(visibility)}
	if _, _, err := c.api.Repositories.Edit(ctx, owner, name, update); err != nil {
		return fmt.Errorf("set visibility %s/%s: %w", owner, name, err)
	}
	return nil
}

// Delete permanently removes a repository, the token needs the delete_repo scope
func (c *Client) Delete(ctx context.Context, owner, name string) error {
	if _, err := c.api.Repositories.Delete(ctx, owner, name); err != nil {
		if isForbidden(err) {
			return fmt.Errorf("delete %s/%s: token is missing the delete_repo scope: %w", owner, name, err)
		}
		return fmt.Errorf("delete %s/%s: %w", owner, name, err)
	}
	return nil
}

// isForbidden tells whether the api answered 403
func isForbidden(err error) bool {
	var apiErr *gh.ErrorResponse
	if !errors.As(err, &apiErr) || apiErr.Response == nil {
		return false
	}
	return apiErr.Response.StatusCode == http.StatusForbidden
}

// repoFromAPI converts the go-github type into our own Repo
func repoFromAPI(r *gh.Repository) Repo {
	visibility := VisibilityPublic
	if r.GetPrivate() {
		visibility = VisibilityPrivate
	}
	return Repo{
		Name:        r.GetName(),
		FullName:    r.GetFullName(),
		Owner:       r.GetOwner().GetLogin(),
		Visibility:  visibility,
		Archived:    r.GetArchived(),
		Fork:        r.GetFork(),
		Description: r.GetDescription(),
		HTMLURL:     r.GetHTMLURL(),
		PushedAt:    r.GetPushedAt().Time,
		CreatedAt:   r.GetCreatedAt().Time,
	}
}
