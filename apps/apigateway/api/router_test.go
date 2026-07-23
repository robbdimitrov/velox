package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersSetOnSuccessResponse(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	assertSecurityHeaders(t, rr.Header())
}

func TestSecurityHeadersSetOnNotFoundResponse(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/no-such-route", nil)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
	assertSecurityHeaders(t, rr.Header())
}

func TestSecurityHeadersDoNotOverrideHandlerContentType(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
}

func TestOversizedRequestBodyRejected(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)

	oversized := bytes.Repeat([]byte("a"), (1<<20)+1)
	body, _ := json.Marshal(map[string]string{"email": string(oversized)})
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	var out apiError
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Error != "invalid_json" {
		t.Fatalf("error = %q, want invalid_json", out.Error)
	}
}

func assertSecurityHeaders(t *testing.T, h http.Header) {
	t.Helper()
	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "0",
		"Referrer-Policy":        "no-referrer",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'; " +
			"base-uri 'none'; form-action 'none'",
	}
	for header, expected := range want {
		if got := h.Get(header); got != expected {
			t.Errorf("%s = %q, want %q", header, got, expected)
		}
	}
	if h.Get("Strict-Transport-Security") != "" {
		t.Error("Strict-Transport-Security must not be set: no TLS termination in this deployment")
	}
}
