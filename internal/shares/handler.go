package shares

import (
    "encoding/json"
    "net/http"
    "path/filepath"
    "time"

    "go.uber.org/zap"

    "shakedown/internal/auth"
    "shakedown/internal/recordings"
)

type Handler struct {
    repo    *Repository
    storage recordings.Storage
    recRepo *recordings.Repository
    logger  *zap.Logger
}

func NewHandler(repo *Repository, recRepo *recordings.Repository, storage recordings.Storage, logger *zap.Logger) *Handler {
    return &Handler{repo: repo, recRepo: recRepo, storage: storage, logger: logger}
}

type createShareRequest struct {
	RecordingID  string     `json:"recording_id"`
	SongID       *string    `json:"song_id"`
	StartSeconds *int      `json:"start_seconds"`
	EndSeconds   *int      `json:"end_seconds"`
	Label        *string    `json:"label"`
	ExpiresAt    *time.Time `json:"expires_at"`
}

func (h *Handler) CreateShare(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req createShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.RecordingID == "" {
		http.Error(w, `{"error":"recording_id required"}`, http.StatusBadRequest)
		return
	}

	share, err := h.repo.Create(r.Context(), req.RecordingID, user.ID,
		req.SongID, req.StartSeconds, req.EndSeconds, req.Label, req.ExpiresAt)
	if err != nil {
		h.logger.Error("failed to create share", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(share)
}

func (h *Handler) GetShare(w http.ResponseWriter, r *http.Request) {
    share := ShareFromContext(r.Context())
    if share == nil {
        http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(share)
}

// StreamShare serves the processed playback file for a share token.
func (h *Handler) StreamShare(w http.ResponseWriter, r *http.Request) {
    share := ShareFromContext(r.Context())
    if share == nil {
        http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
        return
    }

    rec, err := h.recRepo.GetByID(r.Context(), share.RecordingID)
    if err != nil || rec == nil || !rec.PlaybackReady {
        http.Error(w, `{"error":"not available"}`, http.StatusNotFound)
        return
    }

    // Use storage.Read to open the playback file (path: <rec.ID>/playback.m4a)
    relPath := filepath.Join(rec.ID, "playback.m4a")
    f, _, err := h.storage.Read(r.Context(), relPath)
    if err != nil || f == nil {
        http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
        return
    }
    defer f.Close()

    w.Header().Set("Content-Type", "audio/mp4")
    http.ServeContent(w, r, "playback.m4a", rec.UpdatedAt, f)
}

// DownloadShare serves the original uploaded file as an attachment for download.
func (h *Handler) DownloadShare(w http.ResponseWriter, r *http.Request) {
    share := ShareFromContext(r.Context())
    if share == nil {
        http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
        return
    }

    rec, err := h.recRepo.GetByID(r.Context(), share.RecordingID)
    if err != nil || rec == nil {
        http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
        return
    }

    relPath := filepath.Join(rec.ID, "original"+rec.FileExt)
    f, _, err := h.storage.Read(r.Context(), relPath)
    if err != nil || f == nil {
        http.Error(w, `{"error":"file not found"}`, http.StatusNotFound)
        return
    }
    defer f.Close()

    w.Header().Set("Content-Disposition", "attachment; filename=\""+rec.Title+rec.FileExt+"\"")
    w.Header().Set("Content-Type", rec.MimeType)
    http.ServeContent(w, r, rec.Title+rec.FileExt, rec.UpdatedAt, f)
}
