// Package portsession starts port session.
package portsession

import (
	"context"
	"fmt"

	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/jsonutil"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/session"
	"github.com/steveh/ecstoolkit/version"
)

const (
	// LocalPortForwardingType represents the type for local port forwarding.
	LocalPortForwardingType = "LocalPortForwarding"
)

// PortSession represents a session for port forwarding.
type PortSession struct {
	session         session.ISessionTypeSupport
	portParameters  PortParameters
	portSessionType IPortSession
	logger          log.T
}

var _ session.ISessionPlugin = (*PortSession)(nil)

// IPortSession defines the interface for port session operations.
type IPortSession interface {
	IsStreamNotSet() (status bool)
	HandleControlSignals(ctx context.Context) (err error)
	InitializeStreams(agentVersion string) (err error)
	ReadStream(ctx context.Context) (err error)
	WriteStream(outputMessage message.ClientMessage) (err error)
	Stop() error
}

// PortParameters contains the configuration parameters for port forwarding.
type PortParameters struct {
	PortNumber          string `json:"portNumber"`
	LocalPortNumber     string `json:"localPortNumber"`
	LocalUnixSocket     string `json:"localUnixSocket"`
	LocalConnectionType string `json:"localConnectionType"`
	Type                string `json:"type"`
}

// NewPortSession initializes a new port session.
func NewPortSession(logger log.T, sess session.ISessionSupport) (*PortSession, error) {
	s := &PortSession{
		session: sess,
		logger:  logger.With("subsystem", "PortSession"),
	}

	if err := jsonutil.Remarshal(sess.GetSessionProperties(), &s.portParameters); err != nil {
		return nil, fmt.Errorf("remarshalling session properties: %w", err)
	}

	if s.portParameters.Type == LocalPortForwardingType {
		if version.DoesAgentSupportTCPMultiplexing(logger, sess.GetAgentVersion()) {
			s.portSessionType = NewMuxPortForwarding(sess, s.portParameters, logger)
		} else {
			s.portSessionType = NewBasicPortForwarding(sess, s.portParameters, logger)
		}
	} else {
		s.portSessionType = NewStandardStreamForwarding(sess, s.portParameters, logger)
	}

	sess.RegisterOutputStreamHandler(s.ProcessStreamMessagePayload, true)

	sess.RegisterStopHandler(s.Stop)

	sess.RegisterIncomingMessageHandler(func(input []byte) {
		if !s.portSessionType.IsStreamNotSet() {
			return
		}

		outputMessage := &message.ClientMessage{}
		if err := outputMessage.DeserializeClientMessage(input); err != nil {
			logger.Warn("Ignore message deserialize error while stream connection had not set")

			return
		}

		if outputMessage.MessageType == message.OutputStreamMessage {
			logger.Warn("Waiting for user to establish connection before processing incoming messages")

			return
		}

		logger.Warn("Received message while establishing connection", "messageType", outputMessage.MessageType)
	})

	logger.Debug("Connected to instance", "targetId", sess.GetTargetID(), "port", s.portParameters.PortNumber)

	return s, nil
}

// Name is the session name used inputStream the plugin.
func (s *PortSession) Name() string {
	return config.PortPluginName
}

// Stop terminates the port session and cleans up resources.
func (s *PortSession) Stop() error {
	if err := s.portSessionType.Stop(); err != nil {
		return fmt.Errorf("stopping port session: %w", err)
	}

	return nil
}

// SetSessionHandlers redirects inputStream/outputStream data to datachannel.
func (s *PortSession) SetSessionHandlers(ctx context.Context) error {
	go func() {
		if err := s.portSessionType.HandleControlSignals(ctx); err != nil {
			s.logger.Error("handling control signals", "error", err)
		}
	}()

	if err := s.portSessionType.InitializeStreams(s.session.GetAgentVersion()); err != nil {
		return fmt.Errorf("initializing streams: %w", err)
	}

	if err := s.portSessionType.ReadStream(ctx); err != nil {
		return fmt.Errorf("reading stream: %w", err)
	}

	return nil
}

// ProcessStreamMessagePayload writes messages received on datachannel to stdout.
func (s *PortSession) ProcessStreamMessagePayload(outputMessage message.ClientMessage) (bool, error) {
	if s.portSessionType.IsStreamNotSet() {
		s.logger.Debug("Waiting for streams to be established before processing incoming messages")

		return false, nil
	}

	s.logger.Trace("Received payload from datachannel", "size", outputMessage.PayloadLength)

	if err := s.portSessionType.WriteStream(outputMessage); err != nil {
		return true, fmt.Errorf("writing stream: %w", err)
	}

	return true, nil
}
