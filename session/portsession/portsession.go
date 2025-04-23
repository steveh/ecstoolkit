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
	session.Session
	portParameters  PortParameters
	portSessionType IPortSession
}

var _ session.ISessionPlugin = (*PortSession)(nil)

// IPortSession defines the interface for port session operations.
type IPortSession interface {
	IsStreamNotSet() (status bool)
	InitializeStreams(ctx context.Context, log log.T, agentVersion string) (err error)
	ReadStream(ctx context.Context, log log.T) (err error)
	WriteStream(outputMessage message.ClientMessage) (err error)
	Stop(log log.T) error
}

// PortParameters contains the configuration parameters for port forwarding.
type PortParameters struct {
	PortNumber          string `json:"portNumber"`
	LocalPortNumber     string `json:"localPortNumber"`
	LocalUnixSocket     string `json:"localUnixSocket"`
	LocalConnectionType string `json:"localConnectionType"`
	Type                string `json:"type"`
}

// Name is the session name used inputStream the plugin.
func (s *PortSession) Name() string {
	return config.PortPluginName
}

// Initialize sets up the port session with the provided session parameters.
func (s *PortSession) Initialize(ctx context.Context, log log.T, sessionVar *session.Session) {
	s.Session = *sessionVar
	if err := jsonutil.Remarshal(s.SessionProperties, &s.portParameters); err != nil {
		log.Error("Invalid format", "error", err)
	}

	if s.portParameters.Type == LocalPortForwardingType {
		if version.DoesAgentSupportTCPMultiplexing(log, s.DataChannel.GetAgentVersion()) {
			s.portSessionType = &MuxPortForwarding{
				sessionID:      s.SessionID,
				portParameters: s.portParameters,
				session:        s.Session,
			}
		} else {
			s.portSessionType = &BasicPortForwarding{
				sessionID:      s.SessionID,
				portParameters: s.portParameters,
				session:        s.Session,
			}
		}
	} else {
		s.portSessionType = &StandardStreamForwarding{
			portParameters: s.portParameters,
			session:        s.Session,
		}
	}

	s.DataChannel.RegisterOutputStreamHandler(s.ProcessStreamMessagePayload, true)
	s.DataChannel.GetWsChannel().SetOnMessage(func(input []byte) {
		if s.portSessionType.IsStreamNotSet() {
			outputMessage := &message.ClientMessage{}
			if err := outputMessage.DeserializeClientMessage(log, input); err != nil {
				log.Warn("Ignore message deserialize error while stream connection had not set")

				return
			}

			if outputMessage.MessageType == message.OutputStreamMessage {
				log.Warn("Waiting for user to establish connection before processing incoming messages")

				return
			}

			log.Warn("Received message while establishing connection", "messageType", outputMessage.MessageType)
		}

		if err := s.DataChannel.OutputMessageHandler(ctx, log, s.Stop, s.SessionID, input); err != nil {
			log.Error("Failed to handle output message", "error", err)
		}
	})
	log.Debug("Connected to instance", "targetId", sessionVar.TargetID, "port", s.portParameters.PortNumber)
}

// Stop terminates the port session and cleans up resources.
func (s *PortSession) Stop(log log.T) error {
	if err := s.portSessionType.Stop(log); err != nil {
		return fmt.Errorf("stopping port session: %w", err)
	}

	return nil
}

// SetSessionHandlers redirects inputStream/outputStream data to datachannel.
func (s *PortSession) SetSessionHandlers(ctx context.Context, log log.T) error {
	var err error
	if err = s.portSessionType.InitializeStreams(ctx, log, s.DataChannel.GetAgentVersion()); err != nil {
		return fmt.Errorf("initializing streams: %w", err)
	}

	if err = s.portSessionType.ReadStream(ctx, log); err != nil {
		return fmt.Errorf("reading stream: %w", err)
	}

	return nil
}

// ProcessStreamMessagePayload writes messages received on datachannel to stdout.
func (s *PortSession) ProcessStreamMessagePayload(log log.T, outputMessage message.ClientMessage) (bool, error) {
	if s.portSessionType.IsStreamNotSet() {
		log.Debug("Waiting for streams to be established before processing incoming messages")

		return false, nil
	}

	log.Debug("Received payload from datachannel", "size", outputMessage.PayloadLength)

	if err := s.portSessionType.WriteStream(outputMessage); err != nil {
		return true, fmt.Errorf("writing stream: %w", err)
	}

	return true, nil
}
