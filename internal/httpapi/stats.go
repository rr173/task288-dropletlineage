package httpapi

import (
	"net/http"
)

func (a *API) handleBatchStats(w http.ResponseWriter, r *http.Request) {
	stats, err := a.app.BatchStats(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
