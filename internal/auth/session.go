package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const sessionDuration = 24 * time.Hour * 30 // 30 days

// User represents the authenticated user.
type User struct {
	ID          string    `json:"id"`
	OIDCSub     string    `json:"oidc_sub"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Session represents a DB-backed session.
type Session struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
}

// UpsertUser creates or updates a user based on OIDC claims.
// If role is non-nil, the user's role is set/updated (used when ADMIN_GROUP is configured).
// If role is nil, the role column is left unchanged (preserving manual admin management).
func UpsertUser(ctx context.Context, db *pgxpool.Pool, oidcSub, email, displayName string, avatarURL *string, role *string) (*User, error) {
	var user User

	query := `
		INSERT INTO users (oidc_sub, email, display_name, avatar_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (oidc_sub) DO UPDATE
		  SET email = EXCLUDED.email,
		      display_name = EXCLUDED.display_name,
		      avatar_url = EXCLUDED.avatar_url,
		      updated_at = now()
		RETURNING id, oidc_sub, email, display_name, avatar_url, role, created_at, updated_at`
	args := []interface{}{oidcSub, email, displayName, avatarURL}

	if role != nil {
		query = `
		INSERT INTO users (oidc_sub, email, display_name, avatar_url, role)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (oidc_sub) DO UPDATE
		  SET email = EXCLUDED.email,
		      display_name = EXCLUDED.display_name,
		      avatar_url = EXCLUDED.avatar_url,
		      role = EXCLUDED.role,
		      updated_at = now()
		RETURNING id, oidc_sub, email, display_name, avatar_url, role, created_at, updated_at`
		args = append(args, *role)
	}

	err := db.QueryRow(ctx, query, args...).Scan(
		&user.ID, &user.OIDCSub, &user.Email, &user.DisplayName,
		&user.AvatarURL, &user.Role, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to upsert user: %w", err)
	}
	return &user, nil
}

// CreateSession inserts a new session into the DB and returns the session.
func CreateSession(ctx context.Context, db *pgxpool.Pool, userID string) (*Session, error) {
	sessionID, err := GenerateRandomString(32)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(sessionDuration)
	_, err = db.Exec(ctx, `
		INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)
	`, sessionID, userID, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to create session: %w", err)
	}
	return &Session{ID: sessionID, UserID: userID, ExpiresAt: expiresAt}, nil
}

// GetSession looks up a session by ID and returns the associated user.
// Returns (nil, nil) if the session doesn't exist or is expired.
func GetSession(ctx context.Context, db *pgxpool.Pool, sessionID string) (*User, error) {
	var user User
	err := db.QueryRow(ctx, `
		SELECT u.id, u.oidc_sub, u.email, u.display_name, u.avatar_url, u.role, u.created_at, u.updated_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.expires_at > now()
	`, sessionID).Scan(
		&user.ID, &user.OIDCSub, &user.Email, &user.DisplayName,
		&user.AvatarURL, &user.Role, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, nil //nolint:nilerr
	}
	return &user, nil
}

// DeleteSession removes a session from the DB.
func DeleteSession(ctx context.Context, db *pgxpool.Pool, sessionID string) error {
	_, err := db.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
	if err != nil {
		return fmt.Errorf("auth: failed to delete session: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by their internal ID.
func GetUserByID(ctx context.Context, db *pgxpool.Pool, userID string) (*User, error) {
	var user User
	err := db.QueryRow(ctx, `
		SELECT id, oidc_sub, email, display_name, avatar_url, role, created_at, updated_at
		FROM users WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.OIDCSub, &user.Email, &user.DisplayName,
		&user.AvatarURL, &user.Role, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to get user: %w", err)
	}
	return &user, nil
}
