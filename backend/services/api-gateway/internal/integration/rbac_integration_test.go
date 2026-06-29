package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// --- helpers --------------------------------------------------------------

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", s, err)
	}
	return id
}

// login authenticates an existing demo-password account and returns a fresh
// access token (which therefore carries the account's CURRENT role).
func (s *stack) login(t *testing.T, username string) string {
	t.Helper()
	resp := s.post(t, "/api/v1/auth/login", "", map[string]string{
		"login":    username,
		"password": "password123",
	})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("login %s status = %d, body %s", username, resp.StatusCode, body)
	}
	return decodeBody[authResp](t, resp).AccessToken
}

func (s *stack) patch(t *testing.T, path, token string, body any) *http.Response {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPatch, s.server.URL+path, bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.server.Client().Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", path, err)
	}
	return resp
}

// getStatus / patchStatus issue a request and return only the status code,
// closing the body — for the many "this caller is (un)authorized" assertions.
func (s *stack) getStatus(t *testing.T, path, token string) int {
	t.Helper()
	resp := s.get(t, path, token)
	if err := resp.Body.Close(); err != nil {
		t.Errorf("close body: %v", err)
	}
	return resp.StatusCode
}

func (s *stack) patchStatus(t *testing.T, path, token string, body any) int {
	t.Helper()
	resp := s.patch(t, path, token, body)
	if err := resp.Body.Close(); err != nil {
		t.Errorf("close body: %v", err)
	}
	return resp.StatusCode
}

// --- tests ----------------------------------------------------------------

// TestRBACUserManagement is the admin-tier acceptance test: only an admin can
// list users and change roles, the change persists to the source of truth, and
// the anti-lockout / validation guardrails hold. (ADR-0014)
func TestRBACUserManagement(t *testing.T) {
	st := newStack(t)
	ctx := context.Background()

	// Promote a fresh account to admin, then log in so its token carries admin.
	adminName := uniqueName("rbacadmin")
	admin := st.signup(t, adminName)
	if _, err := st.store.UpdateUserRole(ctx, mustUUID(t, admin.User.ID), "admin"); err != nil {
		t.Fatalf("promote admin: %v", err)
	}
	adminTok := st.login(t, adminName)

	memberName := uniqueName("rbacmember")
	member := st.signup(t, memberName)

	// A non-admin, and an anonymous caller, are both refused user management.
	if got := st.getStatus(t, "/api/v1/admin/users", member.AccessToken); got != http.StatusForbidden {
		t.Errorf("member GET /admin/users = %d, want 403", got)
	}
	if got := st.getStatus(t, "/api/v1/admin/users", ""); got != http.StatusUnauthorized {
		t.Errorf("anon GET /admin/users = %d, want 401", got)
	}

	// Admin lists users; the member appears with role "user".
	list := decodeBody[struct {
		Users []struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"users"`
	}](t, st.get(t, "/api/v1/admin/users", adminTok))
	foundMember := false
	for _, u := range list.Users {
		if u.ID == member.User.ID {
			foundMember = true
			if u.Role != "user" {
				t.Errorf("member role = %q, want user", u.Role)
			}
		}
	}
	if !foundMember {
		t.Error("admin user list did not include the member")
	}

	// Admin promotes the member to moderator; the response reflects it.
	promote := decodeBody[struct {
		Role string `json:"role"`
	}](t, st.patch(t, "/api/v1/admin/users/"+member.User.ID+"/role", adminTok,
		map[string]string{"role": "moderator"}))
	if promote.Role != "moderator" {
		t.Errorf("promoted role = %q, want moderator", promote.Role)
	}

	// And it persisted in the source of truth.
	got, err := st.store.GetUserByID(ctx, mustUUID(t, member.User.ID))
	if err != nil {
		t.Fatalf("reload member: %v", err)
	}
	if got.Role != "moderator" {
		t.Errorf("DB role = %q, want moderator", got.Role)
	}

	// Guardrails: self-role-change is refused, and a bogus role is rejected.
	if code := st.patchStatus(t, "/api/v1/admin/users/"+admin.User.ID+"/role", adminTok,
		map[string]string{"role": "user"}); code != http.StatusConflict {
		t.Errorf("self role change = %d, want 409", code)
	}
	if code := st.patchStatus(t, "/api/v1/admin/users/"+member.User.ID+"/role", adminTok,
		map[string]string{"role": "superuser"}); code != http.StatusBadRequest {
		t.Errorf("invalid role = %d, want 400", code)
	}

	// A moderator (now that the member is one) still cannot manage users —
	// moderator < admin. The new login carries the freshly-granted role.
	modTok := st.login(t, memberName)
	if code := st.getStatus(t, "/api/v1/admin/users", modTok); code != http.StatusForbidden {
		t.Errorf("moderator GET /admin/users = %d, want 403", code)
	}
}

// TestRBACModeratorViewsAllSubmissions is the moderator-tier acceptance test:
// a plain user is forbidden from the all-submissions view, but a moderator
// sees every submission in the contest (submission.viewAll). (ADR-0014)
func TestRBACModeratorViewsAllSubmissions(t *testing.T) {
	st := newStack(t)
	ctx := context.Background()

	contestID := st.setupContest(t, "rbacsub")

	// A solver lands one accepted submission.
	solverName := uniqueName("rbacsolver")
	solver := st.signup(t, solverName)
	if _, verdict := st.submitAndAwait(t, contestID, solver.AccessToken); verdict != "accepted" {
		t.Fatalf("solver verdict = %q, want accepted", verdict)
	}

	allPath := "/api/v1/admin/contests/" + contestID.String() + "/submissions"

	// A plain user cannot view all submissions.
	plainName := uniqueName("rbacplain")
	plain := st.signup(t, plainName)
	if code := st.getStatus(t, allPath, plain.AccessToken); code != http.StatusForbidden {
		t.Errorf("plain user all-submissions = %d, want 403", code)
	}

	// Promote that user to moderator and re-login so the token carries it.
	if _, err := st.store.UpdateUserRole(ctx, mustUUID(t, plain.User.ID), "moderator"); err != nil {
		t.Fatalf("promote moderator: %v", err)
	}
	modTok := st.login(t, plainName)

	resp := st.get(t, allPath, modTok)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("moderator all-submissions = %d, want 200", resp.StatusCode)
	}
	all := decodeBody[struct {
		Submissions []struct {
			Username string `json:"username"`
			Verdict  string `json:"verdict"`
		} `json:"submissions"`
	}](t, resp)
	if len(all.Submissions) == 0 {
		t.Fatal("moderator saw no submissions, want >= 1")
	}
	sawSolver := false
	for _, sub := range all.Submissions {
		if sub.Username == solverName {
			sawSolver = true
		}
	}
	if !sawSolver {
		t.Errorf("all-submissions did not include solver %q", solverName)
	}
}
