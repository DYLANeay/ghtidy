package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DYLANeay/ghtidy/internal/github"
)

// mode tells whether keys drive the list or the filter input
type mode int

const (
	modeList mode = iota
	modeFilter
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
}

// NewModel builds the initial model, ready to fetch repositories
func NewModel(service github.RepoService) Model {
	return Model{
		service:  service,
		selected: make(map[string]bool),
		loading:  true,
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
		m.loading = false
		return m, nil
	case errMsg:
		m.err = msg.err
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		if m.mode == modeFilter {
			return m.updateFilter(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

// updateList handles keys while navigating the list
func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
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

// applyFilter recomputes the visible repos from the current filter text
func (m *Model) applyFilter() {
	if m.filter == "" {
		m.filtered = m.repos
		return
	}
	// fresh slice on purpose: reusing the old backing array would overwrite
	// m.repos while we iterate on it, since filtered aliases it
	filtered := make([]github.Repo, 0, len(m.repos))
	needle := strings.ToLower(m.filter)
	for _, repo := range m.repos {
		if strings.Contains(strings.ToLower(repo.FullName), needle) {
			filtered = append(filtered, repo)
		}
	}
	m.filtered = filtered
	// keep the cursor inside the visible list
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
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
	if m.mode == modeFilter {
		fmt.Fprintf(&b, "filter: %s\n", m.filter)
	} else {
		fmt.Fprintf(&b, "%d selected\n", m.selectedCount())
		b.WriteString("j/k or arrows: move | space: toggle | a: select all | A: clear | /: filter | esc: clear filter | q: quit\n")
	}
	return b.String()
}
