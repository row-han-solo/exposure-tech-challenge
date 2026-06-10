package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/ctrl-hub/challenge/data"
	"github.com/ctrl-hub/challenge/db"
	"github.com/ctrl-hub/challenge/exposure"
)

func (h *Handler) RecordExposure(w http.ResponseWriter, r *http.Request) {
	var req data.RecordExposureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateRecordExposureRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := exposure.Record(r.Context(), h.db, h.pub, req)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user or equipment not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func validateRecordExposureRequest(req data.RecordExposureRequest) error {
	if _, err := uuid.Parse(req.EquipmentID); err != nil {
		return errors.New("equipment_id must be a valid UUID")
	}
	if _, err := uuid.Parse(req.UserID); err != nil {
		return errors.New("user_id must be a valid UUID")
	}
	if req.Duration <= 0 {
		return errors.New("duration must be greater than 0")
	}
	return nil
}
