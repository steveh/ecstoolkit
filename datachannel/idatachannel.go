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

// IncomingMessageHandler is a function that handles incoming messages.
type IncomingMessageHandler func(input []byte)

// TimeoutHandler is a function that handles timeout events.
type TimeoutHandler func(ctx context.Context) error

// IDataChannel defines the interface for data channel operations.
//
//nolint:interfacebloat
type IDataChannel interface {
	Open(ctx context.Context, refreshTokenHandler RefreshTokenHandler, timeoutHandler TimeoutHandler) (string, error)
	Close() error
	SendFlag(flagType message.PayloadTypeFlag) error
	SendInputDataMessage(payloadType message.PayloadType, inputData []byte) error
	SendMessage(input []byte, inputType int) error
	GetSessionProperties() any
	GetAgentVersion() string
	GetTargetID() string
	RegisterOutputStreamHandler(handler OutputStreamDataMessageHandler, sessionSpecific bool)
	RegisterIncomingMessageHandler(handler IncomingMessageHandler)
	RegisterStopHandler(handler StopHandler)
	GetDisplayMessages() <-chan message.ClientMessage
}
