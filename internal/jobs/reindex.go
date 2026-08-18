// Package jobs holds the background jobs of the API worker.
package jobs

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Searcher runs a query against the search index.
type Searcher interface {
	Query(ctx context.Context, index string) ([]byte, error)
}

// Sink receives the rebuilt documents.
type Sink interface {
	Store(ctx context.Context, documents []byte) error
}

// ReindexJob rebuilds the orders index from the live one.
type ReindexJob struct {
	Search   Searcher
	Sink     Sink
	Index    string
	Interval time.Duration
}

// Validate reports configuration mistakes before the job is scheduled.
func (j *ReindexJob) Validate() error {
	switch {
	case j.Search == nil:
		return fmt.Errorf("jobs: reindex needs a searcher")
	case j.Sink == nil:
		return fmt.Errorf("jobs: reindex needs a sink")
	case j.Index == "":
		return fmt.Errorf("jobs: reindex needs an index name")
	case j.Interval <= 0:
		return fmt.Errorf("jobs: reindex interval must be positive")
	}
	return nil
}

// Next returns when the job should run after last.
func (j *ReindexJob) Next(last time.Time) time.Time {
	return last.Add(j.Interval)
}

// Run reindexes once. The worker calls it on every tick of Interval.
func (j *ReindexJob) Run(ctx context.Context) error {
	started := time.Now()
	documents, err := j.Search.Query(ctx, j.Index)
	if err != nil {
		return fmt.Errorf("search query failed: %w", err)
	}
	if err := j.Sink.Store(ctx, documents); err != nil {
		return fmt.Errorf("store failed: %w", err)
	}
	log.Printf("reindex: %s done in %s", j.Index, time.Since(started))
	return nil
}
