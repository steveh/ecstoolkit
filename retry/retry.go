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

// Package retry implements back off retry strategy for reconnect web socket connection.
package retry

import (
	"log/slog"
	"time"
)

const sleepConstant = 2

// Retry implements back off retry strategy for reconnect web socket connection.
func Retry(log *slog.Logger, attempts int, sleep time.Duration, fn func() error) error {
	log.Debug("Retrying connection to channel")

	var lastErr error

	for attempts > 0 {
		attempts--

		if lastErr = fn(); lastErr != nil {
			time.Sleep(sleep)
			sleep *= sleepConstant

			log.Debug("Attempts remaining to connect web socket connection", "attempts", attempts)

			continue
		}

		return nil
	}

	return lastErr
}
