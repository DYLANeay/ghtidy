package actions

import (
	"context"
	"sync"
	"time"

	"github.com/DYLANeay/ghtidy/internal/github"
)

const (
	// DefaultLimit is how many repos are processed at the same time
	DefaultLimit = 4
	// perRepoTimeout keeps one stuck request from blocking the whole run
	perRepoTimeout = 30 * time.Second
)

// Run applies the action to every repo with at most limit calls in flight,
// the channel yields one Result per repo and is closed once all are done
func Run(ctx context.Context, action Action, repos []github.Repo, limit int) <-chan Result {
	// buffered so workers never block even if nobody reads yet
	results := make(chan Result, len(repos))

	jobs := make(chan github.Repo, len(repos))
	for _, repo := range repos {
		jobs <- repo
	}
	close(jobs)

	var wg sync.WaitGroup
	for i := 0; i < workerCount(limit, len(repos)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repo := range jobs {
				results <- applyOne(ctx, action, repo)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()
	return results
}

// workerCount picks how many workers to start, never more than there is work
func workerCount(limit, jobs int) int {
	if limit < 1 {
		limit = 1
	}
	return min(limit, jobs)
}

// applyOne runs the action on one repo under its own timeout
func applyOne(ctx context.Context, action Action, repo github.Repo) Result {
	ctx, cancel := context.WithTimeout(ctx, perRepoTimeout)
	defer cancel()
	return Result{Repo: repo, Err: action.Apply(ctx, repo)}
}
