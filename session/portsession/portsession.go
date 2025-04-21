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

	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/jsonutil"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/session"
	"github.com/steveh/ecstoolkit/version"
)

const (
	LocalPortForwardingType = "LocalPortForwarding"
)

type PortSession struct {
	session.Session
	portParameters  PortParameters
	portSessionType IPortSession
}

var _ session.ISessionPlugin = (*PortSession)(nil)

type IPortSession interface {
	IsStreamNotSet() (status bool)
	InitializeStreams(ctx context.Context, log log.T, agentVersion string) (err error)
	ReadStream(ctx context.Context, log log.T) (err error)
	WriteStream(outputMessage message.ClientMessage) (err error)
	Stop(log log.T) error
}

type PortParameters struct {
	PortNumber          string `json:"portNumber"`
	LocalPortNumber     string `json:"localPortNumber"`
	LocalUnixSocket     string `json:"localUnixSocket"`
	LocalConnectionType string `json:"localConnectionType"`
	Type                string `json:"type"`
}

// Name is the session name used inputStream the plugin.
func (PortSession) Name() string {
	return config.PortPluginName
}

func (s *PortSession) Initialize(ctx context.Context, log log.T, sessionVar *session.Session) {
	s.Session = *sessionVar
	if err := jsonutil.Remarshal(s.SessionProperties, &s.portParameters); err != nil {
		log.Error("Invalid format", "error", err)
	}

	if s.portParameters.Type == LocalPortForwardingType {
		if version.DoesAgentSupportTCPMultiplexing(log, s.DataChannel.GetAgentVersion()) {
			s.portSessionType = &MuxPortForwarding{
				sessionId:      s.SessionId,
				portParameters: s.portParameters,
				session:        s.Session,
			}
		} else {
			s.portSessionType = &BasicPortForwarding{
				sessionId:      s.SessionId,
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
				log.Debug("Ignore message deserialize error while stream connection had not set")

				return
			}

			if outputMessage.MessageType == message.OutputStreamMessage {
				log.Debug("Waiting for user to establish connection before processing incoming messages")

				return
			} else {
				log.Debug("Received message while establishing connection", "messageType", outputMessage.MessageType)
			}
		}

		s.DataChannel.OutputMessageHandler(ctx, log, s.Stop, s.SessionId, input)
	})
	log.Debug("Connected to instance", "targetId", sessionVar.TargetId, "port", s.portParameters.PortNumber)
}

func (s *PortSession) Stop(log log.T) error {
	return s.portSessionType.Stop(log)
}

// StartSession redirects inputStream/outputStream data to datachannel.
func (s *PortSession) SetSessionHandlers(ctx context.Context, log log.T) (err error) {
	if err = s.portSessionType.InitializeStreams(ctx, log, s.DataChannel.GetAgentVersion()); err != nil {
		return err
	}

	if err = s.portSessionType.ReadStream(ctx, log); err != nil {
		return err
	}

	return
}

// ProcessStreamMessagePayload writes messages received on datachannel to stdout.
func (s *PortSession) ProcessStreamMessagePayload(log log.T, outputMessage message.ClientMessage) (isHandlerReady bool, err error) {
	if s.portSessionType.IsStreamNotSet() {
		log.Debug("Waiting for streams to be established before processing incoming messages")

		return false, nil
	}

	log.Tracef("Received payload of size %d from datachannel.", outputMessage.PayloadLength)
	err = s.portSessionType.WriteStream(outputMessage)

	return true, err
}
