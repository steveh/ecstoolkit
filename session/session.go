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

// Package session starts the session.
package session

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/datachannel"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/retry"
	"github.com/steveh/ecstoolkit/session/sessionutil"
)

// SessionRegistry stores the mapping of session types to their implementations.
var SessionRegistry = map[string]ISessionPlugin{}

// ISessionPlugin defines the interface for session type implementations.
type ISessionPlugin interface {
	SetSessionHandlers(ctx context.Context, log log.T) error
	ProcessStreamMessagePayload(log log.T, streamDataMessage message.ClientMessage) (isHandlerReady bool, err error)
	Initialize(ctx context.Context, log log.T, sessionVar *Session)
	Stop(log log.T) error
	Name() string
}

// ISession defines the interface for session operations.
type ISession interface {
	Execute(log log.T) error
	OpenDataChannel(log log.T) error
	ProcessFirstMessage(log log.T, outputMessage message.ClientMessage) (isHandlerReady bool, err error)
	Stop(log log.T) error
	GetResumeSessionParams(log log.T) (string, error)
	ResumeSessionHandler(log log.T) error
	TerminateSession(log log.T) error
}

func init() {
	SessionRegistry = make(map[string]ISessionPlugin)
}

// Register adds a session plugin to the registry.
func Register(session ISessionPlugin) {
	SessionRegistry[session.Name()] = session
}

// Session represents an active session with its configuration and state.
type Session struct {
	DataChannel           datachannel.IDataChannel
	SessionID             string
	StreamURL             string
	TokenValue            string
	IsAwsCliUpgradeNeeded bool
	Endpoint              string
	ClientID              string
	TargetID              string
	SSMClient             *ssm.Client
	retryParams           retry.RepeatableExponentialRetryer
	SessionType           string
	SessionProperties     interface{}
	DisplayMode           sessionutil.DisplayMode
}

// setSessionHandlersWithSessionType set session handlers based on session subtype.
var setSessionHandlersWithSessionType = func(ctx context.Context, session *Session, log log.T) error {
	// SessionType is set inside DataChannel
	sessionSubType := SessionRegistry[session.SessionType]
	sessionSubType.Initialize(ctx, log, session)

	return sessionSubType.SetSessionHandlers(ctx, log)
}

// Set up a scheduler to listen on stream data resend timeout event.
var handleStreamMessageResendTimeout = func(ctx context.Context, session *Session, log log.T) {
	log.Debug("Setting up scheduler to listen on IsStreamMessageResendTimeout event.")
	go func() {
		for {
			// Repeat this loop for every 200ms
			time.Sleep(config.ResendSleepInterval)
			if <-session.DataChannel.IsStreamMessageResendTimeout() {
				log.Error("Stream data timeout", "sessionID", session.SessionID)
				if err := session.TerminateSession(ctx, log); err != nil {
					log.Error("Unable to terminate session upon stream data timeout", "error", err)
				}

				return
			}
		}
	}()
}

// Execute create data channel and start the session.
func (s *Session) Execute(ctx context.Context, log log.T) error {
	log.Debug("Starting session", "sessionID", s.SessionID)

	// sets the display mode
	s.DisplayMode = sessionutil.NewDisplayMode(log)

	if err := s.OpenDataChannel(ctx, log); err != nil {
		log.Error("Error opening data channel", "error", err)

		return err
	}

	handleStreamMessageResendTimeout(ctx, s, log)

	// The session type is set either by handshake or the first packet received.
	if !<-s.DataChannel.IsSessionTypeSet() {
		log.Error("Unable to set SessionType", "sessionID", s.SessionID)

		return errors.New("unable to determine SessionType")
	}

	s.SessionType = s.DataChannel.GetSessionType()
	s.SessionProperties = s.DataChannel.GetSessionProperties()

	if err := setSessionHandlersWithSessionType(ctx, s, log); err != nil {
		log.Error("Session ending with error", "error", err)

		return err
	}

	return nil
}
