package recordings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"shakedown/internal/auth"
	"shakedown/internal/config"
)

// Handler handles HTTP requests for recordings.
type Handler struct {
	svc    *Service
	cfg    *config.Config
	logger *zap.Logger
}

// NewHandler creates a recordings Handler.
func NewHandler(svc *Service, cfg *config.Config, logger *zap.Logger) *Handler {
	return &Handler{svc: svc, cfg: cfg, logger: logger}
}

// Routes registers recording routes with subrouter handlers for songs, comments, and tags.
func (h *Handler) Routes(r chi.Router, requireAuth func(http.Handler) http.Handler,
	songHandler interface{ Routes(chi.Router) },
	commentHandler interface{ Routes(chi.Router) },
	tagHandler interface {
		Routes(chi.Router)
		RecordingTagRoutes(chi.Router)
	},
) {
	r.With(requireAuth).Get("/", h.listRecordings)
	r.With(requireAuth).Post("/", h.upload)
	r.With(requireAuth).Post("/probe", h.probe)
	r.With(requireAuth).Route("/{recordingID}", func(r chi.Router) {
		r.Get("/", h.getRecording)
		r.Patch("/", h.updateRecording)
		r.Delete("/", h.deleteRecording)
		r.With(requireAuth).Get("/stream", h.streamRecording)
		r.With(requireAuth).Get("/audio-stream", h.audioStreamRecording)
		r.With(requireAuth).Get("/download", h.downloadRecording)
		r.With(requireAuth).Get("/waveform", h.waveformData)
		r.With(requireAuth).Get("/thumbnail", h.thumbnailRecording)
		r.With(requireAuth).Get("/segment", h.segmentRecording)
		r.Route("/songs", func(r chi.Router) { songHandler.Routes(r) })
		r.Route("/comments", func(r chi.Router) { commentHandler.Routes(r) })
		r.Route("/tags", func(r chi.Router) { tagHandler.RecordingTagRoutes(r) })
	})
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	maxBytes := h.cfg.VideoUploadMaxSizeMB * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid multipart form: %v"}`, err), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"missing file field"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

    mimeType, ext, validated, err := ValidateMediaMagicBytes(file)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusUnprocessableEntity)
		return
	}

	mediaType := "audio"
	if strings.HasPrefix(mimeType, "video/") {
		mediaType = "video"
	}

	tmpFile, err := os.CreateTemp(h.svc.storage.root, "shakedown-upload-*"+ext)
	if err != nil {
		h.logger.Error("failed to create temp file", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	written, err := io.Copy(tmpFile, validated)
	if err != nil {
		tmpFile.Close()
		h.logger.Error("failed to write upload to temp file", zap.Error(err))
		http.Error(w, `{"error":"upload failed"}`, http.StatusInternalServerError)
		return
	}
	tmpFile.Close()

	recordedAt := time.Now().UTC()
	recordedAtSource := "upload_timestamp"

	probeResult, probeErr := runFFprobe(r.Context(), tmpPath)
	if probeErr == nil {
		if tagDate := parseDateFromTags(probeResult.Format.Tags); !tagDate.IsZero() {
			recordedAt = tagDate
			recordedAtSource = "embedded_tags"
		}
	}

	if recordedAtSource == "upload_timestamp" {
		if formDate := r.FormValue("recorded_at"); formDate != "" {
			if t, err := time.Parse("2006-01-02", formDate); err == nil {
				recordedAt = t.UTC()
				recordedAtSource = "user_set"
			}
		}
	}

	title := r.FormValue("title")

	rec, err := h.svc.repo.Create(r.Context(), CreateRecordingInput{
		Title:            title,
		FileExt:          ext,
		FileSizeBytes:    written,
		MimeType:         mimeType,
		MediaType:        mediaType,
		StoragePath:      "",
		UploadedBy:       user.ID,
		RecordedAt:       recordedAt,
		RecordedAtSource: recordedAtSource,
	})
	if err != nil {
		if errors.Is(err, ErrDuplicateTitle) {
			http.Error(w, `{"error":"a recording with this title already exists for this date"}`, http.StatusConflict)
			return
		}
		h.logger.Error("failed to create recording record", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	storagePath := filepath.Join(rec.ID, "original"+ext)
	finalDir := filepath.Join(h.svc.storage.root, rec.ID)
	if err := os.MkdirAll(finalDir, 0o750); err != nil {
		h.logger.Error("failed to create recording directory", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	destPath := filepath.Join(h.svc.storage.root, storagePath)
	if err := os.Rename(tmpPath, destPath); err != nil {
		h.logger.Error("failed to rename upload to final path", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	_, err = h.svc.repo.db.Exec(r.Context(), `UPDATE recordings SET storage_path=$2, updated_at=now() WHERE id=$1`, rec.ID, storagePath)
	if err != nil {
		h.logger.Error("failed to update storage path", zap.Error(err))
	}
	rec.StoragePath = storagePath

	timeout := h.cfg.ProcessingTimeoutSeconds
	if mediaType == "video" {
		timeout = h.cfg.VideoProcessingTimeoutSeconds
	}
	h.svc.Enqueue(ProcessingJob{
		RecordingID: rec.ID,
		StorageRoot: h.svc.storage.root,
		FileExt:     ext,
		MediaType:   mediaType,
	}, timeout)

	h.logger.Info("recording uploaded", zap.String("id", rec.ID), zap.Int64("bytes", written))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(rec)
}

func (h *Handler) probe(w http.ResponseWriter, r *http.Request) {
	maxBytes := h.cfg.VideoUploadMaxSizeMB * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid multipart form: %v"}`, err), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"missing file field"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	tmpFile, err := os.CreateTemp(h.svc.storage.root, "shakedown-probe-*")
	if err != nil {
		h.logger.Error("probe: failed to create temp file", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, file); err != nil {
		tmpFile.Close()
		h.logger.Error("probe: failed to write temp file", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	tmpFile.Close()

	recordedAt := time.Now().UTC()
	recordedAtSource := "upload_timestamp"

	if formDate := r.FormValue("date"); formDate != "" {
		if t, err := time.Parse("2006-01-02", formDate); err == nil {
			recordedAt = t.UTC()
			recordedAtSource = "user_set"
		}
	}

	if recordedAtSource == "upload_timestamp" {
		probeResult, probeErr := runFFprobe(r.Context(), tmpPath)
		if probeErr == nil {
			if tagDate := parseDateFromTags(probeResult.Format.Tags); !tagDate.IsZero() {
				recordedAt = tagDate
				recordedAtSource = "embedded_tags"
			}
		}
	}

	if recordedAtSource == "upload_timestamp" {
		if formDate := r.FormValue("fallback_date"); formDate != "" {
			if t, err := time.Parse("2006-01-02", formDate); err == nil {
				recordedAt = t.UTC()
				recordedAtSource = "user_set"
			}
		}
	}

	nextNum, err := h.svc.repo.NextTitleNumber(r.Context(), recordedAt)
	if err != nil {
		h.logger.Error("probe: failed to get next title number", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	if o := r.FormValue("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n > 0 {
			nextNum += n
		}
	}

	titlePreview := fmt.Sprintf("Recording #%d %s", nextNum, recordedAt.Format("2006-01-02"))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		RecordedAt    string `json:"recorded_at"`
		NextNumber    int    `json:"next_number"`
		TitlePreview  string `json:"title_preview"`
		DateSource    string `json:"date_source"`
	}{
		RecordedAt:   recordedAt.Format("2006-01-02"),
		NextNumber:   nextNum,
		TitlePreview: titlePreview,
		DateSource:   recordedAtSource,
	})
}

func (h *Handler) listRecordings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := 1
	if p := q.Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			page = n
		}
	}
	pageSize := 20
	if ps := q.Get("limit"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil {
			pageSize = n
		}
	}

	filter := ListFilter{
		TagID:    q.Get("tag"),
		Query:    q.Get("search"),
		From:     q.Get("from"),
		To:       q.Get("to"),
		Sort:     q.Get("sort"),
		Page:     page,
		PageSize: pageSize,
	}

	result, err := h.svc.repo.List(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list recordings", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) getRecording(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "recordingID")
	rec, err := h.svc.repo.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if rec == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rec)
}

func (h *Handler) updateRecording(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "recordingID")

	var req struct {
		Title      *string `json:"title"`
		RecordedAt *string `json:"recorded_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	var recordedAt *time.Time
	if req.RecordedAt != nil {
		t, err := time.Parse(time.RFC3339, *req.RecordedAt)
		if err != nil {
			http.Error(w, `{"error":"invalid recorded_at format, use RFC3339"}`, http.StatusBadRequest)
			return
		}
		recordedAt = &t
	}

	rec, err := h.svc.repo.Update(r.Context(), id, req.Title, recordedAt)
	if err != nil {
		if errors.Is(err, ErrDuplicateTitle) {
			http.Error(w, `{"error":"a recording with this title already exists for this date"}`, http.StatusConflict)
			return
		}
		h.logger.Error("failed to update recording", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if rec == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rec)
}

func (h *Handler) deleteRecording(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "recordingID")
	if err := h.svc.repo.SoftDelete(r.Context(), id); err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) streamRecording(w http.ResponseWriter, r *http.Request) {
	recordingID := chi.URLParam(r, "recordingID")
	rec, err := h.svc.repo.GetByID(r.Context(), recordingID)
	if err != nil || rec == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if !rec.PlaybackReady {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "processing"})
		return
	}

	filePath := filepath.Join(h.svc.storage.root, rec.ID, PlaybackFilename(rec.MediaType))
	f, err := os.Open(filePath)
	if err != nil {
		http.Error(w, `{"error":"file not found"}`, http.StatusNotFound)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	contentType := "audio/mp4"
	if rec.MediaType == "video" {
		contentType = "video/mp4"
	}
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, PlaybackFilename(rec.MediaType), fi.ModTime(), f)
}

func (h *Handler) audioStreamRecording(w http.ResponseWriter, r *http.Request) {
	recordingID := chi.URLParam(r, "recordingID")
	rec, err := h.svc.repo.GetByID(r.Context(), recordingID)
	if err != nil || rec == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if rec.MediaType != "video" {
		h.streamRecording(w, r)
		return
	}
	if !rec.AudioExtractReady {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "processing"})
		return
	}

	filePath := filepath.Join(h.svc.storage.root, rec.ID, AudioExtractFilename())
	f, err := os.Open(filePath)
	if err != nil {
		http.Error(w, `{"error":"audio extract not found"}`, http.StatusNotFound)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "audio/mp4")
	http.ServeContent(w, r, AudioExtractFilename(), fi.ModTime(), f)
}

func (h *Handler) downloadRecording(w http.ResponseWriter, r *http.Request) {
	recordingID := chi.URLParam(r, "recordingID")
	rec, err := h.svc.repo.GetByID(r.Context(), recordingID)
	if err != nil || rec == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	filePath := filepath.Join(h.svc.storage.root, rec.ID, "original"+rec.FileExt)
	f, err := os.Open(filePath)
	if err != nil {
		http.Error(w, `{"error":"file not found"}`, http.StatusNotFound)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s%s"`, rec.Title, rec.FileExt))
	w.Header().Set("Content-Type", rec.MimeType)
	http.ServeContent(w, r, rec.Title+rec.FileExt, fi.ModTime(), f)
}

func (h *Handler) waveformData(w http.ResponseWriter, r *http.Request) {
	recordingID := chi.URLParam(r, "recordingID")
	rec, err := h.svc.repo.GetByID(r.Context(), recordingID)
	if err != nil || rec == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if !rec.WaveformReady {
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(http.StatusAccepted)
		return
	}

	filePath := filepath.Join(h.svc.storage.root, rec.ID, "waveform.json")
	f, err := os.Open(filePath)
	if err != nil {
		http.Error(w, `{"error":"waveform not found"}`, http.StatusNotFound)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	http.ServeContent(w, r, "waveform.json", fi.ModTime(), f)
}

func (h *Handler) thumbnailRecording(w http.ResponseWriter, r *http.Request) {
	recordingID := chi.URLParam(r, "recordingID")
	rec, err := h.svc.repo.GetByID(r.Context(), recordingID)
	if err != nil || rec == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if !rec.ThumbnailReady {
		http.Error(w, `{"error":"thumbnail not ready"}`, http.StatusNotFound)
		return
	}

	filePath := filepath.Join(h.svc.storage.root, rec.ID, "thumbnail.jpg")
	f, err := os.Open(filePath)
	if err != nil {
		http.Error(w, `{"error":"thumbnail not found"}`, http.StatusNotFound)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, "thumbnail.jpg", fi.ModTime(), f)
}

func (h *Handler) segmentRecording(w http.ResponseWriter, r *http.Request) {
	recordingID := chi.URLParam(r, "recordingID")
	rec, err := h.svc.repo.GetByID(r.Context(), recordingID)
	if err != nil || rec == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if !rec.PlaybackReady {
		http.Error(w, `{"error":"not ready"}`, http.StatusAccepted)
		return
	}

	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	if startStr == "" || endStr == "" {
		http.Error(w, `{"error":"start and end query params required"}`, http.StatusBadRequest)
		return
	}
	start, err := strconv.ParseFloat(startStr, 64)
	if err != nil || start < 0 {
		http.Error(w, `{"error":"invalid start"}`, http.StatusBadRequest)
		return
	}
	end, err := strconv.ParseFloat(endStr, 64)
	if err != nil || end <= start {
		http.Error(w, `{"error":"invalid end"}`, http.StatusBadRequest)
		return
	}

	inputPath := filepath.Join(h.svc.storage.root, rec.ID, PlaybackFilename(rec.MediaType))
	duration := end - start

	contentType := "audio/mp4"
	if rec.MediaType == "video" {
		contentType = "video/mp4"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, SnippetFilename(rec.MediaType)))

	var cmd *exec.Cmd
	if rec.MediaType == "video" {
		cmd = exec.CommandContext(r.Context(), "ffmpeg",
			"-ss", strconv.FormatFloat(start, 'f', 3, 64),
			"-i", inputPath,
			"-t", strconv.FormatFloat(duration, 'f', 3, 64),
			"-c:v", "copy", "-c:a", "copy",
			"-f", "mp4", "-movflags", "frag_keyframe+empty_moov",
			"pipe:1",
		)
	} else {
		cmd = exec.CommandContext(r.Context(), "ffmpeg",
			"-ss", strconv.FormatFloat(start, 'f', 3, 64),
			"-i", inputPath,
			"-t", strconv.FormatFloat(duration, 'f', 3, 64),
			"-c:a", "aac", "-b:a", "192k",
			"-f", "mp4", "-movflags", "frag_keyframe+empty_moov",
			"pipe:1",
		)
	}
	cmd.Stdout = w
	if err := cmd.Run(); err != nil {
		h.logger.Error("segment ffmpeg failed", zap.Error(err))
	}
}
