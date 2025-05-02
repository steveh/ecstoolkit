// Package retry implements back off retry strategy for reconnect web socket connection.
package retry

import (
	"time"

	"github.com/steveh/ecstoolkit/log"
)

const sleepConstant = 2

// Retry implements back off retry strategy for reconnect web socket connection.
func Retry(logger log.T, attempts int, sleep time.Duration, fn func() error) error {
	logger.Debug("Retrying connection to channel")

	var lastErr error

	for attempts > 0 {
		attempts--

		if lastErr = fn(); lastErr != nil {
			time.Sleep(sleep)
			sleep *= sleepConstant

			logger.Debug("Attempts remaining to connect web socket connection", "attempts", attempts)

			continue
		}

		return nil
	}

	return lastErr
}
