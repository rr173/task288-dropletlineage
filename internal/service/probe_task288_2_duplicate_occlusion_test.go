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

func TestRepeatedTrackingDoesNotDuplicateOcclusion(t *testing.T) {
	app := newProbeApp(t)
	ctx := context.Background()
	batch, err := app.CreateBatch(ctx, "occlusion", 800, 600)
	if err != nil {
		t.Fatal(err)
	}
	for seq := 1; seq <= 4; seq++ {
		if seq == 3 {
			continue
		}
		importFrame(t, app, batch.ID, seq)
	}
	importObs(t, app, batch.ID, 1, "D", 100, 100, 10, 100)
	importObs(t, app, batch.ID, 2, "D", 120, 100, 10, 100)
	importObs(t, app, batch.ID, 4, "D", 400, 300, 10, 100)
	if _, err := app.RunTracking(ctx, batch.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RunTracking(ctx, batch.ID); err != nil {
		t.Fatal(err)
	}
	events, err := app.Events.ListByBatch(ctx, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	open := 0
	for _, e := range events {
		if e.Kind == "occlusion" && !e.IsClosed() {
			open++
		}
	}
	if open != 1 {
		t.Fatalf("open occlusion events=%d want=1", open)
	}
}
