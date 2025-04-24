package session

import (
	"context"

	"github.com/steveh/ecstoolkit/message"
)

// ISessionSubTypeSupport defines the interface for session plugin support operations.
type ISessionSubTypeSupport interface {
	SendFlag(flagType message.PayloadTypeFlag) error
	SendInputDataMessage(payloadType message.PayloadType, inputData []byte) error
	GetAgentVersion() string
	TerminateSession(ctx context.Context) error
	GetTargetID() string
}
