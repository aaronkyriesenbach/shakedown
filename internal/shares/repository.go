package shares

import (
	"context"
	cryptoRand "crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Share struct {
	ID           string     `json:"id"`
	Token        string     `json:"token"`
	RecordingID  string     `json:"recording_id"`
	SongID       *string    `json:"song_id,omitempty"`
	StartSeconds *int       `json:"start_seconds,omitempty"`
	EndSeconds   *int       `json:"end_seconds,omitempty"`
	Label        *string    `json:"label,omitempty"`
	CreatedBy    string     `json:"created_by"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	AccessCount  int        `json:"access_count"`
	CreatedAt    time.Time  `json:"created_at"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, recordingID, userID string, songID *string, startSeconds, endSeconds *int, label *string, expiresAt *time.Time) (*Share, error) {
	id := uuid.New().String()
	tokenBytes := make([]byte, 32)
	if _, err := cryptoRand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("shares: failed to generate token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	var s Share
	err := r.db.QueryRow(ctx, `
		INSERT INTO shares (id, token, recording_id, song_id, start_seconds, end_seconds, label, created_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, token, recording_id, song_id, start_seconds, end_seconds, label, created_by, expires_at, access_count, created_at
	`, id, token, recordingID, songID, startSeconds, endSeconds, label, userID, expiresAt).Scan(
		&s.ID, &s.Token, &s.RecordingID, &s.SongID, &s.StartSeconds, &s.EndSeconds,
		&s.Label, &s.CreatedBy, &s.ExpiresAt, &s.AccessCount, &s.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("shares: failed to create: %w", err)
	}
	return &s, nil
}

func (r *Repository) GetByToken(ctx context.Context, token string) (*Share, error) {
	var s Share
	err := r.db.QueryRow(ctx, `
		SELECT id, token, recording_id, song_id, start_seconds, end_seconds, label, created_by, expires_at, access_count, created_at
		FROM shares WHERE token=$1 AND (expires_at IS NULL OR expires_at > now())
	`, token).Scan(
		&s.ID, &s.Token, &s.RecordingID, &s.SongID, &s.StartSeconds, &s.EndSeconds,
		&s.Label, &s.CreatedBy, &s.ExpiresAt, &s.AccessCount, &s.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("shares: failed to get by token: %w", err)
	}
	return &s, nil
}

func (r *Repository) ListByRecordingID(ctx context.Context, recordingID string) ([]*Share, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, token, recording_id, song_id, start_seconds, end_seconds, label, created_by, expires_at, access_count, created_at
		FROM shares WHERE recording_id=$1
		ORDER BY created_at DESC
	`, recordingID)
	if err != nil {
		return nil, fmt.Errorf("shares: failed to list by recording: %w", err)
	}
	defer rows.Close()

	var shares []*Share
	for rows.Next() {
		var s Share
		if err := rows.Scan(
			&s.ID, &s.Token, &s.RecordingID, &s.SongID, &s.StartSeconds, &s.EndSeconds,
			&s.Label, &s.CreatedBy, &s.ExpiresAt, &s.AccessCount, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("shares: failed to scan row: %w", err)
		}
		shares = append(shares, &s)
	}
	return shares, rows.Err()
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM shares WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("shares: failed to delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) IncrementAccess(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE shares SET access_count=access_count+1 WHERE id=$1`, id)
	return err
}
