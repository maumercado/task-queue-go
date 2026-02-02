package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRateLimiter(t *testing.T) {
	t.Run("creates limiter with specified RPS", func(t *testing.T) {
		limiter := NewRateLimiter(100)
		assert.NotNil(t, limiter)
		assert.Equal(t, float64(100), limiter.maxTokens)
		assert.Equal(t, float64(100), limiter.refillRate)
	})

	t.Run("defaults to 1000 RPS when zero provided", func(t *testing.T) {
		limiter := NewRateLimiter(0)
		assert.Equal(t, float64(1000), limiter.maxTokens)
	})

	t.Run("defaults to 1000 RPS when negative provided", func(t *testing.T) {
		limiter := NewRateLimiter(-5)
		assert.Equal(t, float64(1000), limiter.maxTokens)
	})
}

func TestRateLimiter_Allow(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		limiter := NewRateLimiter(10)

		// Should allow up to 10 requests immediately
		for i := 0; i < 10; i++ {
			assert.True(t, limiter.Allow(), "request %d should be allowed", i)
		}
	})

	t.Run("denies requests over limit", func(t *testing.T) {
		limiter := NewRateLimiter(5)

		// Exhaust the tokens
		for i := 0; i < 5; i++ {
			limiter.Allow()
		}

		// Next request should be denied
		assert.False(t, limiter.Allow())
	})

	t.Run("refills tokens over time", func(t *testing.T) {
		limiter := NewRateLimiter(10)

		// Exhaust tokens
		for i := 0; i < 10; i++ {
			limiter.Allow()
		}
		assert.False(t, limiter.Allow())

		// Wait for refill (10 rps = 1 token per 100ms)
		time.Sleep(150 * time.Millisecond)

		// Should have at least 1 token now
		assert.True(t, limiter.Allow())
	})
}

func TestRateLimit_Middleware(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		handler := RateLimit(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns 429 when limit exceeded", func(t *testing.T) {
		handler := RateLimit(2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Exhaust the rate limit
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if i < 2 {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.Equal(t, http.StatusTooManyRequests, w.Code)
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
				assert.Equal(t, "1", w.Header().Get("Retry-After"))
			}
		}
	})
}

func TestNewClientRateLimiter(t *testing.T) {
	limiter := NewClientRateLimiter(100)
	assert.NotNil(t, limiter)
	assert.NotNil(t, limiter.limiters)
	assert.Equal(t, 100, limiter.rps)
}

func TestClientRateLimiter_GetLimiter(t *testing.T) {
	t.Run("creates new limiter for unknown client", func(t *testing.T) {
		crl := NewClientRateLimiter(10)

		limiter := crl.GetLimiter("client-1")
		assert.NotNil(t, limiter)
		assert.Equal(t, float64(10), limiter.maxTokens)
	})

	t.Run("returns same limiter for same client", func(t *testing.T) {
		crl := NewClientRateLimiter(10)

		limiter1 := crl.GetLimiter("client-1")
		limiter2 := crl.GetLimiter("client-1")

		assert.Same(t, limiter1, limiter2)
	})

	t.Run("returns different limiters for different clients", func(t *testing.T) {
		crl := NewClientRateLimiter(10)

		limiter1 := crl.GetLimiter("client-1")
		limiter2 := crl.GetLimiter("client-2")

		assert.NotSame(t, limiter1, limiter2)
	})
}

func TestClientRateLimit_Middleware(t *testing.T) {
	t.Run("allows requests within client limit", func(t *testing.T) {
		handler := ClientRateLimit(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("uses X-Forwarded-For when available", func(t *testing.T) {
		handler := ClientRateLimit(2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Two different clients with X-Forwarded-For
		for _, client := range []string{"10.0.0.1", "10.0.0.2"} {
			for i := 0; i < 2; i++ {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", client)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
			}
		}
	})

	t.Run("returns 429 when client limit exceeded", func(t *testing.T) {
		handler := ClientRateLimit(2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if i < 2 {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.Equal(t, http.StatusTooManyRequests, w.Code)
			}
		}
	})
}
