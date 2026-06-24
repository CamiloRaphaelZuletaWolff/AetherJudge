package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/caezu/arena/backend/services/api-gateway/internal/db"
)

// ErrInvalidRefreshToken covers unknown, expired, and revoked tokens. As
// with access tokens, callers must not leak which case occurred.
var ErrInvalidRefreshToken = errors.New("auth: invalid refresh token")

// RefreshTokenStore is the persistence surface refresh rotation needs;
// db.Store satisfies it.
type RefreshTokenStore interface {
	CreateRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (db.RefreshToken, error)
	GetRefreshTokenByHash(ctx context.Context, tokenHash []byte) (db.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uuid.UUID, replacedBy *uuid.UUID) error
	RevokeAllUserRefreshTokens(ctx context.Context, userID uuid.UUID) error
}

// RefreshManager issues, rotates, and revokes refresh tokens.
//
// Tokens are 256-bit random values; only their SHA-256 lands in the
// database, so a database leak yields nothing usable. Every refresh rotates
// the token; presenting an already-rotated (revoked) token is treated as
// theft and revokes every token the user holds.
type RefreshManager struct {
	store RefreshTokenStore
	ttl   time.Duration
	log   *slog.Logger
}

// NewRefreshManager builds a manager with the given token TTL.
func NewRefreshManager(store RefreshTokenStore, ttl time.Duration, log *slog.Logger) *RefreshManager {
	return &RefreshManager{store: store, ttl: ttl, log: log}
}

// Issue creates a fresh refresh token for a user (login/signup).
func (m *RefreshManager) Issue(ctx context.Context, userID uuid.UUID, now time.Time) (string, error) {
	raw, hash, err := newToken()
	if err != nil {
		return "", err
	}
	if _, err := m.store.CreateRefreshToken(ctx, userID, hash, now.Add(m.ttl)); err != nil {
		return "", fmt.Errorf("auth: store refresh token: %w", err)
	}
	return raw, nil
}

// Rotate exchanges a valid refresh token for a new one, revoking the old.
// Reuse of a rotated token revokes the user's entire token family.
func (m *RefreshManager) Rotate(ctx context.Context, raw string, now time.Time) (newRaw string, userID uuid.UUID, err error) {
	current, err := m.lookup(ctx, raw)
	if err != nil {
		return "", uuid.Nil, err
	}

	if current.RevokedAt != nil {
		// This token was already rotated or revoked: someone is replaying
		// it. Assume theft and kill every session for this user.
		refreshReuseTotal.Inc()
		m.log.WarnContext(ctx, "refresh token reuse detected; revoking token family", "user_id", current.UserID)
		if err := m.store.RevokeAllUserRefreshTokens(ctx, current.UserID); err != nil {
			return "", uuid.Nil, fmt.Errorf("auth: revoke family after reuse: %w", err)
		}
		return "", uuid.Nil, ErrInvalidRefreshToken
	}
	if now.After(current.ExpiresAt) {
		return "", uuid.Nil, ErrInvalidRefreshToken
	}

	rawNew, hashNew, err := newToken()
	if err != nil {
		return "", uuid.Nil, err
	}
	next, err := m.store.CreateRefreshToken(ctx, current.UserID, hashNew, now.Add(m.ttl))
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("auth: store rotated token: %w", err)
	}
	if err := m.store.RevokeRefreshToken(ctx, current.ID, &next.ID); err != nil {
		return "", uuid.Nil, fmt.Errorf("auth: revoke rotated token: %w", err)
	}

	return rawNew, current.UserID, nil
}

// Revoke invalidates one token (logout). Unknown tokens are not an error —
// logout must be idempotent.
func (m *RefreshManager) Revoke(ctx context.Context, raw string) error {
	current, err := m.lookup(ctx, raw)
	if errors.Is(err, ErrInvalidRefreshToken) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := m.store.RevokeRefreshToken(ctx, current.ID, nil); err != nil {
		return fmt.Errorf("auth: revoke token: %w", err)
	}
	return nil
}

func (m *RefreshManager) lookup(ctx context.Context, raw string) (db.RefreshToken, error) {
	hash := hashToken(raw)
	current, err := m.store.GetRefreshTokenByHash(ctx, hash)
	if errors.Is(err, db.ErrNotFound) {
		return db.RefreshToken{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return db.RefreshToken{}, fmt.Errorf("auth: look up refresh token: %w", err)
	}
	return current, nil
}

func newToken() (raw string, hash []byte, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("auth: generate refresh token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
