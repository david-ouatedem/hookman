package delivery

import (
	"testing"
	"time"
)

func TestNextRetryDelay(t *testing.T) {
	tests := []struct {
		attempt    int
		maxRetries int
		wantDelay  time.Duration
		wantOK     bool
	}{
		{1, 5, 30 * time.Second, true},
		{2, 5, 5 * time.Minute, true},
		{3, 5, 30 * time.Minute, true},
		{4, 5, 2 * time.Hour, true},
		{5, 5, 0, false},              // exhausted
		{1, 1, 0, false},              // single attempt, no retry
		{3, 3, 0, false},              // exhausted at 3
	}

	for _, tt := range tests {
		delay, ok := NextRetryDelay(tt.attempt, tt.maxRetries)
		if ok != tt.wantOK {
			t.Errorf("attempt=%d maxRetries=%d: got ok=%v, want %v", tt.attempt, tt.maxRetries, ok, tt.wantOK)
		}
		if delay != tt.wantDelay {
			t.Errorf("attempt=%d maxRetries=%d: got delay=%v, want %v", tt.attempt, tt.maxRetries, delay, tt.wantDelay)
		}
	}
}

func TestNextRetryAt(t *testing.T) {
	// Should return nil when exhausted
	result := NextRetryAt(5, 5)
	if result != nil {
		t.Error("expected nil when retries exhausted")
	}

	// Should return a future time when retries remain
	before := time.Now()
	result = NextRetryAt(1, 5)
	if result == nil {
		t.Fatal("expected non-nil for first retry")
	}
	if result.Before(before) {
		t.Error("retry time should be in the future")
	}
}
