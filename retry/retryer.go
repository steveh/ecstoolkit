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
	"math"
	"time"
)

// Retryer defines the interface for retry operations.
type Retryer interface {
	Call() error
	NextSleepTime(attempt int32) time.Duration
}

// RepeatableExponentialRetryer implements exponential backoff retry strategy.
type RepeatableExponentialRetryer struct {
	CallableFunc        func() error
	GeometricRatio      float64
	InitialDelayInMilli int
	MaxDelayInMilli     int
	MaxAttempts         int
}

// NextSleepTime calculates the next delay of retry.
func (r *RepeatableExponentialRetryer) NextSleepTime(attempt int) time.Duration {
	return time.Duration(float64(r.InitialDelayInMilli)*math.Pow(r.GeometricRatio, float64(attempt))) * time.Millisecond
}

// Call calls the operation and does exponential retry if error happens.
func (r *RepeatableExponentialRetryer) Call() error {
	attempt := 0
	failedAttemptsSoFar := 0

	for {
		err := r.CallableFunc()
		if err == nil || failedAttemptsSoFar == r.MaxAttempts {
			return err
		}

		sleep := r.NextSleepTime(attempt)
		if int(sleep/time.Millisecond) > r.MaxDelayInMilli {
			attempt = 0
			sleep = r.NextSleepTime(attempt)
		}

		time.Sleep(sleep)

		attempt++
		failedAttemptsSoFar++
	}
}
