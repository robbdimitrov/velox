package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
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

func limitBody(next http.Handler, limit int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	}
}
