package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/service"
	"task288-dropletlineage/internal/store"
)

func TestDoubleConfirmMapsConflict(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/probe-event-closed.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := service.New(db)
	h := New(app).Handler()
	ctx := context.Background()
	batch, err := app.CreateBatch(ctx, "probe", 800, 600)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.Ingest.ImportFrame(ctx, batch.ID, 1, 100, "main", ""); err != nil {
		t.Fatal(err)
	}
	parent, _, err := app.Ingest.ImportObservation(ctx, batch.ID, 1, "P", 100, 100, 10, 1000, "GFP")
	if err != nil {
		t.Fatal(err)
	}
	c1, _, err := app.Ingest.ImportObservation(ctx, batch.ID, 1, "C1", 120, 100, 7, 400, "GFP")
	if err != nil {
		t.Fatal(err)
	}
	c2, _, err := app.Ingest.ImportObservation(ctx, batch.ID, 1, "C2", 140, 100, 7, 400, "GFP")
	if err != nil {
		t.Fatal(err)
	}
	ev, err := app.Verdict.DeclareSplit(ctx, batch.ID, []*model.Observation{parent}, []*model.Observation{c1, c2}, 0.05, "probe")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/events/"+ev.ID+"/confirm", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first confirm=%d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPut, "/api/events/"+ev.ID+"/confirm", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("double confirm status=%d body=%s", rec.Code, rec.Body.String())
	}
}
