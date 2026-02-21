package gateway

import (
	"testing"
	"time"
)

func TestRetryAfterIsSeconds(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)

	// First request allowed
	info1 := rl.CheckRateLimit("k")
	if !info1.Allowed {
		t.Fatalf("expected first request allowed")
	}

	// Second request allowed
	info2 := rl.CheckRateLimit("k")
	if !info2.Allowed {
		t.Fatalf("expected second request allowed")
	}

	// Third request blocked and Retry-After should be a small number of seconds
	info3 := rl.CheckRateLimit("k")
	if info3.Allowed {
		t.Fatalf("expected third request blocked")
	}
	if info3.RetryAfter <= 0 {
		t.Fatalf("expected RetryAfter > 0, got %d", info3.RetryAfter)
	}
	if info3.RetryAfter > 60 {
		t.Fatalf("expected RetryAfter in seconds (<=60), got %d", info3.RetryAfter)
	}
}
