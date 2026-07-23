package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
)

const sessionTTL = 12 * time.Hour

const (
	defaultTokenIssuer   = "velox-apigateway"
	defaultTokenAudience = "velox-api"
	minPasswordLength    = 8
	maxPasswordLength    = 128
)

// scopesForRole returns token scopes for ingress checks; authorization still
// uses the store-backed User loaded by authenticate.
func scopesForRole(role string) string {
	switch role {
	case RoleOrganizer:
		return "organizer:read organizer:write reservations:write orders:read"
	default:
		return "reservations:write orders:read"
	}
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	email := normalizeEmail(req.Email)
	role, ok := normalizedRegistrationRole(req.Role)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_role")
		return
	}
	if len(req.Password) < minPasswordLength || len(req.Password) > maxPasswordLength {
		writeError(w, http.StatusBadRequest, "invalid_password")
		return
	}

	hash, err := argon2id.CreateHash(req.Password, argon2id.DefaultParams)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "password_hashing_failed")
		return
	}

	id := uuid.NewString()
	var user User
	if s.store != nil {
		user, err = s.store.CreateUser(r.Context(), id, email, hash, role)
		if err != nil {
			writeError(w, http.StatusConflict, "user_already_exists")
			return
		}
	} else {
		s.mu.Lock()
		if _, ok := s.users[email]; ok {
			s.mu.Unlock()
			writeError(w, http.StatusConflict, "user_already_exists")
			return
		}
		user = User{ID: id, Email: email, Password: hash, Role: role}
		if role == RoleOrganizer {
			user.OrganizerID = id
		}
		s.users[email] = user
		s.mu.Unlock()
	}

	token, err := s.sign(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_signing_failed")
		return
	}

	s.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]any{"user": publicUser(user)})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	email := normalizeEmail(req.Email)
	attemptKey := email + "|" + clientIP(r)

	s.mu.Lock()
	now := s.now()
	if failure := s.loginFails[attemptKey]; failure.LockedUntil.After(now) {
		s.mu.Unlock()
		writeError(w, http.StatusTooManyRequests, "too_many_login_attempts")
		return
	}
	s.mu.Unlock()

	var user User
	var err error
	if s.store != nil {
		user, err = s.store.GetUserByEmail(r.Context(), email)
	} else {
		s.mu.Lock()
		u, ok := s.users[email]
		s.mu.Unlock()
		if ok {
			user = u
		} else {
			err = errors.New("not found")
		}
	}

	if err != nil {
		s.recordLoginFailure(attemptKey, now)
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}

	var match bool
	if s.store == nil && user.Password == req.Password {
		match = true
	} else {
		match, err = argon2id.ComparePasswordAndHash(req.Password, user.Password)
	}

	if err != nil || !match {
		s.recordLoginFailure(attemptKey, now)
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}

	s.clearLoginFailure(attemptKey)

	token, err := s.sign(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_signing_failed")
		return
	}

	s.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]any{"user": publicUser(user)})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, err := s.authenticate(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication_required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": publicUser(user)})
}

func (s *Server) recordLoginFailure(key string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	failure := s.loginFails[key]
	failure.Count++
	if failure.Count >= 5 {
		failure.LockedUntil = now.Add(5 * time.Minute)
	}
	s.loginFails[key] = failure
}

func (s *Server) clearLoginFailure(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.loginFails, key)
}

func (s *Server) requireAuth(next func(http.ResponseWriter, *http.Request, User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := s.authenticate(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication_required")
			return
		}
		next(w, r, user)
	}
}

func (s *Server) requireRole(role string, next func(http.ResponseWriter, *http.Request, User)) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request, user User) {
		if user.Role != role {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r, user)
	})
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizedRegistrationRole(role string) (string, bool) {
	switch role {
	case "", RoleReserver:
		return RoleReserver, true
	case RoleOrganizer:
		return RoleOrganizer, true
	default:
		return "", false
	}
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  s.now().Add(sessionTTL),
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (s *Server) authenticate(r *http.Request) (User, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return User{}, err
	}
	userID, err := s.verify(cookie.Value)
	if err != nil {
		return User{}, err
	}
	if s.store != nil {
		return s.store.GetUserByID(r.Context(), userID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.ID == userID {
			return u, nil
		}
	}
	return User{}, errors.New("not found")
}

func (s *Server) sign(user User) (string, error) {
	return signHMAC(s.secret, map[string]any{
		"sub":   user.ID,
		"role":  user.Role,
		"exp":   s.now().Add(sessionTTL).Unix(),
		"iss":   s.tokenIssuer,
		"aud":   s.tokenAudience,
		"scope": scopesForRole(user.Role),
	})
}

func (s *Server) verify(token string) (string, error) {
	payload, err := verifyHMAC(s.secret, token)
	if err != nil {
		return "", err
	}
	sub, _ := payload["sub"].(string)
	exp, _ := payload["exp"].(float64)
	iss, _ := payload["iss"].(string)
	aud, _ := payload["aud"].(string)
	scope, _ := payload["scope"].(string)

	if int64(exp) <= s.now().Unix() {
		return "", errors.New("expired token")
	}
	if iss != s.tokenIssuer || aud != s.tokenAudience {
		return "", errors.New("bad issuer or audience")
	}
	if scope == "" {
		return "", errors.New("missing scope")
	}
	return sub, nil
}
