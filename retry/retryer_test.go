// retry implements back off retry strategy for reconnect web socket connection.
package retry_test

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/retry"
	"github.com/stretchr/testify/require"
)

// errCallableFunc is a custom error for the callable function.
var errCallableFunc = errors.New("error occurred in callable function")

func TestRepeatableExponentialRetryerRetriesForGivenNumberOfMaxRetries(t *testing.T) {
	t.Parallel()

	retryer := retry.RepeatableExponentialRetryer{
		func() error {
			return errCallableFunc
		},
		config.RetryBase,
		rand.Intn(config.DataChannelRetryInitialDelayMillis) + config.DataChannelRetryInitialDelayMillis, //nolint:gosec
		config.DataChannelRetryMaxIntervalMillis,
		config.DataChannelNumMaxRetries,
	}
	err := retryer.Call()
	require.Error(t, err)
}
