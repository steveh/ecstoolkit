package session

import (
	"github.com/steveh/ecstoolkit/message"
)

// ISession defines the interface for session operations.
type ISession interface {
	Execute() error
	OpenDataChannel() error
	ProcessFirstMessage(outputMessage message.ClientMessage) (isHandlerReady bool, err error)
	Stop() error
	GetResumeSessionParams() (string, error)
	ResumeSessionHandler() error
	TerminateSession() error
}
