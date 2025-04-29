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

// Package datachannel implements data channel for interactive sessions.
package datachannel

import (
	"container/list"
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/steveh/ecstoolkit/encryption"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
)

// ListMessageBuffer represents a buffer for messages using a linked list data structure.
type ListMessageBuffer struct {
	Messages *list.List
	Capacity int
	Mutex    *sync.Mutex
}

// MapMessageBuffer represents a buffer for messages using a map data structure.
type MapMessageBuffer struct {
	Messages map[int64]StreamingMessage
	Capacity int
	Mutex    *sync.Mutex
}

// StreamingMessage represents a message in the data stream with its metadata.
type StreamingMessage struct {
	Content        []byte
	SequenceNumber int64
	LastSentTime   time.Time
	ResendAttempt  *int
}

// GetRoundTripTime is a function that calculates the round trip time for a streaming message.
func (m StreamingMessage) GetRoundTripTime() time.Duration {
	return time.Since(m.LastSentTime)
}

// OutputStreamDataMessageHandler is a function type that handles output stream data messages.
type OutputStreamDataMessageHandler func(streamDataMessage message.ClientMessage) (bool, error)

// StopHandler is a function type that handles stopping the data channel.
type StopHandler func() error

var newEncrypter = func(ctx context.Context, logger log.T, kmsKeyID string, encryptionConext map[string]string, kmsService *kms.Client) (encryption.IEncrypter, error) {
	return encryption.NewEncrypter(ctx, logger, kmsKeyID, encryptionConext, kmsService)
}
