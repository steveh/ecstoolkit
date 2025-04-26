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

// this package implement base communicator for network connections.
package communicator_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/steveh/ecstoolkit/communicator"
	"github.com/steveh/ecstoolkit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	defaultChannelToken = "channelToken"
	defaultStreamURL    = "streamUrl"
	errDefault          = errors.New("default error")
	defaultMessage      = []byte("Default Message")
)

type ErrorCallbackWrapper struct {
	mock.Mock
	err error
}

func (m *ErrorCallbackWrapper) defaultErrorHandler(err error) {
	m.Called(err)
	m.err = err
}

type MessageCallbackWrapper struct {
	mock.Mock
	message []byte
}

func (m *MessageCallbackWrapper) defaultMessageHandler(msg []byte) {
	m.Called(msg)
	m.message = msg
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// handlerToBeTested echos all incoming input from a websocket connection back to the client while
// adding the word "echo".
func handlerToBeTested(w http.ResponseWriter, req *http.Request) {
	log := log.NewMockLog()

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot upgrade: %v", err), http.StatusInternalServerError)
	}

	for {
		mt, p, err := conn.ReadMessage()
		if err != nil {
			log.Error("Error reading message", "error", err)

			return
		}

		// echo back the same sent string from the client while adding "echo" at the beginning
		if err := conn.WriteMessage(mt, []byte("echo "+string(p))); err != nil {
			log.Error("Error writing message", "error", err)

			return
		}
	}
}

func TestWebSocketChannel_GetChannelToken(t *testing.T) {
	t.Parallel()

	t.Log("Starting test: webSocketChannel.GetChannelToken")

	log := log.NewMockLog()

	channel, err := communicator.NewWebSocketChannel(defaultStreamURL, defaultChannelToken, log)
	require.NoError(t, err)

	token := channel.GetChannelToken()
	assert.Equal(t, defaultChannelToken, token)
}

func TestWebSocketChannel_SetChannelToken(t *testing.T) {
	t.Parallel()

	t.Log("Starting test: webSocketChannel.SetChannelToken")

	log := log.NewMockLog()

	channel, err := communicator.NewWebSocketChannel(defaultStreamURL, "other", log)
	require.NoError(t, err)

	channel.SetChannelToken(defaultChannelToken)
	assert.Equal(t, defaultChannelToken, channel.GetChannelToken())
}

func TestWebSocketChannel_GetStreamURL(t *testing.T) {
	t.Parallel()

	t.Log("Starting test: webSocketChannel.GetStreamURL")

	log := log.NewMockLog()

	channel, err := communicator.NewWebSocketChannel(defaultStreamURL, defaultChannelToken, log)
	require.NoError(t, err)

	url := channel.GetStreamURL()
	assert.Equal(t, defaultStreamURL, url)
}

func TestWebSocketChannel_SetOnError(t *testing.T) {
	t.Parallel()

	t.Log("Starting test: webSocketChannel.SetOnError")

	channel := &communicator.WebSocketChannel{}
	errorCallbackWrapper := &ErrorCallbackWrapper{}
	errorCallbackWrapper.On("defaultErrorHandler", errDefault).Return()

	channel.SetOnError((*errorCallbackWrapper).defaultErrorHandler)
	channel.OnError(errDefault)

	errorCallbackWrapper.AssertCalled(t, "defaultErrorHandler", errDefault)
	assert.Equal(t, errDefault.Error(), errorCallbackWrapper.err.Error())
}

func TestWebsocketChannel_SetOnMessage(t *testing.T) {
	t.Parallel()

	t.Log("Starting test: webSocketChannel.SetOnMessage")

	channel := &communicator.WebSocketChannel{}
	messageCallbackWrapper := &MessageCallbackWrapper{}
	messageCallbackWrapper.On("defaultMessageHandler", defaultMessage).Return()

	channel.SetOnMessage((*messageCallbackWrapper).defaultMessageHandler)
	channel.OnMessage(defaultMessage)

	messageCallbackWrapper.AssertCalled(t, "defaultMessageHandler", defaultMessage)
	assert.Equal(t, defaultMessage, messageCallbackWrapper.message)
}

func TestNewWebSocketChannel(t *testing.T) {
	t.Parallel()

	t.Log("Starting test: TestNewWebSocketChannel")

	log := log.NewMockLog()

	channel, err := communicator.NewWebSocketChannel(defaultStreamURL, defaultChannelToken, log)
	require.NoError(t, err)

	assert.Equal(t, defaultStreamURL, channel.GetStreamURL())
	assert.Equal(t, defaultChannelToken, channel.GetChannelToken())
}

func TestOpenCloseWebSocketChannel(t *testing.T) {
	t.Parallel()

	t.Log("Starting test: TestOpenCloseWebSocketChannel")

	srv := httptest.NewServer(http.HandlerFunc(handlerToBeTested))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	log := log.NewMockLog()

	websocketchannel, err := communicator.NewWebSocketChannel(u.String(), defaultChannelToken, log)
	require.NoError(t, err)

	err = websocketchannel.Open()
	require.NoError(t, err, "Error opening the websocket connection.")
	assert.True(t, websocketchannel.IsOpen(), "IsOpen is not set to true.")

	err = websocketchannel.Close()
	require.NoError(t, err, "Error closing the websocket connection.")
	assert.False(t, websocketchannel.IsOpen(), "IsOpen is not set to false.")
	t.Log("Ending test: TestOpenCloseWebSocketChannel")
}

func TestReadWriteTextToWebSocketChannel(t *testing.T) {
	t.Parallel()

	t.Log("Starting test: TestReadWriteWebSocketChannel ")

	srv := httptest.NewServer(http.HandlerFunc(handlerToBeTested))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	log := log.NewMockLog()

	var wg sync.WaitGroup

	wg.Add(1)

	onMessage := func(input []byte) {
		defer wg.Done()
		t.Log(input)
		// Verify read from websocket server
		assert.Equal(t, "echo channelreadwrite", string(input))
	}

	websocketchannel, err := communicator.NewWebSocketChannel(u.String(), defaultChannelToken, log)
	require.NoError(t, err)

	websocketchannel.OnMessage = onMessage

	// Open the websocket connection
	err = websocketchannel.Open()
	require.NoError(t, err, "Error opening the websocket connection.")

	if err := websocketchannel.SendMessage([]byte("channelreadwrite"), websocket.TextMessage); err != nil {
		t.Errorf("Failed to send message: %v", err)
	}

	wg.Wait()

	err = websocketchannel.Close()
	require.NoError(t, err, "Error closing the websocket connection.")
	assert.False(t, websocketchannel.IsOpen(), "IsOpen is not set to false.")
	t.Log("Ending test: TestReadWriteWebSocketChannel ")
}

func TestReadWriteBinaryToWebSocketChannel(t *testing.T) {
	t.Parallel()

	t.Log("Starting test: TestReadWriteBinaryWebSocketChannel ")

	srv := httptest.NewServer(http.HandlerFunc(handlerToBeTested))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	log := log.NewMockLog()

	var wg sync.WaitGroup

	wg.Add(1)

	onMessage := func(input []byte) {
		defer wg.Done()
		t.Log(input)
		// Verify read from websocket server
		assert.Equal(t, "echo channelreadwrite", string(input))
	}

	websocketchannel, err := communicator.NewWebSocketChannel(u.String(), defaultChannelToken, log)
	require.NoError(t, err)

	websocketchannel.OnMessage = onMessage

	// Open the websocket connection
	err = websocketchannel.Open()
	require.NoError(t, err, "Error opening the websocket connection.")

	if err := websocketchannel.SendMessage([]byte("channelreadwrite"), websocket.BinaryMessage); err != nil {
		t.Errorf("Failed to send message: %v", err)
	}

	wg.Wait()

	err = websocketchannel.Close()
	require.NoError(t, err, "Error closing the websocket connection.")
	assert.False(t, websocketchannel.IsOpen(), "IsOpen is not set to false.")
	t.Log("Ending test: TestReadWriteWebSocketChannel ")
}

func TestMultipleReadWriteWebSocketChannel(t *testing.T) {
	t.Parallel()

	t.Log("Starting test: TestMultipleReadWriteWebSocketChannel")

	srv := httptest.NewServer(http.HandlerFunc(handlerToBeTested))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	log := log.NewMockLog()

	read1 := make(chan bool)
	read2 := make(chan bool)

	onMessage := func(input []byte) {
		t.Log(input)
		// Verify reads from websocket server
		if string(input) == "echo channelreadwrite1" {
			read1 <- true
		}

		if string(input) == "echo channelreadwrite2" {
			read2 <- true
		}
	}

	websocketchannel, err := communicator.NewWebSocketChannel(u.String(), defaultChannelToken, log)
	require.NoError(t, err)

	websocketchannel.OnMessage = onMessage

	// Open the websocket connection
	err = websocketchannel.Open()
	require.NoError(t, err, "Error opening the websocket connection.")

	// Verify writes to websocket server
	err = websocketchannel.SendMessage([]byte("channelreadwrite1"), websocket.TextMessage)
	require.NoError(t, err, "Error sending message 1")
	err = websocketchannel.SendMessage([]byte("channelreadwrite2"), websocket.TextMessage)
	require.NoError(t, err, "Error sending message 2")
	assert.True(t, <-read1, "Didn't read value 1 correctly")
	assert.True(t, <-read2, "Didn't ready value 2 correctly")

	err = websocketchannel.Close()
	require.NoError(t, err, "Error closing the websocket connection.")
	assert.False(t, websocketchannel.IsOpen(), "IsOpen is not set to false.")

	t.Log("Ending test: TestMultipleReadWriteWebSocketChannel")
}
