package ingest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/store"
)

// newTestEnv 打开临时数据库并构造导入服务。
func newTestEnv(t *testing.T) (*Service, *store.DB, *store.BatchStore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ingest.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close(); _ = os.Remove(path) })
	batches := store.NewBatchStore(db)
	frames := store.NewFrameStore(db)
	obs := store.NewObservationStore(db)
	return NewService(db, batches, frames, obs), db, batches, path
}

func TestImportFrameSequenceRegression(t *testing.T) {
	svc, db, batches, _ := newTestEnv(t)
	ctx := context.Background()
	b := model.NewBatch(store.NewID(), "b", 1000, 500, store.Now())
	if err := batches.Create(ctx, b); err != nil {
		t.Fatalf("create batch: %v", err)
	}
	// 导入 seq=1, 3（跳号）
	for _, seq := range []int{1, 3} {
		if _, created, err := svc.ImportFrame(ctx, b.ID, seq, int64(seq*10), "ch1", ""); err != nil || !created {
			t.Fatalf("import frame %d: created=%v err=%v", seq, created, err)
		}
	}
	// 补插 seq=2（<=maxSeq=3 且不存在）→ 帧序倒退被拒绝
	if _, _, err := svc.ImportFrame(ctx, b.ID, 2, 20, "ch1", ""); !errors.Is(err, model.ErrSequenceRegression) {
		t.Fatalf("import frame 2: err=%v, want ErrSequenceRegression", err)
	}
	// 追加 seq=4 正常
	if _, created, err := svc.ImportFrame(ctx, b.ID, 4, 40, "ch1", ""); err != nil || !created {
		t.Fatalf("import frame 4: created=%v err=%v", created, err)
	}
	// 重复导入 seq=4 → 幂等 created=false
	if _, created, err := svc.ImportFrame(ctx, b.ID, 4, 40, "ch1", "other-ref"); err != nil || created {
		t.Fatalf("import frame 4 again: created=%v err=%v", created, err)
	}
	// 未知批次
	if _, _, err := svc.ImportFrame(ctx, "nope", 1, 0, "ch1", ""); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown batch: err=%v, want ErrNotFound", err)
	}
	_ = db
}

func TestImportObservationValidation(t *testing.T) {
	svc, _, batches, _ := newTestEnv(t)
	ctx := context.Background()
	b := model.NewBatch(store.NewID(), "b", 100, 100, store.Now())
	if err := batches.Create(ctx, b); err != nil {
		t.Fatalf("create batch: %v", err)
	}
	if _, _, err := svc.ImportFrame(ctx, b.ID, 1, 0, "ch1", ""); err != nil {
		t.Fatalf("import frame: %v", err)
	}
	cases := []struct {
		name    string
		x, y    float64
		r, I    float64
		marker  string
		wantErr error
	}{
		{"coord out of range x", 150, 50, 3, 10, "GFP", model.ErrCoordOutOfRange},
		{"coord out of range y", 50, 120, 3, 10, "GFP", model.ErrCoordOutOfRange},
		{"negative radius", 50, 50, -1, 10, "GFP", model.ErrInvalid},
		{"negative intensity", 50, 50, 3, -5, "GFP", model.ErrInvalid},
		{"missing marker", 50, 50, 3, 10, "", model.ErrInvalid},
		{"ok", 50, 50, 3, 10, "GFP", nil},
	}
	for _, c := range cases {
		_, created, err := svc.ImportObservation(ctx, b.ID, 1, "D"+c.name, c.x, c.y, c.r, c.I, c.marker)
		if !errors.Is(err, c.wantErr) {
			t.Errorf("%s: err=%v, want %v", c.name, err, c.wantErr)
			continue
		}
		if c.wantErr == nil && !created {
			t.Errorf("%s: not created", c.name)
		}
	}
	// 同帧同液滴幂等
	if _, created, err := svc.ImportObservation(ctx, b.ID, 1, "Dok", 50, 50, 3, 10, "GFP"); err != nil || created {
		t.Errorf("idempotent: created=%v err=%v", created, err)
	}
}
