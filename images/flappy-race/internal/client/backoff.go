package client

import "time"

// Reconnect pacing: 500 ms after the first failure, doubling per attempt
// and capped at 8 s — a quick server restart is picked up almost at once
// while a longer outage stays gentle on both sides.
const (
	retryBaseDelay = 500 * time.Millisecond
	retryMaxDelay  = 8 * time.Second
)

// retryDelay returns the pause after the attempts'th consecutive failure.
func retryDelay(attempts int) time.Duration {
	d := retryBaseDelay
	for range attempts - 1 {
		d *= 2
		if d >= retryMaxDelay {
			return retryMaxDelay
		}
	}
	return d
}