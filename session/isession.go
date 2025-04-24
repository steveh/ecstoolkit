package session

import (
	"context"
)

// ISession defines the interface for session operations.
type ISession interface {
	ISessionSubTypeSupport
	ISessionTypeSupport
	OpenDataChannel(ctx context.Context) error
}
