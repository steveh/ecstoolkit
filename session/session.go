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
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/google/uuid"
	"github.com/steveh/ecstoolkit/communicator"
	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/datachannel"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/retry"
	"github.com/steveh/ecstoolkit/session/sessionutil"
)

// Session represents an active session with its configuration and state.
type Session struct {
	DataChannel           datachannel.IDataChannel
	SessionID             string
	TargetID              string
	SessionType           string
	SessionProperties     interface{}
	DisplayMode           sessionutil.DisplayMode
	streamURL             string
	tokenValue            string
	isAwsCliUpgradeNeeded bool
	endpoint              string
	clientID              string
	ssmClient             *ssm.Client
	kmsClient             *kms.Client
	retryParams           retry.RepeatableExponentialRetryer
}

// NewSession creates a new session instance with the provided parameters.
func NewSession(ssmClient *ssm.Client, kmsClient *kms.Client, ssmSession *types.Session, region string, targetID string, log log.T) (*Session, error) {
	endpoint := url.URL{Scheme: "https", Host: fmt.Sprintf("ssm.%s.amazonaws.com", region)}

	clientID, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("generate UUID: %w", err)
	}

	session := &Session{
		SessionID:   *ssmSession.SessionId,
		TargetID:    targetID,
		DisplayMode: sessionutil.NewDisplayMode(log),
		streamURL:   *ssmSession.StreamUrl,
		tokenValue:  *ssmSession.TokenValue,
		endpoint:    endpoint.String(),
		clientID:    clientID.String(),
		ssmClient:   ssmClient,
		kmsClient:   kmsClient,
	}

	return session, nil
}

// OpenDataChannel initializes datachannel.
func (s *Session) OpenDataChannel(ctx context.Context, log log.T) error {
	s.retryParams = retry.RepeatableExponentialRetryer{
		GeometricRatio:      config.RetryBase,
		InitialDelayInMilli: rand.Intn(config.DataChannelRetryInitialDelayMillis) + config.DataChannelRetryInitialDelayMillis, //nolint:gosec
		MaxDelayInMilli:     config.DataChannelRetryMaxIntervalMillis,
		MaxAttempts:         config.DataChannelNumMaxRetries,
	}

	wsChannel, err := communicator.NewWebSocketChannel(s.streamURL, s.tokenValue)
	if err != nil {
		return fmt.Errorf("creating websocket channel: %w", err)
	}

	dataChannel, err := datachannel.NewDataChannel(
		s.kmsClient,
		s.clientID,
		s.SessionID,
		s.TargetID,
		s.isAwsCliUpgradeNeeded,
		log,
	)
	if err != nil {
		return fmt.Errorf("creating data channel: %w", err)
	}

	s.DataChannel = dataChannel

	s.DataChannel.SetWsChannel(wsChannel)

	s.DataChannel.RegisterOutputMessageHandler(ctx, log, s.Stop, func(_ []byte) {})

	s.DataChannel.RegisterOutputStreamHandler(s.ProcessFirstMessage, false)

	if err := s.DataChannel.Open(log); err != nil {
		log.Error("Retrying connection failed", "sessionID", s.SessionID, "error", err)

		s.retryParams.CallableFunc = func() error { return s.DataChannel.Reconnect(log) }
		if err := s.retryParams.Call(); err != nil {
			log.Error("Failed to call retry parameters", "error", err)
		}
	}

	s.DataChannel.SetOnError(func(wsErr error) {
		log.Error("Trying to reconnect session", "url", s.streamURL, "sequenceNumber", s.DataChannel.GetStreamDataSequenceNumber(), "error", wsErr)

		s.retryParams.CallableFunc = func() error { return s.ResumeSessionHandler(ctx, log) }
		if err := s.retryParams.Call(); err != nil {
			log.Error("Reconnect error", "error", err)
		}
	})

	// Scheduler for resending of data
	if err := s.DataChannel.ResendStreamDataMessageScheduler(log); err != nil {
		log.Error("Failed to schedule resend of stream data messages", "error", err)
	}

	return nil
}

// ProcessFirstMessage only processes messages with PayloadType Output to determine the
// sessionType of the session to be launched. This is a fallback for agent versions that do not support handshake, they
// immediately start sending shell output.
func (s *Session) ProcessFirstMessage(log log.T, outputMessage message.ClientMessage) (bool, error) {
	// Immediately deregister self so that this handler is only called once, for the first message
	s.DataChannel.DeregisterOutputStreamHandler(s.ProcessFirstMessage)
	// Only set session type if the session type has not already been set. Usually session type will be set
	// by handshake protocol which would be the first message but older agents may not perform handshake
	if s.SessionType == "" {
		if outputMessage.PayloadType == uint32(message.Output) {
			log.Warn("Setting session type to shell based on PayloadType!")
			s.DataChannel.SetSessionType(config.ShellPluginName)
			s.DisplayMode.DisplayMessage(log, outputMessage)
		}
	}

	return true, nil
}

// Stop will end the session.
func (s *Session) Stop(_ log.T) error {
	return nil
}

// GetResumeSessionParams calls ResumeSession API and gets tokenvalue for reconnecting.
func (s *Session) GetResumeSessionParams(ctx context.Context, log log.T) (string, error) {
	resumeSessionInput := ssm.ResumeSessionInput{
		SessionId: aws.String(s.SessionID),
	}

	log.Debug("Resume Session input parameters", "input", resumeSessionInput)

	resumeSessionOutput, err := s.ssmClient.ResumeSession(ctx, &resumeSessionInput)
	if err != nil {
		log.Error("Resume Session failed", "error", err)

		return "", fmt.Errorf("resume session: %w", err)
	}

	if resumeSessionOutput.TokenValue == nil {
		return "", nil
	}

	return *resumeSessionOutput.TokenValue, nil
}

// ResumeSessionHandler gets token value and tries to Reconnect to datachannel.
func (s *Session) ResumeSessionHandler(ctx context.Context, log log.T) error {
	var err error

	s.tokenValue, err = s.GetResumeSessionParams(ctx, log)
	if err != nil {
		log.Error("getting token", "error", err)

		return fmt.Errorf("resume session: %w", err)
	} else if s.tokenValue == "" {
		log.Debug("Session timed out", "sessionID", s.SessionID)

		return errors.New("session timed out")
	}

	s.DataChannel.SetChannelToken(s.tokenValue)

	err = s.DataChannel.Reconnect(log)
	if err != nil {
		return fmt.Errorf("reconnecting data channel: %w", err)
	}

	return nil
}

// TerminateSession calls TerminateSession API.
func (s *Session) TerminateSession(ctx context.Context, log log.T) error {
	terminateSessionInput := ssm.TerminateSessionInput{
		SessionId: &s.SessionID,
	}

	log.Debug("Terminate Session input parameters", "input", terminateSessionInput)

	if _, err := s.ssmClient.TerminateSession(ctx, &terminateSessionInput); err != nil {
		log.Error("Terminate Session failed", "error", err)

		return fmt.Errorf("terminate session: %w", err)
	}

	return nil
}
