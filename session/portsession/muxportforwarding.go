// Package portsession starts port session.
package portsession

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/session"
	"github.com/steveh/ecstoolkit/session/sessionutil"
	"github.com/steveh/ecstoolkit/version"
	"github.com/xtaci/smux"
	"golang.org/x/sync/errgroup"
)

// MuxClient contains smux client session and corresponding network connection.
type MuxClient struct {
	conn    net.Conn
	session *smux.Session
}

// MgsConn contains local server and corresponding connection to smux client.
type MgsConn struct {
	listener net.Listener
	conn     net.Conn
}

// MuxPortForwarding is type of port session
// accepts multiple client connections through multiplexing.
type MuxPortForwarding struct {
	sessionID      string
	socketFile     string
	portParameters PortParameters
	session        session.ISessionSubTypeSupport
	muxClient      *MuxClient
	mgsConn        *MgsConn
	logger         log.T
}

// NewMuxPortForwarding creates a new MuxPortForwarding instance.
func NewMuxPortForwarding(
	sess session.ISessionSubTypeSupport,
	portParameters PortParameters,
	logger log.T,
) *MuxPortForwarding {
	return &MuxPortForwarding{
		portParameters: portParameters,
		session:        sess,
		sessionID:      sess.GetSessionID(),
		logger:         logger.With("subsystem", "MuxPortForwarding"),
	}
}

// Ensure MuxPortForwarding implements IPortSession.
var _ IPortSession = (*MuxPortForwarding)(nil)

func (c *MgsConn) close() error {
	var errs []error
	if err := c.listener.Close(); err != nil {
		errs = append(errs, fmt.Errorf("closing listener: %w", err))
	}

	if err := c.conn.Close(); err != nil {
		errs = append(errs, fmt.Errorf("closing connection: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("closing MgsConn: %w", errors.Join(errs...))
	}

	return nil
}

func (c *MuxClient) close() error {
	var errs []error
	if err := c.session.Close(); err != nil {
		errs = append(errs, fmt.Errorf("closing session: %w", err))
	}

	if err := c.conn.Close(); err != nil {
		errs = append(errs, fmt.Errorf("closing connection: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("closing MuxClient: %w", errors.Join(errs...))
	}

	return nil
}

// IsStreamNotSet checks if stream is not set.
func (p *MuxPortForwarding) IsStreamNotSet() bool {
	return p.muxClient.conn == nil
}

// Stop closes all open stream.
func (p *MuxPortForwarding) Stop() error {
	var errs []error

	if p.mgsConn != nil {
		if err := p.mgsConn.close(); err != nil {
			errs = append(errs, fmt.Errorf("closing MGS connection: %w", err))
		}
	}

	if p.muxClient != nil {
		if err := p.muxClient.close(); err != nil {
			errs = append(errs, fmt.Errorf("closing mux client: %w", err))
		}
	}

	if err := p.cleanUp(); err != nil {
		errs = append(errs, fmt.Errorf("cleaning up: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("stopping mux port forwarding: %w", errors.Join(errs...))
	}

	return nil
}

// InitializeStreams initializes i/o streams.
func (p *MuxPortForwarding) InitializeStreams(agentVersion string) error {
	socketFile, err := getUnixSocketPath(p.sessionID, os.TempDir(), "session_manager_plugin_mux.sock")
	if err != nil {
		return fmt.Errorf("getting unix socket path: %w", err)
	}

	p.socketFile = socketFile

	if err := p.initialize(agentVersion); err != nil {
		if cErr := p.cleanUp(); cErr != nil {
			p.logger.Error("Error cleaning up after initialization error", "error", cErr)
		}

		return fmt.Errorf("initializing streams: %w", err)
	}

	return nil
}

// ReadStream reads data from different connections.
func (p *MuxPortForwarding) ReadStream(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)

	// reads data from smux client and transfers to server over datachannel
	eg.Go(func() error {
		return p.transferDataToServer(ctx)
	})

	// set up network listener on SSM port and handle client connections
	eg.Go(func() error {
		return p.handleClientConnections(ctx)
	})

	if err := eg.Wait(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("waiting for goroutine group: %w", err)
	}

	return nil
}

// WriteStream writes data to stream.
func (p *MuxPortForwarding) WriteStream(outputMessage message.ClientMessage) error {
	//nolint:exhaustive
	switch message.PayloadType(outputMessage.PayloadType) {
	case message.Output:
		_, err := p.mgsConn.conn.Write(outputMessage.Payload)
		if err != nil {
			return fmt.Errorf("writing to MGS connection: %w", err)
		}

		return nil
	case message.Flag:
		var flag message.PayloadTypeFlag

		buf := bytes.NewBuffer(outputMessage.Payload)
		if err := binary.Read(buf, binary.BigEndian, &flag); err != nil {
			return fmt.Errorf("reading flag from buffer: %w", err)
		}

		if message.ConnectToPortError == flag {
			return fmt.Errorf("%w: connection to destination port failed", ErrConnectionFailed)
		}
	}

	return nil
}

// HandleControlSignals handles terminate signals.
func (p *MuxPortForwarding) HandleControlSignals(ctx context.Context) error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, sessionutil.ControlSignals...)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-signals:
			p.logger.Debug("Terminate signal received, exiting.")

			if err := p.session.SendFlag(message.TerminateSession); err != nil {
				return fmt.Errorf("sending TerminateSession flag: %w", err)
			}

			p.logger.Debug("Exiting session", "sessionID", p.sessionID)

			if err := p.Stop(); err != nil {
				return fmt.Errorf("stopping session: %w", err)
			}

			return nil
		}
	}
}

// cleanUp deletes unix socket file.
func (p *MuxPortForwarding) cleanUp() error {
	if err := os.Remove(p.socketFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing socket file: %w", err)
	}

	return nil
}

// initialize opens a network connection that acts as smux client.
func (p *MuxPortForwarding) initialize(agentVersion string) error {
	// open a network listener
	listener, err := sessionutil.NewListener(p.socketFile)
	if err != nil {
		return fmt.Errorf("creating new listener: %w", err)
	}

	var g errgroup.Group
	// start a go routine to accept connections on the network listener
	g.Go(func() error {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("accepting connection: %w", err)
		}

		p.mgsConn = &MgsConn{listener, conn}

		return nil
	})

	// start a connection to the local network listener and set up client side of mux
	g.Go(func() error {
		muxConn, err := net.Dial(listener.Addr().Network(), listener.Addr().String())
		if err != nil {
			return fmt.Errorf("dialing listener: %w", err)
		}

		smuxConfig := smux.DefaultConfig()

		disableKeepalive, err := version.DoesAgentSupportDisableSmuxKeepAlive(agentVersion)
		if err != nil {
			p.logger.Error("Checking smux keepalive support", "error", err)
		}

		if disableKeepalive {
			// Disable smux KeepAlive or else it breaks Session Manager idle timeout.
			smuxConfig.KeepAliveDisabled = true
		}

		muxSession, err := smux.Client(muxConn, smuxConfig)
		if err != nil {
			return fmt.Errorf("creating smux client: %w", err)
		}

		p.muxClient = &MuxClient{muxConn, muxSession}

		return nil
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("initializing mux port forwarding: %w", err)
	}

	return nil
}

// transferDataToServer reads from smux client connection and sends on data channel.
func (p *MuxPortForwarding) transferDataToServer(ctx context.Context) error {
	msg := make([]byte, config.StreamDataPayloadSize)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
			numBytes, err := p.mgsConn.conn.Read(msg)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return fmt.Errorf("connection closed: %w", err)
				}

				p.logger.Debug("Reading from port failed", "error", err)

				return fmt.Errorf("reading from MGS connection: %w", err)
			}

			p.logger.Trace("Received message from mux client", "size", numBytes)

			if err = p.session.SendInputDataMessage(message.Output, msg[:numBytes]); err != nil {
				return fmt.Errorf("sending input data message: %w", err)
			}

			// sleep to process more data
			time.Sleep(time.Millisecond)
		}
	}
}

// setupUnixListener sets up a Unix socket listener.
func (p *MuxPortForwarding) setupUnixListener() (net.Listener, error) {
	listener, err := net.Listen(p.portParameters.LocalConnectionType, p.portParameters.LocalUnixSocket)
	if err != nil {
		return nil, fmt.Errorf("listening on unix socket: %w", err)
	}

	p.logger.Info("Unix socket opened", "socket", p.portParameters.LocalUnixSocket)

	return listener, nil
}

// setupTCPListener sets up a TCP listener.
func (p *MuxPortForwarding) setupTCPListener() (net.Listener, error) {
	localPortNumber := p.portParameters.LocalPortNumber
	if p.portParameters.LocalPortNumber == "" {
		localPortNumber = "0"
	}

	listener, err := net.Listen("tcp", net.JoinHostPort("localhost", localPortNumber))
	if err != nil {
		return nil, fmt.Errorf("listening on TCP port: %w", err)
	}

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return nil, ErrNotTCPListener
	}

	p.portParameters.LocalPortNumber = strconv.Itoa(tcpAddr.Port)

	p.logger.Info("TCP port opened", "port", p.portParameters.LocalPortNumber)

	return listener, nil
}

// handleClientConnections sets up network server on local ssm port to accept connections from clients (browser/terminal).
func (p *MuxPortForwarding) handleClientConnections(ctx context.Context) error {
	var listener net.Listener

	var err error
	if p.portParameters.LocalConnectionType == "unix" {
		listener, err = p.setupUnixListener()
	} else {
		listener, err = p.setupTCPListener()
	}

	if err != nil {
		return err
	}

	defer func() {
		if cErr := listener.Close(); cErr != nil && !errors.Is(cErr, net.ErrClosed) {
			p.logger.Error("Error closing listener", "error", cErr)
		}
	}()

	p.logger.Debug("Waiting for connections")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg, ctx := errgroup.WithContext(ctx)

	// Accept connections from clients and handle them
	eg.Go(func() error {
		if err := p.serve(ctx, listener, cancel); err != nil {
			return fmt.Errorf("serving connections: %w", err)
		}

		return nil
	})

	// Close the listener when the context is done
	eg.Go(func() error {
		<-ctx.Done()

		if err := listener.Close(); err != nil {
			return fmt.Errorf("closing listener: %w", err)
		}

		return nil
	})

	if err := eg.Wait(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("waiting for goroutine group: %w", err)
	}

	return nil
}

func (p *MuxPortForwarding) serve(ctx context.Context, listener net.Listener, cancel context.CancelFunc) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("accepting connection: %w", err)
		}

		go func() {
			if err := p.acceptConnection(ctx, conn); err != nil {
				p.logger.Error("Error accepting connection", "error", err)

				cancel()
			}
		}()
	}
}

func (p *MuxPortForwarding) acceptConnection(ctx context.Context, conn net.Conn) error {
	p.logger.Trace("Connection accepted", "remoteAddr", conn.RemoteAddr(), "sessionID", p.sessionID)

	stream, err := p.muxClient.session.OpenStream()
	if err != nil {
		return fmt.Errorf("opening stream: %w", err)
	}

	p.logger.Trace("Client stream opened", "streamId", stream.ID())

	return handleDataTransfer(ctx, stream, conn)
}

// handleDataTransfer launches routines to transfer data between source and destination.
func handleDataTransfer(ctx context.Context, dst io.ReadWriteCloser, src io.ReadWriteCloser) error {
	eg, _ := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if _, err := io.Copy(dst, src); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
				return fmt.Errorf("copying from src to dst: %w", err)
			}
		}

		if err := dst.Close(); err != nil {
			if !errors.Is(err, io.EOF) {
				return fmt.Errorf("closing dst: %w", err)
			}
		}

		return nil
	})

	eg.Go(func() error {
		if _, err := io.Copy(src, dst); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
				return fmt.Errorf("copying from dst to src: %w", err)
			}
		}

		if err := src.Close(); err != nil {
			if !errors.Is(err, io.EOF) {
				return fmt.Errorf("closing src: %w", err)
			}
		}

		return nil
	})

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("waiting for goroutine group: %w", err)
	}

	return nil
}

// getUnixSocketPath generates the unix socket file name based on sessionID and returns the path.
func getUnixSocketPath(sessionID string, dir string, suffix string) (string, error) {
	hash := fnv.New32a()

	if _, err := hash.Write([]byte(sessionID)); err != nil {
		return "", fmt.Errorf("hashing sessionID: %w", err)
	}

	return filepath.Join(dir, fmt.Sprintf("%d_%s", hash.Sum32(), suffix)), nil
}
