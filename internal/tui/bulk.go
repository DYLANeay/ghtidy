package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DYLANeay/ghtidy/internal/actions"
	"github.com/DYLANeay/ghtidy/internal/github"
)

// deleteConfirmation is what the user has to type before repos get deleted
const deleteConfirmation = "delete"

// selectedRepos returns the selected repos in list order
func (m Model) selectedRepos() []github.Repo {
	var repos []github.Repo
	for _, repo := range m.repos {
		if m.isSelected(repo) {
			repos = append(repos, repo)
		}
	}
	return repos
}

// requestAction moves to the confirmation screen for the selected repos
func (m *Model) requestAction(action actions.Action) {
	targets := m.selectedRepos()
	if len(targets) == 0 {
		m.notice = "select at least one repository first"
		return
	}
	m.pending = action
	m.targets = targets
	m.confirmInput = ""
	m.mode = modeConfirm
	if action.Destructive() {
		m.mode = modeConfirmDelete
	}
}

// updateConfirm handles the y/n question of the safe actions
func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		return m.startAction()
	case "n", "esc":
		m.cancelAction()
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// updateConfirmDelete handles the typed confirmation of destructive actions
func (m Model) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.cancelAction()
	case tea.KeyEnter:
		// exact match on purpose: no trimming, no case folding
		if m.confirmInput == deleteConfirmation {
			return m.startAction()
		}
		m.notice = fmt.Sprintf("type %q exactly to confirm", deleteConfirmation)
		m.confirmInput = ""
	case tea.KeyBackspace:
		if len(m.confirmInput) > 0 {
			m.confirmInput = m.confirmInput[:len(m.confirmInput)-1]
		}
	case tea.KeyRunes:
		m.confirmInput += string(msg.Runes)
	}
	return m, nil
}

// updateRunning ignores everything but quit, the run cannot be interrupted halfway
func (m Model) updateRunning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	return m, nil
}

// updateReport goes back to the list on any key
func (m Model) updateReport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	m.pending = nil
	m.targets = nil
	m.results = nil
	m.resultsCh = nil
	m.notice = ""
	m.mode = modeList
	return m, nil
}

// startAction launches the pending action in the background
func (m Model) startAction() (tea.Model, tea.Cmd) {
	m.results = nil
	m.notice = ""
	m.confirmInput = ""
	m.resultsCh = actions.Run(context.Background(), m.pending, m.targets, actions.DefaultLimit)
	m.mode = modeRunning
	return m, waitForResult(m.resultsCh)
}

// cancelAction forgets the pending action and goes back to the list
func (m *Model) cancelAction() {
	m.pending = nil
	m.targets = nil
	m.confirmInput = ""
	m.mode = modeList
}

func (m Model) viewConfirm() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %d repositories?\n\n", m.pending.Name(), len(m.targets))
	b.WriteString(m.targetList())
	b.WriteString("\ny: confirm | n or esc: cancel\n")
	return b.String()
}

func (m Model) viewConfirmDelete() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %d repositories? this cannot be undone\n\n", m.pending.Name(), len(m.targets))
	b.WriteString(m.targetList())
	fmt.Fprintf(&b, "\ntype %q to confirm: %s\n", deleteConfirmation, m.confirmInput)
	if m.notice != "" {
		fmt.Fprintf(&b, "%s\n", m.notice)
	}
	b.WriteString("enter: confirm | esc: cancel\n")
	return b.String()
}

func (m Model) viewRunning() string {
	return fmt.Sprintf("%s: %d/%d done...\n", m.pending.Name(), len(m.results), len(m.targets))
}

func (m Model) viewReport() string {
	var b strings.Builder
	done, skipped, failed := countResults(m.results)
	fmt.Fprintf(&b, "%s finished: %d done, %d skipped, %d failed\n\n", m.pending.Name(), done, skipped, failed)
	for _, result := range m.results {
		switch {
		case result.Skipped():
			fmt.Fprintf(&b, "  skipped %s\n", result.Repo.FullName)
		case !result.Succeeded():
			fmt.Fprintf(&b, "  failed  %s: %v\n", result.Repo.FullName, result.Err)
		}
	}
	b.WriteString("\npress any key to go back\n")
	return b.String()
}

// targetList renders one line per repo the action will touch
func (m Model) targetList() string {
	var b strings.Builder
	for _, repo := range m.targets {
		fmt.Fprintf(&b, "  %s\n", repo.FullName)
	}
	return b.String()
}

// countResults splits the results into done, skipped and failed
func countResults(results []actions.Result) (done, skipped, failed int) {
	for _, result := range results {
		switch {
		case result.Succeeded():
			done++
		case result.Skipped():
			skipped++
		default:
			failed++
		}
	}
	return done, skipped, failed
}
