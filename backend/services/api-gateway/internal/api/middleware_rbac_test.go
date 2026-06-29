package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/caezu/arena/backend/services/api-gateway/internal/auth"
)

func TestRequirePermission(t *testing.T) {
	t.Parallel()

	s := &server{log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	tests := []struct {
		name       string
		role       auth.Role
		perm       auth.Permission
		withUser   bool
		wantStatus int
		wantNext   bool
	}{
		{"admin reaches user.manage", auth.RoleAdmin, auth.PermUserManage, true, http.StatusOK, true},
		{"moderator denied user.manage", auth.RoleModerator, auth.PermUserManage, true, http.StatusForbidden, false},
		{"moderator reaches submission.viewAll", auth.RoleModerator, auth.PermSubmissionViewAll, true, http.StatusOK, true},
		{"user denied submission.viewAll", auth.RoleUser, auth.PermSubmissionViewAll, true, http.StatusForbidden, false},
		{"missing user is unauthorized", auth.RoleAdmin, auth.PermUserManage, false, http.StatusUnauthorized, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			nextCalled := false
			h := s.requirePermission(tt.perm, func(w http.ResponseWriter, _ *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
			if tt.withUser {
				req = req.WithContext(context.WithValue(req.Context(), userCtxKey,
					UserInfo{ID: uuid.New(), Username: "x", Role: tt.role}))
			}
			rec := httptest.NewRecorder()
			h(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if nextCalled != tt.wantNext {
				t.Errorf("next called = %v, want %v", nextCalled, tt.wantNext)
			}
		})
	}
}
