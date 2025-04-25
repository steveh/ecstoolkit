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
	GetSessionType() string
	Establish(ctx context.Context, sessionType string, sleepInterval time.Duration) error
}
