package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/caezu/arena/backend/services/api-gateway/internal/auth"
	"github.com/caezu/arena/backend/services/api-gateway/internal/db"
)

// refreshCookieName holds the rotated opaque refresh token, httpOnly and
// path-scoped so it only ever rides auth requests.
const refreshCookieName = "arena_refresh"

const refreshCookiePath = "/api/v1/auth"

// dummyBcryptHash is compared against when a login names an unknown user, so
// the response time doesn't reveal whether the account exists.
const dummyBcryptHash = "$2a$12$R9h/cIPz0gi.URNNX3kh2OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW"

type userDTO struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserDTO(u db.User) userDTO {
	return userDTO{
		ID:        u.ID.String(),
		Username:  u.Username,
		Email:     u.Email,
		Role:      string(auth.ParseRole(u.Role)),
		CreatedAt: u.CreatedAt,
	}
}

type authResponse struct {
	User        userDTO `json:"user"`
	AccessToken string  `json:"access_token"`
}

func (s *server) signup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, maxAuthBodyBytes, &req); err != nil {
		var reqErr *requestError
		if errors.As(err, &reqErr) {
			respondRequestError(w, s.log, reqErr)
			return
		}
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "invalid request")
		return
	}
	for _, v := range []*requestError{
		validateUsername(req.Username),
		validateEmail(req.Email),
		validatePassword(req.Password),
	} {
		if v != nil {
			respondRequestError(w, s.log, v)
			return
		}
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.log.Error("hash password", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not create account")
		return
	}

	user, err := s.store.CreateUser(r.Context(), req.Username, req.Email, hash)
	switch {
	case errors.Is(err, db.ErrUsernameTaken):
		respondError(w, s.log, http.StatusConflict, "username_taken", "that username is already taken")
		return
	case errors.Is(err, db.ErrEmailTaken):
		respondError(w, s.log, http.StatusConflict, "email_taken", "that email is already registered")
		return
	case err != nil:
		s.log.Error("create user", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not create account")
		return
	}

	s.issueSession(w, r, user, http.StatusCreated)
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"` // username or email
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, maxAuthBodyBytes, &req); err != nil {
		var reqErr *requestError
		if errors.As(err, &reqErr) {
			respondRequestError(w, s.log, reqErr)
			return
		}
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "invalid request")
		return
	}

	user, err := s.store.GetUserByLogin(r.Context(), req.Login)
	if errors.Is(err, db.ErrNotFound) {
		// Burn comparable time so unknown-user and wrong-password responses
		// are indistinguishable.
		auth.VerifyPassword(dummyBcryptHash, req.Password)
		respondError(w, s.log, http.StatusUnauthorized, "invalid_credentials", "invalid login or password")
		return
	}
	if err != nil {
		s.log.Error("load user for login", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not log in")
		return
	}

	if !auth.VerifyPassword(user.PasswordHash, req.Password) {
		respondError(w, s.log, http.StatusUnauthorized, "invalid_credentials", "invalid login or password")
		return
	}

	s.issueSession(w, r, user, http.StatusOK)
}

// issueSession mints both tokens and writes the auth response.
func (s *server) issueSession(w http.ResponseWriter, r *http.Request, user db.User, status int) {
	now := time.Now()

	access, err := s.tokens.Mint(user.ID, user.Username, user.Role, now)
	if err != nil {
		s.log.Error("mint access token", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not issue session")
		return
	}
	refresh, err := s.refresh.Issue(r.Context(), user.ID, now)
	if err != nil {
		s.log.Error("issue refresh token", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not issue session")
		return
	}

	s.setRefreshCookie(w, refresh)
	respondJSON(w, s.log, status, authResponse{User: toUserDTO(user), AccessToken: access})
}

func (s *server) refreshToken(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		respondError(w, s.log, http.StatusUnauthorized, "unauthorized", "missing refresh token")
		return
	}

	now := time.Now()
	newRefresh, userID, err := s.refresh.Rotate(r.Context(), cookie.Value, now)
	if errors.Is(err, auth.ErrInvalidRefreshToken) {
		s.clearRefreshCookie(w)
		respondError(w, s.log, http.StatusUnauthorized, "unauthorized", "invalid refresh token")
		return
	}
	if err != nil {
		s.log.Error("rotate refresh token", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not refresh session")
		return
	}

	// Re-read the user so the re-minted token reflects the CURRENT role: this
	// is the propagation path for a role change (bounded by the access-token
	// TTL). See ADR-0014.
	user, err := s.store.GetUserByID(r.Context(), userID)
	if err != nil {
		s.log.Error("load user for refresh", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not refresh session")
		return
	}

	access, err := s.tokens.Mint(user.ID, user.Username, user.Role, now)
	if err != nil {
		s.log.Error("mint access token", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not refresh session")
		return
	}

	s.setRefreshCookie(w, newRefresh)
	respondJSON(w, s.log, http.StatusOK, map[string]string{"access_token": access})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(refreshCookieName); err == nil {
		if err := s.refresh.Revoke(r.Context(), cookie.Value); err != nil {
			s.log.Error("revoke refresh token", "error", err)
		}
	}
	s.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	userInfo, ok := UserFrom(r.Context())
	if !ok {
		respondError(w, s.log, http.StatusUnauthorized, "unauthorized", "missing access token")
		return
	}
	user, err := s.store.GetUserByID(r.Context(), userInfo.ID)
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, s.log, http.StatusUnauthorized, "unauthorized", "account no longer exists")
		return
	}
	if err != nil {
		s.log.Error("load current user", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not load profile")
		return
	}
	respondJSON(w, s.log, http.StatusOK, toUserDTO(user))
}

func (s *server) setRefreshCookie(w http.ResponseWriter, token string) {
	//nolint:gosec // Secure is enabled in production mode; local dev runs on plain HTTP.
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     refreshCookiePath,
		MaxAge:   int(s.cfg.RefreshTokenTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.cfg.IsProduction(),
		SameSite: s.refreshCookieSameSite(),
	})
}

// refreshCookieSameSite selects the SameSite policy for the refresh cookie.
// In production the SPA is served from a different site than the API (e.g. a
// separate frontend host), so the cookie must be SameSite=None to ride those
// cross-site requests; None requires Secure, which production also sets. Local
// dev is same-origin over plain HTTP, where Lax is correct (browsers reject
// SameSite=None without Secure).
func (s *server) refreshCookieSameSite() http.SameSite {
	if s.cfg.IsProduction() {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

func (s *server) clearRefreshCookie(w http.ResponseWriter) {
	//nolint:gosec // Secure is enabled in production mode; local dev runs on plain HTTP.
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.IsProduction(),
		SameSite: s.refreshCookieSameSite(),
	})
}
