// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package ratelimit_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure/ratelimit"
)

// TestRateLimiterStress verifies that rate limiting kicks in under high load.
func TestRateLimiterStress(t *testing.T) {
	cfg := ratelimit.Config{
		RequestsPerSecond: 5,  // 5 RPS
		Burst:             10, // burst of 10
		Enabled:           true,
	}
	limiter := ratelimit.NewLimiter(cfg)
	instanceID := uint64(1101000001)

	const totalRequests = 30
	var allowed, limited int
	var mu sync.Mutex

	// Fire all requests simultaneously
	var wg sync.WaitGroup
	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			err := limiter.Allow(instanceID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				limited++
				t.Logf("  #%02d → RATE LIMITED: %v", n+1, err)
			} else {
				allowed++
				t.Logf("  #%02d → allowed", n+1)
			}
		}(i)
	}
	wg.Wait()

	fmt.Printf("\n📈 Results: %d allowed, %d rate limited (out of %d)\n", allowed, limited, totalRequests)

	if limited == 0 {
		t.Errorf("expected some requests to be rate limited (burst=%d, total=%d)", cfg.Burst, totalRequests)
	} else {
		t.Logf("✅ Rate limiting confirmed: %d/%d requests throttled", limited, totalRequests)
	}

	// Verify: allowed should be <= burst
	if allowed > cfg.Burst {
		t.Errorf("allowed %d > burst %d — limiter is not working correctly", allowed, cfg.Burst)
	}
}

// TestRateLimiterPerInstance verifies isolation between instances.
func TestRateLimiterPerInstance(t *testing.T) {
	cfg := ratelimit.Config{
		RequestsPerSecond: 2,
		Burst:             3,
		Enabled:           true,
	}
	limiter := ratelimit.NewLimiter(cfg)

	inst1 := uint64(1101000001)
	inst2 := uint64(1101000002)

	// Exhaust inst1 bucket
	for i := 0; i < 5; i++ {
		_ = limiter.Allow(inst1)
	}

	// inst2 should still be fresh
	if err := limiter.Allow(inst2); err != nil {
		t.Errorf("instance 2 should not be rate limited: %v", err)
	} else {
		t.Log("✅ Per-instance isolation confirmed: inst2 fresh after inst1 exhausted")
	}
}
