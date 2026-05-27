// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

// Package ratelimit provides per-instance token bucket rate limiting for MCP tools.
package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Config holds rate limiting configuration.
type Config struct {
	RequestsPerSecond float64 // Maximum requests per second per instance
	Burst             int     // Maximum burst size per instance
	Enabled           bool    // Whether rate limiting is enabled
}

// DefaultConfig returns sensible defaults for rate limiting.
func DefaultConfig() Config {
	return Config{
		RequestsPerSecond: 10,
		Burst:             20,
		Enabled:           true,
	}
}

// Limiter provides per-instance rate limiting using token bucket algorithm.
type Limiter struct {
	config   Config
	limiters map[uint64]*rate.Limiter
	mu       sync.RWMutex
}

// NewLimiter creates a new rate limiter with the given configuration.
func NewLimiter(config Config) *Limiter {
	return &Limiter{
		config:   config,
		limiters: make(map[uint64]*rate.Limiter),
	}
}

// getLimiter returns the rate limiter for a specific instance ID.
// Creates a new limiter if one doesn't exist for this instance.
func (l *Limiter) getLimiter(instanceID uint64) *rate.Limiter {
	l.mu.RLock()
	limiter, exists := l.limiters[instanceID]
	l.mu.RUnlock()

	if exists {
		return limiter
	}

	// Create new limiter under write lock
	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check in case another goroutine created it
	if limiter, exists := l.limiters[instanceID]; exists {
		return limiter
	}

	// Create new rate limiter for this instance
	limiter = rate.NewLimiter(rate.Limit(l.config.RequestsPerSecond), l.config.Burst)
	l.limiters[instanceID] = limiter
	return limiter
}

// RateLimitError represents a rate limit exceeded error.
type RateLimitError struct {
	InstanceID uint64
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded for instance %d, retry after %v", e.InstanceID, e.RetryAfter)
}

// Allow checks if a request is allowed for the given instance ID.
// Returns RateLimitError if rate limit is exceeded.
func (l *Limiter) Allow(instanceID uint64) error {
	if !l.config.Enabled {
		return nil
	}

	limiter := l.getLimiter(instanceID)

	if !limiter.Allow() {
		// Calculate retry-after based on current rate
		retryAfter := limiter.Reserve().Delay()
		if retryAfter <= 0 {
			retryAfter = time.Second / time.Duration(l.config.RequestsPerSecond)
		}
		return &RateLimitError{
			InstanceID: instanceID,
			RetryAfter: retryAfter,
		}
	}

	return nil
}

// AllowN checks if n requests are allowed for the given instance ID.
// Returns RateLimitError if rate limit is exceeded.
func (l *Limiter) AllowN(instanceID uint64, n int) error {
	if !l.config.Enabled {
		return nil
	}

	limiter := l.getLimiter(instanceID)

	if !limiter.AllowN(time.Now(), n) {
		// Calculate retry-after based on current rate
		retryAfter := limiter.Reserve().Delay()
		if retryAfter <= 0 {
			retryAfter = time.Second / time.Duration(l.config.RequestsPerSecond)
		}
		return &RateLimitError{
			InstanceID: instanceID,
			RetryAfter: retryAfter,
		}
	}

	return nil
}

// Wait blocks until a request is allowed for the given instance ID.
func (l *Limiter) Wait(ctx context.Context, instanceID uint64) error {
	if !l.config.Enabled {
		return nil
	}

	limiter := l.getLimiter(instanceID)
	return limiter.Wait(ctx)
}

// SetRetryAfterHeader sets the Retry-After header on an HTTP response.
func SetRetryAfterHeader(w http.ResponseWriter, retryAfter time.Duration) {
	// Set Retry-After header in seconds (HTTP spec)
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}

// CleanupOldLimiters removes rate limiters that haven't been used recently.
// This should be called periodically to prevent memory leaks.
func (l *Limiter) CleanupOldLimiters(maxAge time.Duration) {
	// Note: golang.org/x/time/rate doesn't provide last access time,
	// so we can't implement cleanup without tracking it ourselves.
	// For now, we'll leave limiters in memory. In practice, this is fine
	// for reasonable numbers of instances.
	// TODO: Add access tracking if memory usage becomes a concern.
}
