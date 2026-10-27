package media

import (
	"context"
	"errors"
	"testing"

	"ecommerce.local/catalog-service/internal/domain"
)

type fakeSweepRepo struct {
	assets  []domain.MediaAsset
	deleted []string
}

func (f *fakeSweepRepo) SweepExpiredMedia(ctx context.Context, batchSize int, removeObject func(context.Context, string) error) (int, error) {
	removed := 0
	for i, a := range f.assets {
		if i >= batchSize {
			break
		}
		if err := removeObject(ctx, a.ObjectKey); err != nil {
			continue
		}
		f.deleted = append(f.deleted, a.ID)
		removed++
	}
	return removed, nil
}

type fakeObjectRemover struct {
	failKey string
	calls   []string
}

func (r *fakeObjectRemover) RemoveObject(_ context.Context, objectKey string) error {
	r.calls = append(r.calls, objectKey)
	if objectKey == r.failKey {
		return errors.New("s3 transient error")
	}
	return nil
}

func TestSweeperRunOnceDeletesExpiredAndSkipsFailedObjects(t *testing.T) {
	repo := &fakeSweepRepo{
		assets: []domain.MediaAsset{
			{ID: "asset-1", ObjectKey: "products/2026/10/ok.webp"},
			{ID: "asset-2", ObjectKey: "products/2026/10/fail.mp4"},
		},
	}
	remover := &fakeObjectRemover{failKey: "products/2026/10/fail.mp4"}

	sweeper := NewSweeper(repo, remover, 0, 50)
	deleted, err := sweeper.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != "asset-1" {
		t.Fatalf("repo.deleted = %v, want [asset-1]", repo.deleted)
	}
}
