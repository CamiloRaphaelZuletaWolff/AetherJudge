package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

//nolint:gosec // column list, not a credential
const refreshTokenColumns = "id, user_id, token_hash, expires_at, created_at, revoked_at, replaced_by"

func scanRefreshToken(row pgx.Row) (RefreshToken, error) {
	var t RefreshToken
	err := row.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.CreatedAt, &t.RevokedAt, &t.ReplacedBy)
	return t, err
}

// CreateRefreshToken stores a new (hashed) refresh token.
func (s *Store) CreateRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (RefreshToken, error) {
	t, err := scanRefreshToken(s.pool.QueryRow(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)
		 RETURNING `+refreshTokenColumns,
		userID, tokenHash, expiresAt))
	if err != nil {
		return RefreshToken{}, fmt.Errorf("db: create refresh token: %w", err)
	}
	return t, nil
}

// GetRefreshTokenByHash looks a token up by its hash, or ErrNotFound.
func (s *Store) GetRefreshTokenByHash(ctx context.Context, tokenHash []byte) (RefreshToken, error) {
	t, err := scanRefreshToken(s.pool.QueryRow(ctx,
		`SELECT `+refreshTokenColumns+` FROM refresh_tokens WHERE token_hash = $1`, tokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshToken{}, ErrNotFound
	}
	if err != nil {
		return RefreshToken{}, fmt.Errorf("db: get refresh token: %w", err)
	}
	return t, nil
}

// RevokeRefreshToken marks one token revoked, optionally recording its
// rotation successor.
func (s *Store) RevokeRefreshToken(ctx context.Context, id uuid.UUID, replacedBy *uuid.UUID) error {
	if _, err := s.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now(), replaced_by = $2
		 WHERE id = $1 AND revoked_at IS NULL`,
		id, replacedBy); err != nil {
		return fmt.Errorf("db: revoke refresh token: %w", err)
	}
	return nil
}

// RevokeAllUserRefreshTokens revokes every live token a user holds — the
// response to refresh-token reuse (possible theft) and to logout-everywhere.
func (s *Store) RevokeAllUserRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	if _, err := s.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now()
		 WHERE user_id = $1 AND revoked_at IS NULL`,
		userID); err != nil {
		return fmt.Errorf("db: revoke all refresh tokens: %w", err)
	}
	return nil
}
