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
	"sync"
	"testing"
	"time"

	wsChannelMock "github.com/steveh/ecstoolkit/communicator/mocks"
	dataChannelMock "github.com/steveh/ecstoolkit/datachannel/mocks"
	"github.com/steveh/ecstoolkit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	logger          = log.NewMockLog()
	mockDataChannel = &dataChannelMock.IDataChannel{}
	mockWsChannel   = &wsChannelMock.IWebSocketChannel{}
)

func TestExecute(t *testing.T) {
	sessionMock := &Session{}
	sessionMock.DataChannel = mockDataChannel

	SetupMockActions()
	mockDataChannel.On("Open", mock.Anything).Return(nil)

	isSessionTypeSetMock := make(chan bool, 1)
	isSessionTypeSetMock <- true
	mockDataChannel.On("IsSessionTypeSet").Return(isSessionTypeSetMock)
	mockDataChannel.On("GetSessionType").Return("Standard_Stream")
	mockDataChannel.On("GetSessionProperties").Return("SessionProperties")

	isStreamMessageResendTimeout := make(chan bool, 1)
	mockDataChannel.On("IsStreamMessageResendTimeout").Return(isStreamMessageResendTimeout)

	setSessionHandlersWithSessionType = func(ctx context.Context, session *Session, log log.T) error {
		return fmt.Errorf("start session error for %s", session.SessionType)
	}

	err := sessionMock.Execute(context.TODO(), logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start session error for Standard_Stream")
}

func TestExecuteAndStreamMessageResendTimesOut(t *testing.T) {
	sessionMock := &Session{}
	sessionMock.DataChannel = mockDataChannel

	SetupMockActions()
	mockDataChannel.On("Open", mock.Anything).Return(nil)

	isStreamMessageResendTimeout := make(chan bool, 1)
	mockDataChannel.On("IsStreamMessageResendTimeout").Return(isStreamMessageResendTimeout)

	var wg sync.WaitGroup

	wg.Add(1)

	handleStreamMessageResendTimeout = func(_ context.Context, _ *Session, _ log.T) {
		time.Sleep(10 * time.Millisecond)
		isStreamMessageResendTimeout <- true

		wg.Done()
	}

	isSessionTypeSetMock := make(chan bool, 1)
	isSessionTypeSetMock <- true
	mockDataChannel.On("IsSessionTypeSet").Return(isSessionTypeSetMock)
	mockDataChannel.On("GetSessionType").Return("Standard_Stream")
	mockDataChannel.On("GetSessionProperties").Return("SessionProperties")

	setSessionHandlersWithSessionType = func(ctx context.Context, session *Session, log log.T) error {
		return nil
	}

	var err error
	go func() {
		err = sessionMock.Execute(context.TODO(), logger)

		time.Sleep(200 * time.Millisecond)
	}()
	wg.Wait()
	assert.NoError(t, err)
}

func SetupMockActions() {
	mockDataChannel.On("Initialize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	mockDataChannel.On("SetWebsocket", mock.Anything, mock.Anything, mock.Anything).Return()
	mockDataChannel.On("GetWsChannel").Return(mockWsChannel)
	mockDataChannel.On("RegisterOutputStreamHandler", mock.Anything, mock.Anything)
	mockDataChannel.On("ResendStreamDataMessageScheduler", mock.Anything).Return(nil)

	mockWsChannel.On("SetOnMessage", mock.Anything)
	mockWsChannel.On("SetOnError", mock.Anything)
}
