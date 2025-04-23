package datachannel

import (
	"container/list"
	"context"
	"log/slog"

	"github.com/steveh/ecstoolkit/communicator"
	"github.com/steveh/ecstoolkit/message"
)

// IDataChannel defines the interface for data channel operations.
type IDataChannel interface {
	Initialize(log *slog.Logger, clientID string, sessionID string, targetID string, isAwsCliUpgradeNeeded bool)
	SetWebsocket(log *slog.Logger, streamURL string, tokenValue string)
	Reconnect(log *slog.Logger) error
	SendFlag(log *slog.Logger, flagType message.PayloadTypeFlag) error
	Open(log *slog.Logger) error
	Close(log *slog.Logger) error
	FinalizeDataChannelHandshake(log *slog.Logger, tokenValue string) error
	SendInputDataMessage(log *slog.Logger, payloadType message.PayloadType, inputData []byte) error
	ResendStreamDataMessageScheduler(log *slog.Logger) error
	ProcessAcknowledgedMessage(log *slog.Logger, acknowledgeMessageContent message.AcknowledgeContent) error
	OutputMessageHandler(ctx context.Context, log *slog.Logger, stopHandler Stop, sessionID string, rawMessage []byte) error
	SendAcknowledgeMessage(log *slog.Logger, clientMessage message.ClientMessage) error
	AddDataToOutgoingMessageBuffer(streamMessage StreamingMessage)
	RemoveDataFromOutgoingMessageBuffer(streamMessageElement *list.Element)
	AddDataToIncomingMessageBuffer(streamMessage StreamingMessage)
	RemoveDataFromIncomingMessageBuffer(sequenceNumber int64)
	CalculateRetransmissionTimeout(streamingMessage StreamingMessage)
	SendMessage(input []byte, inputType int) error
	RegisterOutputStreamHandler(handler OutputStreamDataMessageHandler, isSessionSpecificHandler bool)
	DeregisterOutputStreamHandler(handler OutputStreamDataMessageHandler)
	IsSessionTypeSet() chan bool
	IsStreamMessageResendTimeout() chan bool
	GetSessionType() string
	SetSessionType(sessionType string)
	GetSessionProperties() interface{}
	GetWsChannel() communicator.IWebSocketChannel
	SetWsChannel(wsChannel communicator.IWebSocketChannel)
	GetStreamDataSequenceNumber() int64
	GetAgentVersion() string
	SetAgentVersion(agentVersion string)
}
