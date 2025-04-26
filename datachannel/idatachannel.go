package datachannel

import (
	"context"
	"time"

	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/retry"
)

// RoundTripTiming represents the interface for calculating round trip time.
type RoundTripTiming interface {
	GetRoundTripTime() time.Duration
}

// IDataChannel defines the interface for data channel operations.
type IDataChannel interface {
	Close() error
	Reconnect() error
	SendFlag(flagType message.PayloadTypeFlag) error
	SendInputDataMessage(payloadType message.PayloadType, inputData []byte) error
	SendMessage(input []byte, inputType int) error
	RegisterOutputStreamHandler(handler OutputStreamDataMessageHandler, isSessionSpecificHandler bool)
	GetSessionProperties() any
	SetChannelToken(channelToken string)
	RegisterOutputMessageHandler(ctx context.Context, stopHandler Stop, onMessageHandler func(input []byte))
	GetAgentVersion() string
	GetTargetID() string
	EstablishSessionType(ctx context.Context, sessionType string, sleepInterval time.Duration, timeoutHandler func(ctx context.Context) error) (string, error)
	OpenWithRetry(ctx context.Context, retryParams retry.RepeatableExponentialRetryer, messageHandler func(message.ClientMessage), resumeSessionHandler func(context.Context) error) error
}
