package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const userColumns = "id, username, email, password_hash, role, created_at"

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
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

// ListUsers returns every account oldest-first — the admin user-management
// read (RBAC user.manage; see ADR-0014). The user table is small at this
// scale, so no pagination.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("db: list users: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("db: scan user: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list users rows: %w", err)
	}
	return out, nil
}

// UpdateUserRole sets a user's RBAC role and returns the updated row, or
// ErrNotFound if no such user. Callers validate the role string first
// (auth.IsValidRole); the column CHECK is the final backstop.
func (s *Store) UpdateUserRole(ctx context.Context, id uuid.UUID, role string) (User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx,
		`UPDATE users SET role = $2 WHERE id = $1 RETURNING `+userColumns, id, role))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("db: update user role: %w", err)
	}
	return u, nil
}
