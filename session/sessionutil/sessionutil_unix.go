// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

// Package sessionutil provides utility for sessions.
package sessionutil

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"

	"github.com/steveh/ecstoolkit/message"
)

// DisplayMode represents a display mode for Unix-like systems.
type DisplayMode struct{}

// InitDisplayMode initializes the display mode for Unix-like systems.
func (d *DisplayMode) InitDisplayMode(_ *slog.Logger) {
}

// DisplayMessage function displays the output on the screen.
func (d *DisplayMode) DisplayMessage(log *slog.Logger, message message.ClientMessage) {
	var out io.Writer = os.Stdout

	if _, err := fmt.Fprint(out, string(message.Payload)); err != nil {
		log.Error("Failed to write message to output", "error", err)
	}
}

// NewListener starts a new socket listener on the address.
func NewListener(_ *slog.Logger, address string) (net.Listener, error) {
	listener, err := net.Listen("unix", address)
	if err != nil {
		return nil, fmt.Errorf("creating unix socket listener: %w", err)
	}

	return listener, nil
}
