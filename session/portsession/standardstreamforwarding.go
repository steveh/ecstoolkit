// Package portsession starts port session.
package portsession

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/session"
)

// StandardStreamForwarding implements port forwarding using standard input and output streams.
type StandardStreamForwarding struct {
	inputStream    *os.File
	outputStream   *os.File
	portParameters PortParameters
	session        session.ISessionSubTypeSupport
	logger         log.T
}

// NewStandardStreamForwarding creates a new instance of StandardStreamForwarding.
func NewStandardStreamForwarding(
	sess session.ISessionSubTypeSupport,
	portParameters PortParameters,
	logger log.T,
) *StandardStreamForwarding {
	return &StandardStreamForwarding{
		session:        sess,
		portParameters: portParameters,
		logger:         logger,
	}
}

// Ensure StandardStreamForwarding implements IPortSession.
var _ IPortSession = (*StandardStreamForwarding)(nil)

// IsStreamNotSet checks if streams are not set.
func (p *StandardStreamForwarding) IsStreamNotSet() bool {
	return p.inputStream == nil || p.outputStream == nil
}

// Stop closes the streams.
func (p *StandardStreamForwarding) Stop() error {
	var errs []error

	if err := p.inputStream.Close(); err != nil {
		errs = append(errs, fmt.Errorf("closing input stream: %w", err))
	}

	if err := p.outputStream.Close(); err != nil {
		errs = append(errs, fmt.Errorf("closing output stream: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("stopping standard stream forwarding: %w", errors.Join(errs...))
	}

	return nil
}

// InitializeStreams initializes the streams with its file descriptors.
func (p *StandardStreamForwarding) InitializeStreams(_ context.Context, _ string) error {
	p.inputStream = os.Stdin
	p.outputStream = os.Stdout

	return nil
}

// ReadStream reads data from the input stream.
func (p *StandardStreamForwarding) ReadStream(_ context.Context) error {
	msg := make([]byte, config.StreamDataPayloadSize)

	for {
		numBytes, err := p.inputStream.Read(msg)
		if err != nil {
			return p.handleReadError(err)
		}

		p.logger.Trace("Received message from stdin", "size", numBytes)

		if err = p.session.SendInputDataMessage(message.Output, msg[:numBytes]); err != nil {
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
func (p *StandardStreamForwarding) handleReadError(err error) error {
	if errors.Is(err, io.EOF) {
		p.logger.Debug("Session to instance was closed", "targetID", p.session.GetTargetID(), "port", p.portParameters.PortNumber)

		return nil
	}

	return fmt.Errorf("reading input: %w", err)
}
