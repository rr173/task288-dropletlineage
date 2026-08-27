package service_test

import (
	"context"
	"fmt"
	"sync"
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

func TestConcurrentImportDistinctObsCount(t *testing.T) {
	app := newProbeApp(t)
	ctx := context.Background()
	batch, err := app.CreateBatch(ctx, "probe", 800, 600)
	if err != nil {
		t.Fatal(err)
	}
	importFrame(t, app, batch.ID, 1)
	const workers = 20
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 1; i <= workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("D%d", n)
			if _, _, err := app.Ingest.ImportObservation(ctx, batch.ID, 1, key, float64(100+n), 100, 10, 100, "GFP"); err != nil {
				errCh <- fmt.Errorf("%s: %w", key, err)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	got, err := app.GetBatch(ctx, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ObsCount != workers {
		t.Fatalf("obs_count=%d want=%d", got.ObsCount, workers)
	}
}
