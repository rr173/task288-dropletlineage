package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
)

// importFrameReq 是帧导入请求体。
type importFrameReq struct {
	Seq      int    `json:"seq"`
	TMs      int64  `json:"t_ms"`
	Channel  string `json:"channel"`
	ImageRef string `json:"image_ref"`
}

func (a *API) handleImportFrame(w http.ResponseWriter, r *http.Request) {
	var req importFrameReq
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	f, created, err := a.app.Ingest.ImportFrame(r.Context(), r.PathValue("id"), req.Seq, req.TMs, req.Channel, req.ImageRef)
	if err != nil {
		writeErr(w, fmt.Errorf("import frame: %v", err))
		return
	}
	code := http.StatusCreated
	if !created {
		code = http.StatusOK
	}
	writeJSON(w, code, f)
}

func (a *API) handleListFrames(w http.ResponseWriter, r *http.Request) {
	frames, err := a.app.Frames.ListByBatch(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, frames)
}

func (a *API) handleGetFrame(w http.ResponseWriter, r *http.Request) {
	seq, err := strconv.Atoi(r.PathValue("seq"))
	if err != nil {
		writeErr(w, err)
		return
	}
	f, err := a.app.Frames.GetBySeq(r.Context(), r.PathValue("id"), seq)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}
