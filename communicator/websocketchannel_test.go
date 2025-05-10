package communicator

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/steveh/ecstoolkit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultChannelToken = "channelToken"
	defaultStreamURL    = "streamUrl"
)

func mockUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
}

// handlerToBeTested echos all incoming input from a websocket connection back to the client while
// adding the word "echo".
func handlerToBeTested(w http.ResponseWriter, req *http.Request) {
	log := log.NewMockLog()

	upgrader := mockUpgrader()

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

	channel, err := NewWebSocketChannel(defaultStreamURL, defaultChannelToken, log)
	require.NoError(t, err)

	token := channel.GetChannelToken()
	assert.Equal(t, defaultChannelToken, token)
}

func TestWebSocketChannel_SetChannelToken(t *testing.T) {
	t.Parallel()

	t.Log("Starting test: webSocketChannel.SetChannelToken")

	log := log.NewMockLog()

	channel, err := NewWebSocketChannel(defaultStreamURL, "other", log)
	require.NoError(t, err)

	channel.SetChannelToken(defaultChannelToken)
	assert.Equal(t, defaultChannelToken, channel.GetChannelToken())
}

func TestWebSocketChannel_GetStreamURL(t *testing.T) {
	t.Parallel()

	t.Log("Starting test: webSocketChannel.GetStreamURL")

	log := log.NewMockLog()

	channel, err := NewWebSocketChannel(defaultStreamURL, defaultChannelToken, log)
	require.NoError(t, err)

	url := channel.GetStreamURL()
	assert.Equal(t, defaultStreamURL, url)
}

func TestNewWebSocketChannel(t *testing.T) {
	t.Parallel()

	t.Log("Starting test: TestNewWebSocketChannel")

	log := log.NewMockLog()

	channel, err := NewWebSocketChannel(defaultStreamURL, defaultChannelToken, log)
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

	websocketchannel, err := NewWebSocketChannel(u.String(), defaultChannelToken, log)
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

	websocketchannel, err := NewWebSocketChannel(u.String(), defaultChannelToken, log)
	require.NoError(t, err)

	// Open the websocket connection
	err = websocketchannel.Open()
	require.NoError(t, err, "Error opening the websocket connection.")

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		rawMessage, err := websocketchannel.ReadMessage()
		assert.NoError(t, err)

		t.Log(rawMessage)

		assert.Equal(t, "echo channelreadwrite", string(rawMessage))
	}()

	if err := websocketchannel.SendMessage([]byte("channelreadwrite"), websocket.TextMessage); err != nil {
		t.Errorf("Failed to send message: %v", err)
	}

	// Wait for message to be received before closing the connection
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

	websocketchannel, err := NewWebSocketChannel(u.String(), defaultChannelToken, log)
	require.NoError(t, err)

	// Open the websocket connection
	err = websocketchannel.Open()
	require.NoError(t, err, "Error opening the websocket connection.")

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		rawMessage, err := websocketchannel.ReadMessage()
		assert.NoError(t, err)

		t.Log(rawMessage)

		assert.Equal(t, "echo channelreadwrite", string(rawMessage))
	}()

	if err := websocketchannel.SendMessage([]byte("channelreadwrite"), websocket.BinaryMessage); err != nil {
		t.Errorf("Failed to send message: %v", err)
	}

	// Wait for message to be received before closing the connection
	wg.Wait()

	err = websocketchannel.Close()
	require.NoError(t, err, "Error closing the websocket connection.")
	assert.False(t, websocketchannel.IsOpen(), "IsOpen is not set to false.")
	t.Log("Ending test: TestReadWriteBinaryWebSocketChannel ")
}

func TestMultipleReadWriteWebSocketChannel(t *testing.T) {
	t.Parallel()

	t.Log("Starting test: TestMultipleReadWriteWebSocketChannel")

	srv := httptest.NewServer(http.HandlerFunc(handlerToBeTested))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	log := log.NewMockLog()

	// Use atomic values to safely track message receipt
	var message1Received atomic.Bool

	var message2Received atomic.Bool

	websocketchannel, err := NewWebSocketChannel(u.String(), defaultChannelToken, log)
	require.NoError(t, err)

	// Open the websocket connection
	err = websocketchannel.Open()
	require.NoError(t, err, "Error opening the websocket connection.")

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		for range 2 {
			rawMessage, err := websocketchannel.ReadMessage()
			assert.NoError(t, err)

			t.Log(rawMessage)

			if string(rawMessage) == "echo channelreadwrite1" {
				message1Received.Store(true)
				wg.Done()
			}

			if string(rawMessage) == "echo channelreadwrite2" {
				message2Received.Store(true)
				wg.Done()
			}
		}
	}()

	// Verify writes to websocket server
	err = websocketchannel.SendMessage([]byte("channelreadwrite1"), websocket.TextMessage)
	require.NoError(t, err, "Error sending message 1")

	err = websocketchannel.SendMessage([]byte("channelreadwrite2"), websocket.TextMessage)
	require.NoError(t, err, "Error sending message 2")

	// Wait for both messages to be received
	wg.Wait()

	assert.True(t, message1Received.Load(), "Didn't read value 1 correctly")
	assert.True(t, message2Received.Load(), "Didn't read value 2 correctly")

	err = websocketchannel.Close()
	require.NoError(t, err, "Error closing the websocket connection.")
	assert.False(t, websocketchannel.IsOpen(), "IsOpen is not set to false.")

	t.Log("Ending test: TestMultipleReadWriteWebSocketChannel")
}

func TestWebSocketChannel_ReadMessage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(handlerToBeTested))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	log := log.NewMockLog()

	websocketchannel, err := NewWebSocketChannel(u.String(), defaultChannelToken, log)
	require.NoError(t, err)

	// Open the websocket connection
	err = websocketchannel.Open()
	require.NoError(t, err, "Error opening the websocket connection.")

	// Send a message and read it back
	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		msg, err := websocketchannel.ReadMessage()
		assert.NoError(t, err)
		assert.Equal(t, "echo readmessage", string(msg))
	}()

	err = websocketchannel.SendMessage([]byte("readmessage"), websocket.TextMessage)
	require.NoError(t, err)
	wg.Wait()

	// Close the websocket connection and verify ReadMessage returns an error
	err = websocketchannel.Close()
	require.NoError(t, err)
	_, err = websocketchannel.ReadMessage()
	assert.Error(t, err)
}
