package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
)

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	email := strings.ToLower(req.Email)
	req.Role = "user"

	hash, err := argon2id.CreateHash(req.Password, argon2id.DefaultParams)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "password_hashing_failed")
		return
	}

	id := uuid.NewString()
	var user User
	if s.store != nil {
		user, err = s.store.CreateUser(r.Context(), id, email, hash, req.Role)
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
		user = User{ID: id, Email: email, Password: hash, Role: req.Role}
		s.users[email] = user
		s.mu.Unlock()
	}

	token, err := s.sign(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_signing_failed")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  s.now().Add(12 * time.Hour),
	})
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
	email := strings.ToLower(req.Email)
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

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  s.now().Add(12 * time.Hour),
	})
	writeJSON(w, http.StatusOK, map[string]any{"user": publicUser(user)})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
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
	payload, err := json.Marshal(map[string]any{"sub": user.ID, "role": user.Role, "exp": s.now().Add(12 * time.Hour).Unix()})
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Server) verify(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", errors.New("bad token")
	}
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expected, actual) {
		return "", errors.New("bad signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	var payload struct {
		Sub string `json:"sub"`
		Exp int64  `json:"exp"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", err
	}
	if payload.Exp <= s.now().Unix() {
		return "", errors.New("expired token")
	}
	return payload.Sub, nil
}
