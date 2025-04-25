package session

import (
	"context"
	"time"
)

// ISession defines the interface for session operations.
type ISession interface {
	ISessionSubTypeSupport
	ISessionTypeSupport
	OpenDataChannel(ctx context.Context) error
	EstablishSessionType(ctx context.Context, sessionType string, sleepInterval time.Duration) (string, error)
}
