-- +goose Up

-- RBAC: every account carries a role. New accounts default to the least
-- privilege ('user'); the CHECK keeps this column in lockstep with the role
-- set enforced in code (internal/auth/rbac.go) and mirrored on the frontend
-- (frontend/src/lib/rbac.ts). See ADR-0014.
ALTER TABLE users
    ADD COLUMN role text NOT NULL DEFAULT 'user'
    CHECK (role IN ('user', 'moderator', 'admin'));

-- +goose Down
ALTER TABLE users DROP COLUMN role;
