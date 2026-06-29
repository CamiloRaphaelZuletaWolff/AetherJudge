package auth

// RBAC is the canonical authorization model. It is mirrored exactly on the
// frontend (frontend/src/lib/rbac.ts) — same role names, same permission
// strings, same precedence — so the two cannot drift. Frontend gating is a UX
// convenience only; THIS package is the real boundary, enforced server-side by
// the requirePermission middleware. See ADR-0014.

// Role is an account's privilege level. Ordered user < moderator < admin.
type Role string

// The known roles, lowest to highest privilege.
const (
	RoleUser      Role = "user"
	RoleModerator Role = "moderator"
	RoleAdmin     Role = "admin"
)

// roleRank gives the "at least this role" precedence.
var roleRank = map[Role]int{
	RoleUser:      0,
	RoleModerator: 1,
	RoleAdmin:     2,
}

// Permission is a single gated capability. Middleware and handlers reference
// these constants, never raw role strings.
type Permission string

// The gated capabilities, mirrored in frontend/src/lib/rbac.ts.
const (
	PermContestCreate     Permission = "contest.create"
	PermContestEdit       Permission = "contest.edit"
	PermProblemManage     Permission = "problem.manage"
	PermSubmissionViewAll Permission = "submission.viewAll"
	PermUserManage        Permission = "user.manage"
	PermAdminAccess       Permission = "admin.access"
)

// rolePermissions grants capabilities per role. Each set is listed in full
// (matching the frontend) rather than inherited up the rank, so every grant is
// explicit and auditable.
var rolePermissions = map[Role][]Permission{
	RoleUser: {},
	RoleModerator: {
		PermContestCreate,
		PermContestEdit,
		PermProblemManage,
		PermSubmissionViewAll,
	},
	RoleAdmin: {
		PermContestCreate,
		PermContestEdit,
		PermProblemManage,
		PermSubmissionViewAll,
		PermUserManage,
		PermAdminAccess,
	},
}

// ParseRole maps a stored/claimed role string to a Role, falling back to
// RoleUser for anything unrecognized — defense in depth, so an unknown value
// grants the least privilege rather than failing open.
func ParseRole(s string) Role {
	switch Role(s) {
	case RoleModerator:
		return RoleModerator
	case RoleAdmin:
		return RoleAdmin
	default:
		return RoleUser
	}
}

// IsValidRole reports whether s is exactly one of the known roles, with no
// fallback — used to reject bad input on a role-change request.
func IsValidRole(s string) bool {
	switch Role(s) {
	case RoleUser, RoleModerator, RoleAdmin:
		return true
	default:
		return false
	}
}

// Can reports whether the role holds the given permission.
func Can(role Role, perm Permission) bool {
	for _, p := range rolePermissions[ParseRole(string(role))] {
		if p == perm {
			return true
		}
	}
	return false
}

// AtLeast reports whether role ranks at or above minRole.
func AtLeast(role, minRole Role) bool {
	return roleRank[ParseRole(string(role))] >= roleRank[ParseRole(string(minRole))]
}
