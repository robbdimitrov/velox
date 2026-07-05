package internal

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	pipelineErrorThreshold = 5
	pipelineErrorWindow    = 30 * time.Second
)

var ErrPipelineDegraded = errors.New("pipeline degraded")

type PipelineStatus struct {
	Name              string    `json:"name"`
	LastSuccessAt     time.Time `json:"last_success_at,omitempty"`
	FirstErrorAt      time.Time `json:"first_error_at,omitempty"`
	LastErrorAt       time.Time `json:"last_error_at,omitempty"`
	LastError         string    `json:"last_error,omitempty"`
	ConsecutiveErrors int       `json:"consecutive_errors"`
}

type PipelineHealth struct {
	mu       sync.RWMutex
	statuses map[string]PipelineStatus
}

func NewPipelineHealth(names ...string) *PipelineHealth {
	h := &PipelineHealth{statuses: make(map[string]PipelineStatus, len(names))}
	for _, name := range names {
		h.statuses[name] = PipelineStatus{Name: name}
	}
	return h
}

func (h *PipelineHealth) MarkSuccess(name string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	status := h.statuses[name]
	status.Name = name
	status.LastSuccessAt = time.Now()
	status.FirstErrorAt = time.Time{}
	status.ConsecutiveErrors = 0
	status.LastError = ""
	h.statuses[name] = status
}

func (h *PipelineHealth) MarkError(name string, err error) {
	if h == nil || err == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	status := h.statuses[name]
	status.Name = name
	now := time.Now()
	if status.FirstErrorAt.IsZero() {
		status.FirstErrorAt = now
	}
	status.LastErrorAt = now
	status.LastError = err.Error()
	status.ConsecutiveErrors++
	h.statuses[name] = status
}

func (h *PipelineHealth) Snapshot() []PipelineStatus {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	statuses := make([]PipelineStatus, 0, len(h.statuses))
	for _, status := range h.statuses {
		statuses = append(statuses, status)
	}
	return statuses
}

func (h *PipelineHealth) Err() error {
	now := time.Now()
	for _, status := range h.Snapshot() {
		if pipelineStatusDegraded(status, now) {
			return ErrPipelineDegraded
		}
	}
	return nil
}

func (h *PipelineHealth) Metrics(service string) string {
	var b strings.Builder
	now := time.Now()
	for _, status := range h.Snapshot() {
		labels := fmt.Sprintf(`service=%q,pipeline=%q`, service, status.Name)
		fmt.Fprintf(&b, "velox_pipeline_consecutive_errors{%s} %d\n", labels, status.ConsecutiveErrors)
		fmt.Fprintf(&b, "velox_pipeline_unhealthy{%s} %d\n", labels, boolMetric(pipelineStatusDegraded(status, now)))
		if !status.LastSuccessAt.IsZero() {
			fmt.Fprintf(&b, "velox_pipeline_last_success_age_seconds{%s} %.0f\n", labels, now.Sub(status.LastSuccessAt).Seconds())
		}
		if !status.FirstErrorAt.IsZero() {
			fmt.Fprintf(&b, "velox_pipeline_first_error_age_seconds{%s} %.0f\n", labels, now.Sub(status.FirstErrorAt).Seconds())
		}
	}
	return b.String()
}

func pipelineStatusDegraded(status PipelineStatus, now time.Time) bool {
	if status.ConsecutiveErrors >= pipelineErrorThreshold {
		return true
	}
	return !status.FirstErrorAt.IsZero() && now.Sub(status.FirstErrorAt) >= pipelineErrorWindow
}

func boolMetric(value bool) int {
	if value {
		return 1
	}
	return 0
}
