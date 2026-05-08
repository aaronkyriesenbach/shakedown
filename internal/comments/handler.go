package comments

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"shakedown/internal/auth"
)

// Handler handles HTTP requests for comments.
type Handler struct {
	repo   *Repository
	logger *zap.Logger
}

func NewHandler(repo *Repository, logger *zap.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}

// Routes registers comment routes under /api/recordings/{recordingID}/comments
func (h *Handler) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Patch("/{commentID}", h.update)
	r.Delete("/{commentID}", h.delete)
}

type createCommentRequest struct {
	Content          string   `json:"content"`
	TimestampSeconds *float64 `json:"timestamp_seconds"`
	SongID           *string  `json:"song_id"`
	ParentID         *string  `json:"parent_id"`
}

type updateCommentRequest struct {
	Content string `json:"content"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	recordingID := chi.URLParam(r, "recordingID")
	flat, err := h.repo.ListByRecording(r.Context(), recordingID)
	if err != nil {
		h.logger.Error("failed to list comments", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	threaded := BuildThread(flat)
	if threaded == nil {
		threaded = []*Comment{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(threaded)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	recordingID := chi.URLParam(r, "recordingID")

	var req createCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		http.Error(w, `{"error":"content is required"}`, http.StatusBadRequest)
		return
	}

	comment, err := h.repo.Create(r.Context(), recordingID, user.ID, req.Content, req.SongID, req.ParentID, req.TimestampSeconds)
	if err != nil {
		h.logger.Error("failed to create comment", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(comment)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	commentID := chi.URLParam(r, "commentID")

	var req updateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		http.Error(w, `{"error":"content is required"}`, http.StatusBadRequest)
		return
	}

	comment, err := h.repo.Update(r.Context(), commentID, req.Content)
	if err != nil {
		h.logger.Error("failed to update comment", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(comment)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	commentID := chi.URLParam(r, "commentID")
	if err := h.repo.SoftDelete(r.Context(), commentID); err != nil {
		h.logger.Error("failed to delete comment", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
