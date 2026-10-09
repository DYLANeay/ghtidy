// Package tui holds the bubble tea screens of ghtidy
package tui

import (
	"github.com/DYLANeay/ghtidy/internal/actions"
	"github.com/DYLANeay/ghtidy/internal/github"
)

// reposLoadedMsg carries the repositories once the fetch command finishes
type reposLoadedMsg struct {
	repos []github.Repo
}

// errMsg reports a failure that should be shown to the user
type errMsg struct {
	err error
}

// actionResultMsg carries the outcome of one repo while a bulk action runs
type actionResultMsg struct {
	result actions.Result
}

// actionDoneMsg tells the bulk action has handled every repo
type actionDoneMsg struct{}

// detailLoadedMsg carries the detail of the repo the user opened
type detailLoadedMsg struct {
	detail github.RepoDetail
}

// detailErrMsg reports a failed detail fetch, fullName tells which repo it was for
type detailErrMsg struct {
	fullName string
	err      error
}
