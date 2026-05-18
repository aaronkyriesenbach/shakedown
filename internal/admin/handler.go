package admin

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"shakedown/internal/auth"
)

type Handler struct {
	db          *pgxpool.Pool
	storageRoot string
	logger      *zap.Logger
}

type recLookup struct {
	datePath string
	title    string
}

func NewHandler(db *pgxpool.Pool, storageRoot string, logger *zap.Logger) *Handler {
	return &Handler{db: db, storageRoot: storageRoot, logger: logger}
}

func (h *Handler) Routes(r chi.Router) {
	r.With(auth.RequireAdmin).Get("/dump", h.dataDump)
	r.With(auth.RequireAdmin).Get("/users", h.listUsers)
	r.With(auth.RequireAdmin).Patch("/users/{userID}", h.updateUserRole)
}

type userRow struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `SELECT id, email, display_name, role, created_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		h.logger.Error("failed to list users", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []userRow
	for rows.Next() {
		var u userRow
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.CreatedAt); err != nil {
			continue
		}
		users = append(users, u)
	}
	if users == nil {
		users = []userRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(users)
}

func (h *Handler) updateUserRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if req.Role != "user" && req.Role != "admin" {
		http.Error(w, `{"error":"role must be user or admin"}`, http.StatusBadRequest)
		return
	}
	_, err := h.db.Exec(r.Context(), `UPDATE users SET role=$2, updated_at=now() WHERE id=$1`, userID, req.Role)
	if err != nil {
		h.logger.Error("failed to update user role", zap.Error(err))
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) dataDump(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="shakedown-dump-%s.zip"`, time.Now().Format("20060102-150405")))

	zw := zip.NewWriter(w)
	defer zw.Close()

	rows, err := h.db.Query(r.Context(), `
		SELECT id, title, file_ext, recorded_at, duration_seconds, created_at
		FROM recordings WHERE deleted_at IS NULL ORDER BY recorded_at DESC
	`)
	if err != nil {
		h.logger.Error("dump: failed to query recordings", zap.Error(err))
		return
	}
	defer rows.Close()

	type recMeta struct {
		ID              string    `json:"id"`
		Title           string    `json:"title"`
		FileExt         string    `json:"file_ext"`
		RecordedAt      time.Time `json:"recorded_at"`
		DurationSeconds *float64  `json:"duration_seconds,omitempty"`
		CreatedAt       time.Time `json:"created_at"`
	}

	var allRecs []recMeta
	for rows.Next() {
		var rec recMeta
		if err := rows.Scan(&rec.ID, &rec.Title, &rec.FileExt, &rec.RecordedAt, &rec.DurationSeconds, &rec.CreatedAt); err != nil {
			continue
		}
		allRecs = append(allRecs, rec)

		datePath := fmt.Sprintf("%d/%02d/%02d", rec.RecordedAt.Year(), rec.RecordedAt.Month(), rec.RecordedAt.Day())

		audioPath := filepath.Join(h.storageRoot, rec.ID, "original"+rec.FileExt)
		if f, err := os.Open(audioPath); err == nil {
			defer f.Close()
			entry, err := zw.Create(fmt.Sprintf("%s/%s%s", datePath, rec.Title, rec.FileExt))
			if err == nil {
				_, _ = io.Copy(entry, f)
			}
		}

		metaEntry, err := zw.Create(fmt.Sprintf("%s/%s_metadata.json", datePath, rec.Title))
		if err == nil {
			_ = json.NewEncoder(metaEntry).Encode(rec)
		}
	}

	exportEntry, err := zw.Create("metadata.json")
	if err == nil {
		_ = json.NewEncoder(exportEntry).Encode(map[string]interface{}{
			"exported_at": time.Now(),
		})
	}

	recMap := make(map[string]recLookup, len(allRecs))
	for _, rec := range allRecs {
		datePath := fmt.Sprintf("%d/%02d/%02d", rec.RecordedAt.Year(), rec.RecordedAt.Month(), rec.RecordedAt.Day())
		recMap[rec.ID] = recLookup{datePath: datePath, title: rec.Title}
	}

	_ = h.dumpGroupedToZip(r.Context(), zw, recMap, "songs",
		`SELECT s.id, s.recording_id, s.title, s.start_seconds, s.notes, s.created_at
		 FROM songs s JOIN recordings r ON s.recording_id = r.id
		 WHERE r.deleted_at IS NULL ORDER BY s.recording_id, s.start_seconds`)
	_ = h.dumpGroupedToZip(r.Context(), zw, recMap, "comments",
		`SELECT c.id, c.recording_id, c.parent_id, c.timestamp_seconds, c.content, c.author_id, c.created_at
		 FROM comments c JOIN recordings r ON c.recording_id = r.id
		 WHERE c.deleted_at IS NULL AND r.deleted_at IS NULL ORDER BY c.recording_id, c.created_at`)
}

func (h *Handler) dumpGroupedToZip(ctx context.Context, zw *zip.Writer, recMap map[string]recLookup, label, query string) error {
	rows, err := h.db.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	descriptions := rows.FieldDescriptions()

	grouped := make(map[string][]map[string]interface{})
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			continue
		}
		row := make(map[string]interface{}, len(values))
		var recordingID string
		for i, v := range values {
			col := string(descriptions[i].Name)
			row[col] = v
			if col == "recording_id" {
				if id, ok := v.(string); ok {
					recordingID = id
				}
			}
		}
		grouped[recordingID] = append(grouped[recordingID], row)
	}

	for recID, items := range grouped {
		info, ok := recMap[recID]
		if !ok {
			continue
		}
		entry, err := zw.Create(fmt.Sprintf("%s/%s_%s.json", info.datePath, info.title, label))
		if err != nil {
			continue
		}
		_ = json.NewEncoder(entry).Encode(items)
	}
	return nil
}
