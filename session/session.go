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
	retryParams := retry.RepeatableExponentialRetryer{
		GeometricRatio:      config.RetryBase,
		InitialDelayInMilli: rand.Intn(config.DataChannelRetryInitialDelayMillis) + config.DataChannelRetryInitialDelayMillis, //nolint:gosec
		MaxDelayInMilli:     config.DataChannelRetryMaxIntervalMillis,
		MaxAttempts:         config.DataChannelNumMaxRetries,
	}

	if err := s.dataChannel.OpenWithRetry(ctx, retryParams, s.DisplayMessage, s.getResumeSessionParams); err != nil {
		return fmt.Errorf("opening data channel: %w", err)
	}

	return nil
}

// EstablishSessionType ensures the channel session type is set.
func (s *Session) EstablishSessionType(ctx context.Context, sessionType string, sleepInterval time.Duration) (string, error) {
	sessionType, err := s.dataChannel.EstablishSessionType(ctx, sessionType, sleepInterval, s.TerminateSession)
	if err != nil {
		return "", fmt.Errorf("establishing session type: %w", err)
	}

	return sessionType, nil
}

// TerminateSession calls TerminateSession API.
func (s *Session) TerminateSession(ctx context.Context) error {
	terminateSessionInput := ssm.TerminateSessionInput{
		SessionId: &s.sessionID,
	}

	s.logger.Debug("Terminate Session input parameters", "input", terminateSessionInput)

	if _, err := s.ssmClient.TerminateSession(ctx, &terminateSessionInput); err != nil {
		return fmt.Errorf("terminating session: %w", err)
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

// getResumeSessionParams calls ResumeSession API and gets tokenvalue for reconnecting.
func (s *Session) getResumeSessionParams(ctx context.Context) (string, error) {
	resumeSessionInput := ssm.ResumeSessionInput{
		SessionId: aws.String(s.sessionID),
	}

	s.logger.Debug("Resume Session input parameters", "input", resumeSessionInput)

	resumeSessionOutput, err := s.ssmClient.ResumeSession(ctx, &resumeSessionInput)
	if err != nil {
		return "", fmt.Errorf("resuming session: %w", err)
	}

	if resumeSessionOutput.TokenValue == nil {
		return "", nil
	}

	return *resumeSessionOutput.TokenValue, nil
}
