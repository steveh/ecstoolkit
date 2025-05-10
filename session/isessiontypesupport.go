package session

import (
	"github.com/steveh/ecstoolkit/datachannel"
	"github.com/steveh/ecstoolkit/message"
)

// ISessionTypeSupport is an interface that defines the methods for session type support.
type ISessionTypeSupport interface {
	GetAgentVersion() string
	GetSessionID() string
	GetSessionProperties() any
	RegisterOutputStreamHandler(handler datachannel.OutputStreamDataMessageHandler, isSessionSpecificHandler bool)
	RegisterIncomingMessageHandler(handler datachannel.IncomingMessageHandler)
	RegisterStopHandler(handler datachannel.StopHandler)
	DisplayMessage(message message.ClientMessage)
	SendInputDataMessage(payloadType message.PayloadType, inputData []byte) error
	Close() error
}
