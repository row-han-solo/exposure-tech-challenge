package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ctrl-hub/challenge/db"
	"github.com/ctrl-hub/challenge/exposure"
)

func (h *Handler) GetExposure(w http.ResponseWriter, r *http.Request) {
	exposureID := chi.URLParam(r, "exposureId")
	if _, err := uuid.Parse(exposureID); err != nil {
		writeError(w, http.StatusBadRequest, "exposureId must be a valid UUID")
		return
	}

	result, err := exposure.Get(r.Context(), h.db, exposureID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "exposure not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
