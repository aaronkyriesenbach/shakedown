package songs

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"shakedown/internal/auth"
)

// Handler handles HTTP requests for song markers.
type Handler struct {
	repo   *Repository
	logger *zap.Logger
}

func NewHandler(repo *Repository, logger *zap.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}

// Routes registers song routes under /api/recordings/{recordingID}/songs
func (h *Handler) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Patch("/{songID}", h.update)
	r.Delete("/{songID}", h.delete)
}

type createSongRequest struct {
	Title        string  `json:"title"`
	StartSeconds int     `json:"start_seconds"`
	EndSeconds   *int    `json:"end_seconds"`
	Notes        *string `json:"notes"`
}

type updateSongRequest struct {
	Title        string  `json:"title"`
	StartSeconds int     `json:"start_seconds"`
	EndSeconds   *int    `json:"end_seconds"`
	Notes        *string `json:"notes"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	recordingID := chi.URLParam(r, "recordingID")
	songs, err := h.repo.List(r.Context(), recordingID)
	if err != nil {
		h.logger.Error("failed to list songs", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if songs == nil {
		songs = []*Song{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(songs)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	recordingID := chi.URLParam(r, "recordingID")

	var req createSongRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, `{"error":"title is required"}`, http.StatusBadRequest)
		return
	}
	if req.EndSeconds != nil && *req.EndSeconds <= req.StartSeconds {
		http.Error(w, `{"error":"end_seconds must be greater than start_seconds"}`, http.StatusBadRequest)
		return
	}

	song, err := h.repo.Create(r.Context(), recordingID, user.ID, req.Title, req.StartSeconds, req.EndSeconds, req.Notes)
	if err != nil {
		h.logger.Error("failed to create song", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(song)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	songID := chi.URLParam(r, "songID")

	var req updateSongRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.EndSeconds != nil && *req.EndSeconds <= req.StartSeconds {
		http.Error(w, `{"error":"end_seconds must be greater than start_seconds"}`, http.StatusBadRequest)
		return
	}

	song, err := h.repo.Update(r.Context(), songID, req.Title, req.StartSeconds, req.EndSeconds, req.Notes)
	if err != nil {
		h.logger.Error("failed to update song", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(song)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	songID := chi.URLParam(r, "songID")
	if err := h.repo.Delete(r.Context(), songID); err != nil {
		h.logger.Error("failed to delete song", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
