package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/maumercado/task-queue-go/internal/logger"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter with the specified requests per second
func NewRateLimiter(rps int) *RateLimiter {
	if rps <= 0 {
		rps = 1000 // default
	}
	return &RateLimiter{
		tokens:     float64(rps),
		maxTokens:  float64(rps),
		refillRate: float64(rps),
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed under the rate limit
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
	rl.lastRefill = now

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

// RateLimit returns a middleware that enforces rate limiting
func RateLimit(rps int) func(next http.Handler) http.Handler {
	limiter := NewRateLimiter(rps)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				logger.Warn().
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("remote_addr", r.RemoteAddr).
					Msg("rate limit exceeded")

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"Too Many Requests","message":"rate limit exceeded"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientRateLimiter maintains per-client rate limiters
type ClientRateLimiter struct {
	limiters map[string]*RateLimiter
	rps      int
	mu       sync.RWMutex
	cleanup  time.Duration
}

// NewClientRateLimiter creates a new per-client rate limiter
func NewClientRateLimiter(rps int) *ClientRateLimiter {
	crl := &ClientRateLimiter{
		limiters: make(map[string]*RateLimiter),
		rps:      rps,
		cleanup:  5 * time.Minute,
	}
	go crl.cleanupLoop()
	return crl
}

func (crl *ClientRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(crl.cleanup)
	defer ticker.Stop()
	for range ticker.C {
		crl.mu.Lock()
		// Simple cleanup: reset all limiters periodically
		// In production, you'd track last access time
		crl.limiters = make(map[string]*RateLimiter)
		crl.mu.Unlock()
	}
}

// GetLimiter returns the rate limiter for a client
func (crl *ClientRateLimiter) GetLimiter(clientID string) *RateLimiter {
	crl.mu.RLock()
	limiter, exists := crl.limiters[clientID]
	crl.mu.RUnlock()

	if exists {
		return limiter
	}

	crl.mu.Lock()
	defer crl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = crl.limiters[clientID]; exists {
		return limiter
	}

	limiter = NewRateLimiter(crl.rps)
	crl.limiters[clientID] = limiter
	return limiter
}

// ClientRateLimit returns a middleware that enforces per-client rate limiting
func ClientRateLimit(rps int) func(next http.Handler) http.Handler {
	limiter := NewClientRateLimiter(rps)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use X-Forwarded-For or RemoteAddr as client identifier
			clientID := r.Header.Get("X-Forwarded-For")
			if clientID == "" {
				clientID = r.RemoteAddr
			}

			clientLimiter := limiter.GetLimiter(clientID)
			if !clientLimiter.Allow() {
				logger.Warn().
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("client", clientID).
					Msg("client rate limit exceeded")

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"Too Many Requests","message":"rate limit exceeded"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
