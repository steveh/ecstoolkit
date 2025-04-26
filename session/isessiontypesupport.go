package session

import (
	"context"

	"github.com/steveh/ecstoolkit/datachannel"
	"github.com/steveh/ecstoolkit/message"
)

// ISessionTypeSupport is an interface that defines the methods for session type support.
type ISessionTypeSupport interface {
	GetAgentVersion() string
	GetSessionID() string
	GetSessionProperties() any
	RegisterOutputStreamHandler(handler datachannel.OutputStreamDataMessageHandler, isSessionSpecificHandler bool)
	RegisterOutputMessageHandler(ctx context.Context, stopHandler datachannel.StopHandler, onMessageHandler func(input []byte))
	DisplayMessage(message message.ClientMessage)
	SendInputDataMessage(payloadType message.PayloadType, inputData []byte) error
	Close() error
}
