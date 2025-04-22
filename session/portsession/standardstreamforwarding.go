// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package portsession starts port session.
package portsession

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/session"
)

type StandardStreamForwarding struct {
	port           IPortSession
	inputStream    *os.File
	outputStream   *os.File
	portParameters PortParameters
	session        session.Session
}

// Ensure StandardStreamForwarding implements IPortSession.
var _ IPortSession = (*StandardStreamForwarding)(nil)

// IsStreamNotSet checks if streams are not set.
func (p *StandardStreamForwarding) IsStreamNotSet() bool {
	return p.inputStream == nil || p.outputStream == nil
}

// Stop closes the streams.
func (p *StandardStreamForwarding) Stop(log *slog.Logger) error {
	var errs []error

	if err := p.inputStream.Close(); err != nil {
		errs = append(errs, fmt.Errorf("closing input stream: %w", err))
	}

	if err := p.outputStream.Close(); err != nil {
		errs = append(errs, fmt.Errorf("closing output stream: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("stopping standard stream forwarding: %v", errs)
	}

	return nil
}

// InitializeStreams initializes the streams with its file descriptors.
func (p *StandardStreamForwarding) InitializeStreams(_ context.Context, log *slog.Logger, agentVersion string) error {
	p.inputStream = os.Stdin
	p.outputStream = os.Stdout

	return nil
}

// ReadStream reads data from the input stream.
func (p *StandardStreamForwarding) ReadStream(_ context.Context, log *slog.Logger) error {
	msg := make([]byte, config.StreamDataPayloadSize)

	for {
		numBytes, err := p.inputStream.Read(msg)
		if err != nil {
			return p.handleReadError(log, err)
		}

		log.Debug("Received message from stdin", "size", numBytes)

		if err = p.session.DataChannel.SendInputDataMessage(log, message.Output, msg[:numBytes]); err != nil {
			log.Error("sending packet", "error", err)

			return fmt.Errorf("sending input data message: %w", err)
		}
		// Sleep to process more data
		time.Sleep(time.Millisecond)
	}
}

// WriteStream writes data to output stream.
func (p *StandardStreamForwarding) WriteStream(outputMessage message.ClientMessage) error {
	_, err := p.outputStream.Write(outputMessage.Payload)
	if err != nil {
		return fmt.Errorf("writing to output stream: %w", err)
	}

	return nil
}

// handleReadError handles read error.
func (p *StandardStreamForwarding) handleReadError(log *slog.Logger, err error) error {
	if errors.Is(err, io.EOF) {
		log.Debug("Session to instance was closed", "targetId", p.session.TargetId, "port", p.portParameters.PortNumber)

		return nil
	}

	log.Error("Reading input failed", "error", err)

	return fmt.Errorf("reading input: %w", err)
}
