package tags

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"shakedown/internal/auth"
)

// Handler handles HTTP requests for tags.
type Handler struct {
	repo   *Repository
	logger *zap.Logger
}

func NewHandler(repo *Repository, logger *zap.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}

// Routes registers top-level tag routes (GET /api/tags, POST /api/tags).
func (h *Handler) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
}

// RecordingTagRoutes registers routes for attaching/detaching tags on a recording.
// Mount at /api/recordings/{recordingID}/tags
func (h *Handler) RecordingTagRoutes(r chi.Router) {
	r.Post("/", h.attach)
	r.Delete("/{tagID}", h.detach)
}

type createTagRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type attachTagRequest struct {
	TagID string `json:"tag_id"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tags, err := h.repo.List(r.Context())
	if err != nil {
		h.logger.Error("failed to list tags", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if tags == nil {
		tags = []*Tag{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tags)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req createTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}
	if req.Color == "" {
		req.Color = "#6366f1"
	}

	tag, err := h.repo.Create(r.Context(), req.Name, req.Color, user.ID)
	if err != nil {
		h.logger.Error("failed to create tag", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(tag)
}

func (h *Handler) attach(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	recordingID := chi.URLParam(r, "recordingID")

	var req attachTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.TagID == "" {
		http.Error(w, `{"error":"tag_id is required"}`, http.StatusBadRequest)
		return
	}

	if err := h.repo.Attach(r.Context(), recordingID, req.TagID, user.ID); err != nil {
		h.logger.Error("failed to attach tag", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) detach(w http.ResponseWriter, r *http.Request) {
	recordingID := chi.URLParam(r, "recordingID")
	tagID := chi.URLParam(r, "tagID")

	if err := h.repo.Detach(r.Context(), recordingID, tagID); err != nil {
		h.logger.Error("failed to detach tag", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
