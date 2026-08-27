package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task288-dropletlineage/internal/service"
	"task288-dropletlineage/internal/store"
)

func newProbeServer(t *testing.T) http.Handler {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/probe-http.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(service.New(db)).Handler()
}

func TestSequenceRegressionMapsUnprocessable(t *testing.T) {
	h := newProbeServer(t)
	body := strings.NewReader(`{"name":"probe","chamber_width":800,"chamber_height":600}`)
	req := httptest.NewRequest(http.MethodPost, "/api/batches", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create batch=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/batches/"+resp.Data.ID+"/frames", strings.NewReader(`{"seq":1,"t_ms":100,"channel":"main"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("frame1=%d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/batches/"+resp.Data.ID+"/frames", strings.NewReader(`{"seq":3,"t_ms":300,"channel":"main"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("frame3=%d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/batches/"+resp.Data.ID+"/frames", strings.NewReader(`{"seq":2,"t_ms":200,"channel":"main"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("sequence regression status=%d body=%s", rec.Code, rec.Body.String())
	}
}
