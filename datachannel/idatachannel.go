package datachannel

import (
	"context"
	"time"

	"github.com/steveh/ecstoolkit/communicator"
	"github.com/steveh/ecstoolkit/message"
)

// RoundTripTiming represents the interface for calculating round trip time.
type RoundTripTiming interface {
	GetRoundTripTime() time.Duration
}

// IDataChannel defines the interface for data channel operations.
type IDataChannel interface {
	Reconnect() error
	SendFlag(flagType message.PayloadTypeFlag) error
	Open() error
	Close() error
	SendInputDataMessage(payloadType message.PayloadType, inputData []byte) error
	ResendStreamDataMessageScheduler() error
	SendMessage(input []byte, inputType int) error
	RegisterOutputStreamHandler(handler OutputStreamDataMessageHandler, isSessionSpecificHandler bool)
	DeregisterOutputStreamHandler(handler OutputStreamDataMessageHandler)
	IsSessionTypeSet() chan bool
	IsStreamMessageResendTimeout() chan bool
	GetSessionType() string
	SetSessionType(sessionType string)
	GetSessionProperties() interface{}
	SetWsChannel(wsChannel communicator.IWebSocketChannel)
	SetChannelToken(channelToken string)
	SetOnError(onErrorHandler func(error))
	RegisterOutputMessageHandler(ctx context.Context, stopHandler Stop, onMessageHandler func(input []byte))
	GetStreamDataSequenceNumber() int64
	GetAgentVersion() string
}
