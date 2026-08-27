package httpapi

import (
	"errors"
	"net/http"

	"task288-dropletlineage/internal/model"
)

// errInvalidTarget 表示版本发布目标非法。
func errInvalidTarget() error {
	return errors.Join(model.ErrInvalid, errors.New("target must be shared or frozen"))
}

// createVersionReq 是版本创建请求体。
type createVersionReq struct {
	Note string `json:"note"`
}

func (a *API) handleCreateVersion(w http.ResponseWriter, r *http.Request) {
	var req createVersionReq
	_ = decode(r, &req)
	v, err := a.app.Version.CreateDraft(r.Context(), r.PathValue("id"), req.Note)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (a *API) handleListVersions(w http.ResponseWriter, r *http.Request) {
	list, err := a.app.Version.List(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	detail, err := a.app.Version.DetailOf(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// publishVersionReq 是版本发布请求体：target ∈ shared | frozen。
type publishVersionReq struct {
	Target string `json:"target"`
}

func (a *API) handlePublishVersion(w http.ResponseWriter, r *http.Request) {
	var req publishVersionReq
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if req.Target != "shared" && req.Target != "frozen" {
		writeErr(w, errInvalidTarget())
		return
	}
	v, err := a.app.Version.Publish(r.Context(), r.PathValue("id"), req.Target)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) handleDeriveVersion(w http.ResponseWriter, r *http.Request) {
	var req createVersionReq
	_ = decode(r, &req)
	v, err := a.app.Version.Derive(r.Context(), r.PathValue("id"), req.Note)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}
