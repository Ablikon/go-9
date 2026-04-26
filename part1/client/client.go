package client

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// RetryConfig holds parameters for the retry logic
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// IsRetryable determines if an error or HTTP response status warrants a retry.
// True for timeouts, network issues, and statuses: 429, 500, 502, 503, 504.
func IsRetryable(resp *http.Response, err error) bool {
	if err != nil {
		return true // Treat all underlying transport errors as retryable for simplicity
	}
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusTooManyRequests, // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout:      // 504
			return true
		}
	}
	return false
}

// CalculateBackoff calculates the exponential backoff with full jitter to distribute load.
func CalculateBackoff(attempt int, cfg RetryConfig) time.Duration {
	backoff := float64(cfg.BaseDelay) * math.Pow(2, float64(attempt))
	if backoff > float64(cfg.MaxDelay) {
		backoff = float64(cfg.MaxDelay)
	}

	// Add Full Jitter
	jitter := time.Duration(rand.Int63n(int64(backoff)))
	return jitter
}

// ExecutePayment performs the HTTP request with retries and a global context timeout.
func ExecutePayment(ctx context.Context, httpClient *http.Client, req *http.Request, cfg RetryConfig) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt < cfg.MaxRetries; attempt++ {
		// Check for cancellation before each attempt
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Execute request
		resp, err = httpClient.Do(req.WithContext(ctx))

		// If successful and not a retryable error status, return immediately.
		if err == nil && !IsRetryable(resp, nil) {
			return resp, nil
		}

		// Also check if it's the last attempt. Exiting loop.
		if attempt == cfg.MaxRetries-1 {
			break
		}

		// If the error IS retryable, perform backoff + jitter.
		if IsRetryable(resp, err) {
			delay := CalculateBackoff(attempt, cfg)
			fmt.Printf("Attempt %d failed: waiting %v...\n", attempt+1, delay)

			// Drain and close the body before retrying to reuse connections (if response exists).
			if resp != nil && resp.Body != nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}

			// Sleep for the jittered delay, but also respect early context cancellation
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return resp, err
}
