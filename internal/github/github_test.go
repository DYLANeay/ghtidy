package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// repoJSON is the shape of one repository as the github api returns it,
// with only the fields our conversion reads
type repoJSON struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
	Private     bool   `json:"private"`
	Archived    bool   `json:"archived"`
	Fork        bool   `json:"fork"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
	PushedAt    string `json:"pushed_at"`
	CreatedAt   string `json:"created_at"`
}

// fakeRepo builds the api json for a repo, keeping test data compact
func fakeRepo(name string, private bool, pushedAt string) repoJSON {
	r := repoJSON{
		Name:      name,
		FullName:  "dylan/" + name,
		Private:   private,
		HTMLURL:   "https://github.com/dylan/" + name,
		PushedAt:  pushedAt,
		CreatedAt: "2024-01-10T09:00:00Z",
	}
	r.Owner.Login = "dylan"
	return r
}

func TestListOwnedFollowsPagination(t *testing.T) {
	// the api answers with 2 pages: one repo on the first, two on the second
	pages := map[int][]repoJSON{
		1: {fakeRepo("alpha", false, "2026-09-01T12:00:00Z")},
		2: {
			fakeRepo("beta", true, "2026-08-15T10:30:00Z"),
			fakeRepo("gamma", false, "2026-07-20T08:00:00Z"),
		},
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// WithEnterpriseURLs prefixes api paths with /api/v3
		if r.URL.Path != "/api/v3/user/repos" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var page int
		if _, err := fmt.Sscanf(r.URL.Query().Get("page"), "%d", &page); err != nil || page == 0 {
			page = 1
		}

		if page < len(pages) {
			// tell the client there is a next page, like the real api does with Link headers
			w.Header().Set("Link", fmt.Sprintf(`<%s/api/v3/user/repos?page=%d>; rel="next"`, server.URL, page+1))
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(pages[page]); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := newClientWithBaseURL("fake-token", server.URL+"/")
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	repos, err := client.ListOwned(context.Background())
	if err != nil {
		t.Fatalf("ListOwned: %v", err)
	}

	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}
	if repos[0].Name != "alpha" || repos[1].Name != "beta" || repos[2].Name != "gamma" {
		t.Errorf("unexpected repo order: %v", []string{repos[0].Name, repos[1].Name, repos[2].Name})
	}
}

func TestListOwnedMapsFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repo := fakeRepo("private-thing", true, "2026-09-01T12:00:00Z")
		repo.Archived = true
		repo.Description = "secret project"
		repo.CreatedAt = "2024-01-10T09:00:00Z"

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]repoJSON{repo}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := newClientWithBaseURL("fake-token", server.URL+"/")
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	repos, err := client.ListOwned(context.Background())
	if err != nil {
		t.Fatalf("ListOwned: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}

	repo := repos[0]
	if repo.Name != "private-thing" {
		t.Errorf("name: got %q", repo.Name)
	}
	if repo.FullName != "dylan/private-thing" {
		t.Errorf("full name: got %q", repo.FullName)
	}
	if repo.Owner != "dylan" {
		t.Errorf("owner: got %q", repo.Owner)
	}
	if repo.Visibility != VisibilityPrivate {
		t.Errorf("visibility: got %q, want %q", repo.Visibility, VisibilityPrivate)
	}
	if !repo.Archived {
		t.Error("archived: got false, want true")
	}
	if repo.Description != "secret project" {
		t.Errorf("description: got %q", repo.Description)
	}
	wantPushed := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	if !repo.PushedAt.Equal(wantPushed) {
		t.Errorf("pushed at: got %v, want %v", repo.PushedAt, wantPushed)
	}
}

func TestListOwnedReturnsApiError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client, err := newClientWithBaseURL("bad-token", server.URL+"/")
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	_, err = client.ListOwned(context.Background())
	if err == nil {
		t.Fatal("expected an error on 401, got nil")
	}
}

// editServer records the last request and answers with the given status
func editServer(t *testing.T, status int, gotMethod, gotPath, gotBody *string) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		*gotMethod, *gotPath, *gotBody = r.Method, r.URL.Path, string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"message":"nope"}`))
	}))
	t.Cleanup(server.Close)

	client, err := newClientWithBaseURL("fake-token", server.URL+"/")
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	return client
}

func TestArchiveSendsPatch(t *testing.T) {
	var method, path, body string
	client := editServer(t, http.StatusOK, &method, &path, &body)

	if err := client.Archive(context.Background(), "dylan", "alpha"); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	if method != http.MethodPatch || path != "/api/v3/repos/dylan/alpha" {
		t.Errorf("unexpected request: %s %s", method, path)
	}
	if !strings.Contains(body, `"archived":true`) {
		t.Errorf("body should archive the repo, got %s", body)
	}
}

func TestSetVisibilitySendsVisibility(t *testing.T) {
	var method, path, body string
	client := editServer(t, http.StatusOK, &method, &path, &body)

	if err := client.SetVisibility(context.Background(), "dylan", "alpha", VisibilityPrivate); err != nil {
		t.Fatalf("SetVisibility: %v", err)
	}

	if method != http.MethodPatch || path != "/api/v3/repos/dylan/alpha" {
		t.Errorf("unexpected request: %s %s", method, path)
	}
	if !strings.Contains(body, `"visibility":"private"`) {
		t.Errorf("body should set visibility, got %s", body)
	}
}

func TestSetVisibilityRejectsInternal(t *testing.T) {
	var method, path, body string
	client := editServer(t, http.StatusOK, &method, &path, &body)

	err := client.SetVisibility(context.Background(), "dylan", "alpha", VisibilityInternal)

	if err == nil {
		t.Fatal("expected an error for internal visibility, got nil")
	}
	if method != "" {
		t.Errorf("no request should be sent, got %s %s", method, path)
	}
}

func TestDeleteSendsDelete(t *testing.T) {
	var method, path, body string
	client := editServer(t, http.StatusNoContent, &method, &path, &body)

	if err := client.Delete(context.Background(), "dylan", "alpha"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if method != http.MethodDelete || path != "/api/v3/repos/dylan/alpha" {
		t.Errorf("unexpected request: %s %s", method, path)
	}
}

func TestDeleteForbiddenMentionsScope(t *testing.T) {
	var method, path, body string
	client := editServer(t, http.StatusForbidden, &method, &path, &body)

	err := client.Delete(context.Background(), "dylan", "alpha")

	if err == nil {
		t.Fatal("expected an error on 403, got nil")
	}
	if !strings.Contains(err.Error(), "delete_repo") {
		t.Errorf("error should mention the delete_repo scope, got %v", err)
	}
}

func TestArchiveReturnsApiError(t *testing.T) {
	var method, path, body string
	client := editServer(t, http.StatusNotFound, &method, &path, &body)

	err := client.Archive(context.Background(), "dylan", "alpha")

	if err == nil {
		t.Fatal("expected an error on 404, got nil")
	}
	if !strings.Contains(err.Error(), "archive dylan/alpha") {
		t.Errorf("error should name the action and repo, got %v", err)
	}
}

const detailRepoJSON = `{
	"name": "alpha",
	"full_name": "dylan/alpha",
	"owner": {"login": "dylan"},
	"private": true,
	"description": "first repo",
	"html_url": "https://github.com/dylan/alpha",
	"clone_url": "https://github.com/dylan/alpha.git",
	"ssh_url": "git@github.com:dylan/alpha.git",
	"language": "Go",
	"pushed_at": "2026-09-01T12:00:00Z",
	"created_at": "2024-01-10T09:00:00Z"
}`

// detailServer answers the three endpoints behind Detail and records the commits query
func detailServer(t *testing.T, languages, commits string, commitsStatus int, gotQuery *string) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/dylan/alpha", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(detailRepoJSON))
	})
	mux.HandleFunc("/api/v3/repos/dylan/alpha/languages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(languages))
	})
	mux.HandleFunc("/api/v3/repos/dylan/alpha/commits", func(w http.ResponseWriter, r *http.Request) {
		if gotQuery != nil {
			*gotQuery = r.URL.RawQuery
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(commitsStatus)
		_, _ = w.Write([]byte(commits))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client, err := newClientWithBaseURL("fake-token", server.URL+"/")
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	return client
}

const oneCommitJSON = `[{
	"sha": "abcdef1234567890",
	"commit": {
		"message": "fix: first line\n\nlonger body",
		"author": {"name": "Dylan", "date": "2026-09-01T12:00:00Z"}
	}
}]`

func TestDetailMapsFields(t *testing.T) {
	client := detailServer(t, `{"Go": 900}`, oneCommitJSON, http.StatusOK, nil)

	detail, err := client.Detail(context.Background(), "dylan", "alpha")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}

	if detail.Repo.FullName != "dylan/alpha" || detail.Repo.Visibility != VisibilityPrivate {
		t.Errorf("unexpected repo: %+v", detail.Repo)
	}
	if detail.CloneURL != "https://github.com/dylan/alpha.git" {
		t.Errorf("clone url: got %q", detail.CloneURL)
	}
	if detail.SSHURL != "git@github.com:dylan/alpha.git" {
		t.Errorf("ssh url: got %q", detail.SSHURL)
	}
	if detail.PrimaryLanguage != "Go" {
		t.Errorf("primary language: got %q", detail.PrimaryLanguage)
	}
	if len(detail.Languages) != 1 || detail.Languages[0] != (Language{Name: "Go", Bytes: 900}) {
		t.Errorf("languages: got %+v", detail.Languages)
	}
	if len(detail.Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(detail.Commits))
	}
	commit := detail.Commits[0]
	wantDate := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	if commit.SHA != "abcdef1234567890" || commit.Author != "Dylan" || !commit.Date.Equal(wantDate) {
		t.Errorf("unexpected commit: %+v", commit)
	}
}

func TestDetailSortsLanguages(t *testing.T) {
	client := detailServer(t, `{"Shell": 100, "Go": 900, "Makefile": 100}`, `[]`, http.StatusOK, nil)

	detail, err := client.Detail(context.Background(), "dylan", "alpha")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}

	var names []string
	for _, language := range detail.Languages {
		names = append(names, language.Name)
	}
	// biggest first, ties broken by name
	if got := strings.Join(names, ","); got != "Go,Makefile,Shell" {
		t.Errorf("language order: got %s", got)
	}
}

func TestDetailKeepsFirstLineOfCommitMessage(t *testing.T) {
	client := detailServer(t, `{}`, oneCommitJSON, http.StatusOK, nil)

	detail, err := client.Detail(context.Background(), "dylan", "alpha")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}

	if got := detail.Commits[0].Message; got != "fix: first line" {
		t.Errorf("message: got %q", got)
	}
}

func TestDetailAsksForFiveCommits(t *testing.T) {
	var query string
	client := detailServer(t, `{}`, `[]`, http.StatusOK, &query)

	if _, err := client.Detail(context.Background(), "dylan", "alpha"); err != nil {
		t.Fatalf("detail: %v", err)
	}

	if !strings.Contains(query, "per_page=5") {
		t.Errorf("expected per_page=5 in query, got %q", query)
	}
}

func TestDetailEmptyRepoHasNoCommits(t *testing.T) {
	// the api answers 409 when a repository has no commit yet
	client := detailServer(t, `{}`, `{"message":"Git Repository is empty."}`, http.StatusConflict, nil)

	detail, err := client.Detail(context.Background(), "dylan", "alpha")
	if err != nil {
		t.Fatalf("an empty repo should not be an error, got %v", err)
	}
	if len(detail.Commits) != 0 {
		t.Errorf("expected no commits, got %d", len(detail.Commits))
	}
}

func TestDetailNoLanguages(t *testing.T) {
	client := detailServer(t, `{}`, `[]`, http.StatusOK, nil)

	detail, err := client.Detail(context.Background(), "dylan", "alpha")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if len(detail.Languages) != 0 {
		t.Errorf("expected no languages, got %+v", detail.Languages)
	}
}

func TestDetailReturnsApiError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := newClientWithBaseURL("fake-token", server.URL+"/")
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	if _, err := client.Detail(context.Background(), "dylan", "alpha"); err == nil {
		t.Fatal("expected an error on 404, got nil")
	}
}
