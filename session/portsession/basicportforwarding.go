// Package portsession starts port session.
package portsession

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/session"
	"github.com/steveh/ecstoolkit/session/sessionutil"
	"github.com/steveh/ecstoolkit/version"
)

var (
	// ErrNotTCPListener is returned when the listener is not a TCP listener.
	ErrNotTCPListener = errors.New("not a TCP listener")

	// ErrConnectionFailed is returned when the connection fails.
	ErrConnectionFailed = errors.New("connection failed")
)

// BuildListenerFunc builds a listener for the given host and port.
type BuildListenerFunc func(host string, port string) (net.Listener, error)

// AcceptConnectionFunc returns connection to the listener.
type AcceptConnectionFunc func(listener net.Listener) (net.Conn, error)

// BasicPortForwarding is type of port session
// accepts one client connection at a time.
type BasicPortForwarding struct {
	stream           net.Conn
	listener         net.Listener
	sessionID        string
	portParameters   PortParameters
	session          session.ISessionSubTypeSupport
	logger           log.T
	buildListener    BuildListenerFunc
	acceptConnection AcceptConnectionFunc
	mu               sync.Mutex
}

// NewBasicPortForwarding creates a new BasicPortForwarding instance.
func NewBasicPortForwarding(
	sess session.ISessionSubTypeSupport,
	portParameters PortParameters,
	logger log.T,
) *BasicPortForwarding {
	return &BasicPortForwarding{
		portParameters: portParameters,
		session:        sess,
		sessionID:      sess.GetSessionID(),
		logger:         logger,
		buildListener:  net.Listen,
		acceptConnection: func(listener net.Listener) (net.Conn, error) {
			return listener.Accept()
		},
	}
}

// Ensure BasicPortForwarding implements IPortSession.
var _ IPortSession = (*BasicPortForwarding)(nil)

// IsStreamNotSet checks if stream is not set.
func (p *BasicPortForwarding) IsStreamNotSet() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.stream == nil
}

// Stop closes the stream.
func (p *BasicPortForwarding) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stream != nil {
		if err := p.stream.Close(); err != nil {
			return fmt.Errorf("closing stream: %w", err)
		}
	}

	return nil
}

// InitializeStreams establishes connection and initializes the stream.
func (p *BasicPortForwarding) InitializeStreams(ctx context.Context, _ string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.handleControlSignals(ctx)

	if err := p.startLocalConn(); err != nil {
		return err
	}

	return nil
}

// ReadStream reads data from the stream.
func (p *BasicPortForwarding) ReadStream(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	msg := make([]byte, config.StreamDataPayloadSize)

	for {
		numBytes, err := p.stream.Read(msg)
		if err != nil {
			p.logger.Debug("Reading from port failed", "port", p.portParameters.PortNumber, "error", err)

			// Send DisconnectToPort flag to agent when client tcp connection drops to ensure agent closes tcp connection too with server port
			if err := p.session.SendFlag(message.DisconnectToPort); err != nil {
				return fmt.Errorf("sending disconnect flag: %w", err)
			}

			if err := p.reconnect(); err != nil {
				return fmt.Errorf("reconnecting: %w", err)
			}

			// continue to read from connection as it has been re-established
			continue
		}

		p.logger.Trace("Received message from stdin", "size", numBytes)

		if err := p.session.SendInputDataMessage(message.Output, msg[:numBytes]); err != nil {
			p.logger.Error("sending packet", "error", err)

			return fmt.Errorf("sending input data message: %w", err)
		}
		// Sleep to process more data
		time.Sleep(time.Millisecond)
	}
}

// WriteStream writes data to stream.
func (p *BasicPortForwarding) WriteStream(outputMessage message.ClientMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, err := p.stream.Write(outputMessage.Payload); err != nil {
		return fmt.Errorf("writing to stream: %w", err)
	}

	return nil
}

// startLocalConn establishes a new local connection to forward remote server packets to.
func (p *BasicPortForwarding) startLocalConn() error {
	// When localPortNumber is not specified, set port number to 0 to let net.conn choose an open port at random
	localPortNumber := p.portParameters.LocalPortNumber
	if p.portParameters.LocalPortNumber == "" {
		localPortNumber = "0"
	}

	listener, err := p.startLocalListener(localPortNumber)
	if err != nil {
		p.logger.Error("Unable to open tcp connection to port", "error", err)

		return fmt.Errorf("starting local listener: %w", err)
	}

	tcpConn, err := p.acceptConnection(listener)
	if err != nil {
		p.logger.Error("accepting connection", "error", err)

		return fmt.Errorf("accepting connection: %w", err)
	}

	p.logger.Debug("Connection accepted", "sessionID", p.sessionID)

	p.listener = listener
	p.stream = tcpConn

	return nil
}

// startLocalListener starts a local listener to given address.
func (p *BasicPortForwarding) startLocalListener(portNumber string) (net.Listener, error) {
	var displayMessage string

	var listener net.Listener

	var err error

	switch p.portParameters.LocalConnectionType {
	case "unix":
		listener, err = p.buildListener(p.portParameters.LocalConnectionType, p.portParameters.LocalUnixSocket)
		if err != nil {
			return nil, err
		}

		displayMessage = fmt.Sprintf("Unix socket %s opened for sessionID %s.", p.portParameters.LocalUnixSocket, p.sessionID)
	default:
		listener, err = p.buildListener("tcp", "localhost:"+portNumber)
		if err != nil {
			return nil, err
		}
		// get port number the TCP listener opened
		tcpAddr, ok := listener.Addr().(*net.TCPAddr)
		if !ok {
			return nil, ErrNotTCPListener
		}

		p.portParameters.LocalPortNumber = strconv.Itoa(tcpAddr.Port)
		displayMessage = fmt.Sprintf("Port %s opened for sessionID %s.", p.portParameters.LocalPortNumber, p.sessionID)
	}

	p.logger.Debug(displayMessage)

	return listener, nil
}

// handleControlSignals handles terminate signals.
func (p *BasicPortForwarding) handleControlSignals(ctx context.Context) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, sessionutil.ControlSignals...)

	go func() {
		<-c
		p.logger.Debug("Terminate signal received, exiting.")

		if version.DoesAgentSupportTerminateSessionFlag(p.logger, p.session.GetAgentVersion()) {
			if err := p.session.SendFlag(message.TerminateSession); err != nil {
				p.logger.Error("sending TerminateSession flag", "error", err)
			}

			p.logger.Debug("Exiting session", "sessionID", p.sessionID)

			if err := p.Stop(); err != nil {
				p.logger.Error("stopping session", "error", err)
			}
		} else {
			if err := p.session.TerminateSession(ctx); err != nil {
				p.logger.Error("terminating session", "error", err)
			}
		}
	}()
}

// reconnect closes existing connection, listens to new connection and accept it.
func (p *BasicPortForwarding) reconnect() error {
	// close existing connection as it is in a state from which data cannot be read
	if err := p.stream.Close(); err != nil {
		p.logger.Error("closing existing stream", "error", err)
	}

	// wait for new connection on listener and accept it
	conn, err := p.acceptConnection(p.listener)
	if err != nil {
		return fmt.Errorf("accepting connection: %w", err)
	}

	p.stream = conn

	return nil
}
