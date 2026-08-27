package httpapi

import (
	"net/http"
)

func (a *API) handleRunTracking(w http.ResponseWriter, r *http.Request) {
	res, err := a.app.RunTracking(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) handleListTracks(w http.ResponseWriter, r *http.Request) {
	list, err := a.app.Tracks.ListByBatch(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleGetTrack(w http.ResponseWriter, r *http.Request) {
	t, err := a.app.Tracks.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	links, err := a.app.Tracks.ListLinksByTrack(r.Context(), t.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"track": t, "links": links})
}
