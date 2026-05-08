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

		audioPath := filepath.Join(h.storageRoot, rec.ID, "original"+rec.FileExt)
		if f, err := os.Open(audioPath); err == nil {
			defer f.Close()
			entry, err := zw.Create(fmt.Sprintf("%s/%s_original%s", rec.ID, rec.Title, rec.FileExt))
			if err == nil {
				_, _ = io.Copy(entry, f)
			}
		}
	}

	metaEntry, err := zw.Create("metadata.json")
	if err == nil {
		_ = json.NewEncoder(metaEntry).Encode(map[string]interface{}{
			"exported_at": time.Now(),
			"recordings":  allRecs,
		})
	}

	_ = h.dumpTableToZip(r.Context(), zw, "songs", `SELECT id, recording_id, title, start_seconds, end_seconds, notes, created_at FROM songs ORDER BY recording_id, start_seconds`)
	_ = h.dumpTableToZip(r.Context(), zw, "comments", `SELECT id, recording_id, parent_id, timestamp_seconds, content, author_id, created_at FROM comments WHERE deleted_at IS NULL ORDER BY recording_id, created_at`)
}

func (h *Handler) dumpTableToZip(ctx context.Context, zw *zip.Writer, name, query string) error {
	rows, err := h.db.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	entry, err := zw.Create(name + ".json")
	if err != nil {
		return err
	}

	descriptions := rows.FieldDescriptions()
	var results []map[string]interface{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			continue
		}
		row := make(map[string]interface{}, len(values))
		for i, v := range values {
			row[string(descriptions[i].Name)] = v
		}
		results = append(results, row)
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	return json.NewEncoder(entry).Encode(results)
}
