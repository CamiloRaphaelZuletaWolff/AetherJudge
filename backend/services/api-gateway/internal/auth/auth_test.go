package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/caezu/arena/backend/services/api-gateway/internal/db"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestPasswordHashAndVerify(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("hash equals plaintext")
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Error("VerifyPassword rejected the correct password")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Error("VerifyPassword accepted a wrong password")
	}
}

func TestPasswordTooLong(t *testing.T) {
	t.Parallel()

	if _, err := HashPassword(strings.Repeat("a", 73)); !errors.Is(err, ErrPasswordTooLong) {
		t.Errorf("HashPassword(73 bytes) error = %v, want ErrPasswordTooLong", err)
	}
}

func TestJWTRoundtrip(t *testing.T) {
	t.Parallel()

	issuer := NewTokenIssuer("test-secret", 15*time.Minute)
	userID := uuid.New()
	now := time.Now()

	token, err := issuer.Mint(userID, "alice", now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	claims, err := issuer.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Username != "alice" {
		t.Errorf("Username = %q, want alice", claims.Username)
	}
	got, err := claims.UserID()
	if err != nil {
		t.Fatalf("UserID: %v", err)
	}
	if got != userID {
		t.Errorf("UserID = %v, want %v", got, userID)
	}
}

func TestJWTRejectsExpired(t *testing.T) {
	t.Parallel()

	issuer := NewTokenIssuer("test-secret", time.Minute)
	token, err := issuer.Mint(uuid.New(), "alice", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if _, err := issuer.Verify(token); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Verify(expired) error = %v, want ErrInvalidToken", err)
	}
}

func TestJWTRejectsTamperedAndForeign(t *testing.T) {
	t.Parallel()

	issuer := NewTokenIssuer("test-secret", time.Minute)
	token, err := issuer.Mint(uuid.New(), "alice", time.Now())
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	if _, err := issuer.Verify(token + "x"); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Verify(tampered) error = %v, want ErrInvalidToken", err)
	}

	other := NewTokenIssuer("different-secret", time.Minute)
	if _, err := other.Verify(token); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Verify(foreign secret) error = %v, want ErrInvalidToken", err)
	}
}

// fakeTokenStore is an in-memory RefreshTokenStore.
type fakeTokenStore struct {
	tokens map[string]db.RefreshToken // keyed by hash
}

func newFakeTokenStore() *fakeTokenStore {
	return &fakeTokenStore{tokens: map[string]db.RefreshToken{}}
}

func (f *fakeTokenStore) CreateRefreshToken(_ context.Context, userID uuid.UUID, hash []byte, expiresAt time.Time) (db.RefreshToken, error) {
	t := db.RefreshToken{ID: uuid.New(), UserID: userID, TokenHash: hash, ExpiresAt: expiresAt, CreatedAt: time.Now()}
	f.tokens[string(hash)] = t
	return t, nil
}

func (f *fakeTokenStore) GetRefreshTokenByHash(_ context.Context, hash []byte) (db.RefreshToken, error) {
	t, ok := f.tokens[string(hash)]
	if !ok {
		return db.RefreshToken{}, db.ErrNotFound
	}
	return t, nil
}

func (f *fakeTokenStore) RevokeRefreshToken(_ context.Context, id uuid.UUID, replacedBy *uuid.UUID) error {
	for k, t := range f.tokens {
		if t.ID == id && t.RevokedAt == nil {
			now := time.Now()
			t.RevokedAt, t.ReplacedBy = &now, replacedBy
			f.tokens[k] = t
		}
	}
	return nil
}

func (f *fakeTokenStore) RevokeAllUserRefreshTokens(_ context.Context, userID uuid.UUID) error {
	for k, t := range f.tokens {
		if t.UserID == userID && t.RevokedAt == nil {
			now := time.Now()
			t.RevokedAt = &now
			f.tokens[k] = t
		}
	}
	return nil
}

func (f *fakeTokenStore) live(userID uuid.UUID) int {
	n := 0
	for _, t := range f.tokens {
		if t.UserID == userID && t.RevokedAt == nil {
			n++
		}
	}
	return n
}

func TestRefreshRotation(t *testing.T) {
	t.Parallel()

	store := newFakeTokenStore()
	mgr := NewRefreshManager(store, time.Hour, discardLogger())
	userID := uuid.New()
	ctx := context.Background()
	now := time.Now()

	raw1, err := mgr.Issue(ctx, userID, now)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	raw2, gotUser, err := mgr.Rotate(ctx, raw1, now)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if gotUser != userID {
		t.Errorf("Rotate user = %v, want %v", gotUser, userID)
	}
	if raw2 == raw1 {
		t.Error("rotation returned the same token")
	}
	if store.live(userID) != 1 {
		t.Errorf("live tokens = %d, want 1 (old revoked, new live)", store.live(userID))
	}
}

func TestRefreshReuseRevokesFamily(t *testing.T) {
	t.Parallel()

	store := newFakeTokenStore()
	mgr := NewRefreshManager(store, time.Hour, discardLogger())
	userID := uuid.New()
	ctx := context.Background()
	now := time.Now()

	raw1, err := mgr.Issue(ctx, userID, now)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, _, err := mgr.Rotate(ctx, raw1, now); err != nil {
		t.Fatalf("first Rotate: %v", err)
	}

	// Replay the consumed token: must fail AND nuke every live token.
	if _, _, err := mgr.Rotate(ctx, raw1, now); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("replayed Rotate error = %v, want ErrInvalidRefreshToken", err)
	}
	if store.live(userID) != 0 {
		t.Errorf("live tokens after reuse = %d, want 0 (family revoked)", store.live(userID))
	}
}

func TestRefreshRejectsExpiredAndUnknown(t *testing.T) {
	t.Parallel()

	store := newFakeTokenStore()
	mgr := NewRefreshManager(store, time.Hour, discardLogger())
	ctx := context.Background()
	now := time.Now()

	raw, err := mgr.Issue(ctx, uuid.New(), now.Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, _, err := mgr.Rotate(ctx, raw, now); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Errorf("Rotate(expired) error = %v, want ErrInvalidRefreshToken", err)
	}

	if _, _, err := mgr.Rotate(ctx, "never-issued", now); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Errorf("Rotate(unknown) error = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestRevokeIsIdempotent(t *testing.T) {
	t.Parallel()

	store := newFakeTokenStore()
	mgr := NewRefreshManager(store, time.Hour, discardLogger())
	ctx := context.Background()

	raw, err := mgr.Issue(ctx, uuid.New(), time.Now())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if err := mgr.Revoke(ctx, raw); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if err := mgr.Revoke(ctx, "unknown-token"); err != nil {
		t.Errorf("Revoke(unknown) = %v, want nil (idempotent logout)", err)
	}
}

func TestTokensAreHashedAtRest(t *testing.T) {
	t.Parallel()

	store := newFakeTokenStore()
	mgr := NewRefreshManager(store, time.Hour, discardLogger())

	raw, err := mgr.Issue(context.Background(), uuid.New(), time.Now())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	sum := sha256.Sum256([]byte(raw))
	if _, ok := store.tokens[string(sum[:])]; !ok {
		t.Error("stored key is not the SHA-256 of the raw token")
	}
	for k := range store.tokens {
		if k == raw {
			t.Error("raw token stored at rest")
		}
	}
}
