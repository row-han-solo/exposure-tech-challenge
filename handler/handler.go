package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ctrl-hub/challenge/db"
	"github.com/ctrl-hub/challenge/exposure"
)

// Handler holds the shared dependencies for all HTTP handlers.
type Handler struct {
	db  db.Client
	pub exposure.Publisher
}

// New constructs a Handler with the given database client and event publisher.
func New(db db.Client, pub exposure.Publisher) *Handler {
	return &Handler{db: db, pub: pub}
}

// Routes builds and returns the application router.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/exposure", h.GetExposures)
	r.Post("/exposure", h.RecordExposure)
	r.Get("/exposure/{exposureId}", h.GetExposure)
	r.Get("/users/{userId}/exposure-summary", h.GetUserExposureSummary)

	return r
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
