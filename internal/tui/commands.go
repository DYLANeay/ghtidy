package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DYLANeay/ghtidy/internal/actions"
	"github.com/DYLANeay/ghtidy/internal/github"
)

// fetchRepos wraps the blocking api call in a command bubble tea can run in the background
func fetchRepos(service github.RepoService) tea.Cmd {
	return func() tea.Msg {
		repos, err := service.ListOwned(context.Background())
		if err != nil {
			return errMsg{err: err}
		}
		return reposLoadedMsg{repos: repos}
	}
}

// waitForResult blocks on the results channel until one repo is done,
// it has to be scheduled again after each result to keep reading
func waitForResult(results <-chan actions.Result) tea.Cmd {
	return func() tea.Msg {
		result, ok := <-results
		if !ok {
			return actionDoneMsg{}
		}
		return actionResultMsg{result: result}
	}
}
