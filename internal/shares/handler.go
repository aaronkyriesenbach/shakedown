package shares

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"shakedown/internal/auth"
	"shakedown/internal/recordings"
	"shakedown/internal/songs"
)

type Handler struct {
	repo     *Repository
	storage  recordings.Storage
	recRepo  *recordings.Repository
	songRepo *songs.Repository
	logger   *zap.Logger
}

func NewHandler(repo *Repository, recRepo *recordings.Repository, songRepo *songs.Repository, storage recordings.Storage, logger *zap.Logger) *Handler {
	return &Handler{repo: repo, recRepo: recRepo, songRepo: songRepo, storage: storage, logger: logger}
}

type createShareRequest struct {
	RecordingID  string     `json:"recording_id"`
	SongID       *string    `json:"song_id"`
	StartSeconds *int       `json:"start_seconds"`
	EndSeconds   *int       `json:"end_seconds"`
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

	if req.StartSeconds != nil && req.EndSeconds != nil {
		rec, err := h.recRepo.GetByID(r.Context(), req.RecordingID)
		if err != nil || rec == nil {
			h.logger.Error("failed to fetch recording for share segment", zap.Error(err))
		} else {
			inputPath, err := h.storage.FullPath(filepath.Join(req.RecordingID, recordings.PlaybackFilename(rec.MediaType)))
			if err != nil {
				h.logger.Error("failed to resolve playback path for share segment", zap.Error(err))
			} else {
				segDir, err := h.storage.FullPath(filepath.Join("shares", share.ID))
				if err != nil {
					h.logger.Error("failed to resolve share segment dir", zap.Error(err))
				} else if err := recordings.ExtractSegment(r.Context(), inputPath, segDir, float64(*req.StartSeconds), float64(*req.EndSeconds), rec.MediaType); err != nil {
					h.logger.Error("failed to extract share segment", zap.Error(err))
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(share)
}

type shareWithRecording struct {
	*Share
	Recording *recordings.Recording `json:"recording,omitempty"`
	Songs     []*songs.Song         `json:"songs,omitempty"`
}

func (h *Handler) GetShare(w http.ResponseWriter, r *http.Request) {
	share := ShareFromContext(r.Context())
	if share == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	rec, err := h.recRepo.GetByID(r.Context(), share.RecordingID)
	if err != nil {
		h.logger.Error("failed to fetch recording for share", zap.Error(err))
	}

	shareSongs, err := h.songRepo.List(r.Context(), share.RecordingID)
	if err != nil {
		h.logger.Error("failed to fetch songs for share", zap.Error(err))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(shareWithRecording{Share: share, Recording: rec, Songs: shareSongs})
}

// StreamShare serves the processed playback file for a share token.
// Section shares serve the pre-extracted snippet from shares/<id>/snippet.m4a (audio) or snippet.mp4 (video).
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

	if share.StartSeconds != nil && share.EndSeconds != nil {
		f, _, err := h.storage.Read(r.Context(), filepath.Join("shares", share.ID, recordings.SnippetFilename(rec.MediaType)))
		if err != nil || f == nil {
			http.Error(w, `{"error":"snippet not found"}`, http.StatusNotFound)
			return
		}
		defer f.Close()

		contentType := "audio/mp4"
		if rec.MediaType == "video" {
			contentType = "video/mp4"
		}
		w.Header().Set("Content-Type", contentType)
		http.ServeContent(w, r, recordings.SnippetFilename(rec.MediaType), rec.UpdatedAt, f)
		return
	}

	f, _, err := h.storage.Read(r.Context(), filepath.Join(rec.ID, recordings.PlaybackFilename(rec.MediaType)))
	if err != nil || f == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	defer f.Close()

	contentType := "audio/mp4"
	if rec.MediaType == "video" {
		contentType = "video/mp4"
	}
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, recordings.PlaybackFilename(rec.MediaType), rec.UpdatedAt, f)
}

func (h *Handler) AudioStreamShare(w http.ResponseWriter, r *http.Request) {
	share := ShareFromContext(r.Context())
	if share == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	rec, err := h.recRepo.GetByID(r.Context(), share.RecordingID)
	if err != nil || rec == nil {
		http.Error(w, `{"error":"not available"}`, http.StatusNotFound)
		return
	}

	if rec.MediaType != "video" {
		h.StreamShare(w, r)
		return
	}

	if !rec.AudioExtractReady {
		http.Error(w, `{"error":"audio not ready"}`, http.StatusAccepted)
		return
	}

	if share.StartSeconds != nil && share.EndSeconds != nil {
		f, _, err := h.storage.Read(r.Context(), filepath.Join("shares", share.ID, recordings.SnippetFilename("audio")))
		if err != nil || f == nil {
			f2, _, err2 := h.storage.Read(r.Context(), filepath.Join(rec.ID, recordings.AudioExtractFilename()))
			if err2 != nil || f2 == nil {
				http.Error(w, `{"error":"audio not found"}`, http.StatusNotFound)
				return
			}
			defer f2.Close()
			w.Header().Set("Content-Type", "audio/mp4")
			http.ServeContent(w, r, recordings.AudioExtractFilename(), rec.UpdatedAt, f2)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "audio/mp4")
		http.ServeContent(w, r, recordings.SnippetFilename("audio"), rec.UpdatedAt, f)
		return
	}

	f, _, err := h.storage.Read(r.Context(), filepath.Join(rec.ID, recordings.AudioExtractFilename()))
	if err != nil || f == nil {
		http.Error(w, `{"error":"audio not found"}`, http.StatusNotFound)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "audio/mp4")
	http.ServeContent(w, r, recordings.AudioExtractFilename(), rec.UpdatedAt, f)
}

func (h *Handler) WaveformShare(w http.ResponseWriter, r *http.Request) {
	share := ShareFromContext(r.Context())
	if share == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	var relPath string
	if share.StartSeconds != nil && share.EndSeconds != nil {
		relPath = filepath.Join("shares", share.ID, "waveform.json")
	} else {
		relPath = filepath.Join(share.RecordingID, "waveform.json")
	}

	f, _, err := h.storage.Read(r.Context(), relPath)
	if err != nil || f == nil {
		http.Error(w, `{"error":"waveform not found"}`, http.StatusNotFound)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, "waveform.json", time.Now(), f)
}

// RecordingRoutes registers share routes nested under /api/recordings/{recordingID}/shares.
func (h *Handler) RecordingRoutes(r chi.Router) {
	r.Get("/", h.ListShares)
	r.Delete("/{shareID}", h.DeleteShare)
}

// ListShares returns all shares for a recording.
func (h *Handler) ListShares(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	recordingID := chi.URLParam(r, "recordingID")
	if recordingID == "" {
		http.Error(w, `{"error":"recording_id required"}`, http.StatusBadRequest)
		return
	}

	shares, err := h.repo.ListByRecordingID(r.Context(), recordingID)
	if err != nil {
		h.logger.Error("failed to list shares", zap.String("recording_id", recordingID), zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	if shares == nil {
		shares = []*Share{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(shares)
}

// DeleteShare removes a share and cleans up any extracted snippet storage.
func (h *Handler) DeleteShare(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	shareID := chi.URLParam(r, "shareID")
	if shareID == "" {
		http.Error(w, `{"error":"share_id required"}`, http.StatusBadRequest)
		return
	}

	// Clean up snippet storage if it exists.
	snippetDir, err := h.storage.FullPath(filepath.Join("shares", shareID))
	if err == nil {
		_ = os.RemoveAll(snippetDir)
	}

	if err := h.repo.Delete(r.Context(), shareID); err != nil {
		h.logger.Error("failed to delete share", zap.String("share_id", shareID), zap.Error(err))
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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

	if share.StartSeconds != nil && share.EndSeconds != nil {
		f, _, err := h.storage.Read(r.Context(), filepath.Join("shares", share.ID, recordings.SnippetFilename(rec.MediaType)))
		if err != nil || f == nil {
			http.Error(w, `{"error":"snippet not found"}`, http.StatusNotFound)
			return
		}
		defer f.Close()

		snippetName := recordings.SnippetFilename(rec.MediaType)
		contentType := "audio/mp4"
		if rec.MediaType == "video" {
			contentType = "video/mp4"
		}
		w.Header().Set("Content-Disposition", "attachment; filename=\""+rec.Title+" (snippet)"+filepath.Ext(snippetName)+"\"")
		w.Header().Set("Content-Type", contentType)
		http.ServeContent(w, r, snippetName, rec.UpdatedAt, f)
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
