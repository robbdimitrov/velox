package main

import (
	"errors"
	"sync"
	"time"
)

const consumerErrorThreshold = 5

var errConsumerDegraded = errors.New("consumer degraded")

type consumerHealth struct {
	mu                sync.RWMutex
	lastSuccessAt     time.Time
	lastErrorAt       time.Time
	lastError         string
	consecutiveErrors int
}

type consumerHealthSnapshot struct {
	LastSuccessAt     time.Time `json:"last_success_at,omitempty"`
	LastErrorAt       time.Time `json:"last_error_at,omitempty"`
	LastError         string    `json:"last_error,omitempty"`
	ConsecutiveErrors int       `json:"consecutive_errors"`
}

func (h *consumerHealth) markSuccess() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastSuccessAt = time.Now()
	h.consecutiveErrors = 0
	h.lastError = ""
}

func (h *consumerHealth) markError(err error) {
	if err == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastErrorAt = time.Now()
	h.lastError = err.Error()
	h.consecutiveErrors++
}

func (h *consumerHealth) snapshot() consumerHealthSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return consumerHealthSnapshot{
		LastSuccessAt:     h.lastSuccessAt,
		LastErrorAt:       h.lastErrorAt,
		LastError:         h.lastError,
		ConsecutiveErrors: h.consecutiveErrors,
	}
}

func (h *consumerHealth) err() error {
	if h.snapshot().ConsecutiveErrors >= consumerErrorThreshold {
		return errConsumerDegraded
	}
	return nil
}
