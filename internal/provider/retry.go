package provider

import (
	"context"
	"math"
	"math/rand/v2"
	"net/http"
	"time"
)

type RetryConfig struct {
	MaxAttempts    int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	Jitter         time.Duration
	RateLimitDelay time.Duration
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:    3,
		BaseDelay:      1 * time.Second,
		MaxDelay:       30 * time.Second,
		Jitter:         500 * time.Millisecond,
		RateLimitDelay: 5 * time.Second,
	}
}

func IsRetryable(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

func (rc RetryConfig) Backoff(attempt, statusCode int) time.Duration {
	base := rc.BaseDelay
	if statusCode == http.StatusTooManyRequests || statusCode == 529 {
		base = rc.RateLimitDelay
	}
	exp := time.Duration(float64(base) * math.Pow(2, float64(attempt)))
	if exp > rc.MaxDelay {
		exp = rc.MaxDelay
	}
	if rc.Jitter > 0 {
		exp += time.Duration(rand.Int64N(int64(rc.Jitter)))
	}
	return exp
}

func DoWithRetry(ctx context.Context, rc RetryConfig, fn func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error
	for attempt := range rc.MaxAttempts {
		resp, err := fn()
		if err == nil && resp.StatusCode < 400 {
			return resp, nil
		}

		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
			if statusCode > 0 && !IsRetryable(statusCode) {
				return resp, err
			}
			resp.Body.Close()
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = &HTTPError{StatusCode: statusCode}
		}

		if attempt == rc.MaxAttempts-1 {
			break
		}

		delay := rc.Backoff(attempt, statusCode)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}

type HTTPError struct {
	StatusCode int
}

func (e *HTTPError) Error() string {
	return http.StatusText(e.StatusCode)
}
