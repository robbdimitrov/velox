package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
)

func publicUser(user User) map[string]string {
	out := map[string]string{"id": user.ID, "email": user.Email, "role": user.Role}
	if user.OrganizerID != "" {
		out["organizer_id"] = user.OrganizerID
	}
	return out
}

func requestHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func constantTimeStringEqual(expected, actual string) bool {
	expectedHash := sha256.Sum256([]byte(expected))
	actualHash := sha256.Sum256([]byte(actual))
	return hmac.Equal(expectedHash[:], actualHash[:])
}

// signHMAC and verifyHMAC implement the gateway's opaque signed-token format
// (base64(payload).base64(hmac)), shared by session tokens and any other
// short-lived signed token the gateway issues (e.g. wallet QR tokens), so
// domain-specific callers don't reimplement the HMAC framing.
func signHMAC(secret []byte, payload map[string]any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifyHMAC(secret []byte, token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("bad token")
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expected, actual) {
		return nil, errors.New("bad signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func decodeJSONBytes(w http.ResponseWriter, r *http.Request, dst any) ([]byte, bool) {
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return nil, false
	}
	if len(raw) == 0 {
		writeError(w, http.StatusBadRequest, "empty_json")
		return nil, false
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_schema")
		return nil, false
	}
	return raw, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	_, ok := decodeJSONBytes(w, r, dst)
	return ok
}

// decodeJSONStrict enforces the order-command ingress contract from
// docs/architecture.md: reject unknown JSON fields and trailing payload data.
// It is intentionally not used by every handler (e.g. auth endpoints), since
// the frontend proxy forwards some request bodies with extra fields the
// backend already ignores; only order-command handlers require this rigor.
func decodeJSONStrict(w http.ResponseWriter, r *http.Request, dst any) ([]byte, bool) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return nil, false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return nil, false
	}
	return body, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, apiError{Error: code})
}

func writeStoreError(w http.ResponseWriter, err error) {
	if err.Error() == "store not found" {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error")
}

// doOrderServiceRequest issues a POST to s.orderSvcBaseURL+path, propagating
// the inbound request's X-Request-ID so orderservice logs can be correlated
// with the originating gateway request. body may be nil for the body-less
// action endpoints (confirm, cancel, cancel-event); when non-nil it is sent
// as a JSON request body (e.g. order creation). Shared by every gateway
// endpoint that forwards a request to orderservice's internal API; callers
// own status-code branching and response-body decoding since those differ
// per call site.
func (s *Server) doOrderServiceRequest(ctx context.Context, path string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.orderSvcBaseURL+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok {
		httpReq.Header.Set("X-Request-ID", reqID)
	}
	return s.httpClient.Do(httpReq)
}

// writeUpstreamError maps a >=400 orderservice response to a client error,
// decoding its {"error": "..."} body when present and falling back to a
// generic upstream_error code otherwise.
func writeUpstreamError(w http.ResponseWriter, resp *http.Response) {
	var errResp struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Error == "" {
		errResp.Error = "upstream_error"
	}
	writeError(w, resp.StatusCode, errResp.Error)
}

func limitBody(next http.Handler, limit int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	}
}
