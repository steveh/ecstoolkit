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
	"testing"

	wsChannelMock "github.com/steveh/ecstoolkit/communicator/mocks"
	dataChannelMock "github.com/steveh/ecstoolkit/datachannel/mocks"
	"github.com/steveh/ecstoolkit/log"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	logger          = log.NewMockLog()
	mockDataChannel = &dataChannelMock.IDataChannel{}
	mockWsChannel   = &wsChannelMock.IWebSocketChannel{}
)

func SetupMockActions() {
	mockDataChannel.On("Open", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", nil)
	mockDataChannel.On("SetOnMessage", mock.Anything)
	mockDataChannel.On("RegisterOutputStreamHandler", mock.Anything, mock.Anything)
	mockDataChannel.On("RegisterOutputMessageHandler", mock.Anything, mock.Anything, mock.Anything)
	mockDataChannel.On("ResendStreamDataMessageScheduler", mock.Anything).Return(nil)
}

func TestOpenDataChannel(t *testing.T) {
	t.Parallel()

	mockDataChannel = &dataChannelMock.IDataChannel{}
	mockWsChannel = &wsChannelMock.IWebSocketChannel{}

	sessionMock := &Session{
		logger: logger,
	}
	sessionMock.dataChannel = mockDataChannel

	SetupMockActions()
	mockDataChannel.On("Open", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", nil)

	_, err := sessionMock.OpenDataChannel(context.TODO())
	require.NoError(t, err)
}

func TestOpenDataChannelWithError(t *testing.T) {
	t.Parallel()

	mockDataChannel = &dataChannelMock.IDataChannel{}
	mockWsChannel = &wsChannelMock.IWebSocketChannel{}

	sessionMock := &Session{
		logger: logger,
	}
	sessionMock.dataChannel = mockDataChannel

	SetupMockActions()

	// First reconnection failed when open datachannel, success after retry
	mockDataChannel.On("Open", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", nil)

	_, err := sessionMock.OpenDataChannel(context.TODO())
	require.NoError(t, err)
}
