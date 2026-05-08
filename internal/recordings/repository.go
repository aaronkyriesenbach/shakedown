package recordings

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Recording represents a recording row from the database.
type Recording struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	FileExt       string     `json:"file_ext"`
	FileSizeBytes    int64      `json:"file_size_bytes"`
	MimeType         string     `json:"mime_type"`
	StoragePath      string     `json:"storage_path"`
	UploadedBy       string     `json:"uploaded_by"`
	RecordedAt       time.Time  `json:"recorded_at"`
	RecordedAtSource string     `json:"recorded_at_source"`
	DurationSeconds  *float64   `json:"duration_seconds,omitempty"`
	Bitrate          *int       `json:"bitrate,omitempty"`
	SampleRate       *int       `json:"sample_rate,omitempty"`
	Channels         *int       `json:"channels,omitempty"`
	PlaybackReady    bool       `json:"playback_ready"`
	WaveformReady    bool       `json:"waveform_ready"`
	ProcessingError  *string    `json:"processing_error,omitempty"`
	ProcessingStep   string     `json:"processing_step"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

// CreateRecordingInput holds the data needed to create a recording.
type CreateRecordingInput struct {
	Title     string
	FileExt   string
	FileSizeBytes    int64
	MimeType         string
	StoragePath      string
	UploadedBy       string
	RecordedAt       time.Time
	RecordedAtSource string
}

// Repository handles database operations for recordings.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new recordings Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new recording into the database.
// When input.Title is empty, it atomically assigns the next sequential
// recording number using an advisory lock so concurrent uploads never collide.
func (repo *Repository) Create(ctx context.Context, input CreateRecordingInput) (*Recording, error) {
	id := uuid.New().String()

	if input.Title == "" {
		return repo.createWithAutoTitle(ctx, id, input)
	}

	var rec Recording
	err := repo.db.QueryRow(ctx, `
		INSERT INTO recordings (id, title, file_ext, file_size_bytes, mime_type,
			storage_path, uploaded_by, recorded_at, recorded_at_source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, title, file_ext, file_size_bytes, mime_type,
			storage_path, uploaded_by, recorded_at, recorded_at_source,
			duration_seconds, bitrate, sample_rate, channels,
			playback_ready, waveform_ready, processing_error, processing_step,
			created_at, updated_at, deleted_at
	`, id, input.Title, input.FileExt, input.FileSizeBytes, input.MimeType,
		input.StoragePath, input.UploadedBy, input.RecordedAt, input.RecordedAtSource,
	).Scan(
		&rec.ID, &rec.Title, &rec.FileExt, &rec.FileSizeBytes, &rec.MimeType,
		&rec.StoragePath, &rec.UploadedBy, &rec.RecordedAt, &rec.RecordedAtSource,
		&rec.DurationSeconds, &rec.Bitrate, &rec.SampleRate, &rec.Channels,
		&rec.PlaybackReady, &rec.WaveformReady, &rec.ProcessingError, &rec.ProcessingStep,
		&rec.CreatedAt, &rec.UpdatedAt, &rec.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("recordings: failed to create: %w", err)
	}
	return &rec, nil
}

// createWithAutoTitle generates a unique sequential title inside a transaction
// guarded by a PostgreSQL advisory lock, then inserts the recording.
func (repo *Repository) createWithAutoTitle(ctx context.Context, id string, input CreateRecordingInput) (*Recording, error) {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("recordings: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Serialize concurrent auto-title generation.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", int64(0x7368616B65)); err != nil {
		return nil, fmt.Errorf("recordings: advisory lock: %w", err)
	}

	var count int
	if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM recordings WHERE deleted_at IS NULL").Scan(&count); err != nil {
		return nil, fmt.Errorf("recordings: count for auto-title: %w", err)
	}

	input.Title = fmt.Sprintf("Recording #%d %s", count+1, input.RecordedAt.Format("2006-01-02"))

	var rec Recording
	err = tx.QueryRow(ctx, `
		INSERT INTO recordings (id, title, file_ext, file_size_bytes, mime_type,
			storage_path, uploaded_by, recorded_at, recorded_at_source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, title, file_ext, file_size_bytes, mime_type,
			storage_path, uploaded_by, recorded_at, recorded_at_source,
			duration_seconds, bitrate, sample_rate, channels,
			playback_ready, waveform_ready, processing_error, processing_step,
			created_at, updated_at, deleted_at
	`, id, input.Title, input.FileExt, input.FileSizeBytes, input.MimeType,
		input.StoragePath, input.UploadedBy, input.RecordedAt, input.RecordedAtSource,
	).Scan(
		&rec.ID, &rec.Title, &rec.FileExt, &rec.FileSizeBytes, &rec.MimeType,
		&rec.StoragePath, &rec.UploadedBy, &rec.RecordedAt, &rec.RecordedAtSource,
		&rec.DurationSeconds, &rec.Bitrate, &rec.SampleRate, &rec.Channels,
		&rec.PlaybackReady, &rec.WaveformReady, &rec.ProcessingError, &rec.ProcessingStep,
		&rec.CreatedAt, &rec.UpdatedAt, &rec.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("recordings: failed to create: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("recordings: commit: %w", err)
	}

	return &rec, nil
}

// GetByID fetches a recording by ID. Returns nil if not found or soft-deleted.
func (repo *Repository) GetByID(ctx context.Context, id string) (*Recording, error) {
	var rec Recording
	err := repo.db.QueryRow(ctx, `
		SELECT id, title, file_ext, file_size_bytes, mime_type,
			storage_path, uploaded_by, recorded_at, recorded_at_source,
			duration_seconds, bitrate, sample_rate, channels,
			playback_ready, waveform_ready, processing_error, processing_step,
			created_at, updated_at, deleted_at
		FROM recordings
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&rec.ID, &rec.Title, &rec.FileExt, &rec.FileSizeBytes, &rec.MimeType,
		&rec.StoragePath, &rec.UploadedBy, &rec.RecordedAt, &rec.RecordedAtSource,
		&rec.DurationSeconds, &rec.Bitrate, &rec.SampleRate, &rec.Channels,
		&rec.PlaybackReady, &rec.WaveformReady, &rec.ProcessingError, &rec.ProcessingStep,
		&rec.CreatedAt, &rec.UpdatedAt, &rec.DeletedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recordings: failed to get by ID: %w", err)
	}
	return &rec, nil
}

// SoftDelete marks a recording as deleted.
func (repo *Repository) SoftDelete(ctx context.Context, id string) error {
	_, err := repo.db.Exec(ctx, `
		UPDATE recordings SET deleted_at = now(), updated_at = now() WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("recordings: failed to soft delete: %w", err)
	}
	return nil
}

// UpdateProcessingResult updates processing metadata after pipeline completion.
func (repo *Repository) UpdateProcessingResult(
	ctx context.Context,
	id string,
	durationSeconds float64,
	bitrate, sampleRate, channels int,
	playbackReady, waveformReady bool,
	processingError *string,
) error {
	step := "complete"
	if processingError != nil {
		step = "complete"
	}
	_, err := repo.db.Exec(ctx, `
		UPDATE recordings SET
			duration_seconds = $2, bitrate = $3, sample_rate = $4, channels = $5,
			playback_ready = $6, waveform_ready = $7, processing_error = $8,
			processing_step = $9,
			updated_at = now()
		WHERE id = $1
	`, id, durationSeconds, bitrate, sampleRate, channels, playbackReady, waveformReady, processingError, step)
	if err != nil {
		return fmt.Errorf("recordings: failed to update processing result: %w", err)
	}
	return nil
}

func (repo *Repository) UpdateProcessingStep(ctx context.Context, id string, step string) error {
	_, err := repo.db.Exec(ctx, `
		UPDATE recordings SET processing_step = $2, updated_at = now() WHERE id = $1
	`, id, step)
	if err != nil {
		return fmt.Errorf("recordings: failed to update processing step: %w", err)
	}
	return nil
}

// ListFilter defines query parameters for listing recordings.
type ListFilter struct {
	TagID    string // filter by tag
	Query    string // full-text search
	From     string // ISO date string (recorded_at >= from)
	To       string // ISO date string (recorded_at <= to)
	Sort     string // "recorded_at_desc" (default) or "recorded_at_asc"
	Page     int    // 1-indexed
	PageSize int    // default 20, max 100
}

// ListResult contains paginated results.
type ListResult struct {
	Recordings []*Recording `json:"data"`
	Total      int          `json:"total"`
	Page       int          `json:"page"`
	PageSize   int          `json:"limit"`
}

// List returns paginated, filterable recordings.
func (repo *Repository) List(ctx context.Context, f ListFilter) (*ListResult, error) {
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 20
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	offset := (f.Page - 1) * f.PageSize

	// Build dynamic WHERE conditions.
	conditions := []string{"r.deleted_at IS NULL"}
	args := []interface{}{}
	argIdx := 1

	if f.TagID != "" {
		conditions = append(conditions, fmt.Sprintf("EXISTS (SELECT 1 FROM recording_tags rt WHERE rt.recording_id=r.id AND rt.tag_id=$%d)", argIdx))
		args = append(args, f.TagID)
		argIdx++
	}
	if f.Query != "" {
		conditions = append(conditions, fmt.Sprintf("r.search_vector @@ plainto_tsquery('english', $%d)", argIdx))
		args = append(args, f.Query)
		argIdx++
	}
	if f.From != "" {
		conditions = append(conditions, fmt.Sprintf("r.recorded_at >= $%d", argIdx))
		args = append(args, f.From)
		argIdx++
	}
	if f.To != "" {
		conditions = append(conditions, fmt.Sprintf("r.recorded_at <= $%d", argIdx))
		args = append(args, f.To)
		argIdx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")
	orderBy := "ORDER BY r.recorded_at DESC"
	if f.Sort == "recorded_at_asc" {
		orderBy = "ORDER BY r.recorded_at ASC"
	}

	// Count query.
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM recordings r %s", where)
	var total int
	if err := repo.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("recordings: failed to count: %w", err)
	}

	// Data query.
	args = append(args, f.PageSize, offset)
	dataSQL := fmt.Sprintf(`
		SELECT r.id, r.title, r.file_ext, r.file_size_bytes, r.mime_type,
			r.storage_path, r.uploaded_by, r.recorded_at, r.recorded_at_source,
			r.duration_seconds, r.bitrate, r.sample_rate, r.channels,
			r.playback_ready, r.waveform_ready, r.processing_error, r.processing_step,
			r.created_at, r.updated_at, r.deleted_at
		FROM recordings r
		%s %s
		LIMIT $%d OFFSET $%d
	`, where, orderBy, argIdx, argIdx+1)

	rows, err := repo.db.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("recordings: failed to list: %w", err)
	}
	defer rows.Close()

	var recs []*Recording
	for rows.Next() {
		var rec Recording
		if err := rows.Scan(
			&rec.ID, &rec.Title, &rec.FileExt, &rec.FileSizeBytes, &rec.MimeType,
			&rec.StoragePath, &rec.UploadedBy, &rec.RecordedAt, &rec.RecordedAtSource,
			&rec.DurationSeconds, &rec.Bitrate, &rec.SampleRate, &rec.Channels,
			&rec.PlaybackReady, &rec.WaveformReady, &rec.ProcessingError, &rec.ProcessingStep,
			&rec.CreatedAt, &rec.UpdatedAt, &rec.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("recordings: scan error: %w", err)
		}
		recs = append(recs, &rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if recs == nil {
		recs = []*Recording{}
	}

	return &ListResult{
		Recordings: recs,
		Total:      total,
		Page:       f.Page,
		PageSize:   f.PageSize,
	}, nil
}

// Update modifies a recording's title and/or recorded_at.
func (repo *Repository) Update(ctx context.Context, id string, title *string, recordedAt *time.Time) (*Recording, error) {
	var rec Recording
	err := repo.db.QueryRow(ctx, `
		UPDATE recordings SET
			title = COALESCE($2, title),
			recorded_at = COALESCE($3, recorded_at),
			recorded_at_source = CASE WHEN $3 IS NOT NULL THEN 'user_set' ELSE recorded_at_source END,
			updated_at = now()
		WHERE id=$1 AND deleted_at IS NULL
		RETURNING id, title, file_ext, file_size_bytes, mime_type,
			storage_path, uploaded_by, recorded_at, recorded_at_source,
			duration_seconds, bitrate, sample_rate, channels,
			playback_ready, waveform_ready, processing_error, processing_step,
			created_at, updated_at, deleted_at
	`, id, title, recordedAt).Scan(
		&rec.ID, &rec.Title, &rec.FileExt, &rec.FileSizeBytes, &rec.MimeType,
		&rec.StoragePath, &rec.UploadedBy, &rec.RecordedAt, &rec.RecordedAtSource,
		&rec.DurationSeconds, &rec.Bitrate, &rec.SampleRate, &rec.Channels,
		&rec.PlaybackReady, &rec.WaveformReady, &rec.ProcessingError, &rec.ProcessingStep,
		&rec.CreatedAt, &rec.UpdatedAt, &rec.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("recordings: failed to update: %w", err)
	}
	return &rec, nil
}

func (repo *Repository) CountAll(ctx context.Context) (int, error) {
	var count int
	err := repo.db.QueryRow(ctx, "SELECT COUNT(*) FROM recordings").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("recordings: failed to count: %w", err)
	}
	return count, nil
}
