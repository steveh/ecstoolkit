// Package sessionutil provides utility for sessions.
package sessionutil

import (
	"fmt"
	"net"
)

// NewListener starts a new socket listener on the address.
func NewListener(address string) (net.Listener, error) {
	listener, err := net.Listen("unix", address)
	if err != nil {
		return nil, fmt.Errorf("creating unix socket listener: %w", err)
	}

	return listener, nil
}
