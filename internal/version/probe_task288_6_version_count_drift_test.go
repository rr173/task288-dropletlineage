package version_test

import (
	"context"
	"testing"

	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/store"
	"task288-dropletlineage/internal/version"
)

func TestCreateDraftIncrementsBatchVersionCount(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	batches := store.NewBatchStore(db)
	events := store.NewEventStore(db)
	versions := store.NewVersionStore(db)
	svc := version.NewService(db, versions, events, batches)
	now := store.Now()
	b := model.NewBatch("b1", "probe", 800, 600, now)
	if err := batches.Create(ctx, b); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateDraft(ctx, b.ID, "draft"); err != nil {
		t.Fatal(err)
	}
	got, err := batches.Get(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.VersionCount != 1 {
		t.Fatalf("version_count=%d want=1", got.VersionCount)
	}
}
