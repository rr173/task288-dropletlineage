package httpapi

import (
	"net/http"
)

// createBatchReq 是创建批次请求体。
type createBatchReq struct {
	Name          string  `json:"name"`
	ChamberWidth  float64 `json:"chamber_width"`
	ChamberHeight float64 `json:"chamber_height"`
}

func (a *API) handleCreateBatch(w http.ResponseWriter, r *http.Request) {
	var req createBatchReq
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	b, err := a.app.CreateBatch(r.Context(), req.Name, req.ChamberWidth, req.ChamberHeight)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (a *API) handleListBatches(w http.ResponseWriter, r *http.Request) {
	list, err := a.app.ListBatches(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleGetBatch(w http.ResponseWriter, r *http.Request) {
	b, err := a.app.GetBatch(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// transitionReq 是状态机推进请求体。
type transitionReq struct {
	To string `json:"to"`
}

func (a *API) handleTransitionBatch(w http.ResponseWriter, r *http.Request) {
	var req transitionReq
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	b, err := a.app.TransitionBatch(r.Context(), r.PathValue("id"), req.To)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}
