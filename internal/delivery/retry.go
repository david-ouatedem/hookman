package delivery

import "time"

// retryDelays defines the backoff schedule after each failed attempt.
// Index 0 = delay after 1st failure, index 1 = delay after 2nd failure, etc.
var retryDelays = []time.Duration{
	30 * time.Second,  // 1st retry
	5 * time.Minute,   // 2nd retry
	30 * time.Minute,  // 3rd retry
	2 * time.Hour,     // 4th retry
	5 * time.Hour,     // 5th retry
}

// NextRetryDelay returns the delay before the next retry based on the current
// attempt number (1-indexed). Returns 0 and false if max retries are exhausted.
func NextRetryDelay(attemptNumber int, maxRetries int) (time.Duration, bool) {
	if attemptNumber >= maxRetries {
		return 0, false
	}

	idx := attemptNumber - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(retryDelays) {
		return retryDelays[len(retryDelays)-1], true
	}
	return retryDelays[idx], true
}

// NextRetryAt returns the absolute time for the next retry. Returns nil if
// max retries have been exhausted (event should be marked dead).
func NextRetryAt(attemptNumber int, maxRetries int) *time.Time {
	delay, ok := NextRetryDelay(attemptNumber, maxRetries)
	if !ok {
		return nil
	}
	t := time.Now().Add(delay)
	return &t
}
