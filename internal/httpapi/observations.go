package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"task288-dropletlineage/internal/model"
)

// errInvalidStatus 表示观测状态取值非法。
func errInvalidStatus() error {
	return errors.Join(model.ErrInvalid, errors.New("status must be tracked, occluded or excluded"))
}

// importObsReq 是液滴观测导入请求体。
type importObsReq struct {
	FrameSeq   int     `json:"frame_seq"`
	DropletKey string  `json:"droplet_key"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Radius     float64 `json:"radius"`
	Intensity  float64 `json:"intensity"`
	MarkerID   string  `json:"marker_id"`
}

func (a *API) handleImportObservation(w http.ResponseWriter, r *http.Request) {
	seq, err := strconv.Atoi(r.PathValue("seq"))
	if err != nil {
		writeErr(w, err)
		return
	}
	var req importObsReq
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	o, created, err := a.app.Ingest.ImportObservation(r.Context(), r.PathValue("id"), seq,
		req.DropletKey, req.X, req.Y, req.Radius, req.Intensity, req.MarkerID)
	if err != nil {
		writeErr(w, err)
		return
	}
	code := http.StatusCreated
	if !created {
		code = http.StatusOK
	}
	writeJSON(w, code, o)
}

func (a *API) handleListObservations(w http.ResponseWriter, r *http.Request) {
	list, err := a.app.Obs.ListByBatch(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleGetObservation(w http.ResponseWriter, r *http.Request) {
	o, err := a.app.Obs.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// setStatusReq 是观测状态变更请求体（tracked/occluded/excluded）。
type setStatusReq struct {
	Status string `json:"status"`
}

func (a *API) handleSetObservationStatus(w http.ResponseWriter, r *http.Request) {
	var req setStatusReq
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if req.Status != "tracked" && req.Status != "occluded" && req.Status != "excluded" {
		writeErr(w, errInvalidStatus())
		return
	}
	if err := a.app.Obs.UpdateStatus(r.Context(), r.PathValue("id"), req.Status); err != nil {
		writeErr(w, err)
		return
	}
	o, err := a.app.Obs.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}
