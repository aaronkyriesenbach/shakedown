package songs

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Song represents a timestamp marker within a recording.
type Song struct {
	ID           string    `json:"id"`
	RecordingID  string    `json:"recording_id"`
	Title        string    `json:"title"`
	StartSeconds int   `json:"start_seconds"`
	EndSeconds   *int  `json:"end_seconds,omitempty"`
	Notes        *string   `json:"notes,omitempty"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Repository handles DB operations for songs.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new song marker.
func (r *Repository) Create(ctx context.Context, recordingID, userID, title string, startSeconds int, endSeconds *int, notes *string) (*Song, error) {
	id := uuid.New().String()
	var song Song
	err := r.db.QueryRow(ctx, `
		INSERT INTO songs (id, recording_id, title, start_seconds, end_seconds, notes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, recording_id, title, start_seconds, end_seconds, notes, created_by, created_at, updated_at
	`, id, recordingID, title, startSeconds, endSeconds, notes, userID).Scan(
		&song.ID, &song.RecordingID, &song.Title, &song.StartSeconds, &song.EndSeconds,
		&song.Notes, &song.CreatedBy, &song.CreatedAt, &song.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("songs: failed to create: %w", err)
	}
	return &song, nil
}

// List returns all songs for a recording, ordered by start_seconds.
func (r *Repository) List(ctx context.Context, recordingID string) ([]*Song, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, recording_id, title, start_seconds, end_seconds, notes, created_by, created_at, updated_at
		FROM songs
		WHERE recording_id = $1
		ORDER BY start_seconds ASC
	`, recordingID)
	if err != nil {
		return nil, fmt.Errorf("songs: failed to list: %w", err)
	}
	defer rows.Close()

	var songs []*Song
	for rows.Next() {
		var s Song
		if err := rows.Scan(&s.ID, &s.RecordingID, &s.Title, &s.StartSeconds, &s.EndSeconds,
			&s.Notes, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("songs: failed to scan: %w", err)
		}
		songs = append(songs, &s)
	}
	return songs, rows.Err()
}

// GetByID fetches a single song by ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Song, error) {
	var s Song
	err := r.db.QueryRow(ctx, `
		SELECT id, recording_id, title, start_seconds, end_seconds, notes, created_by, created_at, updated_at
		FROM songs WHERE id = $1
	`, id).Scan(&s.ID, &s.RecordingID, &s.Title, &s.StartSeconds, &s.EndSeconds,
		&s.Notes, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("songs: failed to get: %w", err)
	}
	return &s, nil
}

// Update modifies a song's fields.
func (r *Repository) Update(ctx context.Context, id, title string, startSeconds int, endSeconds *int, notes *string) (*Song, error) {
	var s Song
	err := r.db.QueryRow(ctx, `
		UPDATE songs SET title=$2, start_seconds=$3, end_seconds=$4, notes=$5, updated_at=now()
		WHERE id=$1
		RETURNING id, recording_id, title, start_seconds, end_seconds, notes, created_by, created_at, updated_at
	`, id, title, startSeconds, endSeconds, notes).Scan(
		&s.ID, &s.RecordingID, &s.Title, &s.StartSeconds, &s.EndSeconds,
		&s.Notes, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("songs: failed to update: %w", err)
	}
	return &s, nil
}

// Delete removes a song by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM songs WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("songs: failed to delete: %w", err)
	}
	return nil
}
