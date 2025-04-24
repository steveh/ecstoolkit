package session

import (
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
)

// ISession defines the interface for session operations.
type ISession interface {
	Execute(log log.T) error
	OpenDataChannel(log log.T) error
	ProcessFirstMessage(log log.T, outputMessage message.ClientMessage) (isHandlerReady bool, err error)
	Stop(log log.T) error
	GetResumeSessionParams(log log.T) (string, error)
	ResumeSessionHandler(log log.T) error
	TerminateSession(log log.T) error
}
