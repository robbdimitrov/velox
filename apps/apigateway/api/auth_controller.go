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
)

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
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
	user, ok := s.users[email]
	s.mu.Unlock()
	if !ok || !constantTimeStringEqual(user.Password, req.Password) {
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
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: s.now().Add(12 * time.Hour)})
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

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) requireRole(role string, next func(http.ResponseWriter, *http.Request, User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := s.authenticate(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication_required")
			return
		}
		if user.Role != role {
			writeError(w, http.StatusForbidden, "forbidden")
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
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, user := range s.users {
		if user.ID == userID {
			return user, nil
		}
	}
	return User{}, errors.New("unknown user")
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
