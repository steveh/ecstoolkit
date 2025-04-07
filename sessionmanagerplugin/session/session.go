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
	"github.com/steveh/ecstoolkit/sessionmanagerplugin/session/sessionutil"
)

var SessionRegistry = map[string]ISessionPlugin{}

type ISessionPlugin interface {
	SetSessionHandlers(ctx context.Context, log log.T) error
	ProcessStreamMessagePayload(log log.T, streamDataMessage message.ClientMessage) (isHandlerReady bool, err error)
	Initialize(ctx context.Context, log log.T, sessionVar *Session)
	Stop(log log.T) error
	Name() string
}

type ISession interface {
	Execute(log.T) error
	OpenDataChannel(log.T) error
	ProcessFirstMessage(log log.T, outputMessage message.ClientMessage) (isHandlerReady bool, err error)
	Stop(log log.T) error
	GetResumeSessionParams(log.T) (string, error)
	ResumeSessionHandler(log.T) error
	TerminateSession(log.T) error
}

func init() {
	SessionRegistry = make(map[string]ISessionPlugin)
}

func Register(session ISessionPlugin) {
	SessionRegistry[session.Name()] = session
}

type Session struct {
	DataChannel           datachannel.IDataChannel
	SessionId             string
	StreamUrl             string
	TokenValue            string
	IsAwsCliUpgradeNeeded bool
	Endpoint              string
	ClientId              string
	TargetId              string
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
				log.Errorf("Terminating session %s as the stream data was not processed before timeout.", session.SessionId)
				if err := session.TerminateSession(ctx, log); err != nil {
					log.Errorf("Unable to terminate session upon stream data timeout. %v", err)
				}

				return
			}
		}
	}()
}

// Execute create data channel and start the session.
func (s *Session) Execute(ctx context.Context, log log.T) (err error) {
	log.Infof("Starting session with SessionId: %s", s.SessionId)

	// sets the display mode
	s.DisplayMode = sessionutil.NewDisplayMode(log)

	if err = s.OpenDataChannel(ctx, log); err != nil {
		log.Errorf("Error in Opening data channel: %v", err)

		return err
	}

	handleStreamMessageResendTimeout(ctx, s, log)

	// The session type is set either by handshake or the first packet received.
	if !<-s.DataChannel.IsSessionTypeSet() {
		log.Errorf("unable to set SessionType for session %s", s.SessionId)

		return errors.New("unable to determine SessionType")
	} else {
		s.SessionType = s.DataChannel.GetSessionType()
		s.SessionProperties = s.DataChannel.GetSessionProperties()

		if err = setSessionHandlersWithSessionType(ctx, s, log); err != nil {
			log.Errorf("Session ending with error: %v", err)

			return err
		}
	}

	return err
}
