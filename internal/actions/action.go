// Package actions holds the bulk operations ghtidy can run on repositories
package actions

import (
	"context"
	"errors"

	"github.com/DYLANeay/ghtidy/internal/github"
)

// ErrSkipped means the action had nothing to do for this repo
var ErrSkipped = errors.New("skipped")

// Action is one operation applied to a single repository
type Action interface {
	Name() string
	// Destructive actions cannot be undone and need a stronger confirmation
	Destructive() bool
	Apply(ctx context.Context, repo github.Repo) error
}

// Result is the outcome of one action on one repository
type Result struct {
	Repo github.Repo
	Err  error
}

// Succeeded tells whether the action went through
func (r Result) Succeeded() bool {
	return r.Err == nil
}

// Skipped tells whether the repo was left untouched on purpose
func (r Result) Skipped() bool {
	return errors.Is(r.Err, ErrSkipped)
}
