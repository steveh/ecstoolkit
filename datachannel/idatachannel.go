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

// GetReconnectionToken is a function that retrieves a reconnection token.
type GetReconnectionToken func(context.Context) (string, error)

// IDataChannel defines the interface for data channel operations.
type IDataChannel interface {
	Close() error
	SendFlag(flagType message.PayloadTypeFlag) error
	SendInputDataMessage(payloadType message.PayloadType, inputData []byte) error
	SendMessage(input []byte, inputType int) error
	RegisterOutputStreamHandler(handler OutputStreamDataMessageHandler, isSessionSpecificHandler bool)
	GetSessionProperties() any
	RegisterOutputMessageHandler(ctx context.Context, stopHandler Stop, onMessageHandler func(input []byte))
	GetAgentVersion() string
	GetTargetID() string
	EstablishSessionType(ctx context.Context, sessionType string, sleepInterval time.Duration, timeoutHandler func(ctx context.Context) error) (string, error)
	OpenWithRetry(ctx context.Context, messageHandler func(message.ClientMessage), getReconnectionToken GetReconnectionToken) error
}
