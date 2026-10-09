package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DYLANeay/ghtidy/internal/github"
)

var keyCtrlC = tea.KeyMsg{Type: tea.KeyCtrlC}

// alphaDetail is a fully filled detail for dylan/alpha
func alphaDetail() github.RepoDetail {
	return github.RepoDetail{
		Repo: github.Repo{
			FullName:    "dylan/alpha",
			Visibility:  github.VisibilityPublic,
			Description: "first repo",
			HTMLURL:     "https://github.com/dylan/alpha",
			CreatedAt:   time.Date(2024, 1, 10, 9, 0, 0, 0, time.UTC),
			PushedAt:    time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
		},
		CloneURL: "https://github.com/dylan/alpha.git",
		SSHURL:   "git@github.com:dylan/alpha.git",
		Languages: []github.Language{
			{Name: "Go", Bytes: 750},
			{Name: "Shell", Bytes: 250},
		},
		Commits: []github.Commit{
			{SHA: "abcdef1234567", Message: "fix: typo", Author: "Dylan", Date: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)},
		},
	}
}

// openedDetail returns a model on the detail screen of dylan/alpha
func openedDetail(t *testing.T) Model {
	t.Helper()
	return press(t, loadedModel(), keyEnter)
}

func TestEnterOpensDetailAndFetches(t *testing.T) {
	model := loadedModel()

	updated, cmd := model.Update(keyEnter)
	m := updated.(Model)

	if m.mode != modeDetail || !m.detailLoading {
		t.Errorf("expected detail mode while loading, got mode=%v loading=%v", m.mode, m.detailLoading)
	}
	if m.detailRepo.FullName != "dylan/alpha" {
		t.Errorf("detail repo: got %q", m.detailRepo.FullName)
	}
	if cmd == nil {
		t.Fatal("expected a fetch command")
	}
}

func TestEnterUsesRepoUnderCursorAfterFilter(t *testing.T) {
	m := loadedModel()
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = typeText(t, m, "gamma")
	m = press(t, m, keyEnter)

	m = press(t, m, keyEnter)

	if m.detailRepo.FullName != "dylan/gamma" {
		t.Errorf("detail repo: got %q, want dylan/gamma", m.detailRepo.FullName)
	}
}

func TestEnterOnEmptyListDoesNothing(t *testing.T) {
	updated, _ := NewModel(fakeService{}).Update(reposLoadedMsg{})

	next, cmd := updated.Update(keyEnter)

	if next.(Model).mode != modeList {
		t.Error("should stay on the list")
	}
	if cmd != nil {
		t.Error("should not fetch anything")
	}
}

func TestDetailLoadedIsShown(t *testing.T) {
	m := openedDetail(t)

	updated, _ := m.Update(detailLoadedMsg{detail: alphaDetail()})
	view := updated.(Model).View()

	for _, want := range []string{
		"dylan/alpha", "first repo", "public", "https://github.com/dylan/alpha.git",
		"git@github.com:dylan/alpha.git", "2024-01-10", "2026-09-01",
		"Go 75.0%", "Shell 25.0%", "abcdef1", "fix: typo", "Dylan",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view should contain %q, got:\n%s", want, view)
		}
	}
	if strings.Contains(view, "abcdef12") {
		t.Error("commit hash should be shortened to 7 characters")
	}
}

func TestDetailErrorStaysOnDetailScreen(t *testing.T) {
	m := openedDetail(t)

	updated, _ := m.Update(detailErrMsg{fullName: "dylan/alpha", err: errors.New("boom")})
	m = updated.(Model)

	if m.mode != modeDetail || m.detailLoading {
		t.Errorf("expected detail mode, not loading, got mode=%v loading=%v", m.mode, m.detailLoading)
	}
	if !strings.Contains(m.View(), "boom") {
		t.Error("view should show the error")
	}
	// the screen stays usable
	if press(t, m, keyEsc).mode != modeList {
		t.Error("esc should still go back")
	}
}

func TestEscFromDetailKeepsListState(t *testing.T) {
	m := loadedModel()
	m = press(t, m, keySpace)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = press(t, m, keyEnter)

	m = press(t, m, keyEsc)

	if m.mode != modeList {
		t.Errorf("expected list mode, got %v", m.mode)
	}
	if m.cursor != 1 || !m.selected["dylan/alpha"] || len(m.filtered) != 3 {
		t.Errorf("list state changed: cursor=%d selected=%v filtered=%d", m.cursor, m.selected, len(m.filtered))
	}
	if m.detailRepo.FullName != "" || m.detailLoading {
		t.Error("detail state should be reset")
	}
}

func TestQFromDetailGoesBackWithoutQuitting(t *testing.T) {
	m := openedDetail(t)

	updated, cmd := m.Update(keyQ)

	if updated.(Model).mode != modeList {
		t.Error("q should go back to the list")
	}
	if cmd != nil {
		t.Error("q should not quit from the detail screen")
	}
}

func TestCtrlCQuitsFromDetail(t *testing.T) {
	m := openedDetail(t)

	_, cmd := m.Update(keyCtrlC)

	if cmd == nil {
		t.Fatal("expected a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c should quit")
	}
}

func TestStaleDetailIsIgnored(t *testing.T) {
	m := openedDetail(t)
	m = press(t, m, keyEsc)

	// answer arrives after the user went back
	updated, _ := m.Update(detailLoadedMsg{detail: alphaDetail()})
	if updated.(Model).mode != modeList {
		t.Error("late answer should not reopen the detail screen")
	}

	// answer for another repo while another detail is open
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = press(t, m, keyEnter)
	updated, _ = m.Update(detailLoadedMsg{detail: alphaDetail()})
	if !updated.(Model).detailLoading {
		t.Error("answer for another repo should be ignored")
	}
	updated, _ = m.Update(detailErrMsg{fullName: "dylan/alpha", err: errors.New("boom")})
	if updated.(Model).detailErr != nil {
		t.Error("error for another repo should be ignored")
	}
}

func TestDetailViewFallbacks(t *testing.T) {
	m := openedDetail(t)
	bare := github.RepoDetail{Repo: github.Repo{FullName: "dylan/alpha"}}

	updated, _ := m.Update(detailLoadedMsg{detail: bare})
	view := updated.(Model).View()

	for _, want := range []string{"no description", "none detected", "no commits yet", "created: -", "archived: no"} {
		if !strings.Contains(view, want) {
			t.Errorf("view should contain %q, got:\n%s", want, view)
		}
	}
}

func TestEnterInFilterModeStillClosesFilter(t *testing.T) {
	m := loadedModel()
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})

	updated, cmd := m.Update(keyEnter)

	if updated.(Model).mode != modeList {
		t.Errorf("expected list mode, got %v", updated.(Model).mode)
	}
	if cmd != nil {
		t.Error("enter in filter mode should not open a detail")
	}
}
