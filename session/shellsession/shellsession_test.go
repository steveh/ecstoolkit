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

// Package shellsession starts shell session.
package shellsession

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/steveh/ecstoolkit/communicator/mocks"
	"github.com/steveh/ecstoolkit/datachannel"
	dataChannelMock "github.com/steveh/ecstoolkit/datachannel/mocks"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/session"
	"github.com/steveh/ecstoolkit/session/sessionutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	expectedSequenceNumber = int64(0)
	logger                 = log.NewMockLog()
	clientID               = "clientId"
	sessionID              = "sessionId"
	instanceID             = "instanceId"
	mockDataChannel        = &dataChannelMock.IDataChannel{}
	mockWsChannel          = &mocks.IWebSocketChannel{}
)

func TestName(t *testing.T) {
	shellSession := ShellSession{}
	name := shellSession.Name()
	assert.Equal(t, "Standard_Stream", name)
}

func TestInitialize(t *testing.T) {
	session := &session.Session{}
	shellSession := ShellSession{}
	session.DataChannel = mockDataChannel
	mockDataChannel.On("RegisterOutputStreamHandler", mock.Anything, true).Times(1)
	mockDataChannel.On("SetOnMessage", mock.Anything)
	mockDataChannel.On("SetOnError", mock.Anything)
	shellSession.Initialize(context.TODO(), logger, session)
	assert.Equal(t, shellSession.Session, *session)
}

func TestHandleControlSignals(t *testing.T) {
	session := session.Session{}
	session.DataChannel = mockDataChannel
	shellSession := ShellSession{}
	shellSession.Session = session

	waitCh := make(chan int, 1)
	counter := 0
	sendDataMessage := func() error {
		counter++

		return errors.New("SendInputDataMessage error")
	}
	mockDataChannel.On("SendInputDataMessage", mock.Anything, mock.Anything, mock.Anything).Return(sendDataMessage())

	signalCh := make(chan os.Signal, 1)
	go func() {
		p, err := os.FindProcess(os.Getpid())
		if err != nil {
			t.Errorf("Failed to find process: %v", err)

			return
		}

		signal.Notify(signalCh, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTSTP)
		shellSession.handleControlSignals(logger)

		if err := p.Signal(syscall.SIGINT); err != nil {
			t.Errorf("Failed to send signal: %v", err)

			return
		}

		time.Sleep(200 * time.Millisecond)
		close(waitCh)
	}()

	<-waitCh
	assert.Equal(t, syscall.SIGINT, <-signalCh)
	assert.Equal(t, 1, counter)
}

func TestSendInputDataMessageWithPayloadTypeSize(t *testing.T) {
	sizeData := message.SizeData{
		Cols: 100,
		Rows: 100,
	}

	sizeDataBytes, err := json.Marshal(sizeData)
	if err != nil {
		t.Fatalf("marshaling size data: %v", err)
	}

	dataChannel := getDataChannel(t)
	mockChannel := &mocks.IWebSocketChannel{}
	dataChannel.SetWsChannel(mockChannel)

	SendMessageCallCount := 0
	datachannel.SendMessageCall = func(_ *datachannel.DataChannel, _ []byte, _ int) error {
		SendMessageCallCount++

		return nil
	}

	err = dataChannel.SendInputDataMessage(logger, message.Size, sizeDataBytes)
	require.NoError(t, err)
	assert.Equal(t, expectedSequenceNumber, dataChannel.ExpectedSequenceNumber)
	assert.Equal(t, 1, SendMessageCallCount)
}

func TestTerminalResizeWhenSessionSizeDataIsNotEqualToActualSize(t *testing.T) {
	dataChannel := getDataChannel(t)

	session := session.Session{
		DataChannel: dataChannel,
	}

	sizeData := message.SizeData{
		Cols: 100,
		Rows: 100,
	}

	shellSession := ShellSession{
		Session:  session,
		SizeData: sizeData,
	}
	GetTerminalSizeCall = func(_ int) (int, int, error) {
		return 123, 123, nil
	}

	var wg sync.WaitGroup

	wg.Add(1)
	// Spawning a separate go routine to close websocket connection.
	// This is required as handleTerminalResize has a for loop which will continuously check for
	// size data every 500ms.
	go func() {
		time.Sleep(1 * time.Second)
		wg.Done()
	}()

	SendMessageCallCount := 0
	datachannel.SendMessageCall = func(_ *datachannel.DataChannel, _ []byte, _ int) error {
		SendMessageCallCount++

		return nil
	}

	go shellSession.handleTerminalResize(logger)
	wg.Wait()
	assert.Equal(t, 1, SendMessageCallCount)
}

func TestProcessStreamMessagePayload(t *testing.T) {
	shellSession := ShellSession{}
	shellSession.DisplayMode = sessionutil.NewDisplayMode(logger)

	msg := message.ClientMessage{
		Payload: []byte("Hello Agent\n"),
	}
	isReady, err := shellSession.ProcessStreamMessagePayload(logger, msg)
	assert.True(t, isReady)
	require.NoError(t, err)
}

func getDataChannel(t *testing.T) *datachannel.DataChannel {
	t.Helper()

	mockKMSClient := &kms.Client{}

	dataChannel, err := datachannel.NewDataChannel(mockKMSClient, clientID, sessionID, instanceID, false, logger)
	require.NoError(t, err)

	dataChannel.SetWsChannel(mockWsChannel)

	return dataChannel
}
