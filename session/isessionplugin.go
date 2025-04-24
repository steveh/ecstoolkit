package session

import (
	"context"

	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
)

// ISessionPlugin defines the interface for session type implementations.
type ISessionPlugin interface {
	SetSessionHandlers(ctx context.Context, log log.T) error
	ProcessStreamMessagePayload(log log.T, streamDataMessage message.ClientMessage) (isHandlerReady bool, err error)
	Stop() error
	Name() string
}
