package datachannel

import (
	"container/list"
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
	FinalizeDataChannelHandshake(log log.T, tokenValue string) error
	SendInputDataMessage(log log.T, payloadType message.PayloadType, inputData []byte) error
	ResendStreamDataMessageScheduler(log log.T) error
	ProcessAcknowledgedMessage(log log.T, acknowledgeMessageContent message.AcknowledgeContent) error
	OutputMessageHandler(ctx context.Context, log log.T, stopHandler Stop, sessionID string, rawMessage []byte) error
	SendAcknowledgeMessage(log log.T, clientMessage message.ClientMessage) error
	AddDataToOutgoingMessageBuffer(streamMessage StreamingMessage)
	RemoveDataFromOutgoingMessageBuffer(streamMessageElement *list.Element)
	AddDataToIncomingMessageBuffer(streamMessage StreamingMessage)
	RemoveDataFromIncomingMessageBuffer(sequenceNumber int64)
	CalculateRetransmissionTimeout(streamingMessage RoundTripTiming)
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
	SetOnMessage(onMessageHandler func([]byte))
	GetStreamDataSequenceNumber() int64
	GetAgentVersion() string
	SetAgentVersion(agentVersion string)
}
