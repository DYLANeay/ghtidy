package actions

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/DYLANeay/ghtidy/internal/github"
)

// fakeEditor records every call instead of hitting the api
type fakeEditor struct {
	mu    sync.Mutex
	calls []string
	errs  map[string]error
}

func (f *fakeEditor) record(call, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call+" "+name)
	return f.errs[name]
}

func (f *fakeEditor) Archive(ctx context.Context, owner, name string) error {
	return f.record("archive", owner+"/"+name)
}

func (f *fakeEditor) SetVisibility(ctx context.Context, owner, name, visibility string) error {
	return f.record("visibility:"+visibility, owner+"/"+name)
}

func (f *fakeEditor) Delete(ctx context.Context, owner, name string) error {
	return f.record("delete", owner+"/"+name)
}

func repoNamed(name string) github.Repo {
	return github.Repo{
		Name:       name,
		Owner:      "dylan",
		FullName:   "dylan/" + name,
		Visibility: github.VisibilityPublic,
	}
}

func TestArchiveCallsEditor(t *testing.T) {
	editor := &fakeEditor{}

	err := NewArchive(editor).Apply(context.Background(), repoNamed("alpha"))

	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(editor.calls) != 1 || editor.calls[0] != "archive dylan/alpha" {
		t.Errorf("unexpected calls: %v", editor.calls)
	}
}

func TestArchiveSkipsArchivedRepo(t *testing.T) {
	editor := &fakeEditor{}
	repo := repoNamed("alpha")
	repo.Archived = true

	err := NewArchive(editor).Apply(context.Background(), repo)

	if !errors.Is(err, ErrSkipped) {
		t.Errorf("expected ErrSkipped, got %v", err)
	}
	if len(editor.calls) != 0 {
		t.Errorf("editor should not be called, got %v", editor.calls)
	}
}

func TestToggleVisibilityFlipsPublicToPrivate(t *testing.T) {
	editor := &fakeEditor{}

	err := NewToggleVisibility(editor).Apply(context.Background(), repoNamed("alpha"))

	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if editor.calls[0] != "visibility:private dylan/alpha" {
		t.Errorf("unexpected calls: %v", editor.calls)
	}
}

func TestToggleVisibilityFlipsPrivateToPublic(t *testing.T) {
	editor := &fakeEditor{}
	repo := repoNamed("alpha")
	repo.Visibility = github.VisibilityPrivate

	err := NewToggleVisibility(editor).Apply(context.Background(), repo)

	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if editor.calls[0] != "visibility:public dylan/alpha" {
		t.Errorf("unexpected calls: %v", editor.calls)
	}
}

func TestToggleVisibilitySkipsArchivedAndInternal(t *testing.T) {
	archived := repoNamed("old")
	archived.Archived = true
	internal := repoNamed("corp")
	internal.Visibility = github.VisibilityInternal

	for _, repo := range []github.Repo{archived, internal} {
		editor := &fakeEditor{}

		err := NewToggleVisibility(editor).Apply(context.Background(), repo)

		if !errors.Is(err, ErrSkipped) {
			t.Errorf("%s: expected ErrSkipped, got %v", repo.Name, err)
		}
		if len(editor.calls) != 0 {
			t.Errorf("%s: editor should not be called, got %v", repo.Name, editor.calls)
		}
	}
}

func TestDeleteCallsEditor(t *testing.T) {
	editor := &fakeEditor{}

	err := NewDelete(editor).Apply(context.Background(), repoNamed("alpha"))

	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if editor.calls[0] != "delete dylan/alpha" {
		t.Errorf("unexpected calls: %v", editor.calls)
	}
}

func TestEditorErrorIsReturned(t *testing.T) {
	boom := errors.New("boom")
	editor := &fakeEditor{errs: map[string]error{"dylan/alpha": boom}}

	err := NewDelete(editor).Apply(context.Background(), repoNamed("alpha"))

	if !errors.Is(err, boom) {
		t.Errorf("expected boom, got %v", err)
	}
}

func TestActionNamesAndDestructive(t *testing.T) {
	editor := &fakeEditor{}
	tests := []struct {
		action      Action
		name        string
		destructive bool
	}{
		{NewArchive(editor), "archive", false},
		{NewToggleVisibility(editor), "toggle visibility", false},
		{NewDelete(editor), "delete", true},
	}

	for _, tt := range tests {
		if tt.action.Name() != tt.name {
			t.Errorf("name: got %q, want %q", tt.action.Name(), tt.name)
		}
		if tt.action.Destructive() != tt.destructive {
			t.Errorf("%s destructive: got %v", tt.name, tt.action.Destructive())
		}
	}
}

func TestResultStates(t *testing.T) {
	ok := Result{}
	skipped := Result{Err: ErrSkipped}
	failed := Result{Err: errors.New("boom")}

	if !ok.Succeeded() || ok.Skipped() {
		t.Error("nil error should succeed and not be skipped")
	}
	if skipped.Succeeded() || !skipped.Skipped() {
		t.Error("ErrSkipped should be skipped and not succeeded")
	}
	if failed.Succeeded() || failed.Skipped() {
		t.Error("other errors should be neither succeeded nor skipped")
	}
}
