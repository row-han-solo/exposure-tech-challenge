package handler

import (
	"net/http"

	"github.com/ctrl-hub/challenge/data"
	"github.com/ctrl-hub/challenge/exposure"
)

func (h *Handler) GetExposures(w http.ResponseWriter, r *http.Request) {
	results, err := exposure.List(r.Context(), h.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Return an empty array rather than null when there are no exposures.
	if results == nil {
		results = []*data.Exposure{}
	}

	writeJSON(w, http.StatusOK, results)
}
