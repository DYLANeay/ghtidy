package actions

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DYLANeay/ghtidy/internal/github"
)

// slowAction tracks how many calls run at the same time
type slowAction struct {
	running atomic.Int32
	peak    atomic.Int32
	failOn  string
}

func (s *slowAction) Name() string      { return "slow" }
func (s *slowAction) Destructive() bool { return false }

func (s *slowAction) Apply(ctx context.Context, repo github.Repo) error {
	now := s.running.Add(1)
	defer s.running.Add(-1)
	for {
		peak := s.peak.Load()
		if now <= peak || s.peak.CompareAndSwap(peak, now) {
			break
		}
	}
	time.Sleep(10 * time.Millisecond)
	if repo.Name == s.failOn {
		return errors.New("boom")
	}
	return nil
}

func reposNamed(names ...string) []github.Repo {
	repos := make([]github.Repo, 0, len(names))
	for _, name := range names {
		repos = append(repos, repoNamed(name))
	}
	return repos
}

// collect drains the channel, it only returns once the channel is closed
func collect(results <-chan Result) []Result {
	var all []Result
	for r := range results {
		all = append(all, r)
	}
	return all
}

func TestRunYieldsOneResultPerRepo(t *testing.T) {
	editor := &fakeEditor{}
	repos := reposNamed("a", "b", "c", "d", "e")

	results := collect(Run(context.Background(), NewArchive(editor), repos, 2))

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	seen := make(map[string]bool)
	for _, r := range results {
		seen[r.Repo.Name] = true
		if !r.Succeeded() {
			t.Errorf("%s: unexpected error %v", r.Repo.Name, r.Err)
		}
	}
	if len(seen) != 5 {
		t.Errorf("every repo should appear once, got %v", seen)
	}
}

func TestRunReportsFailuresPerRepo(t *testing.T) {
	action := &slowAction{failOn: "b"}

	results := collect(Run(context.Background(), action, reposNamed("a", "b", "c"), 3))

	failed := 0
	for _, r := range results {
		if r.Err != nil {
			failed++
			if r.Repo.Name != "b" {
				t.Errorf("wrong repo failed: %s", r.Repo.Name)
			}
		}
	}
	if len(results) != 3 || failed != 1 {
		t.Errorf("expected 3 results with 1 failure, got %d and %d", len(results), failed)
	}
}

func TestRunRespectsConcurrencyLimit(t *testing.T) {
	action := &slowAction{}
	repos := reposNamed("a", "b", "c", "d", "e", "f", "g", "h")

	collect(Run(context.Background(), action, repos, 2))

	if peak := action.peak.Load(); peak > 2 {
		t.Errorf("at most 2 calls at once, saw %d", peak)
	}
}

func TestRunWithNoReposClosesChannel(t *testing.T) {
	results := collect(Run(context.Background(), NewArchive(&fakeEditor{}), nil, 4))

	if len(results) != 0 {
		t.Errorf("expected no result, got %d", len(results))
	}
}

func TestRunTreatsZeroLimitAsOne(t *testing.T) {
	action := &slowAction{}

	results := collect(Run(context.Background(), action, reposNamed("a", "b", "c"), 0))

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	if peak := action.peak.Load(); peak != 1 {
		t.Errorf("expected one call at a time, saw %d", peak)
	}
}
