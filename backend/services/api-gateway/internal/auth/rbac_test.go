package auth

import "testing"

func TestCan(t *testing.T) {
	t.Parallel()
	tests := []struct {
		role Role
		perm Permission
		want bool
	}{
		{RoleUser, PermAdminAccess, false},
		{RoleUser, PermSubmissionViewAll, false},
		{RoleModerator, PermSubmissionViewAll, true},
		{RoleModerator, PermProblemManage, true},
		{RoleModerator, PermUserManage, false},
		{RoleModerator, PermAdminAccess, false},
		{RoleAdmin, PermUserManage, true},
		{RoleAdmin, PermAdminAccess, true},
		{RoleAdmin, PermSubmissionViewAll, true},
		{"bogus", PermSubmissionViewAll, false}, // unknown role → least privilege
	}
	for _, tt := range tests {
		if got := Can(tt.role, tt.perm); got != tt.want {
			t.Errorf("Can(%q, %q) = %v, want %v", tt.role, tt.perm, got, tt.want)
		}
	}
}

func TestAtLeast(t *testing.T) {
	t.Parallel()
	tests := []struct {
		role, minRole Role
		want          bool
	}{
		{RoleUser, RoleUser, true},
		{RoleUser, RoleModerator, false},
		{RoleModerator, RoleUser, true},
		{RoleModerator, RoleAdmin, false},
		{RoleAdmin, RoleModerator, true},
		{RoleAdmin, RoleAdmin, true},
		{"bogus", RoleUser, true}, // unknown role ranks as user
		{"bogus", RoleModerator, false},
	}
	for _, tt := range tests {
		if got := AtLeast(tt.role, tt.minRole); got != tt.want {
			t.Errorf("AtLeast(%q, %q) = %v, want %v", tt.role, tt.minRole, got, tt.want)
		}
	}
}

func TestParseRoleAndValidity(t *testing.T) {
	t.Parallel()
	parse := map[string]Role{
		"user":      RoleUser,
		"moderator": RoleModerator,
		"admin":     RoleAdmin,
		"":          RoleUser,
		"root":      RoleUser,
	}
	for in, want := range parse {
		if got := ParseRole(in); got != want {
			t.Errorf("ParseRole(%q) = %q, want %q", in, got, want)
		}
	}

	valid := map[string]bool{
		"user":      true,
		"moderator": true,
		"admin":     true,
		"":          false,
		"superuser": false,
	}
	for in, want := range valid {
		if got := IsValidRole(in); got != want {
			t.Errorf("IsValidRole(%q) = %v, want %v", in, got, want)
		}
	}
}
