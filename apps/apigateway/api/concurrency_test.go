package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

func TestConcurrency50Reservations(t *testing.T) {
	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"order_id": "mock-order",
			"status":   "PENDING",
		})
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	server.SetOrderServiceURL(mockOrderSvc.URL)
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)

	var wg sync.WaitGroup
	var successes int32
	var conflicts int32
	var others int32

	reqCount := 50
	wg.Add(reqCount)

	cookie := client.login(t, "reserver@velox.local", "reserver")

	for i := 0; i < reqCount; i++ {
		go func(idx int) {
			defer wg.Done()

			key := fmt.Sprintf("idem-concurr-%d", idx)
			payload := map[string]any{"event_id": "evt_neon_riot", "section_id": "A", "seat_ids": []string{"A-05"}}
			body, _ := json.Marshal(payload)

			req := httptest.NewRequest(http.MethodPost, "/reservations", bytes.NewReader(body))
			req.Header.Set("Idempotency-Key", key)
			req.AddCookie(cookie)

			rr := httptest.NewRecorder()
			server.Routes().ServeHTTP(rr, req)

			if rr.Code == http.StatusOK {
				atomic.AddInt32(&successes, 1)
			} else if rr.Code == http.StatusConflict {
				atomic.AddInt32(&conflicts, 1)
			} else {
				atomic.AddInt32(&others, 1)
				t.Logf("Unexpected status code %d for request %d. Body: %s", rr.Code, idx, rr.Body.String())
			}
		}(i)
	}

	wg.Wait()

	if successes != 1 {
		t.Errorf("Expected exactly 1 success, got %d", successes)
	}
	if conflicts != int32(reqCount-1) {
		t.Errorf("Expected exactly %d conflicts, got %d", reqCount-1, conflicts)
	}
	if others > 0 {
		t.Errorf("Expected 0 other errors, got %d", others)
	}
}
