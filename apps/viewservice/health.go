package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	consumerErrorThreshold = 5
	consumerErrorWindow    = 30 * time.Second
)

var errConsumerDegraded = errors.New("consumer degraded")

type consumerHealth struct {
	mu                sync.RWMutex
	lastSuccessAt     time.Time
	firstErrorAt      time.Time
	lastErrorAt       time.Time
	lastError         string
	consecutiveErrors int
	errorCount        uint64
}

type consumerHealthSnapshot struct {
	LastSuccessAt     time.Time `json:"last_success_at,omitempty"`
	FirstErrorAt      time.Time `json:"first_error_at,omitempty"`
	LastErrorAt       time.Time `json:"last_error_at,omitempty"`
	LastError         string    `json:"last_error,omitempty"`
	ConsecutiveErrors int       `json:"consecutive_errors"`
	ErrorCount        uint64    `json:"error_count"`
}

func (h *consumerHealth) markSuccess() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastSuccessAt = time.Now()
	h.firstErrorAt = time.Time{}
	h.consecutiveErrors = 0
	h.lastError = ""
}

func (h *consumerHealth) markError(err error) {
	if err == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	if h.firstErrorAt.IsZero() {
		h.firstErrorAt = now
	}
	h.lastErrorAt = now
	h.lastError = err.Error()
	h.consecutiveErrors++
	h.errorCount++
}

func (h *consumerHealth) markRecovered() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.firstErrorAt = time.Time{}
	h.consecutiveErrors = 0
	h.lastError = ""
}

func (h *consumerHealth) snapshot() consumerHealthSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return consumerHealthSnapshot{
		LastSuccessAt:     h.lastSuccessAt,
		FirstErrorAt:      h.firstErrorAt,
		LastErrorAt:       h.lastErrorAt,
		LastError:         h.lastError,
		ConsecutiveErrors: h.consecutiveErrors,
		ErrorCount:        h.errorCount,
	}
}

func (h *consumerHealth) err() error {
	snapshot := h.snapshot()
	if consumerStatusDegraded(snapshot, time.Now()) {
		return errConsumerDegraded
	}
	return nil
}

func (h *consumerHealth) metrics(service string) string {
	var b strings.Builder
	now := time.Now()
	snapshot := h.snapshot()
	labels := fmt.Sprintf(`service=%q,consumer=%q`, service, "events")
	fmt.Fprintf(&b, "velox_consumer_consecutive_errors{%s} %d\n", labels, snapshot.ConsecutiveErrors)
	fmt.Fprintf(&b, "velox_consumer_unhealthy{%s} %d\n", labels, boolMetric(consumerStatusDegraded(snapshot, now)))
	canonicalLabels := fmt.Sprintf(`app=%q,service=%q,pipeline=%q`, "velox", service, "events")
	fmt.Fprintf(&b, "app_pipeline_running{%s} 1\n", canonicalLabels)
	fmt.Fprintf(&b, "app_pipeline_unhealthy{%s} %d\n", canonicalLabels, boolMetric(consumerStatusDegraded(snapshot, now)))
	fmt.Fprintf(&b, "app_pipeline_error_streak{%s} %d\n", canonicalLabels, snapshot.ConsecutiveErrors)
	fmt.Fprintf(&b, "app_pipeline_errors_total{%s} %d\n", canonicalLabels, snapshot.ErrorCount)
	if !snapshot.LastSuccessAt.IsZero() {
		fmt.Fprintf(&b, "velox_consumer_last_success_age_seconds{%s} %.0f\n", labels, now.Sub(snapshot.LastSuccessAt).Seconds())
		fmt.Fprintf(&b, "app_pipeline_last_success_age_seconds{%s} %.0f\n", canonicalLabels, now.Sub(snapshot.LastSuccessAt).Seconds())
		fmt.Fprintf(&b, "app_pipeline_last_progress_age_seconds{%s} %.0f\n", canonicalLabels, now.Sub(snapshot.LastSuccessAt).Seconds())
	}
	if !snapshot.FirstErrorAt.IsZero() {
		fmt.Fprintf(&b, "velox_consumer_first_error_age_seconds{%s} %.0f\n", labels, now.Sub(snapshot.FirstErrorAt).Seconds())
		fmt.Fprintf(&b, "app_pipeline_first_error_age_seconds{%s} %.0f\n", canonicalLabels, now.Sub(snapshot.FirstErrorAt).Seconds())
	}
	return b.String()
}

func consumerStatusDegraded(snapshot consumerHealthSnapshot, now time.Time) bool {
	if snapshot.ConsecutiveErrors >= consumerErrorThreshold {
		return true
	}
	return !snapshot.FirstErrorAt.IsZero() && now.Sub(snapshot.FirstErrorAt) >= consumerErrorWindow
}

func boolMetric(value bool) int {
	if value {
		return 1
	}
	return 0
}
