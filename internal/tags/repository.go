package tags

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Tag represents a tag.
type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// Repository handles DB operations for tags.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new tag.
func (r *Repository) Create(ctx context.Context, name, color, userID string) (*Tag, error) {
	id := uuid.New().String()
	var tag Tag
	err := r.db.QueryRow(ctx, `
        INSERT INTO tags (id, name, color, created_by)
        VALUES ($1, $2, $3, $4)
        RETURNING id, name, color, created_by, created_at
    `, id, name, color, userID).Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CreatedBy, &tag.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("tags: failed to create: %w", err)
	}
	return &tag, nil
}

// List returns all tags.
func (r *Repository) List(ctx context.Context) ([]*Tag, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, name, color, created_by, created_at FROM tags ORDER BY name ASC
    `)
	if err != nil {
		return nil, fmt.Errorf("tags: failed to list: %w", err)
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("tags: scan error: %w", err)
		}
		tags = append(tags, &t)
	}
	return tags, rows.Err()
}

// GetByID fetches a tag by ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Tag, error) {
	var t Tag
	err := r.db.QueryRow(ctx, `
        SELECT id, name, color, created_by, created_at FROM tags WHERE id=$1
    `, id).Scan(&t.ID, &t.Name, &t.Color, &t.CreatedBy, &t.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("tags: failed to get: %w", err)
	}
	return &t, nil
}

// ListByRecording returns all tags attached to a recording.
func (r *Repository) ListByRecording(ctx context.Context, recordingID string) ([]*Tag, error) {
	rows, err := r.db.Query(ctx, `
        SELECT t.id, t.name, t.color, t.created_by, t.created_at
        FROM tags t
        JOIN recording_tags rt ON rt.tag_id = t.id
        WHERE rt.recording_id = $1
        ORDER BY t.name ASC
    `, recordingID)
	if err != nil {
		return nil, fmt.Errorf("tags: failed to list for recording: %w", err)
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("tags: scan error: %w", err)
		}
		tags = append(tags, &t)
	}
	return tags, rows.Err()
}

// Attach associates a tag with a recording.
func (r *Repository) Attach(ctx context.Context, recordingID, tagID, userID string) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO recording_tags (recording_id, tag_id, tagged_by)
        VALUES ($1, $2, $3)
        ON CONFLICT DO NOTHING
    `, recordingID, tagID, userID)
	if err != nil {
		return fmt.Errorf("tags: failed to attach: %w", err)
	}
	return nil
}

// Detach removes a tag association from a recording.
func (r *Repository) Detach(ctx context.Context, recordingID, tagID string) error {
	_, err := r.db.Exec(ctx, `
        DELETE FROM recording_tags WHERE recording_id=$1 AND tag_id=$2
    `, recordingID, tagID)
	if err != nil {
		return fmt.Errorf("tags: failed to detach: %w", err)
	}
	return nil
}
