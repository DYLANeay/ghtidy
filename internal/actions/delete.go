package actions

import (
	"context"

	"github.com/DYLANeay/ghtidy/internal/github"
)

type deleteRepo struct {
	editor github.RepoEditor
}

// NewDelete builds the action that permanently deletes repositories
func NewDelete(editor github.RepoEditor) Action {
	return deleteRepo{editor: editor}
}

func (deleteRepo) Name() string { return "delete" }

func (deleteRepo) Destructive() bool { return true }

func (d deleteRepo) Apply(ctx context.Context, repo github.Repo) error {
	return d.editor.Delete(ctx, repo.Owner, repo.Name)
}
