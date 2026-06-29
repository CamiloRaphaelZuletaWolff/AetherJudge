package integration

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func (s *stack) postStatus(t *testing.T, path, token string, body any) int {
	t.Helper()
	resp := s.post(t, path, token, body)
	if err := resp.Body.Close(); err != nil {
		t.Errorf("close body: %v", err)
	}
	return resp.StatusCode
}

// TestAdminContentAuthoring is the ADR-0015 acceptance test: an admin creates a
// contest, adds a problem and test cases over the API, and a competitor's
// submission to that AUTHORED problem judges accepted — proving authored
// content is immediately functional. It also checks the authz and validation
// guardrails (403 for non-admin, 409 on duplicate slug, 400 on bad limits).
func TestAdminContentAuthoring(t *testing.T) {
	st := newStack(t)
	ctx := context.Background()

	// Promote a fresh account to admin and re-login so the token carries it.
	adminName := uniqueName("author")
	admin := st.signup(t, adminName)
	if _, err := st.store.UpdateUserRole(ctx, mustUUID(t, admin.User.ID), "admin"); err != nil {
		t.Fatalf("promote admin: %v", err)
	}
	adminTok := st.login(t, adminName)

	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	futureEnd := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)

	// A plain user cannot create a contest.
	plain := st.signup(t, uniqueName("plain"))
	if code := st.postStatus(t, "/api/v1/admin/contests", plain.AccessToken, map[string]any{
		"title": "Nope", "starts_at": future, "ends_at": futureEnd,
	}); code != http.StatusForbidden {
		t.Errorf("plain user create contest = %d, want 403", code)
	}

	// Admin creates an ACTIVE contest (so a submission is accepted immediately).
	slug := uniqueSlug("authored")
	activeStart := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	activeEnd := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	contest := decodeBody[struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	}](t, st.post(t, "/api/v1/admin/contests", adminTok, map[string]any{
		"title": "Authored Contest", "slug": slug,
		"description": "created via the admin API", "starts_at": activeStart, "ends_at": activeEnd,
	}))
	if contest.ID == "" || contest.Slug != slug {
		t.Fatalf("create contest returned %+v", contest)
	}

	// Re-using the slug is a 409.
	if code := st.postStatus(t, "/api/v1/admin/contests", adminTok, map[string]any{
		"title": "Dup", "slug": slug, "starts_at": activeStart, "ends_at": activeEnd,
	}); code != http.StatusConflict {
		t.Errorf("duplicate slug = %d, want 409", code)
	}

	// A problem with an out-of-range time limit is rejected.
	if code := st.postStatus(t, "/api/v1/admin/contests/"+contest.ID+"/problems", adminTok, map[string]any{
		"title": "Bad", "statement_md": "x", "time_limit_ms": 50, "memory_limit_mb": 128,
	}); code != http.StatusBadRequest {
		t.Errorf("bad time limit = %d, want 400", code)
	}

	// Add a real problem (ord auto-assigned to 1).
	problem := decodeBody[struct {
		ID  string `json:"id"`
		Ord int    `json:"ord"`
	}](t, st.post(t, "/api/v1/admin/contests/"+contest.ID+"/problems", adminTok, map[string]any{
		"title": "Echo", "statement_md": "Print the input.", "time_limit_ms": 2000, "memory_limit_mb": 128,
	}))
	if problem.ID == "" || problem.Ord != 1 {
		t.Fatalf("create problem returned %+v", problem)
	}

	// Attach test cases in a batch.
	if code := st.postStatus(t, "/api/v1/admin/problems/"+problem.ID+"/test-cases", adminTok, map[string]any{
		"cases": []map[string]string{
			{"stdin": "x", "expected_output": "x"},
			{"stdin": "y", "expected_output": "y"},
		},
	}); code != http.StatusCreated {
		t.Errorf("create test cases = %d, want 201", code)
	}

	// The admin problems list reflects the problem and its test-case count.
	probs := decodeBody[struct {
		Problems []struct {
			ID            string `json:"id"`
			TestCaseCount int    `json:"test_case_count"`
		} `json:"problems"`
	}](t, st.get(t, "/api/v1/admin/contests/"+contest.ID+"/problems", adminTok))
	if len(probs.Problems) != 1 || probs.Problems[0].TestCaseCount != 2 {
		t.Fatalf("admin problems = %+v, want one problem with 2 test cases", probs.Problems)
	}

	// End to end: a competitor submits to the authored problem and it judges
	// accepted (the stub executor echoes each test case's expected output).
	solver := st.signup(t, uniqueName("solver"))
	if _, verdict := st.submitAndAwait(t, mustUUID(t, contest.ID), solver.AccessToken); verdict != "accepted" {
		t.Errorf("authored-problem verdict = %q, want accepted", verdict)
	}
}
