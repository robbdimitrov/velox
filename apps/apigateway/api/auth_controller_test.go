package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegister(t *testing.T) {
	server := NewServerWithStore("test", nil)

	// Valid registration
	reqBody := `{"email":"new_user@velox.local","password":"pass","role":"reserver"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader([]byte(reqBody)))
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. body=%s", rr.Code, rr.Body.String())
	}

	// Verify cookie is set
	var cookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == CookieName {
			cookie = c
			break
		}
	}
	if cookie == nil {
		t.Fatalf("expected velox_session cookie")
	}

	// Invalid role
	reqBody = `{"email":"bad@velox.local","password":"pass","role":"admin"}`
	req = httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader([]byte(reqBody)))
	rr = httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", rr.Code)
	}
}

func TestLogout(t *testing.T) {
	server := NewServerWithStore("test", nil)
	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content, got %d", rr.Code)
	}

	found := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == CookieName && c.MaxAge < 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected cookie to be cleared")
	}
}

func TestMe(t *testing.T) {
	server := NewServerWithStore("test", nil)
	client := newTestClient(server)

	// Unauthorized
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rr.Code)
	}

	// Authorized
	cookie := client.login(t, "reserver@velox.local", "reserver")
	req = httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	var resp struct {
		User User `json:"user"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.User.Email != "reserver@velox.local" {
		t.Fatalf("expected user email, got %s", resp.User.Email)
	}
}
