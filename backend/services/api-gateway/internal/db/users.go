package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const userColumns = "id, username, email, password_hash, created_at"

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

// CreateUser inserts a new account. Duplicate usernames/emails map to
// ErrUsernameTaken / ErrEmailTaken so handlers can answer precisely.
func (s *Store) CreateUser(ctx context.Context, username, email, passwordHash string) (User, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO users (username, email, password_hash)
		 VALUES ($1, $2, $3)
		 RETURNING `+userColumns,
		username, email, passwordHash)

	u, err := scanUser(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case "users_username_key":
				return User{}, ErrUsernameTaken
			case "users_email_key":
				return User{}, ErrEmailTaken
			}
		}
		return User{}, fmt.Errorf("db: create user: %w", err)
	}
	return u, nil
}

// GetUserByID fetches one user or ErrNotFound.
func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("db: get user by id: %w", err)
	}
	return u, nil
}

// GetUserByLogin resolves a username OR email (both citext, so the match is
// case-insensitive) to a user, or ErrNotFound.
func (s *Store) GetUserByLogin(ctx context.Context, login string) (User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE username = $1 OR email = $1`, login))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("db: get user by login: %w", err)
	}
	return u, nil
}
