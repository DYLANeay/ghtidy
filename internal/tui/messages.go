// Package tui holds the bubble tea screens of ghtidy
package tui

import (
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
