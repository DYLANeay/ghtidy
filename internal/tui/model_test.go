package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DYLANeay/ghtidy/internal/github"
)

// fakeService returns fixed repos without any network call
type fakeService struct {
	repos []github.Repo
	err   error
}

func (f fakeService) ListOwned(ctx context.Context) ([]github.Repo, error) {
	return f.repos, f.err
}

// sampleRepos gives three repos to drive the tests
func sampleRepos() []github.Repo {
	return []github.Repo{
		{FullName: "dylan/alpha", Visibility: github.VisibilityPublic},
		{FullName: "dylan/beta", Visibility: github.VisibilityPrivate},
		{FullName: "dylan/gamma", Visibility: github.VisibilityPublic},
	}
}

func TestReposLoadedFillsBothLists(t *testing.T) {
	model := NewModel(fakeService{repos: sampleRepos()})

	updated, _ := model.Update(reposLoadedMsg{repos: sampleRepos()})
	m := updated.(Model)

	if m.loading {
		t.Error("loading should be false after repos arrive")
	}
	if len(m.repos) != 3 || len(m.filtered) != 3 {
		t.Errorf("expected 3 repos and 3 filtered, got %d and %d", len(m.repos), len(m.filtered))
	}
}

func TestErrorMessageStopsLoading(t *testing.T) {
	model := NewModel(fakeService{err: errors.New("boom")})

	updated, _ := model.Update(errMsg{err: errors.New("boom")})
	m := updated.(Model)

	if m.loading {
		t.Error("loading should be false after an error")
	}
	if m.err == nil {
		t.Error("err should be set")
	}
	if !strings.Contains(m.View(), "boom") {
		t.Error("view should show the error")
	}
}

func TestCursorMovesWithinBounds(t *testing.T) {
	model := NewModel(fakeService{})
	model.repos = sampleRepos()
	model.applyFilter()

	// move down twice: cursor on last repo
	m := model
	for i := 0; i < 2; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
	}
	if m.cursor != 2 {
		t.Errorf("expected cursor 2, got %d", m.cursor)
	}

	// one more down: stays on last
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.cursor != 2 {
		t.Errorf("cursor should stay at 2, got %d", m.cursor)
	}

	// up above the top is also clamped
	for i := 0; i < 5; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		m = updated.(Model)
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}
}

func TestFilterNarrowsTheList(t *testing.T) {
	model := NewModel(fakeService{})
	model.repos = sampleRepos()
	model.applyFilter()

	// enter filter mode and type "alp"
	m := enterFilter(t, model, "alp")

	if len(m.filtered) != 1 || m.filtered[0].FullName != "dylan/alpha" {
		t.Errorf("expected only dylan/alpha, got %v", m.filtered)
	}
	// cursor must be reset into the visible range
	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}
}

func TestFilterIsCaseInsensitive(t *testing.T) {
	model := NewModel(fakeService{})
	model.repos = sampleRepos()
	model.applyFilter()

	m := enterFilter(t, model, "BETA")

	if len(m.filtered) != 1 || m.filtered[0].FullName != "dylan/beta" {
		t.Errorf("expected dylan/beta, got %v", m.filtered)
	}
}

func TestEscClearsTheFilter(t *testing.T) {
	model := NewModel(fakeService{})
	model.repos = sampleRepos()
	model.applyFilter()

	m := enterFilter(t, model, "alp")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.filter != "" {
		t.Errorf("expected empty filter, got %q", m.filter)
	}
	if len(m.filtered) != 3 {
		t.Errorf("expected 3 repos after clearing, got %d", len(m.filtered))
	}
}

func TestQuitOnQ(t *testing.T) {
	model := NewModel(fakeService{})
	model.repos = sampleRepos()
	model.applyFilter()

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected a quit command, got nil")
	}
	// the command must produce tea.QuitMsg
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}
}

// enterFilter switches to filter mode and types the given text
func enterFilter(t *testing.T, m Model, text string) Model {
	t.Helper()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = updated.(Model)
	for _, r := range text {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	return m
}
