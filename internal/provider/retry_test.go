package provider

import (
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	rc := DefaultRetryConfig()

	d0 := rc.Backoff(0, 500)
	if d0 < rc.BaseDelay || d0 > rc.BaseDelay+rc.Jitter {
		t.Errorf("attempt 0: expected ~%v, got %v", rc.BaseDelay, d0)
	}

	d1 := rc.Backoff(1, 500)
	if d1 < 2*rc.BaseDelay || d1 > 2*rc.BaseDelay+rc.Jitter {
		t.Errorf("attempt 1: expected ~%v, got %v", 2*rc.BaseDelay, d1)
	}
}

func TestBackoff_RateLimit(t *testing.T) {
	rc := DefaultRetryConfig()

	d := rc.Backoff(0, 429)
	if d < rc.RateLimitDelay || d > rc.RateLimitDelay+rc.Jitter {
		t.Errorf("rate limit: expected ~%v, got %v", rc.RateLimitDelay, d)
	}
}

func TestBackoff_MaxDelay(t *testing.T) {
	rc := RetryConfig{
		BaseDelay: 1 * time.Second,
		MaxDelay:  5 * time.Second,
		Jitter:    0,
	}

	d := rc.Backoff(10, 500)
	if d > rc.MaxDelay {
		t.Errorf("expected capped at %v, got %v", rc.MaxDelay, d)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{200, false},
		{400, false},
		{401, false},
		{429, true},
		{500, true},
		{502, true},
		{529, true},
	}
	for _, tt := range tests {
		if got := IsRetryable(tt.code); got != tt.expected {
			t.Errorf("IsRetryable(%d) = %v, want %v", tt.code, got, tt.expected)
		}
	}
}
