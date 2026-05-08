package comments

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Comment represents a comment in the DB.
type Comment struct {
	ID               string     `json:"id"`
	RecordingID      string     `json:"recording_id"`
	SongID           *string    `json:"song_id,omitempty"`
	ParentID         *string    `json:"parent_id,omitempty"`
	TimestampSeconds *float64   `json:"timestamp_seconds,omitempty"`
	Content          string     `json:"content"`
	AuthorID         string     `json:"author_id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
	// Nested replies (populated at app level, not DB)
	Replies []*Comment `json:"replies,omitempty"`
}

// Repository handles DB operations for comments.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new comment.
func (r *Repository) Create(ctx context.Context, recordingID, authorID, content string, songID, parentID *string, timestampSeconds *float64) (*Comment, error) {
	id := uuid.New().String()
	var c Comment
	err := r.db.QueryRow(ctx, `
		INSERT INTO comments (id, recording_id, song_id, parent_id, timestamp_seconds, content, author_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, recording_id, song_id, parent_id, timestamp_seconds, content, author_id, created_at, updated_at, deleted_at
	`, id, recordingID, songID, parentID, timestampSeconds, content, authorID).Scan(
		&c.ID, &c.RecordingID, &c.SongID, &c.ParentID, &c.TimestampSeconds,
		&c.Content, &c.AuthorID, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("comments: failed to create: %w", err)
	}
	return &c, nil
}

// ListByRecording returns all non-deleted comments for a recording, ordered by created_at.
// Returns a flat list; the handler builds the threaded structure.
func (r *Repository) ListByRecording(ctx context.Context, recordingID string) ([]*Comment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, recording_id, song_id, parent_id, timestamp_seconds, content, author_id, created_at, updated_at, deleted_at
		FROM comments
		WHERE recording_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
	`, recordingID)
	if err != nil {
		return nil, fmt.Errorf("comments: failed to list: %w", err)
	}
	defer rows.Close()

	var comments []*Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.RecordingID, &c.SongID, &c.ParentID, &c.TimestampSeconds,
			&c.Content, &c.AuthorID, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt); err != nil {
			return nil, fmt.Errorf("comments: scan error: %w", err)
		}
		comments = append(comments, &c)
	}
	return comments, rows.Err()
}

// GetByID fetches a comment by ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Comment, error) {
	var c Comment
	err := r.db.QueryRow(ctx, `
		SELECT id, recording_id, song_id, parent_id, timestamp_seconds, content, author_id, created_at, updated_at, deleted_at
		FROM comments WHERE id = $1
	`, id).Scan(&c.ID, &c.RecordingID, &c.SongID, &c.ParentID, &c.TimestampSeconds,
		&c.Content, &c.AuthorID, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("comments: failed to get: %w", err)
	}
	return &c, nil
}

// Update changes the content of a comment.
func (r *Repository) Update(ctx context.Context, id, content string) (*Comment, error) {
	var c Comment
	err := r.db.QueryRow(ctx, `
		UPDATE comments SET content=$2, updated_at=now()
		WHERE id=$1 AND deleted_at IS NULL
		RETURNING id, recording_id, song_id, parent_id, timestamp_seconds, content, author_id, created_at, updated_at, deleted_at
	`, id, content).Scan(&c.ID, &c.RecordingID, &c.SongID, &c.ParentID, &c.TimestampSeconds,
		&c.Content, &c.AuthorID, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt)
	if err != nil {
		return nil, fmt.Errorf("comments: failed to update: %w", err)
	}
	return &c, nil
}

// SoftDelete marks a comment as deleted.
func (r *Repository) SoftDelete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE comments SET deleted_at=now(), updated_at=now() WHERE id=$1
	`, id)
	if err != nil {
		return fmt.Errorf("comments: failed to soft delete: %w", err)
	}
	return nil
}

// BuildThread converts a flat list of comments into a threaded structure.
// Top-level comments have ParentID == nil; their Replies are populated.
func BuildThread(flat []*Comment) []*Comment {
	index := make(map[string]*Comment, len(flat))
	for _, c := range flat {
		index[c.ID] = c
	}

	var roots []*Comment
	for _, c := range flat {
		if c.ParentID == nil {
			roots = append(roots, c)
		} else if parent, ok := index[*c.ParentID]; ok {
			parent.Replies = append(parent.Replies, c)
		}
	}
	return roots
}
