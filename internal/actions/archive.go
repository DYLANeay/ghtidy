package actions

import (
	"context"

	"github.com/DYLANeay/ghtidy/internal/github"
)

type archive struct {
	editor github.RepoEditor
}

// NewArchive builds the action that archives repositories
func NewArchive(editor github.RepoEditor) Action {
	return archive{editor: editor}
}

func (archive) Name() string { return "archive" }

func (archive) Destructive() bool { return false }

func (a archive) Apply(ctx context.Context, repo github.Repo) error {
	if repo.Archived {
		return ErrSkipped
	}
	return a.editor.Archive(ctx, repo.Owner, repo.Name)
}
