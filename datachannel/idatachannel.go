package datachannel

import (
	"context"
	"time"

	"github.com/steveh/ecstoolkit/message"
)

// RoundTripTiming represents the interface for calculating round trip time.
type RoundTripTiming interface {
	GetRoundTripTime() time.Duration
}

// RefreshTokenHandler is a function that retrieves a reconnection token.
type RefreshTokenHandler func(context.Context) (string, error)

// DisplayMessageHandler is a function that handles display messages.
type DisplayMessageHandler func(message message.ClientMessage)

// OnMessageHandler is a function that handles incoming messages.
type OnMessageHandler func(input []byte)

// TimeoutHandler is a function that handles timeout events.
type TimeoutHandler func(ctx context.Context) error

// IDataChannel defines the interface for data channel operations.
type IDataChannel interface {
	Open(ctx context.Context, displayMessageHandler DisplayMessageHandler, refreshTokenHandler RefreshTokenHandler, timeoutHandler TimeoutHandler) (string, error)
	Close() error
	SendFlag(flagType message.PayloadTypeFlag) error
	SendInputDataMessage(payloadType message.PayloadType, inputData []byte) error
	SendMessage(input []byte, inputType int) error
	GetSessionProperties() any
	GetAgentVersion() string
	GetTargetID() string
	RegisterOutputStreamHandler(handler OutputStreamDataMessageHandler, sessionSpecific bool)
	RegisterOutputMessageHandler(ctx context.Context, stopHandler StopHandler, onMessageHandler OnMessageHandler)
}
