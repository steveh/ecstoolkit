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
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/steveh/ecstoolkit/encryption"
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
type OutputStreamDataMessageHandler func(log *slog.Logger, streamDataMessage message.ClientMessage) (bool, error)

// Stop is a function type that handles stopping the data channel.
type Stop func(log *slog.Logger) error

// SendAcknowledgeMessageCall is a function that sends an acknowledgment message for a stream data message.
var SendAcknowledgeMessageCall = func(log *slog.Logger, dataChannel *DataChannel, streamDataMessage message.ClientMessage) error {
	return dataChannel.SendAcknowledgeMessage(log, streamDataMessage)
}

// ProcessAcknowledgedMessageCall is a function that processes an acknowledged message.
var ProcessAcknowledgedMessageCall = func(log *slog.Logger, dataChannel *DataChannel, acknowledgeMessage message.AcknowledgeContent) error {
	return dataChannel.ProcessAcknowledgedMessage(log, acknowledgeMessage)
}

// SendMessageCall is a function that sends a message through the data channel.
var SendMessageCall = func(dataChannel *DataChannel, input []byte, inputType int) error {
	return dataChannel.SendMessage(input, inputType)
}

var newEncrypter = func(ctx context.Context, log *slog.Logger, kmsKeyID string, encryptionConext map[string]string, kmsService *kms.Client) (encryption.IEncrypter, error) {
	return encryption.NewEncrypter(ctx, log, kmsKeyID, encryptionConext, kmsService)
}
