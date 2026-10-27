package media

import (
	"context"
	"log/slog"
	"time"
)

const defaultSweepBatchSize = 50

// ExpiredMediaRepository claims and deletes expired PENDING/ORPHANED media rows.
type ExpiredMediaRepository interface {
	SweepExpiredMedia(ctx context.Context, batchSize int, removeObject func(context.Context, string) error) (int, error)
}

// ObjectRemover deletes an object from object storage by key.
type ObjectRemover interface {
	RemoveObject(ctx context.Context, objectKey string) error
}

// Sweeper periodically cleans up expired PENDING and ORPHANED media assets.
type Sweeper struct {
	repo      ExpiredMediaRepository
	remover   ObjectRemover
	interval  time.Duration
	batchSize int
}

// NewSweeper constructs a background orphan media sweeper.
func NewSweeper(repo ExpiredMediaRepository, remover ObjectRemover, interval time.Duration, batchSize int) *Sweeper {
	if batchSize <= 0 {
		batchSize = defaultSweepBatchSize
	}
	return &Sweeper{
		repo:      repo,
		remover:   remover,
		interval:  interval,
		batchSize: batchSize,
	}
}

// RunOnce executes a single sweep pass.
func (s *Sweeper) RunOnce(ctx context.Context) (int, error) {
	if s.repo == nil || s.remover == nil {
		return 0, nil
	}
	if bootstrapper, ok := s.remover.(interface{ EnsureBucket(context.Context) error }); ok {
		_ = bootstrapper.EnsureBucket(ctx)
	}
	return s.repo.SweepExpiredMedia(ctx, s.batchSize, s.remover.RemoveObject)
}

// Start runs periodic sweeps until ctx is cancelled.
func (s *Sweeper) Start(ctx context.Context) {
	if s.interval <= 0 || s.repo == nil || s.remover == nil {
		return
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleted, err := s.RunOnce(ctx)
			if err != nil && ctx.Err() == nil {
				slog.WarnContext(ctx, "media orphan sweep failed", "error", err)
			} else if deleted > 0 {
				slog.InfoContext(ctx, "media orphan sweep completed", "deleted_count", deleted)
			}
		}
	}
}
