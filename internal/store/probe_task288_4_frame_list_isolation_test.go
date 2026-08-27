package store_test

import (
	"context"
	"testing"

	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/store"
)

func TestFrameListByBatchIsolation(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	batches := store.NewBatchStore(db)
	frames := store.NewFrameStore(db)
	now := store.Now()
	b := model.NewBatch("b1", "probe", 800, 600, now)
	if err := batches.Create(ctx, b); err != nil {
		t.Fatal(err)
	}
	for seq := 1; seq <= 2; seq++ {
		f := model.NewFrame(store.NewID(), b.ID, seq, int64(seq)*100, "main", "", now)
		if err := frames.Create(ctx, nil, f); err != nil {
			t.Fatal(err)
		}
	}
	first, err := frames.ListByBatch(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 2 {
		t.Fatal("want 2 frames")
	}
	wantSeq := first[0].Seq
	second, err := frames.ListByBatch(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	second[0].Seq = 999
	if first[0].Seq != wantSeq {
		t.Fatalf("first list mutated: seq=%d want=%d", first[0].Seq, wantSeq)
	}
}
