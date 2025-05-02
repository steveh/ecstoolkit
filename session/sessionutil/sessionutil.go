// Package sessionutil provides utility for sessions.
package sessionutil

import "github.com/steveh/ecstoolkit/log"

// NewDisplayMode creates and initializes a new DisplayMode instance.
func NewDisplayMode(logger log.T) DisplayMode {
	displayMode := DisplayMode{
		logger: logger,
	}

	return displayMode
}
