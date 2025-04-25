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
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	wsChannelMock "github.com/steveh/ecstoolkit/communicator/mocks"
	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/datachannel"
	dataChannelMock "github.com/steveh/ecstoolkit/datachannel/mocks"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	logger          = log.NewMockLog()
	mockDataChannel = &dataChannelMock.IDataChannel{}
	mockWsChannel   = &wsChannelMock.IWebSocketChannel{}
)

var (
	clientID   = "clientId_abc"
	sessionID  = "sessionId_abc"
	instanceID = "i-123456"
)

func SetupMockActions() {
	mockDataChannel.On("Initialize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	mockDataChannel.On("SetWsChannel", mock.Anything)
	mockDataChannel.On("SetOnMessage", mock.Anything)
	mockDataChannel.On("SetOnError", mock.Anything)
	mockDataChannel.On("RegisterOutputStreamHandler", mock.Anything, mock.Anything)
	mockDataChannel.On("RegisterOutputMessageHandler", mock.Anything, mock.Anything, mock.Anything)
	mockDataChannel.On("ResendStreamDataMessageScheduler", mock.Anything).Return(nil)
}

func TestOpenDataChannel(t *testing.T) {
	mockDataChannel = &dataChannelMock.IDataChannel{}
	mockWsChannel = &wsChannelMock.IWebSocketChannel{}

	sessionMock := &Session{
		logger: logger,
	}
	sessionMock.dataChannel = mockDataChannel

	SetupMockActions()
	mockDataChannel.On("Open", mock.Anything).Return(nil)

	err := sessionMock.OpenDataChannel(context.TODO())
	require.NoError(t, err)
}

func TestOpenDataChannelWithError(t *testing.T) {
	mockDataChannel = &dataChannelMock.IDataChannel{}
	mockWsChannel = &wsChannelMock.IWebSocketChannel{}

	sessionMock := &Session{
		logger: logger,
	}
	sessionMock.dataChannel = mockDataChannel

	SetupMockActions()

	// First reconnection failed when open datachannel, success after retry
	mockDataChannel.On("Open", mock.Anything).Return(errors.New("error"))
	mockDataChannel.On("Reconnect", mock.Anything).Return(errors.New("error")).Once()
	mockDataChannel.On("Reconnect", mock.Anything).Return(nil).Once()

	err := sessionMock.OpenDataChannel(context.TODO())
	require.NoError(t, err)
}

func TestProcessFirstMessageOutputMessageFirst(t *testing.T) {
	outputMessage := message.ClientMessage{
		PayloadType: uint32(message.Output),
		Payload:     []byte("testing"),
	}

	mockKMSClient := &kms.Client{}

	dataChannel, err := datachannel.NewDataChannel(mockKMSClient, clientID, sessionID, instanceID, logger)
	require.NoError(t, err)

	session := Session{
		dataChannel: dataChannel,
		logger:      logger,
	}

	_, err = session.processFirstMessage(outputMessage)
	if err != nil {
		t.Errorf("Failed to process first message: %v", err)
	}

	assert.Equal(t, config.ShellPluginName, session.dataChannel.GetSessionType())
	assert.True(t, <-session.dataChannel.IsSessionTypeSet())
}
