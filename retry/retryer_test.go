// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

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

// ErrCallableFunc is a custom error for the callable function.
var ErrCallableFunc = errors.New("error occurred in callable function")

var callableFunc = func() error {
	return ErrCallableFunc
}

func TestRepeatableExponentialRetryerRetriesForGivenNumberOfMaxRetries(t *testing.T) {
	t.Parallel()

	retryer := retry.RepeatableExponentialRetryer{
		callableFunc,
		config.RetryBase,
		rand.Intn(config.DataChannelRetryInitialDelayMillis) + config.DataChannelRetryInitialDelayMillis, //nolint:gosec
		config.DataChannelRetryMaxIntervalMillis,
		config.DataChannelNumMaxRetries,
	}
	err := retryer.Call()
	require.Error(t, err)
}
