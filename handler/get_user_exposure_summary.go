package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ctrl-hub/challenge/db"
	"github.com/ctrl-hub/challenge/exposure"
)

func (h *Handler) GetUserExposureSummary(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if _, err := uuid.Parse(userID); err != nil {
		writeError(w, http.StatusBadRequest, "userId must be a valid UUID")
		return
	}

	start, err := parseOptionalTime(r, "starting_at")
	if err != nil {
		writeError(w, http.StatusBadRequest, "starting_at must be a valid RFC3339 timestamp")
		return
	}

	end, err := parseOptionalTime(r, "ending_at")
	if err != nil {
		writeError(w, http.StatusBadRequest, "ending_at must be a valid RFC3339 timestamp")
		return
	}

	summary, err := exposure.GetSummary(r.Context(), h.db, userID, start, end)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func parseOptionalTime(r *http.Request, param string) (*time.Time, error) {
	raw := r.URL.Query().Get(param)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
