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
	repos     []github.Repo
	err       error
	detail    github.RepoDetail
	detailErr error
}

func (f fakeService) ListOwned(ctx context.Context) ([]github.Repo, error) {
	return f.repos, f.err
}

func (f fakeService) Archive(ctx context.Context, owner, name string) error {
	return f.err
}

func (f fakeService) SetVisibility(ctx context.Context, owner, name, visibility string) error {
	return f.err
}

func (f fakeService) Delete(ctx context.Context, owner, name string) error {
	return f.err
}

func (f fakeService) Detail(ctx context.Context, owner, name string) (github.RepoDetail, error) {
	return f.detail, f.detailErr
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

// loadedModel returns a model with the sample repos loaded
func loadedModel() Model {
	updated, _ := NewModel(fakeService{}).Update(reposLoadedMsg{repos: sampleRepos()})
	return updated.(Model)
}

// press sends one key message and returns the updated model
func press(t *testing.T, m Model, msg tea.KeyMsg) Model {
	t.Helper()
	updated, _ := m.Update(msg)
	return updated.(Model)
}

var (
	keySpace  = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	keyA      = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	keyShiftA = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")}
)

func TestSpaceTogglesSelection(t *testing.T) {
	m := press(t, loadedModel(), keySpace)
	if !m.selected["dylan/alpha"] || m.selectedCount() != 1 {
		t.Errorf("expected only dylan/alpha selected, got %v", m.selected)
	}

	m = press(t, m, keySpace)
	if m.selectedCount() != 0 {
		t.Errorf("expected empty selection, got %v", m.selected)
	}
}

func TestSelectAllSelectsOnlyVisible(t *testing.T) {
	m := enterFilter(t, loadedModel(), "alp")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = press(t, m, keyA)

	if m.selectedCount() != 1 || !m.selected["dylan/alpha"] {
		t.Errorf("expected only dylan/alpha selected, got %v", m.selected)
	}
}

func TestSelectionSurvivesFilterChange(t *testing.T) {
	m := press(t, loadedModel(), keySpace)

	m = enterFilter(t, m, "beta")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if !m.selected["dylan/alpha"] {
		t.Errorf("dylan/alpha should still be selected, got %v", m.selected)
	}
}

func TestShiftAClearsSelection(t *testing.T) {
	m := press(t, loadedModel(), keyA)
	if m.selectedCount() != 3 {
		t.Fatalf("expected 3 selected, got %d", m.selectedCount())
	}

	m = press(t, m, keyShiftA)
	if m.selectedCount() != 0 {
		t.Errorf("expected empty selection, got %v", m.selected)
	}
}

func TestToggleOnEmptyListDoesNothing(t *testing.T) {
	m := NewModel(fakeService{})

	m = press(t, m, keySpace)

	if m.selectedCount() != 0 {
		t.Errorf("expected empty selection, got %v", m.selected)
	}
}

func TestViewShowsMarkersAndCount(t *testing.T) {
	m := press(t, loadedModel(), keySpace)
	view := m.View()

	if !strings.Contains(view, "> [x] dylan/alpha") {
		t.Errorf("expected a checked marker on alpha, got:\n%s", view)
	}
	if !strings.Contains(view, "  [ ] dylan/beta") {
		t.Errorf("expected an unchecked marker on beta, got:\n%s", view)
	}
	if !strings.Contains(view, "1 selected") {
		t.Errorf("expected the selected count, got:\n%s", view)
	}
}

func TestSelectKeysAreTextInFilterMode(t *testing.T) {
	m := loadedModel()
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = press(t, m, keyA)
	m = press(t, m, keyShiftA)

	if m.filter != "aA" {
		t.Errorf("expected filter %q, got %q", "aA", m.filter)
	}
	if m.selectedCount() != 0 {
		t.Errorf("nothing should be selected, got %v", m.selected)
	}
}
