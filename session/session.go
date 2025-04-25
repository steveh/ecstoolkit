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
	"fmt"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/datachannel"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/retry"
	"github.com/steveh/ecstoolkit/session/sessionutil"
)

// Session represents an active session with its configuration and state.
type Session struct {
	ssmClient   *ssm.Client
	dataChannel datachannel.IDataChannel
	displayMode sessionutil.DisplayMode
	retryParams retry.RepeatableExponentialRetryer
	logger      log.T
	sessionID   string
}

var _ ISession = (*Session)(nil)

// NewSession creates a new session instance with the provided parameters.
func NewSession(ssmClient *ssm.Client, dataChannel datachannel.IDataChannel, sessionID string, logger log.T) (*Session, error) {
	session := &Session{
		dataChannel: dataChannel,
		sessionID:   sessionID,
		displayMode: sessionutil.NewDisplayMode(logger),
		ssmClient:   ssmClient,
		logger:      logger,
	}

	return session, nil
}

// OpenDataChannel initializes datachannel.
func (s *Session) OpenDataChannel(ctx context.Context) error {
	s.retryParams = retry.RepeatableExponentialRetryer{
		GeometricRatio:      config.RetryBase,
		InitialDelayInMilli: rand.Intn(config.DataChannelRetryInitialDelayMillis) + config.DataChannelRetryInitialDelayMillis, //nolint:gosec
		MaxDelayInMilli:     config.DataChannelRetryMaxIntervalMillis,
		MaxAttempts:         config.DataChannelNumMaxRetries,
	}

	s.dataChannel.RegisterOutputMessageHandler(ctx, func() error { return nil }, func(_ []byte) {})

	s.dataChannel.RegisterOutputStreamHandler(s.processFirstMessage, false)

	if err := s.dataChannel.Open(); err != nil {
		s.logger.Error("Retrying connection failed", "sessionID", s.sessionID, "error", err)

		s.retryParams.CallableFunc = func() error {
			return s.dataChannel.Reconnect()
		}

		if err := s.retryParams.Call(); err != nil {
			s.logger.Error("Failed to call retry parameters", "error", err)
		}
	}

	s.dataChannel.SetOnError(func(wsErr error) {
		s.logger.Error("Trying to reconnect session", "sequenceNumber", s.dataChannel.GetStreamDataSequenceNumber(), "error", wsErr)

		s.retryParams.CallableFunc = func() error {
			return s.resumeSessionHandler(ctx)
		}

		if err := s.retryParams.Call(); err != nil {
			s.logger.Error("Reconnect error", "error", err)
		}
	})

	// Scheduler for resending of data
	if err := s.dataChannel.ResendStreamDataMessageScheduler(); err != nil {
		s.logger.Error("Failed to schedule resend of stream data messages", "error", err)
	}

	return nil
}

// EstablishSessionType ensures the channel session type is set.
func (s *Session) EstablishSessionType(ctx context.Context, sessionType string, sleepInterval time.Duration) (string, error) {
	s.dataChannel.SetSessionType(sessionType)

	go func() {
		for {
			// Repeat this loop for every 200ms
			time.Sleep(sleepInterval)

			if <-s.dataChannel.IsStreamMessageResendTimeout() {
				s.logger.Error("Stream data timeout", "sessionID", s.GetSessionID())

				if err := s.TerminateSession(ctx); err != nil {
					s.logger.Error("Unable to terminate session upon stream data timeout", "error", err)
				}

				return
			}
		}
	}()

	// The session type is set either by handshake or the first packet received.
	if !<-s.dataChannel.IsSessionTypeSet() {
		return "", errors.New("unable to determine session type")
	}

	return s.dataChannel.GetSessionType(), nil
}

// TerminateSession calls TerminateSession API.
func (s *Session) TerminateSession(ctx context.Context) error {
	terminateSessionInput := ssm.TerminateSessionInput{
		SessionId: &s.sessionID,
	}

	s.logger.Debug("Terminate Session input parameters", "input", terminateSessionInput)

	if _, err := s.ssmClient.TerminateSession(ctx, &terminateSessionInput); err != nil {
		s.logger.Error("Terminate Session failed", "error", err)

		return fmt.Errorf("terminate session: %w", err)
	}

	return nil
}

// GetAgentVersion retrieves the agent version from the data channel.
func (s *Session) GetAgentVersion() string {
	return s.dataChannel.GetAgentVersion()
}

// GetTargetID retrieves the target ID from the session.
func (s *Session) GetTargetID() string {
	return s.dataChannel.GetTargetID()
}

// SendFlag sends a flag message through the data channel.
func (s *Session) SendFlag(flagType message.PayloadTypeFlag) error {
	if err := s.dataChannel.SendFlag(flagType); err != nil {
		return fmt.Errorf("sending flag: %w", err)
	}

	return nil
}

// SendInputDataMessage sends input data messages through the data channel.
func (s *Session) SendInputDataMessage(payloadType message.PayloadType, inputData []byte) error {
	if err := s.dataChannel.SendInputDataMessage(payloadType, inputData); err != nil {
		return fmt.Errorf("sending input data message: %w", err)
	}

	return nil
}

// Close closes the data channel.
func (s *Session) Close() error {
	if err := s.dataChannel.Close(); err != nil {
		return fmt.Errorf("closing data channel: %w", err)
	}

	return nil
}

// DisplayMessage displays a message to the user.
func (s *Session) DisplayMessage(message message.ClientMessage) {
	s.displayMode.DisplayMessage(message)
}

// GetSessionID retrieves the session ID from the session.
func (s *Session) GetSessionID() string {
	return s.sessionID
}

// GetSessionProperties retrieves the session properties from the session.
func (s *Session) GetSessionProperties() any {
	return s.dataChannel.GetSessionProperties()
}

// RegisterOutputMessageHandler registers a handler for output messages.
func (s *Session) RegisterOutputMessageHandler(ctx context.Context, stopHandler datachannel.Stop, onMessageHandler func(input []byte)) {
	s.dataChannel.RegisterOutputMessageHandler(ctx, stopHandler, onMessageHandler)
}

// RegisterOutputStreamHandler registers a handler for output stream messages.
func (s *Session) RegisterOutputStreamHandler(handler datachannel.OutputStreamDataMessageHandler, isSessionSpecificHandler bool) {
	s.dataChannel.RegisterOutputStreamHandler(handler, isSessionSpecificHandler)
}

// processFirstMessage only processes messages with PayloadType Output to determine the
// sessionType of the session to be launched. This is a fallback for agent versions that do not support handshake, they
// immediately start sending shell output.
func (s *Session) processFirstMessage(outputMessage message.ClientMessage) (bool, error) {
	// Immediately deregister self so that this handler is only called once, for the first message
	s.dataChannel.DeregisterOutputStreamHandler(s.processFirstMessage)
	// Only set session type if the session type has not already been set. Usually session type will be set
	// by handshake protocol which would be the first message but older agents may not perform handshake
	if s.dataChannel.GetSessionType() == "" {
		if outputMessage.PayloadType == uint32(message.Output) {
			s.logger.Warn("Setting session type to shell based on PayloadType!")
			s.dataChannel.SetSessionType(config.ShellPluginName)
			s.displayMode.DisplayMessage(outputMessage)
		}
	}

	return true, nil
}

// getResumeSessionParams calls ResumeSession API and gets tokenvalue for reconnecting.
func (s *Session) getResumeSessionParams(ctx context.Context) (string, error) {
	resumeSessionInput := ssm.ResumeSessionInput{
		SessionId: aws.String(s.sessionID),
	}

	s.logger.Debug("Resume Session input parameters", "input", resumeSessionInput)

	resumeSessionOutput, err := s.ssmClient.ResumeSession(ctx, &resumeSessionInput)
	if err != nil {
		s.logger.Error("Resume Session failed", "error", err)

		return "", fmt.Errorf("resume session: %w", err)
	}

	if resumeSessionOutput.TokenValue == nil {
		return "", nil
	}

	return *resumeSessionOutput.TokenValue, nil
}

// resumeSessionHandler gets token value and tries to Reconnect to datachannel.
func (s *Session) resumeSessionHandler(ctx context.Context) error {
	tokenValue, err := s.getResumeSessionParams(ctx)
	if err != nil {
		s.logger.Error("getting token", "error", err)

		return fmt.Errorf("resume session: %w", err)
	}

	if tokenValue == "" {
		s.logger.Debug("Session timed out", "sessionID", s.sessionID)

		return errors.New("session timed out")
	}

	s.dataChannel.SetChannelToken(tokenValue)

	if err := s.dataChannel.Reconnect(); err != nil {
		return fmt.Errorf("reconnecting data channel: %w", err)
	}

	return nil
}
