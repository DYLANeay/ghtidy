package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DYLANeay/ghtidy/internal/actions"
	"github.com/DYLANeay/ghtidy/internal/github"
)

var (
	keyR     = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	keyV     = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")}
	keyD     = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}
	keyQ     = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	keyY     = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
	keyN     = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	keyEnter = tea.KeyMsg{Type: tea.KeyEnter}
	keyEsc   = tea.KeyMsg{Type: tea.KeyEsc}
)

// typeText sends each rune as its own key press
func typeText(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

// withFirstSelected returns a loaded model with dylan/alpha selected
func withFirstSelected(t *testing.T) Model {
	t.Helper()
	return press(t, loadedModel(), keySpace)
}

func TestActionWithoutSelectionShowsNotice(t *testing.T) {
	m := press(t, loadedModel(), keyR)

	if m.mode != modeList {
		t.Errorf("expected list mode, got %v", m.mode)
	}
	if !strings.Contains(m.View(), "select at least one") {
		t.Errorf("view should show the notice, got:\n%s", m.View())
	}
}

func TestConfirmThenCancel(t *testing.T) {
	m := press(t, withFirstSelected(t), keyR)
	if m.mode != modeConfirm || len(m.targets) != 1 {
		t.Fatalf("expected confirm mode with 1 target, got mode %v and %d", m.mode, len(m.targets))
	}

	m = press(t, m, keyN)

	if m.mode != modeList || m.pending != nil || len(m.targets) != 0 {
		t.Errorf("cancel should reset the state, got mode %v", m.mode)
	}
	if m.selectedCount() != 1 {
		t.Error("cancel should keep the selection")
	}
}

func TestConfirmThenStart(t *testing.T) {
	m := press(t, withFirstSelected(t), keyV)

	updated, cmd := m.Update(keyY)
	m = updated.(Model)

	if m.mode != modeRunning {
		t.Errorf("expected running mode, got %v", m.mode)
	}
	if cmd == nil {
		t.Fatal("expected a command waiting for results")
	}
	if _, ok := cmd().(actionResultMsg); !ok {
		t.Error("the command should produce a result message")
	}
}

func TestDeleteAsksForTypedConfirmation(t *testing.T) {
	m := press(t, withFirstSelected(t), keyD)

	if m.mode != modeConfirmDelete {
		t.Fatalf("expected delete confirmation mode, got %v", m.mode)
	}
	view := m.View()
	if !strings.Contains(view, "cannot be undone") || !strings.Contains(view, "dylan/alpha") {
		t.Errorf("view should warn and list the repo, got:\n%s", view)
	}
}

func TestDeleteWithExactInputStarts(t *testing.T) {
	m := press(t, withFirstSelected(t), keyD)
	m = typeText(t, m, "delete")

	updated, cmd := m.Update(keyEnter)
	m = updated.(Model)

	if m.mode != modeRunning || cmd == nil {
		t.Errorf("expected running mode with a command, got mode %v", m.mode)
	}
}

func TestDeleteWithWrongInputStays(t *testing.T) {
	for _, input := range []string{"Delete", "delete ", "del"} {
		m := press(t, withFirstSelected(t), keyD)
		m = typeText(t, m, input)

		updated, cmd := m.Update(keyEnter)
		m = updated.(Model)

		if m.mode != modeConfirmDelete || cmd != nil {
			t.Errorf("%q should not start the delete, got mode %v", input, m.mode)
		}
	}
}

func TestDeleteWithEmptyInputStays(t *testing.T) {
	m := press(t, withFirstSelected(t), keyD)

	updated, cmd := m.Update(keyEnter)
	m = updated.(Model)

	if m.mode != modeConfirmDelete || cmd != nil {
		t.Errorf("empty input should not start the delete, got mode %v", m.mode)
	}
}

func TestEscCancelsDeleteConfirmation(t *testing.T) {
	m := press(t, withFirstSelected(t), keyD)
	m = typeText(t, m, "dele")

	m = press(t, m, keyEsc)

	if m.mode != modeList || m.confirmInput != "" || m.pending != nil {
		t.Errorf("esc should reset the state, got mode %v", m.mode)
	}
}

func TestRunningIgnoresKeys(t *testing.T) {
	m := press(t, withFirstSelected(t), keyR)
	m = press(t, m, keyY)

	for _, key := range []tea.KeyMsg{keyEsc, keyN, keyQ, keyEnter} {
		updated, cmd := m.Update(key)
		m = updated.(Model)
		if m.mode != modeRunning || cmd != nil {
			t.Fatalf("running mode should ignore keys, got mode %v", m.mode)
		}
	}
}

func TestActionResultIsStoredAndNextOneAwaited(t *testing.T) {
	m := press(t, withFirstSelected(t), keyR)
	m = press(t, m, keyY)

	msg := actionResultMsg{result: actions.Result{Repo: sampleRepos()[0]}}
	updated, cmd := m.Update(msg)
	m = updated.(Model)

	if len(m.results) != 1 {
		t.Errorf("expected 1 stored result, got %d", len(m.results))
	}
	if cmd == nil {
		t.Error("expected a command waiting for the next result")
	}
	if !strings.Contains(m.View(), "1/1 done") {
		t.Errorf("view should show progress, got:\n%s", m.View())
	}
}

func TestActionDoneShowsReportAndRefreshes(t *testing.T) {
	m := press(t, withFirstSelected(t), keyR)
	m = press(t, m, keyY)

	updated, cmd := m.Update(actionDoneMsg{})
	m = updated.(Model)

	if m.mode != modeReport {
		t.Errorf("expected report mode, got %v", m.mode)
	}
	if cmd == nil {
		t.Error("expected a command reloading the repos")
	}
}

func TestReportKeyGoesBackToList(t *testing.T) {
	m := press(t, withFirstSelected(t), keyR)
	m = press(t, m, keyY)
	updated, _ := m.Update(actionDoneMsg{})

	m = press(t, updated.(Model), keyEnter)

	if m.mode != modeList || m.pending != nil || len(m.results) != 0 {
		t.Errorf("report should reset the state, got mode %v", m.mode)
	}
}

func TestReloadPrunesSelection(t *testing.T) {
	m := press(t, loadedModel(), keyA)

	remaining := sampleRepos()[1:]
	updated, _ := m.Update(reposLoadedMsg{repos: remaining})
	m = updated.(Model)

	if m.selectedCount() != 2 || m.selected["dylan/alpha"] {
		t.Errorf("deleted repo should leave the selection, got %v", m.selected)
	}
}

func TestReportViewListsFailuresAndSkips(t *testing.T) {
	repos := sampleRepos()
	m := press(t, withFirstSelected(t), keyR)
	m.mode = modeReport
	m.results = []actions.Result{
		{Repo: repos[0]},
		{Repo: repos[1], Err: actions.ErrSkipped},
		{Repo: repos[2], Err: errors.New("boom")},
	}

	view := m.View()

	for _, want := range []string{"1 done, 1 skipped, 1 failed", "skipped dylan/beta", "failed  dylan/gamma: boom"} {
		if !strings.Contains(view, want) {
			t.Errorf("view should contain %q, got:\n%s", want, view)
		}
	}
}

func TestActionKeysAreTextInFilterMode(t *testing.T) {
	m := press(t, withFirstSelected(t), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = typeText(t, m, "rvd")

	if m.filter != "rvd" || m.mode != modeFilter {
		t.Errorf("expected filter %q in filter mode, got %q and %v", "rvd", m.filter, m.mode)
	}
}

func TestSelectedReposKeepsListOrder(t *testing.T) {
	m := press(t, loadedModel(), keyA)

	var names []string
	for _, repo := range m.selectedRepos() {
		names = append(names, repo.FullName)
	}

	if strings.Join(names, ",") != "dylan/alpha,dylan/beta,dylan/gamma" {
		t.Errorf("unexpected order: %v", names)
	}
}

var _ github.RepoService = fakeService{}
