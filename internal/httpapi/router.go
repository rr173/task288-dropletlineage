package httpapi

import (
	"net/http"

	"task288-dropletlineage/internal/service"
)

// API 持有应用聚合与全部处理器。
type API struct {
	app *service.App
}

// New 构造 HTTP 层。
func New(app *service.App) *API { return &API{app: app} }

// Handler 返回根路由（含 /api 前缀与健康检查）。
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", a.handleHealth)
	mux.HandleFunc("POST /api/batches", a.handleCreateBatch)
	mux.HandleFunc("GET /api/batches", a.handleListBatches)
	mux.HandleFunc("GET /api/batches/{id}", a.handleGetBatch)
	mux.HandleFunc("POST /api/batches/{id}/transition", a.handleTransitionBatch)
	mux.HandleFunc("POST /api/batches/{id}/frames", a.handleImportFrame)
	mux.HandleFunc("GET /api/batches/{id}/frames", a.handleListFrames)
	mux.HandleFunc("GET /api/batches/{id}/frames/{seq}", a.handleGetFrame)
	mux.HandleFunc("POST /api/batches/{id}/frames/{seq}/observations", a.handleImportObservation)
	mux.HandleFunc("GET /api/batches/{id}/observations", a.handleListObservations)
	mux.HandleFunc("GET /api/observations/{id}", a.handleGetObservation)
	mux.HandleFunc("PUT /api/observations/{id}/status", a.handleSetObservationStatus)
	mux.HandleFunc("POST /api/batches/{id}/track", a.handleRunTracking)
	mux.HandleFunc("GET /api/batches/{id}/tracks", a.handleListTracks)
	mux.HandleFunc("GET /api/tracks/{id}", a.handleGetTrack)
	mux.HandleFunc("POST /api/batches/{id}/events", a.handleDeclareEvent)
	mux.HandleFunc("GET /api/batches/{id}/events", a.handleListEvents)
	mux.HandleFunc("GET /api/events/{id}", a.handleGetEvent)
	mux.HandleFunc("POST /api/events/{id}/check", a.handleCheckEvent)
	mux.HandleFunc("POST /api/batches/{id}/conservation-check", a.handleCheckAll)
	mux.HandleFunc("GET /api/batches/{id}/conservation-results", a.handleListConservation)
	mux.HandleFunc("PUT /api/events/{id}/confirm", a.handleConfirmEvent)
	mux.HandleFunc("PUT /api/events/{id}/reject", a.handleRejectEvent)
	mux.HandleFunc("POST /api/batches/{id}/versions", a.handleCreateVersion)
	mux.HandleFunc("GET /api/batches/{id}/versions", a.handleListVersions)
	mux.HandleFunc("GET /api/versions/{id}", a.handleGetVersion)
	mux.HandleFunc("POST /api/versions/{id}/publish", a.handlePublishVersion)
	mux.HandleFunc("POST /api/versions/{id}/derive", a.handleDeriveVersion)
	mux.HandleFunc("GET /api/batches/{id}/stats", a.handleBatchStats)

	return mux
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
