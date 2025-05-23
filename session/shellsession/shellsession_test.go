// Package shellsession starts shell session.
package shellsession

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/steveh/ecstoolkit/communicator/mocks"
	"github.com/steveh/ecstoolkit/datachannel"
	dataChannelMock "github.com/steveh/ecstoolkit/datachannel/mocks"
	encryptionmocks "github.com/steveh/ecstoolkit/encryption/mocks"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	expectedSequenceNumber = int64(0)
	clientID               = "clientId"
	sessionID              = "sessionId"
	instanceID             = "instanceId"
)

var errMock = errors.New("mock error")

func TestName(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()

	shellSession := ShellSession{
		logger: mockLogger,
	}
	name := shellSession.Name()
	assert.Equal(t, "Standard_Stream", name)
}

func TestInitialize(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()
	mockDataChannel := &dataChannelMock.IDataChannel{}

	session, err := session.NewSession(nil, mockDataChannel, "", mockLogger)
	require.NoError(t, err)

	mockDataChannel.On("RegisterOutputStreamHandler", mock.Anything, true).Times(1)
	mockDataChannel.On("RegisterIncomingMessageHandler", mock.Anything, mock.Anything)
	mockDataChannel.On("RegisterStopHandler", mock.Anything)

	shellSession, err := NewShellSession(mockLogger, session)
	require.NoError(t, err, "Initialize port session")

	assert.Equal(t, shellSession.session, session)
}

//nolint:paralleltest // uses signal handling
func TestHandleControlSignals(t *testing.T) {
	mockLogger := log.NewMockLog()
	mockDataChannel := &dataChannelMock.IDataChannel{}

	sess, err := session.NewSession(nil, mockDataChannel, "", mockLogger)
	require.NoError(t, err)

	shellSession := ShellSession{
		logger:  mockLogger,
		session: sess,
	}

	waitCh := make(chan int, 1)
	counter := 0
	sendDataMessage := func() error {
		counter++

		return errMock
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

		go func() {
			err := shellSession.HandleControlSignals(context.TODO())
			assert.ErrorIs(t, err, errMock)
		}()

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
	t.Parallel()

	sizeData := message.SizeData{
		Cols: 100,
		Rows: 100,
	}

	sizeDataBytes, err := json.Marshal(sizeData)
	if err != nil {
		t.Fatalf("marshaling size data: %v", err)
	}

	mockWsChannel := &mocks.IWebSocketChannel{}
	dataChannel := getDataChannelWithMockWs(t, mockWsChannel)

	SendMessageCallCount := 0
	// Mock SendMessage on the wsChannel
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything).Return(func([]byte, int) error {
		SendMessageCallCount++

		return nil
	})

	err = dataChannel.SendInputDataMessage(message.Size, sizeDataBytes)
	require.NoError(t, err)
	assert.Equal(t, expectedSequenceNumber, dataChannel.GetExpectedSequenceNumber())
	// Assert that SendMessage was called on the mock channel
	mockWsChannel.AssertExpectations(t)
	assert.Equal(t, 1, SendMessageCallCount)
}

func TestTerminalResizeWhenSessionSizeDataIsNotEqualToActualSize(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()
	mockWsChannel := &mocks.IWebSocketChannel{}
	dataChannel := getDataChannelWithMockWs(t, mockWsChannel)

	sess, err := session.NewSession(nil, dataChannel, "", mockLogger)
	require.NoError(t, err)

	sizeData := message.SizeData{
		Cols: 100,
		Rows: 100,
	}

	shellSession := ShellSession{
		session:  sess,
		SizeData: sizeData,
		logger:   mockLogger,
		terminalSizer: func(_ int) (int, int, error) {
			return 123, 123, nil
		},
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

	// Use atomic counter to avoid race condition
	var sendMessageCallCount atomic.Int32

	// Mock SendMessage on the wsChannel
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything).Return(func([]byte, int) error {
		sendMessageCallCount.Add(1)

		return nil
	})

	go func() {
		err := shellSession.handleTerminalResize(context.TODO())
		assert.NoError(t, err)
	}()

	wg.Wait()
	// Assert that SendMessage was called on the mock channel
	mockWsChannel.AssertExpectations(t)
	assert.Equal(t, int32(1), sendMessageCallCount.Load())
}

func TestProcessStreamMessagePayload(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()
	mockDataChannel := &dataChannelMock.IDataChannel{}

	sess, err := session.NewSession(nil, mockDataChannel, "", mockLogger)
	require.NoError(t, err)

	shellSession := ShellSession{
		session: sess,
		logger:  mockLogger,
	}

	msg := message.ClientMessage{
		Payload: []byte("Hello Agent\n"),
	}
	isReady, err := shellSession.ProcessStreamMessagePayload(msg)
	assert.True(t, isReady)
	require.NoError(t, err)
}

// Helper function to get DataChannel with a specific mock WebSocket channel.
func getDataChannelWithMockWs(t *testing.T, mockWsChannel *mocks.IWebSocketChannel) *datachannel.DataChannel {
	t.Helper()

	mockLogger := log.NewMockLog()
	mockEncryptorBuilder := encryptionmocks.NewMockEncryptorBuilder(nil)

	dataChannel, err := datachannel.NewDataChannel(mockWsChannel, mockEncryptorBuilder, clientID, sessionID, instanceID, mockLogger)
	require.NoError(t, err)

	return dataChannel
}
