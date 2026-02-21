package gateway

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateInfo contains detailed rate limit information
type RateInfo struct {
	Allowed      bool
	Limit        int
	Remaining    int
	ResetTime    time.Time
	RetryAfter   int
	CurrentCount int
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
	limits  map[string]int           // per-key limits
	windows map[string]time.Duration // per-key windows
	limit   int
	window  time.Duration
}

type bucket struct {
	tokens    int
	lastReset time.Time
	count     int // Track total requests in window
	mu        sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*bucket),
		limits:  make(map[string]int),
		windows: make(map[string]time.Duration),
		limit:   limit,
		window:  window,
	}
}

// SetKeyLimit sets a custom rate limit for a specific key
func (rl *RateLimiter) SetKeyLimit(key string, limit int, window time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.limits[key] = limit
	rl.windows[key] = window
}

// GetKeyLimit gets the rate limit for a specific key
func (rl *RateLimiter) GetKeyLimit(key string) (int, time.Duration) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if limit, ok := rl.limits[key]; ok {
		if window, ok := rl.windows[key]; ok {
			return limit, window
		}
	}

	return rl.limit, rl.window
}

// ParseRateLimit parses a rate limit string like "60/min" or "100/hour"
func ParseRateLimit(s string) (int, time.Duration, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid rate limit format: %s", s)
	}

	limit, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid rate limit number: %s", parts[0])
	}

	window := strings.TrimSpace(parts[1])
	var duration time.Duration
	switch window {
	case "sec", "second", "seconds":
		duration = time.Second
	case "min", "minute", "minutes":
		duration = time.Minute
	case "hour", "hours":
		duration = time.Hour
	case "day", "days":
		duration = 24 * time.Hour
	default:
		return 0, 0, fmt.Errorf("invalid time window: %s", window)
	}

	return limit, duration, nil
}

// Allow checks if a request is allowed
func (rl *RateLimiter) Allow(key string) bool {
	info := rl.CheckRateLimit(key)
	return info.Allowed
}

// AllowWithModel checks if a request is allowed with per-model tracking
func (rl *RateLimiter) AllowWithModel(key, model string) bool {
	info := rl.CheckRateLimitWithModel(key, model)
	return info.Allowed
}

// CheckRateLimit provides detailed rate limit information for a key
func (rl *RateLimiter) CheckRateLimit(key string) RateInfo {
	limit, window := rl.GetKeyLimit(key)

	rl.mu.RLock()
	b, exists := rl.buckets[key]
	rl.mu.RUnlock()

	now := time.Now()

	if !exists {
		rl.mu.Lock()
		b = &bucket{
			tokens:    limit - 1,
			lastReset: now,
			count:     1,
		}
		rl.buckets[key] = b
		rl.mu.Unlock()
		return RateInfo{
			Allowed:      true,
			Limit:        limit,
			Remaining:    limit - 1,
			ResetTime:    now.Add(window),
			RetryAfter:   0,
			CurrentCount: 1,
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if window has reset
	if now.Sub(b.lastReset) >= window {
		b.tokens = limit - 1
		b.lastReset = now
		b.count = 1
		return RateInfo{
			Allowed:      true,
			Limit:        limit,
			Remaining:    limit - 1,
			ResetTime:    now.Add(window),
			RetryAfter:   0,
			CurrentCount: 1,
		}
	}

	b.count++

	if b.tokens <= 0 {
		remaining := window - now.Sub(b.lastReset)
		if remaining < 0 {
			remaining = 0
		}
		// Retry-After is in seconds (RFC 9110). Round up so clients don't retry too early.
		retryAfter := 0
		if remaining > 0 {
			retryAfter = int((remaining + time.Second - 1) / time.Second)
		}
		return RateInfo{
			Allowed:      false,
			Limit:        limit,
			Remaining:    0,
			ResetTime:    b.lastReset.Add(window),
			RetryAfter:   retryAfter,
			CurrentCount: b.count,
		}
	}

	b.tokens--
	remaining := b.tokens
	if remaining < 0 {
		remaining = 0
	}

	return RateInfo{
		Allowed:      true,
		Limit:        limit,
		Remaining:    remaining,
		ResetTime:    b.lastReset.Add(window),
		RetryAfter:   0,
		CurrentCount: b.count,
	}
}

// CheckRateLimitWithModel provides detailed rate limit information for a key and model combination
func (rl *RateLimiter) CheckRateLimitWithModel(key, model string) RateInfo {
	// Use composite key for per-model tracking
	compositeKey := key + ":" + model
	return rl.CheckRateLimit(compositeKey)
}

// StartCleanup starts a goroutine to cleanup old buckets
func (rl *RateLimiter) StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			rl.cleanup()
		}
	}()
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for k, b := range rl.buckets {
		b.mu.Lock()
		if now.Sub(b.lastReset) > 2*rl.window {
			delete(rl.buckets, k)
		}
		b.mu.Unlock()
	}
}

// UpdateLimit updates the rate limit dynamically
func (rl *RateLimiter) UpdateLimit(limit int, window time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.limit = limit
	rl.window = window
}

// Middleware returns HTTP middleware for rate limiting
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting for dashboard routes, landing page, and chat completions (handled separately)
		if strings.HasPrefix(r.URL.Path, "/admin") || r.URL.Path == "/health" || r.URL.Path == "/" || r.URL.Path == "" || r.URL.Path == "/index.html" || r.URL.Path == "/v1/chat/completions" {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get("Authorization")
		if key == "" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.limit))
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": {"code": "MISSING_API_KEY", "message": "Authorization header is required", "action": "Add header: Authorization: Bearer YOUR_API_KEY"}}`))
			return
		}
		if strings.HasPrefix(key, "Bearer ") {
			key = key[7:]
		}

		info := rl.CheckRateLimit(key)

		// Set rate limit headers on all responses
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", info.Limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", info.Remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", info.ResetTime.Unix()))

		if !info.Allowed {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", fmt.Sprintf("%d", info.RetryAfter))
			w.WriteHeader(http.StatusTooManyRequests)

			resetTimeStr := info.ResetTime.Format("2006-01-02T15:04:05Z")
			errorMsg := fmt.Sprintf(
				`{"error": {"code": "RATE_LIMIT_EXCEEDED", "message": "Rate limit exceeded", "retry_after": %d, "reset_time": "%s", "limit": %d, "student_friendly": "You've made too many requests. Please wait %d seconds before trying again."}}`,
				info.RetryAfter, resetTimeStr, info.Limit, info.RetryAfter,
			)
			w.Write([]byte(errorMsg))
			return
		}

		next.ServeHTTP(w, r)
	})
}
