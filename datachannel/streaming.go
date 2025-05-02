// Package datachannel implements data channel for interactive sessions.
package datachannel

import (
	"time"

	"github.com/steveh/ecstoolkit/message"
)

// StreamingMessage represents a message in the data stream with its metadata.
type StreamingMessage struct {
	Content        []byte
	SequenceNumber int64
	LastSentTime   time.Time
	ResendAttempt  int
}

// GetRoundTripTime is a function that calculates the round trip time for a streaming message.
func (m StreamingMessage) GetRoundTripTime() time.Duration {
	return time.Since(m.LastSentTime)
}

// OutputStreamDataMessageHandler is a function type that handles output stream data messages.
type OutputStreamDataMessageHandler func(streamDataMessage message.ClientMessage) (bool, error)

// StopHandler is a function type that handles stopping the data channel.
type StopHandler func() error
