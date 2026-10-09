// Package github wraps the github api behind interfaces the rest of the app depends on
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
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

// maxDetailCommits is how many recent commits the detail view shows
const maxDetailCommits = 5

// Language is one language of a repository with the bytes of code written in it
type Language struct {
	Name  string
	Bytes int
}

// Commit is a recent commit, reduced to what the detail view shows
type Commit struct {
	SHA     string
	Message string
	Author  string
	Date    time.Time
}

// RepoDetail is a repository with the extra data fetched when opening it
type RepoDetail struct {
	Repo            Repo
	CloneURL        string
	SSHURL          string
	PrimaryLanguage string
	// sorted from the most used to the least used
	Languages []Language
	Commits   []Commit
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

// RepoDetailer loads the detail of one repository
type RepoDetailer interface {
	Detail(ctx context.Context, owner, name string) (RepoDetail, error)
}

// RepoService is everything the app needs from github
type RepoService interface {
	RepoLister
	RepoEditor
	RepoDetailer
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
		if hasStatus(err, http.StatusForbidden) {
			return fmt.Errorf("delete %s/%s: token is missing the delete_repo scope: %w", owner, name, err)
		}
		return fmt.Errorf("delete %s/%s: %w", owner, name, err)
	}
	return nil
}

// Detail fetches a repository with its languages and latest commits, stopping at the first error
func (c *Client) Detail(ctx context.Context, owner, name string) (RepoDetail, error) {
	repo, err := c.fetchRepo(ctx, owner, name)
	if err != nil {
		return RepoDetail{}, err
	}
	languages, err := c.fetchLanguages(ctx, owner, name)
	if err != nil {
		return RepoDetail{}, err
	}
	commits, err := c.fetchCommits(ctx, owner, name)
	if err != nil {
		return RepoDetail{}, err
	}

	return RepoDetail{
		Repo:            repoFromAPI(repo),
		CloneURL:        repo.GetCloneURL(),
		SSHURL:          repo.GetSSHURL(),
		PrimaryLanguage: repo.GetLanguage(),
		Languages:       languages,
		Commits:         commits,
	}, nil
}

func (c *Client) fetchRepo(ctx context.Context, owner, name string) (*gh.Repository, error) {
	repo, _, err := c.api.Repositories.Get(ctx, owner, name)
	if err != nil {
		return nil, fmt.Errorf("get %s/%s: %w", owner, name, err)
	}
	return repo, nil
}

func (c *Client) fetchLanguages(ctx context.Context, owner, name string) ([]Language, error) {
	bytesByName, _, err := c.api.Repositories.ListLanguages(ctx, owner, name)
	if err != nil {
		return nil, fmt.Errorf("list languages of %s/%s: %w", owner, name, err)
	}
	return languagesFromAPI(bytesByName), nil
}

func (c *Client) fetchCommits(ctx context.Context, owner, name string) ([]Commit, error) {
	opts := &gh.CommitsListOptions{ListOptions: gh.ListOptions{PerPage: maxDetailCommits}}
	page, _, err := c.api.Repositories.ListCommits(ctx, owner, name, opts)
	if err != nil {
		// the api answers 409 for a repo that has no commit yet
		if isConflict(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list commits of %s/%s: %w", owner, name, err)
	}

	commits := make([]Commit, 0, len(page))
	for _, c := range page {
		commits = append(commits, commitFromAPI(c))
	}
	return commits, nil
}

// hasStatus tells whether the api answered with the given http status
func hasStatus(err error, status int) bool {
	var apiErr *gh.ErrorResponse
	if !errors.As(err, &apiErr) || apiErr.Response == nil {
		return false
	}
	return apiErr.Response.StatusCode == status
}

// isConflict tells whether the api answered 409
func isConflict(err error) bool {
	return hasStatus(err, http.StatusConflict)
}

// languagesFromAPI turns the name to bytes map into a list, biggest first
func languagesFromAPI(bytesByName map[string]int) []Language {
	languages := make([]Language, 0, len(bytesByName))
	for name, bytes := range bytesByName {
		languages = append(languages, Language{Name: name, Bytes: bytes})
	}
	// map order is random, so ties fall back on the name to stay stable
	sort.Slice(languages, func(i, j int) bool {
		if languages[i].Bytes != languages[j].Bytes {
			return languages[i].Bytes > languages[j].Bytes
		}
		return languages[i].Name < languages[j].Name
	})
	return languages
}

// commitFromAPI converts a commit, keeping only the first line of the message
func commitFromAPI(c *gh.RepositoryCommit) Commit {
	message, _, _ := strings.Cut(c.GetCommit().GetMessage(), "\n")
	author := c.GetCommit().GetAuthor()
	return Commit{
		SHA:     c.GetSHA(),
		Message: message,
		Author:  author.GetName(),
		Date:    author.GetDate().Time,
	}
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
