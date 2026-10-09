package actions

import (
	"context"

	"github.com/DYLANeay/ghtidy/internal/github"
)

type toggleVisibility struct {
	editor github.RepoEditor
}

// NewToggleVisibility builds the action that flips a repo between public and private
func NewToggleVisibility(editor github.RepoEditor) Action {
	return toggleVisibility{editor: editor}
}

func (toggleVisibility) Name() string { return "toggle visibility" }

func (toggleVisibility) Destructive() bool { return false }

func (t toggleVisibility) Apply(ctx context.Context, repo github.Repo) error {
	// archived repos are read only and internal ones cannot be switched by hand
	if repo.Archived || repo.Visibility == github.VisibilityInternal {
		return ErrSkipped
	}
	return t.editor.SetVisibility(ctx, repo.Owner, repo.Name, flipped(repo.Visibility))
}

// flipped returns the opposite of the given visibility
func flipped(visibility string) string {
	if visibility == github.VisibilityPublic {
		return github.VisibilityPrivate
	}
	return github.VisibilityPublic
}
