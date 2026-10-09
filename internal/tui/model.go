package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DYLANeay/ghtidy/internal/actions"
	"github.com/DYLANeay/ghtidy/internal/github"
)

// mode tells which screen the keys are driving
type mode int

const (
	modeList mode = iota
	modeFilter
	// y/n question before a bulk action
	modeConfirm
	// the user must type "delete" before repos are removed
	modeConfirmDelete
	modeRunning
	modeReport
)

// Model is the whole state of the repo list screen
type Model struct {
	service  github.RepoService
	repos    []github.Repo
	filtered []github.Repo
	// keyed by full name so it survives filtering
	selected map[string]bool
	cursor   int
	filter   string
	mode     mode
	loading  bool
	err      error

	// bulk actions, keyed by the list key that triggers them
	actions      map[string]actions.Action
	pending      actions.Action
	targets      []github.Repo
	confirmInput string
	results      []actions.Result
	resultsCh    <-chan actions.Result
	// one line feedback shown under the list
	notice string
}

// NewModel builds the initial model, ready to fetch repositories
func NewModel(service github.RepoService) Model {
	return Model{
		service:  service,
		selected: make(map[string]bool),
		loading:  true,
		actions: map[string]actions.Action{
			"r": actions.NewArchive(service),
			"v": actions.NewToggleVisibility(service),
			"d": actions.NewDelete(service),
		},
	}
}

// Init kicks off the first command: load the repositories
func (m Model) Init() tea.Cmd {
	return fetchRepos(m.service)
}

// Update handles every message and returns the next state of the screen
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case reposLoadedMsg:
		m.repos = msg.repos
		m.applyFilter()
		m.pruneSelection()
		m.loading = false
		return m, nil
	case errMsg:
		m.err = msg.err
		m.loading = false
		return m, nil
	case actionResultMsg:
		m.results = append(m.results, msg.result)
		return m, waitForResult(m.resultsCh)
	case actionDoneMsg:
		m.mode = modeReport
		// reload so the list shows the new state of the repos
		return m, fetchRepos(m.service)

	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

// updateKey sends a key press to the handler of the current mode
func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeFilter:
		return m.updateFilter(msg)
	case modeConfirm:
		return m.updateConfirm(msg)
	case modeConfirmDelete:
		return m.updateConfirmDelete(msg)
	case modeRunning:
		return m.updateRunning(msg)
	case modeReport:
		return m.updateReport(msg)
	}
	return m.updateList(msg)
}

// updateList handles keys while navigating the list
func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.notice = ""
	key := msg.String()
	if action, ok := m.actions[key]; ok {
		m.requestAction(action)
		return m, nil
	}

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	case " ":
		m.toggleCurrent()
	case "a":
		m.selectAllVisible()
	case "A":
		m.clearSelection()
	case "/":
		m.mode = modeFilter
		m.filter = ""
	case "esc":
		m.filter = ""
		m.applyFilter()
	}
	return m, nil
}

// updateFilter handles keys while typing in the filter input
func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.mode = modeList
	case tea.KeyEsc:
		m.mode = modeList
		m.filter = ""
		m.applyFilter()
	case tea.KeyBackspace:
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.applyFilter()
		}
	case tea.KeyRunes:
		m.filter += string(msg.Runes)
		m.applyFilter()
	}
	return m, nil
}

// toggleCurrent flips the selection of the repo under the cursor
func (m *Model) toggleCurrent() {
	if len(m.filtered) == 0 {
		return
	}
	name := m.filtered[m.cursor].FullName
	if m.selected[name] {
		delete(m.selected, name)
	} else {
		m.selected[name] = true
	}
}

// selectAllVisible selects every repo matching the current filter
func (m *Model) selectAllVisible() {
	for _, repo := range m.filtered {
		m.selected[repo.FullName] = true
	}
}

// clearSelection forgets every selected repo, visible or not
func (m *Model) clearSelection() {
	m.selected = make(map[string]bool)
}

func (m Model) isSelected(repo github.Repo) bool {
	return m.selected[repo.FullName]
}

func (m Model) selectedCount() int {
	return len(m.selected)
}

// pruneSelection drops selected repos that no longer exist, e.g. after a delete
func (m *Model) pruneSelection() {
	existing := make(map[string]bool, len(m.repos))
	for _, repo := range m.repos {
		existing[repo.FullName] = true
	}
	for name := range m.selected {
		if !existing[name] {
			delete(m.selected, name)
		}
	}
}

// applyFilter recomputes the visible repos from the current filter text
func (m *Model) applyFilter() {
	m.filtered = m.repos
	if m.filter != "" {
		m.filtered = m.matchingRepos()
	}
	// keep the cursor inside the visible list
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

// matchingRepos returns the repos whose name contains the filter text
func (m Model) matchingRepos() []github.Repo {
	// fresh slice on purpose: reusing the old backing array would overwrite
	// m.repos while we iterate on it, since filtered aliases it
	matching := make([]github.Repo, 0, len(m.repos))
	needle := strings.ToLower(m.filter)
	for _, repo := range m.repos {
		if strings.Contains(strings.ToLower(repo.FullName), needle) {
			matching = append(matching, repo)
		}
	}
	return matching
}

// View renders the whole screen as a string
func (m Model) View() string {
	var b strings.Builder

	b.WriteString("ghtidy\n\n")

	if m.loading {
		b.WriteString("loading repositories...\n")
		return b.String()
	}
	if m.err != nil {
		fmt.Fprintf(&b, "error: %v\n", m.err)
		return b.String()
	}

	switch m.mode {
	case modeConfirm:
		b.WriteString(m.viewConfirm())
	case modeConfirmDelete:
		b.WriteString(m.viewConfirmDelete())
	case modeRunning:
		b.WriteString(m.viewRunning())
	case modeReport:
		b.WriteString(m.viewReport())
	default:
		b.WriteString(m.viewList())
	}
	return b.String()
}

// viewList renders the repos with their selection marks and the help line
func (m Model) viewList() string {
	var b strings.Builder

	for i, repo := range m.filtered {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		mark := "[ ]"
		if m.isSelected(repo) {
			mark = "[x]"
		}
		fmt.Fprintf(&b, "%s %s %s (%s)\n", cursor, mark, repo.FullName, repo.Visibility)
	}
	if len(m.filtered) == 0 {
		b.WriteString("  no repository matches\n")
	}

	b.WriteString("\n")
	if m.notice != "" {
		fmt.Fprintf(&b, "%s\n", m.notice)
	}
	if m.mode == modeFilter {
		fmt.Fprintf(&b, "filter: %s\n", m.filter)
	} else {
		fmt.Fprintf(&b, "%d selected\n", m.selectedCount())
		b.WriteString("j/k or arrows: move | space: toggle | a: select all | A: clear | /: filter | esc: clear filter | q: quit\n")
		b.WriteString("r: archive | v: toggle visibility | d: delete\n")
	}
	return b.String()
}
