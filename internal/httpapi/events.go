package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/verdict"
)

// errInvalidKind 表示谱系事件类型非法。
func errInvalidKind() error {
	return errors.Join(model.ErrInvalid, errors.New("kind must be split or merge"))
}

// verdictDecision 组装裁决请求。
func verdictDecision(id string, confirm bool, note string) verdict.Decision {
	return verdict.Decision{EventID: id, Confirm: confirm, Note: note}
}

// declareEventReq 是谱系事件申报请求体。
// 分裂：kind=split，parents=[1]，children=[2+]；合并：kind=merge，parents=[2+]，children=[1]。
type declareEventReq struct {
	Kind      string   `json:"kind"`
	ParentObs []string `json:"parent_obs_ids"`
	ChildObs  []string `json:"child_obs_ids"`
	Tolerance float64  `json:"tolerance"`
	Reason    string   `json:"reason"`
}

func (a *API) handleDeclareEvent(w http.ResponseWriter, r *http.Request) {
	var req declareEventReq
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if req.Tolerance <= 0 {
		req.Tolerance = 0.05
	}
	var ev *model.LineageEvent
	var err error
	switch req.Kind {
	case "split":
		parents, err := a.app.Obs.ListByIDs(r.Context(), req.ParentObs)
		if err != nil {
			writeErr(w, err)
			return
		}
		children, err := a.app.Obs.ListByIDs(r.Context(), req.ChildObs)
		if err != nil {
			writeErr(w, err)
			return
		}
		ev, err = a.app.Verdict.DeclareSplit(r.Context(), r.PathValue("id"), parents, children, req.Tolerance, req.Reason)
	case "merge":
		parents, err := a.app.Obs.ListByIDs(r.Context(), req.ParentObs)
		if err != nil {
			writeErr(w, err)
			return
		}
		children, err := a.app.Obs.ListByIDs(r.Context(), req.ChildObs)
		if err != nil {
			writeErr(w, err)
			return
		}
		ev, err = a.app.Verdict.DeclareMerge(r.Context(), r.PathValue("id"), parents, children, req.Tolerance, req.Reason)
	default:
		writeErr(w, errInvalidKind())
		return
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ev)
}

func (a *API) handleListEvents(w http.ResponseWriter, r *http.Request) {
	list, err := a.app.Events.ListByBatch(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	ev, err := a.app.Events.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	parts, err := a.app.Events.Participants(r.Context(), ev.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	checks, err := a.app.Events.ChecksByEvent(r.Context(), ev.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"event": ev, "participants": parts, "checks": checks})
}

func (a *API) handleCheckEvent(w http.ResponseWriter, r *http.Request) {
	res, err := a.app.Conservation.CheckEvent(r.Context(), r.PathValue("id"), 0.80)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) handleCheckAll(w http.ResponseWriter, r *http.Request) {
	res, err := a.app.Conservation.CheckAll(r.Context(), r.PathValue("id"), 0.80)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) handleListConservation(w http.ResponseWriter, r *http.Request) {
	list, err := a.app.Conservation.ListResults(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// confirmReq 是裁决请求体（可选备注）。
type confirmReq struct {
	Note string `json:"note"`
}

func (a *API) handleConfirmEvent(w http.ResponseWriter, r *http.Request) {
	var req confirmReq
	_ = decode(r, &req)
	ev, err := a.app.Verdict.Decide(r.Context(), verdictDecision(r.PathValue("id"), true, req.Note))
	if err != nil {
		writeErr(w, fmt.Errorf("confirm: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, ev)
}

func (a *API) handleRejectEvent(w http.ResponseWriter, r *http.Request) {
	var req confirmReq
	_ = decode(r, &req)
	ev, err := a.app.Verdict.Decide(r.Context(), verdictDecision(r.PathValue("id"), false, req.Note))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ev)
}
