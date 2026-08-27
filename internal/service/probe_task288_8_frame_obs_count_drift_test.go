package service_test

import (
	"context"
	"testing"

	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/service"
	"task288-dropletlineage/internal/store"
)

func newProbeApp(t *testing.T) *service.App {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return service.New(db)
}

func importFrame(t *testing.T, app *service.App, batchID string, seq int) {
	t.Helper()
	ctx := context.Background()
	if _, _, err := app.Ingest.ImportFrame(ctx, batchID, seq, int64(seq)*100, "main", ""); err != nil {
		t.Fatalf("import frame %d: %v", seq, err)
	}
}

func importObs(t *testing.T, app *service.App, batchID string, seq int, key string, x, y, r, intensity float64) *model.Observation {
	t.Helper()
	ctx := context.Background()
	obs, _, err := app.Ingest.ImportObservation(ctx, batchID, seq, key, x, y, r, intensity, "GFP")
	if err != nil {
		t.Fatalf("import obs %s@%d: %v", key, seq, err)
	}
	return obs
}

func TestImportObservationIncrementsFrameObsCount(t *testing.T) {
	app := newProbeApp(t)
	ctx := context.Background()
	batch, err := app.CreateBatch(ctx, "obs", 800, 600)
	if err != nil {
		t.Fatal(err)
	}
	importFrame(t, app, batch.ID, 1)
	if _, _, err := app.Ingest.ImportObservation(ctx, batch.ID, 1, "D1", 100, 100, 10, 100, "GFP"); err != nil {
		t.Fatal(err)
	}
	frame, err := app.Frames.GetBySeq(ctx, batch.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if frame.ObsCount != 1 {
		t.Fatalf("frame obs_count=%d want=1", frame.ObsCount)
	}
}
