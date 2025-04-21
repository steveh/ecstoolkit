// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/session"
	"github.com/steveh/ecstoolkit/session/sessionutil"
	"github.com/steveh/ecstoolkit/version"
)

// BasicPortForwarding is type of port session
// accepts one client connection at a time.
type BasicPortForwarding struct {
	port           IPortSession
	stream         *net.Conn
	listener       *net.Listener
	sessionId      string
	portParameters PortParameters
	session        session.Session
}

// Ensure BasicPortForwarding implements IPortSession.
var _ IPortSession = (*BasicPortForwarding)(nil)

// getNewListener returns a new listener to given address and type like tcp, unix etc.
var getNewListener = func(listenerType string, listenerAddress string) (listener net.Listener, err error) {
	return net.Listen(listenerType, listenerAddress)
}

// acceptConnection returns connection to the listener.
var acceptConnection = func(log *slog.Logger, listener net.Listener) (tcpConn net.Conn, err error) {
	return listener.Accept()
}

// IsStreamNotSet checks if stream is not set.
func (p *BasicPortForwarding) IsStreamNotSet() (status bool) {
	return p.stream == nil
}

// Stop closes the stream.
func (p *BasicPortForwarding) Stop(log *slog.Logger) error {
	if p.stream != nil {
		(*p.stream).Close()
	}

	return nil
}

// InitializeStreams establishes connection and initializes the stream.
func (p *BasicPortForwarding) InitializeStreams(ctx context.Context, log *slog.Logger, agentVersion string) (err error) {
	p.handleControlSignals(ctx, log)

	if err = p.startLocalConn(log); err != nil {
		return
	}

	return
}

// ReadStream reads data from the stream.
func (p *BasicPortForwarding) ReadStream(ctx context.Context, log *slog.Logger) (err error) {
	msg := make([]byte, config.StreamDataPayloadSize)

	for {
		numBytes, err := (*p.stream).Read(msg)
		if err != nil {
			log.Debug("Reading from port failed", "port", p.portParameters.PortNumber, "error", err)

			// Send DisconnectToPort flag to agent when client tcp connection drops to ensure agent closes tcp connection too with server port
			if err = p.session.DataChannel.SendFlag(log, message.DisconnectToPort); err != nil {
				log.Error("Failed to send packet", "error", err)

				return err
			}

			if err = p.reconnect(log); err != nil {
				return err
			}

			// continue to read from connection as it has been re-established
			continue
		}

		log.Debug("Received message from stdin", "size", numBytes)

		if err = p.session.DataChannel.SendInputDataMessage(log, message.Output, msg[:numBytes]); err != nil {
			log.Error("Failed to send packet", "error", err)

			return err
		}
		// Sleep to process more data
		time.Sleep(time.Millisecond)
	}
}

// WriteStream writes data to stream.
func (p *BasicPortForwarding) WriteStream(outputMessage message.ClientMessage) error {
	_, err := (*p.stream).Write(outputMessage.Payload)

	return err
}

// startLocalConn establishes a new local connection to forward remote server packets to.
func (p *BasicPortForwarding) startLocalConn(log *slog.Logger) (err error) {
	// When localPortNumber is not specified, set port number to 0 to let net.conn choose an open port at random
	localPortNumber := p.portParameters.LocalPortNumber
	if p.portParameters.LocalPortNumber == "" {
		localPortNumber = "0"
	}

	var listener net.Listener

	if listener, err = p.startLocalListener(log, localPortNumber); err != nil {
		log.Error("Unable to open tcp connection to port", "error", err)

		return err
	}

	var tcpConn net.Conn

	if tcpConn, err = acceptConnection(log, listener); err != nil {
		log.Error("Failed to accept connection", "error", err)

		return err
	}

	log.Debug("Connection accepted", "sessionId", p.sessionId)

	p.listener = &listener
	p.stream = &tcpConn

	return
}

// startLocalListener starts a local listener to given address.
func (p *BasicPortForwarding) startLocalListener(log *slog.Logger, portNumber string) (listener net.Listener, err error) {
	var displayMessage string

	switch p.portParameters.LocalConnectionType {
	case "unix":
		if listener, err = getNewListener(p.portParameters.LocalConnectionType, p.portParameters.LocalUnixSocket); err != nil {
			return
		}

		displayMessage = fmt.Sprintf("Unix socket %s opened for sessionId %s.", p.portParameters.LocalUnixSocket, p.sessionId)
	default:
		if listener, err = getNewListener("tcp", "localhost:"+portNumber); err != nil {
			return
		}
		// get port number the TCP listener opened
		p.portParameters.LocalPortNumber = strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
		displayMessage = fmt.Sprintf("Port %s opened for sessionId %s.", p.portParameters.LocalPortNumber, p.sessionId)
	}

	log.Debug(displayMessage)

	return
}

// handleControlSignals handles terminate signals.
func (p *BasicPortForwarding) handleControlSignals(ctx context.Context, log *slog.Logger) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, sessionutil.ControlSignals...)

	go func() {
		<-c
		log.Debug("Terminate signal received, exiting.")

		if version.DoesAgentSupportTerminateSessionFlag(log, p.session.DataChannel.GetAgentVersion()) {
			if err := p.session.DataChannel.SendFlag(log, message.TerminateSession); err != nil {
				log.Error("Failed to send TerminateSession flag", "error", err)
			}

			log.Debug("Exiting session", "sessionId", p.sessionId)
			p.Stop(log)
		} else {
			p.session.TerminateSession(ctx, log)
		}
	}()
}

// reconnect closes existing connection, listens to new connection and accept it.
func (p *BasicPortForwarding) reconnect(log *slog.Logger) (err error) {
	// close existing connection as it is in a state from which data cannot be read
	(*p.stream).Close()

	// wait for new connection on listener and accept it
	var conn net.Conn

	if conn, err = acceptConnection(log, *p.listener); err != nil {
		return fmt.Errorf("Failed to accept connection with error. %w", err)
	}

	p.stream = &conn

	return
}
