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
	"math/rand"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/retry"
)

// OpenDataChannel initializes datachannel.
func (s *Session) OpenDataChannel(ctx context.Context, log log.T) (err error) {
	s.retryParams = retry.RepeatableExponentialRetryer{
		GeometricRatio:      config.RetryBase,
		InitialDelayInMilli: rand.Intn(config.DataChannelRetryInitialDelayMillis) + config.DataChannelRetryInitialDelayMillis,
		MaxDelayInMilli:     config.DataChannelRetryMaxIntervalMillis,
		MaxAttempts:         config.DataChannelNumMaxRetries,
	}

	s.DataChannel.Initialize(log, s.ClientId, s.SessionId, s.TargetId, s.IsAwsCliUpgradeNeeded)
	s.DataChannel.SetWebsocket(log, s.StreamUrl, s.TokenValue)
	s.DataChannel.GetWsChannel().SetOnMessage(
		func(input []byte) {
			s.DataChannel.OutputMessageHandler(ctx, log, s.Stop, s.SessionId, input)
		})
	s.DataChannel.RegisterOutputStreamHandler(s.ProcessFirstMessage, false)

	if err = s.DataChannel.Open(log); err != nil {
		log.Errorf("Retrying connection for data channel id: %s failed with error: %s", s.SessionId, err)

		s.retryParams.CallableFunc = func() (err error) { return s.DataChannel.Reconnect(log) }
		if err = s.retryParams.Call(); err != nil {
			log.Errorf("%+v", err)
		}
	}

	s.DataChannel.GetWsChannel().SetOnError(
		func(err error) {
			log.Errorf("Trying to reconnect the session: %v with seq num: %d", s.StreamUrl, s.DataChannel.GetStreamDataSequenceNumber())

			s.retryParams.CallableFunc = func() (err error) { return s.ResumeSessionHandler(ctx, log) }
			if err = s.retryParams.Call(); err != nil {
				log.Errorf("%+v", err)
			}
		})

	// Scheduler for resending of data
	s.DataChannel.ResendStreamDataMessageScheduler(log)

	return nil
}

// ProcessFirstMessage only processes messages with PayloadType Output to determine the
// sessionType of the session to be launched. This is a fallback for agent versions that do not support handshake, they
// immediately start sending shell output.
func (s *Session) ProcessFirstMessage(log log.T, outputMessage message.ClientMessage) (isHandlerReady bool, err error) {
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
func (s *Session) Stop(log log.T) error {
	return nil
}

// GetResumeSessionParams calls ResumeSession API and gets tokenvalue for reconnecting.
func (s *Session) GetResumeSessionParams(ctx context.Context, log log.T) (string, error) {
	resumeSessionInput := ssm.ResumeSessionInput{
		SessionId: &s.SessionId,
	}

	log.Debugf("Resume Session input parameters: %v", resumeSessionInput)

	resumeSessionOutput, err := s.SSMClient.ResumeSession(ctx, &resumeSessionInput)
	if err != nil {
		log.Errorf("Resume Session failed: %v", err)

		return "", err
	}

	if resumeSessionOutput.TokenValue == nil {
		return "", nil
	}

	return *resumeSessionOutput.TokenValue, nil
}

// ResumeSessionHandler gets token value and tries to Reconnect to datachannel.
func (s *Session) ResumeSessionHandler(ctx context.Context, log log.T) (err error) {
	s.TokenValue, err = s.GetResumeSessionParams(ctx, log)
	if err != nil {
		log.Errorf("Failed to get token: %v", err)

		return
	} else if s.TokenValue == "" {
		log.Debugf("Session: %s timed out.", s.SessionId)

		return errors.New("session timed out")
	}

	s.DataChannel.GetWsChannel().SetChannelToken(s.TokenValue)
	err = s.DataChannel.Reconnect(log)

	return
}

// TerminateSession calls TerminateSession API.
func (s *Session) TerminateSession(ctx context.Context, log log.T) error {
	terminateSessionInput := ssm.TerminateSessionInput{
		SessionId: &s.SessionId,
	}

	log.Debugf("Terminate Session input parameters: %v", terminateSessionInput)

	if _, err := s.SSMClient.TerminateSession(ctx, &terminateSessionInput); err != nil {
		log.Errorf("Terminate Session failed: %v", err)

		return err
	}

	return nil
}
