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
	"errors"
	"testing"

	wsChannelMock "github.com/steveh/ecstoolkit/communicator/mocks"
	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/datachannel"
	dataChannelMock "github.com/steveh/ecstoolkit/datachannel/mocks"
	"github.com/steveh/ecstoolkit/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	clientId   = "clientId_abc"
	sessionId  = "sessionId_abc"
	instanceId = "i-123456"
)

func TestOpenDataChannel(t *testing.T) {
	mockDataChannel = &dataChannelMock.IDataChannel{}
	mockWsChannel = &wsChannelMock.IWebSocketChannel{}

	sessionMock := &Session{}
	sessionMock.DataChannel = mockDataChannel

	SetupMockActions()
	mockDataChannel.On("Open", mock.Anything).Return(nil)

	err := sessionMock.OpenDataChannel(logger)
	assert.NoError(t, err)
}

func TestOpenDataChannelWithError(t *testing.T) {
	mockDataChannel = &dataChannelMock.IDataChannel{}
	mockWsChannel = &wsChannelMock.IWebSocketChannel{}

	sessionMock := &Session{}
	sessionMock.DataChannel = mockDataChannel

	SetupMockActions()

	// First reconnection failed when open datachannel, success after retry
	mockDataChannel.On("Open", mock.Anything).Return(errors.New("error"))
	mockDataChannel.On("Reconnect", mock.Anything).Return(errors.New("error")).Once()
	mockDataChannel.On("Reconnect", mock.Anything).Return(nil).Once()

	err := sessionMock.OpenDataChannel(logger)
	assert.NoError(t, err)
}

func TestProcessFirstMessageOutputMessageFirst(t *testing.T) {
	outputMessage := message.ClientMessage{
		PayloadType: uint32(message.Output),
		Payload:     []byte("testing"),
	}

	dataChannel := &datachannel.DataChannel{}
	dataChannel.Initialize(logger, clientId, sessionId, instanceId, false)
	session := Session{
		DataChannel: dataChannel,
	}

	session.ProcessFirstMessage(logger, outputMessage)
	assert.Equal(t, config.ShellPluginName, session.DataChannel.GetSessionType())
	assert.True(t, <-session.DataChannel.IsSessionTypeSet())
}
