package datachannel

import (
	"context"
	"time"

	"github.com/steveh/ecstoolkit/communicator"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
)

// RoundTripTiming represents the interface for calculating round trip time.
type RoundTripTiming interface {
	GetRoundTripTime() time.Duration
}

// IDataChannel defines the interface for data channel operations.
type IDataChannel interface {
	Reconnect(log log.T) error
	SendFlag(log log.T, flagType message.PayloadTypeFlag) error
	Open(log log.T) error
	Close(log log.T) error
	SendInputDataMessage(log log.T, payloadType message.PayloadType, inputData []byte) error
	ResendStreamDataMessageScheduler(log log.T) error
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
	RegisterOutputMessageHandler(ctx context.Context, log log.T, stopHandler Stop, onMessageHandler func(input []byte))
	GetStreamDataSequenceNumber() int64
	GetAgentVersion() string
}
