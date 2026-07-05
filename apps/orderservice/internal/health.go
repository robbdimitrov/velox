package internal

import (
	"errors"
	"sync"
	"time"
)

const pipelineErrorThreshold = 5

var ErrPipelineDegraded = errors.New("pipeline degraded")

type PipelineStatus struct {
	Name              string    `json:"name"`
	LastSuccessAt     time.Time `json:"last_success_at,omitempty"`
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
	status.LastErrorAt = time.Now()
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
	for _, status := range h.Snapshot() {
		if status.ConsecutiveErrors >= pipelineErrorThreshold {
			return ErrPipelineDegraded
		}
	}
	return nil
}
