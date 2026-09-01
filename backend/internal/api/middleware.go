package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// MaxPayloadSize is 10MB
	MaxPayloadSize = 10 * 1024 * 1024
	// Authorization header format: "Bearer sk_..."
	AuthorizationHeader = "Authorization"
	BearerPrefix        = "Bearer "
)

// withIngestAuth middleware validates API key for ingest endpoint
func (handler *Handler) withIngestAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get API key from Authorization header
		authHeader := r.Header.Get(AuthorizationHeader)
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing Authorization header"))
			return
		}

		if !strings.HasPrefix(authHeader, BearerPrefix) {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid Authorization header format, expected 'Bearer <api_key>'"))
			return
		}

		apiKeyStr := strings.TrimPrefix(authHeader, BearerPrefix)
		if apiKeyStr == "" {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("empty API key"))
			return
		}

		// Validate API key
		if handler.apiKeyManager == nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("api key manager not initialized"))
			return
		}

		ak, err := handler.apiKeyManager.ValidateKey(r.Context(), apiKeyStr)
		if err != nil {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid or expired API key: %v", err))
			return
		}

		// Store API key info in request context for later use
		r.Header.Set("X-Asset-ID", fmt.Sprintf("%d", ak.AssetID))
		r.Header.Set("X-Hostname", ak.Hostname)

		next.ServeHTTP(w, r)
	})
}

// withRateLimit middleware enforces rate limiting per API key
func (handler *Handler) withRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handler.rateLimiter == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Use hostname as rate limit identifier (from auth middleware)
		hostname := r.Header.Get("X-Hostname")
		if hostname == "" {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("hostname not found in request context"))
			return
		}

		allowed, remaining, resetAt, err := handler.rateLimiter.Allow(r.Context(), hostname)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("rate limit check failed: %v", err))
			return
		}

		// Set rate limit headers
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", handler.rateLimiter.RequestLimit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))

		if !allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int64(resetAt.Sub(time.Now()).Seconds())))
			writeError(w, http.StatusTooManyRequests, fmt.Errorf("rate limit exceeded"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// withRequestSizeLimit middleware enforces maximum payload size
func (handler *Handler) withRequestSizeLimit(maxSize int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check Content-Length header
			if r.ContentLength > maxSize {
				writeError(w, http.StatusRequestEntityTooLarge,
					fmt.Errorf("payload too large: %d bytes (max: %d bytes)", r.ContentLength, maxSize))
				return
			}

			// Wrap request body with LimitedReader
			r.Body = io.NopCloser(io.LimitReader(r.Body, maxSize))

			next.ServeHTTP(w, r)
		})
	}
}
