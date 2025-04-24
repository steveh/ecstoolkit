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
	"sync"
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
	session        session.Session
	muxClient      *MuxClient
	mgsConn        *MgsConn
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
		return fmt.Errorf("closing MgsConn: %v", errs)
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
		return fmt.Errorf("closing MuxClient: %v", errs)
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
		return fmt.Errorf("stopping mux port forwarding: %v", errs)
	}

	return nil
}

// InitializeStreams initializes i/o streams.
func (p *MuxPortForwarding) InitializeStreams(_ context.Context, log log.T, agentVersion string) error {
	p.handleControlSignals(log)

	socketFile, err := getUnixSocketPath(p.sessionID, os.TempDir(), "session_manager_plugin_mux.sock")
	if err != nil {
		return fmt.Errorf("getting unix socket path: %w", err)
	}

	p.socketFile = socketFile

	if err := p.initialize(log, agentVersion); err != nil {
		if cleanErr := p.cleanUp(); cleanErr != nil {
			log.Error("Failed to clean up after initialization error", "error", cleanErr)
		}

		return fmt.Errorf("initializing streams: %w", err)
	}

	return nil
}

// ReadStream reads data from different connections.
func (p *MuxPortForwarding) ReadStream(ctx context.Context, log log.T) error {
	g, ctx := errgroup.WithContext(ctx)

	// reads data from smux client and transfers to server over datachannel
	g.Go(func() error {
		return p.transferDataToServer(ctx, log)
	})

	// set up network listener on SSM port and handle client connections
	g.Go(func() error {
		return p.handleClientConnections(ctx, log)
	})

	if err := g.Wait(); err != nil {
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
			return errors.New("connection to destination port failed")
		}
	}

	return nil
}

// cleanUp deletes unix socket file.
func (p *MuxPortForwarding) cleanUp() error {
	if err := os.Remove(p.socketFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing socket file: %w", err)
	}

	return nil
}

// initialize opens a network connection that acts as smux client.
func (p *MuxPortForwarding) initialize(log log.T, agentVersion string) error {
	// open a network listener
	listener, err := sessionutil.NewListener(log, p.socketFile)
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
		if version.DoesAgentSupportDisableSmuxKeepAlive(log, agentVersion) {
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

// handleControlSignals handles terminate signals.
func (p *MuxPortForwarding) handleControlSignals(log log.T) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, sessionutil.ControlSignals...)

	go func() {
		<-c
		log.Debug("Terminate signal received, exiting.")

		if err := p.session.DataChannel.SendFlag(message.TerminateSession); err != nil {
			log.Error("sending TerminateSession flag", "error", err)
		}

		log.Debug("Exiting session", "sessionID", p.sessionID)

		if err := p.Stop(); err != nil {
			log.Error("Failed to stop session", "error", err)
		}
	}()
}

// transferDataToServer reads from smux client connection and sends on data channel.
func (p *MuxPortForwarding) transferDataToServer(ctx context.Context, log log.T) error {
	msg := make([]byte, config.StreamDataPayloadSize)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
			var numBytes int

			var err error

			if numBytes, err = p.mgsConn.conn.Read(msg); err != nil {
				log.Debug("Reading from port failed", "error", err)

				return fmt.Errorf("reading from MGS connection: %w", err)
			}

			log.Trace("Received message from mux client", "size", numBytes)

			if err = p.session.DataChannel.SendInputDataMessage(message.Output, msg[:numBytes]); err != nil {
				log.Error("sending packet on data channel", "error", err)

				return fmt.Errorf("sending input data message: %w", err)
			}
			// sleep to process more data
			time.Sleep(time.Millisecond)
		}
	}
}

// setupUnixListener sets up a Unix socket listener.
func (p *MuxPortForwarding) setupUnixListener() (net.Listener, string, error) {
	listener, err := net.Listen(p.portParameters.LocalConnectionType, p.portParameters.LocalUnixSocket)
	if err != nil {
		return nil, "", fmt.Errorf("listening on unix socket: %w", err)
	}

	displayMsg := fmt.Sprintf("Unix socket %s opened for sessionID %s.", p.portParameters.LocalUnixSocket, p.sessionID)

	return listener, displayMsg, nil
}

// setupTCPListener sets up a TCP listener.
func (p *MuxPortForwarding) setupTCPListener() (net.Listener, string, error) {
	localPortNumber := p.portParameters.LocalPortNumber
	if p.portParameters.LocalPortNumber == "" {
		localPortNumber = "0"
	}

	listener, err := net.Listen("tcp", "localhost:"+localPortNumber)
	if err != nil {
		return nil, "", fmt.Errorf("listening on TCP port: %w", err)
	}

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return nil, "", errors.New("failed to type assert listener.Addr() to *net.TCPAddr")
	}

	p.portParameters.LocalPortNumber = strconv.Itoa(tcpAddr.Port)
	displayMsg := fmt.Sprintf("Port %s opened for sessionID %s.", p.portParameters.LocalPortNumber, p.sessionID)

	return listener, displayMsg, nil
}

// handleClientConnections sets up network server on local ssm port to accept connections from clients (browser/terminal).
func (p *MuxPortForwarding) handleClientConnections(ctx context.Context, log log.T) (err error) {
	var (
		listener   net.Listener
		displayMsg string
	)

	if p.portParameters.LocalConnectionType == "unix" {
		listener, displayMsg, err = p.setupUnixListener()
	} else {
		listener, displayMsg, err = p.setupTCPListener()
	}

	if err != nil {
		return err
	}

	defer func() {
		if closeErr := listener.Close(); closeErr != nil {
			log.Error("Failed to close listener", "error", closeErr)

			if err == nil {
				err = closeErr
			}
		}
	}()

	log.Debug(displayMsg)
	log.Debug("Waiting for connections")

	var once sync.Once

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
			if conn, err := listener.Accept(); err != nil {
				log.Error("Error while accepting connection", "error", err)
			} else {
				log.Debug("Connection accepted", "remoteAddr", conn.RemoteAddr(), "sessionID", p.sessionID)

				once.Do(func() {
					log.Debug("Connection accepted", "sessionID", p.sessionID)
				})

				stream, err := p.muxClient.session.OpenStream()
				if err != nil {
					log.Error("opening stream", "error", err)

					continue
				}

				log.Debug("Client stream opened", "streamId", stream.ID())

				go handleDataTransfer(stream, conn)
			}
		}
	}
}

// handleDataTransfer launches routines to transfer data between source and destination.
func handleDataTransfer(dst io.ReadWriteCloser, src io.ReadWriteCloser) {
	var wait sync.WaitGroup

	errChan := make(chan error, 2) //nolint:mnd

	wait.Add(2) //nolint:mnd

	go func() {
		defer wait.Done()

		if _, err := io.Copy(dst, src); err != nil {
			errChan <- fmt.Errorf("error copying from src to dst: %w", err)

			return
		}

		if err := dst.Close(); err != nil {
			errChan <- fmt.Errorf("error closing dst: %w", err)
		}
	}()

	go func() {
		defer wait.Done()

		if _, err := io.Copy(src, dst); err != nil {
			errChan <- fmt.Errorf("error copying from dst to src: %w", err)

			return
		}

		if err := src.Close(); err != nil {
			errChan <- fmt.Errorf("error closing src: %w", err)
		}
	}()

	wait.Wait()
	close(errChan)
}

// getUnixSocketPath generates the unix socket file name based on sessionID and returns the path.
func getUnixSocketPath(sessionID string, dir string, suffix string) (string, error) {
	hash := fnv.New32a()

	if _, err := hash.Write([]byte(sessionID)); err != nil {
		return "", fmt.Errorf("hashing sessionID: %w", err)
	}

	return filepath.Join(dir, fmt.Sprintf("%d_%s", hash.Sum32(), suffix)), nil
}
