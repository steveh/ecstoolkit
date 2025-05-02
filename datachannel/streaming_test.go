package datachannel

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	communicatorMocks "github.com/steveh/ecstoolkit/communicator/mocks"
	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/encryption"
	"github.com/steveh/ecstoolkit/encryption/mocks"
	"github.com/steveh/ecstoolkit/jsonutil"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var errMock = errors.New("mock error")

func mockPayload() []byte {
	return []byte("testPayload")
}

func mockCipherTextKey() []byte {
	return []byte("cipher-text-key")
}

func addDataToIncomingMessageBuffer(dataChannel *DataChannel, streamingMessage StreamingMessage) {
	_ = dataChannel.incomingMessageBuffer.SetUnlessFull(streamingMessage.SequenceNumber, streamingMessage)
}

const (
	streamURL    = "stream-url"
	channelToken = "channel-token"
	sessionID    = "session-id"
	clientID     = "client-id"
	kmsKeyID     = "some-key-id"
	instanceID   = "some-instance-id"
	messageID    = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	messageType  = message.OutputStreamMessage

	schemaVersion            = uint32(1)
	streamDataSequenceNumber = int64(0)
	expectedSequenceNumber   = int64(0)
	createdDate              = uint64(1503434274948)
)

func TestNewDataChannel(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}
	mockEncryptorBuilder := mocks.NewMockEncryptorBuilder(nil)

	dataChannel, err := NewDataChannel(mockWsChannel, mockEncryptorBuilder, clientID, sessionID, instanceID, mockLogger)
	require.NoError(t, err)

	assert.Equal(t, config.RolePublishSubscribe, dataChannel.role)
	assert.Equal(t, clientID, dataChannel.clientID)
	assert.Equal(t, int64(0), dataChannel.expectedSequenceNumber.Load())
	assert.Equal(t, int64(0), dataChannel.streamDataSequenceNumber.Load())
	assert.NotNil(t, dataChannel.outgoingMessageBuffer)
	assert.NotNil(t, dataChannel.incomingMessageBuffer)
	assert.InDelta(t, float64(config.DefaultRoundTripTime), dataChannel.roundTripTime, 0.01)
	assert.InDelta(t, float64(config.DefaultRoundTripTimeVariation), dataChannel.roundTripTimeVariation, 0.01)
	assert.Equal(t, config.DefaultTransmissionTimeout, dataChannel.retransmissionTimeout)
}

func TestReconnect(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)

	mockWsChannel.On("Close", mock.Anything).Return(nil)
	mockWsChannel.On("Open", mock.Anything).Return(nil)
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockWsChannel.On("GetStreamURL").Return(streamURL)
	mockWsChannel.On("GetChannelToken").Return(channelToken)

	// test reconnect
	err := dataChannel.reconnect()

	require.NoError(t, err)
	mockWsChannel.AssertExpectations(t)
}

func TestOpen(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)

	mockWsChannel.On("Open", mock.Anything).Return(nil)
	mockWsChannel.On("GetChannelToken").Return(channelToken)
	mockWsChannel.On("GetStreamURL").Return(streamURL)
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything).Return(nil)

	err := dataChannel.open()

	require.NoError(t, err)
	mockWsChannel.AssertExpectations(t)
}

func TestClose(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}
	mockWsChannel.On("GetStreamURL").Return(streamURL)

	dataChannel := getDataChannel(t, mockWsChannel)

	mockWsChannel.On("Close", mock.Anything).Return(nil)

	// test close
	err := dataChannel.Close()

	require.NoError(t, err)
	mockWsChannel.AssertExpectations(t)
}

func TestFinalizeDataChannelHandshake(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)

	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockWsChannel.On("GetStreamURL").Return(streamURL)

	err := dataChannel.finalizeDataChannelHandshake(channelToken)

	require.NoError(t, err)
	assert.Equal(t, streamURL, dataChannel.wsChannel.GetStreamURL())
	mockWsChannel.AssertExpectations(t)
}

func TestSendMessage(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)

	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := dataChannel.SendMessage([]byte{10}, websocket.BinaryMessage)

	require.NoError(t, err)
	mockWsChannel.AssertExpectations(t)
}

func TestSendInputDataMessage(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)

	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything).Return(nil)

	err := dataChannel.SendInputDataMessage(message.Output, mockPayload())
	require.NoError(t, err, "Error sending input data message")

	assert.Equal(t, streamDataSequenceNumber+1, dataChannel.streamDataSequenceNumber.Load())
	assert.Equal(t, 1, dataChannel.outgoingMessageBuffer.messages.Len())
	mockWsChannel.AssertExpectations(t)
}

func TestProcessAcknowledgedMessage(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	_, streamingMessages := getClientAndStreamingMessageList()

	dataChannel := getDataChannel(t, mockWsChannel)
	dataChannel.outgoingMessageBuffer.ForcePushBack(streamingMessages[0])

	dataStreamAcknowledgeContent := message.AcknowledgeContent{
		MessageType:         messageType,
		MessageID:           messageID,
		SequenceNumber:      0,
		IsSequentialMessage: true,
	}

	dataChannel.processAcknowledgedMessage(dataStreamAcknowledgeContent)
	assert.Equal(t, 0, dataChannel.outgoingMessageBuffer.messages.Len())
}

type mockStreamingMessage struct{}

func (s mockStreamingMessage) GetRoundTripTime() time.Duration {
	return 140 * time.Millisecond
}

func TestCalculateRetransmissionTimeout(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)

	streamingMessage := mockStreamingMessage{}

	dataChannel.calculateRetransmissionTimeout(&streamingMessage)
	assert.Equal(t, int64(105), int64(time.Duration(dataChannel.roundTripTime)/time.Millisecond))
	assert.Equal(t, int64(10), int64(time.Duration(dataChannel.roundTripTimeVariation)/time.Millisecond))
	assert.Equal(t, int64(145), int64(dataChannel.retransmissionTimeout/time.Millisecond))
}

func TestAddDataToOutgoingMessageBuffer(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)
	dataChannel.outgoingMessageBuffer.capacity = 2

	_, streamingMessages := getClientAndStreamingMessageList()

	dataChannel.outgoingMessageBuffer.ForcePushBack(streamingMessages[0])
	assert.Equal(t, 1, dataChannel.outgoingMessageBuffer.messages.Len())
	bufferedStreamMessage, ok := dataChannel.outgoingMessageBuffer.messages.Front().Value.(StreamingMessage)
	assert.True(t, ok, "Failed to type assert to StreamingMessage")
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)

	dataChannel.outgoingMessageBuffer.ForcePushBack(streamingMessages[1])
	assert.Equal(t, 2, dataChannel.outgoingMessageBuffer.messages.Len())
	value := dataChannel.outgoingMessageBuffer.messages.Front().Value
	bufferedStreamMessage, ok = value.(StreamingMessage)
	assert.True(t, ok, "Failed to type assert to StreamingMessage")
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)

	value = dataChannel.outgoingMessageBuffer.messages.Back().Value
	bufferedStreamMessage, ok = value.(StreamingMessage)
	assert.True(t, ok, "Failed to type assert to StreamingMessage")
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)

	dataChannel.outgoingMessageBuffer.ForcePushBack(streamingMessages[2])
	assert.Equal(t, 2, dataChannel.outgoingMessageBuffer.messages.Len())
	value = dataChannel.outgoingMessageBuffer.messages.Front().Value
	bufferedStreamMessage, ok = value.(StreamingMessage)
	assert.True(t, ok, "Failed to type assert to StreamingMessage")
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)

	value = dataChannel.outgoingMessageBuffer.messages.Back().Value
	bufferedStreamMessage, ok = value.(StreamingMessage)
	assert.True(t, ok, "Failed to type assert to StreamingMessage")
	assert.Equal(t, int64(2), bufferedStreamMessage.SequenceNumber)
}

func TestAddDataToIncomingMessageBuffer(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)
	dataChannel.incomingMessageBuffer.capacity = 2

	_, streamingMessages := getClientAndStreamingMessageList()

	addDataToIncomingMessageBuffer(dataChannel, streamingMessages[0])
	assert.Len(t, dataChannel.incomingMessageBuffer.messages, 1)
	bufferedStreamMessage := dataChannel.incomingMessageBuffer.messages[0]
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)

	addDataToIncomingMessageBuffer(dataChannel, streamingMessages[1])
	assert.Len(t, dataChannel.incomingMessageBuffer.messages, 2)
	bufferedStreamMessage = dataChannel.incomingMessageBuffer.messages[0]
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.incomingMessageBuffer.messages[1]
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)

	addDataToIncomingMessageBuffer(dataChannel, streamingMessages[2])
	assert.Len(t, dataChannel.incomingMessageBuffer.messages, 2)
	bufferedStreamMessage = dataChannel.incomingMessageBuffer.messages[0]
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.incomingMessageBuffer.messages[1]
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.incomingMessageBuffer.messages[2]
	assert.Nil(t, bufferedStreamMessage.Content)
}

func TestRemoveDataFromOutgoingMessageBuffer(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	_, streamingMessages := getClientAndStreamingMessageList()

	dataChannel := getDataChannel(t, mockWsChannel)
	for i := range 3 {
		dataChannel.outgoingMessageBuffer.ForcePushBack(streamingMessages[i])
	}

	dataChannel.outgoingMessageBuffer.messages.Remove(dataChannel.outgoingMessageBuffer.messages.Front())
	assert.Equal(t, 2, dataChannel.outgoingMessageBuffer.messages.Len())
}

func TestRemoveDataFromIncomingMessageBuffer(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	_, streamingMessages := getClientAndStreamingMessageList()

	dataChannel := getDataChannel(t, mockWsChannel)
	for i := range 3 {
		addDataToIncomingMessageBuffer(dataChannel, streamingMessages[i])
	}

	_, _ = dataChannel.incomingMessageBuffer.Remove(0)
	assert.Len(t, dataChannel.incomingMessageBuffer.messages, 2)
}

func TestResendStreamDataMessageScheduler(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)

	_, streamingMessages := getClientAndStreamingMessageList()

	for i := range 3 {
		dataChannel.outgoingMessageBuffer.ForcePushBack(streamingMessages[i])
	}

	var wg sync.WaitGroup
	// Use atomic counter instead of mutex
	var sendMessageCallCount atomic.Int64

	wg.Add(1)
	// Spawning a separate go routine to close websocket connection.
	// This is required as ResendStreamDataMessageScheduler has a for loop which will continuously resend data until channel is closed.
	go func() {
		time.Sleep(1 * time.Second)
		wg.Done()
	}()

	// Setup mock expectation for SendMessage
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything).Return(func([]byte, int) error {
		// Use atomic increment instead of mutex locking
		sendMessageCallCount.Add(1)

		return nil
	})

	dataChannel.resendStreamDataMessageScheduler()

	wg.Wait()
	// Assert that SendMessage was called on the mock channel
	mockWsChannel.AssertExpectations(t)
	// Use atomic load instead of mutex locking
	assert.Positive(t, sendMessageCallCount.Load(), "SendMessage should have been called at least once") // Check if called, exact count might vary
}

func TestDataChannelIncomingMessageHandlerForExpectedInputStreamDataMessage(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)

	SendAcknowledgeMessageCallCount := 0
	// Setup mock expectation for SendMessage
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything).Return(func([]byte, int) error {
		SendAcknowledgeMessageCallCount++

		return nil
	})

	var handler OutputStreamDataMessageHandler = func(_ message.ClientMessage) (bool, error) {
		return true, nil
	}

	var stopHandler StopHandler = func() error {
		return nil
	}

	serializedClientMessages, streamingMessages := getClientAndStreamingMessageList()

	dataChannel.RegisterOutputStreamHandler(handler, true)
	// First scenario is to test when incoming message sequence number matches with expected sequence number
	// and no message found in incomingMessageBuffer
	err := dataChannel.outputMessageHandler(context.TODO(), stopHandler, serializedClientMessages[0])
	require.NoError(t, err)
	assert.Equal(t, int64(1), dataChannel.expectedSequenceNumber.Load())
	assert.Empty(t, dataChannel.incomingMessageBuffer.messages)
	assert.Equal(t, 1, SendAcknowledgeMessageCallCount)

	// Second scenario is to test when incoming message sequence number matches with expected sequence number
	// and there are more messages found in incomingMessageBuffer to be processed
	addDataToIncomingMessageBuffer(dataChannel, streamingMessages[2])
	addDataToIncomingMessageBuffer(dataChannel, streamingMessages[6])
	addDataToIncomingMessageBuffer(dataChannel, streamingMessages[4])
	addDataToIncomingMessageBuffer(dataChannel, streamingMessages[3])

	err = dataChannel.outputMessageHandler(context.TODO(), stopHandler, serializedClientMessages[1])
	require.NoError(t, err)
	assert.Equal(t, int64(5), dataChannel.expectedSequenceNumber.Load())
	assert.Len(t, dataChannel.incomingMessageBuffer.messages, 1)

	// All messages from buffer should get processed except sequence number 6 as expected number to be processed at this time is 5
	bufferedStreamMessage := dataChannel.incomingMessageBuffer.messages[6]
	assert.Equal(t, int64(6), bufferedStreamMessage.SequenceNumber)
}

func TestDataChannelIncomingMessageHandlerForUnexpectedInputStreamDataMessage(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)
	dataChannel.incomingMessageBuffer.capacity = 2

	serializedClientMessages, _ := getClientAndStreamingMessageList()

	SendAcknowledgeMessageCallCount := 0
	// Setup mock expectation for SendMessage
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything).Return(func([]byte, int) error {
		SendAcknowledgeMessageCallCount++

		return nil
	})

	var stopHandler StopHandler = func() error {
		return nil
	}

	err := dataChannel.outputMessageHandler(context.TODO(), stopHandler, serializedClientMessages[1])
	require.NoError(t, err)

	err = dataChannel.outputMessageHandler(context.TODO(), stopHandler, serializedClientMessages[2])
	require.NoError(t, err)

	err = dataChannel.outputMessageHandler(context.TODO(), stopHandler, serializedClientMessages[3])
	require.NoError(t, err)

	// Since capacity of incomingMessageBuffer is 2, stream data with sequence number 3 should be ignored without sending acknowledgement
	assert.Equal(t, expectedSequenceNumber, dataChannel.expectedSequenceNumber.Load())
	assert.Len(t, dataChannel.incomingMessageBuffer.messages, 2)
	assert.Equal(t, 2, SendAcknowledgeMessageCallCount)

	bufferedStreamMessage := dataChannel.incomingMessageBuffer.messages[1]
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.incomingMessageBuffer.messages[2]
	assert.Equal(t, int64(2), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.incomingMessageBuffer.messages[3]
	assert.Nil(t, bufferedStreamMessage.Content)
}

func TestDataChannelIncomingMessageHandlerForAcknowledgeMessage(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)

	_, streamingMessages := getClientAndStreamingMessageList()

	var stopHandler StopHandler = func() error {
		return nil
	}

	acknowledgeContent := message.AcknowledgeContent{
		MessageType:         messageType,
		MessageID:           messageID,
		SequenceNumber:      1,
		IsSequentialMessage: true,
	}

	payload, err := json.Marshal(acknowledgeContent)
	if err != nil {
		t.Fatalf("marshaling acknowledge content: %v", err)
	}

	clientMessage := getClientMessage(0, message.AcknowledgeMessage, uint32(message.Output), payload)

	serializedClientMessage, err := clientMessage.SerializeClientMessage()
	require.NoError(t, err)

	streamingMessages[1].Content = serializedClientMessage

	for i := range 3 {
		dataChannel.outgoingMessageBuffer.ForcePushBack(streamingMessages[i])
	}

	acknowledgeMessageDetected := func(dc *DataChannel) bool {
		for e := dc.outgoingMessageBuffer.messages.Front(); e != nil; e = e.Next() {
			sm, ok := e.Value.(StreamingMessage)
			require.True(t, ok, "Failed to type assert to StreamingMessage")

			var cm message.ClientMessage
			err := cm.DeserializeClientMessage(sm.Content)
			require.NoError(t, err)

			if cm.MessageType == message.AcknowledgeMessage {
				return true
			}
		}

		return false
	}

	assert.Equal(t, 3, dataChannel.outgoingMessageBuffer.messages.Len())
	assert.True(t, acknowledgeMessageDetected(dataChannel))

	if err := dataChannel.outputMessageHandler(context.TODO(), stopHandler, serializedClientMessage); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, dataChannel.outgoingMessageBuffer.messages.Len())
	assert.False(t, acknowledgeMessageDetected(dataChannel))
}

func TestDataChannelIncomingMessageHandlerForPausePublicationessage(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	// Setup mock expectation for SendMessage
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything).Return(func([]byte, int) error {
		return nil
	})

	dataChannel := getDataChannel(t, mockWsChannel)

	serializedClientMessages, _ := getClientAndStreamingMessageList()

	size := 5
	streamingMessages := make([]StreamingMessage, size)
	serializedClientMessage := make([][]byte, size)

	for i := range size {
		clientMessage := getClientMessage(int64(i), message.PausePublicationMessage, uint32(message.Output), []byte(""))
		serializedClientMessage[i], _ = clientMessage.SerializeClientMessage()

		streamingMessages[i] = StreamingMessage{
			serializedClientMessage[i],
			int64(i),
			time.Now(),
			0,
		}
	}

	var handler OutputStreamDataMessageHandler = func(_ message.ClientMessage) (bool, error) {
		return true, nil
	}

	var stopHandler StopHandler = func() error {
		return nil
	}

	dataChannel.RegisterOutputStreamHandler(handler, true)
	err := dataChannel.outputMessageHandler(context.TODO(), stopHandler, serializedClientMessages[0])
	require.NoError(t, err)
}

type mockEncryptorBuilder struct {
	t         *testing.T
	encryptor encryption.IEncrypter
}

//nolint:ireturn
func (b *mockEncryptorBuilder) Build(_ context.Context, providedKMSKeyID string, providedSessionID string, providedTargetID string) (encryption.IEncrypter, error) {
	assert.Equal(b.t, kmsKeyID, providedKMSKeyID)
	assert.Equal(b.t, sessionID, providedSessionID)
	assert.Equal(b.t, instanceID, providedTargetID)

	return b.encryptor, nil
}

func TestHandshakeRequestHandler(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)
	mockEncrypter := mocks.IEncrypter{}

	dataChannel.encryptorBuilder = &mockEncryptorBuilder{encryptor: &mockEncrypter, t: t}

	handshakeRequestBytes, err := json.Marshal(buildHandshakeRequest(t))
	if err != nil {
		t.Fatalf("marshaling handshake request: %v", err)
	}

	srcMessage := getClientMessage(0, message.OutputStreamMessage, uint32(message.HandshakeRequestPayloadType), handshakeRequestBytes)

	srcMessageBytes, err := srcMessage.SerializeClientMessage()
	require.NoError(t, err)

	mockEncrypter.On("GetEncryptedDataKey").Return(mockCipherTextKey())

	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything).Return(func(sentData []byte, _ int) error {
		var cm message.ClientMessage
		if err := cm.DeserializeClientMessage(sentData); err != nil {
			t.Fatalf("Failed to deserialize client message: %v", err)
		}

		var handshakeResponse message.HandshakeResponsePayload
		if err := json.Unmarshal(cm.Payload, &handshakeResponse); err != nil {
			t.Fatalf("Failed to unmarshal handshake response: %v", err)
		}

		// Return if any other message type (typically to account for acknowledge)
		if cm.MessageType != message.InputStreamMessage {
			return nil
		}

		if cm.PayloadType != uint32(message.HandshakeResponsePayloadType) {
			return nil
		}

		require.Equal(t, version.Version, handshakeResponse.ClientVersion)

		var expectedActions []message.ProcessedClientAction

		srcActionResult := message.KMSEncryptionResponse{
			KMSCipherTextKey: mockCipherTextKey(),
		}

		var dstActionResult any
		err := jsonutil.Remarshal(srcActionResult, &dstActionResult)
		require.NoError(t, err)

		expectedActions = append(expectedActions, message.ProcessedClientAction{
			ActionType:   message.KMSEncryption,
			ActionStatus: message.Success,
			ActionResult: dstActionResult,
		})

		expectedActions = append(expectedActions, message.ProcessedClientAction{
			ActionType:   message.SessionType,
			ActionStatus: message.Success,
		})

		require.Len(t, handshakeResponse.ProcessedClientActions, 2)
		assert.Equal(t, expectedActions[0], handshakeResponse.ProcessedClientActions[0])
		assert.Equal(t, expectedActions[1], handshakeResponse.ProcessedClientActions[1])

		return nil
	})

	if err := dataChannel.outputMessageHandler(context.TODO(), func() error { return nil }, srcMessageBytes); err != nil {
		t.Errorf("Failed to handle output message: %v", err)
	}
}

func TestHandleOutputMessageForDefaultTypeWithError(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)
	clientMessage := getClientMessage(0, message.OutputStreamMessage,
		uint32(message.Output), mockPayload())
	rawMessage := []byte("rawMessage")

	var handler OutputStreamDataMessageHandler = func(_ message.ClientMessage) (bool, error) {
		return true, errMock
	}

	dataChannel.RegisterOutputStreamHandler(handler, true)

	err := dataChannel.HandleOutputMessage(context.TODO(), clientMessage, rawMessage)
	require.Error(t, err)
}

func TestHandleOutputMessageForExitCodePayloadTypeWithError(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	mockEncrypter := mocks.IEncrypter{}
	mockEncrypter.On("Decrypt", mock.Anything, mock.Anything).Return([]byte{10, 11, 12}, errMock)

	dataChannel := getDataChannel(t, mockWsChannel)

	clientMessage := getClientMessage(0, message.OutputStreamMessage,
		uint32(message.ExitCode), mockPayload())
	dataChannel.encryptionEnabled = true
	dataChannel.encryption = &mockEncrypter

	rawMessage := []byte("rawMessage")

	err := dataChannel.HandleOutputMessage(context.TODO(), clientMessage, rawMessage)
	require.ErrorIs(t, err, errMock)
}

func TestHandleHandshakeRequestWithMessageDeserializeError(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)

	handshakeRequestBytes, err := json.Marshal(buildHandshakeRequest(t))
	if err != nil {
		t.Fatalf("marshaling handshake request: %v", err)
	}

	clientMessage := getClientMessage(0, message.OutputStreamMessage,
		uint32(message.HandshakeCompletePayloadType), handshakeRequestBytes)

	err = dataChannel.handleHandshakeRequest(context.TODO(), clientMessage)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not of type HandshakeRequestPayloadType")
}

func TestProcessOutputMessageWithHandlers(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel := getDataChannel(t, mockWsChannel)

	var handler OutputStreamDataMessageHandler = func(_ message.ClientMessage) (bool, error) {
		return true, errMock
	}

	dataChannel.RegisterOutputStreamHandler(handler, true)

	handshakeRequestBytes, err := json.Marshal(buildHandshakeRequest(t))
	if err != nil {
		t.Fatalf("marshaling handshake request: %v", err)
	}

	clientMessage := getClientMessage(0, message.OutputStreamMessage,
		uint32(message.HandshakeCompletePayloadType), handshakeRequestBytes)

	isHandlerReady, err := dataChannel.processOutputMessageWithHandlers(clientMessage)
	require.Error(t, err)
	assert.True(t, isHandlerReady)
}

func TestProcessSessionTypeHandshakeActionForInteractiveCommands(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	actionParams := []byte("{\"SessionType\":\"InteractiveCommands\"}")
	dataChannel := getDataChannel(t, mockWsChannel)

	err := dataChannel.ProcessSessionTypeHandshakeAction(actionParams)

	// Test that InteractiveCommands is a valid session type
	require.NoError(t, err)
	// Test that InteractiveCommands is translated to Standard_Stream in data channel
	assert.Equal(t, config.ShellPluginName, dataChannel.sessionType)
}

func TestProcessSessionTypeHandshakeActionForNonInteractiveCommands(t *testing.T) {
	t.Parallel()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	actionParams := []byte("{\"SessionType\":\"NonInteractiveCommands\"}")
	dataChannel := getDataChannel(t, mockWsChannel)

	err := dataChannel.ProcessSessionTypeHandshakeAction(actionParams)

	// Test that NonInteractiveCommands is a valid session type
	require.NoError(t, err)
	// Test that NonInteractiveCommands is translated to Standard_Stream in data channel
	assert.Equal(t, config.ShellPluginName, dataChannel.sessionType)
}

func TestProcessFirstMessageOutputMessageFirst(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	outputMessage := message.ClientMessage{
		PayloadType: uint32(message.Output),
		Payload:     []byte("testing"),
	}

	mockEncryptorBuilder := mocks.NewMockEncryptorBuilder(nil)

	dataChannel, err := NewDataChannel(mockWsChannel, mockEncryptorBuilder, clientID, sessionID, instanceID, mockLogger)
	require.NoError(t, err)

	dataChannel.displayHandler = func(_ message.ClientMessage) {}

	_, err = dataChannel.firstMessageHandler(outputMessage)
	if err != nil {
		t.Errorf("Failed to process first message: %v", err)
	}

	assert.Equal(t, config.ShellPluginName, dataChannel.sessionType)
	assert.True(t, <-dataChannel.isSessionTypeSet)
}

func TestOpenWithRetryWithError(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()

	mockWsChannel := &communicatorMocks.IWebSocketChannel{}

	mockEncryptorBuilder := mocks.NewMockEncryptorBuilder(nil)

	dataChannel, err := NewDataChannel(mockWsChannel, mockEncryptorBuilder, clientID, sessionID, instanceID, mockLogger)
	require.NoError(t, err)

	// First reconnection failed when open data channel, success after retry
	mockWsChannel.On("Open").Return(errMock).Once()
	mockWsChannel.On("Open").Return(nil).Once()
	mockWsChannel.On("SetOnMessage", mock.Anything)
	mockWsChannel.On("SetOnError", mock.Anything)
	mockWsChannel.On("GetStreamURL").Return(streamURL)
	mockWsChannel.On("GetChannelToken").Return(channelToken)
	mockWsChannel.On("Close").Return(nil)
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything).Return(nil)

	_, err = dataChannel.Open(
		context.TODO(),
		func(_ message.ClientMessage) {},
		func(_ context.Context) (string, error) { return "", nil },
		func(_ context.Context) error { return nil },
	)
	require.NoError(t, err)
}

func buildHandshakeRequest(t *testing.T) message.HandshakeRequestPayload {
	t.Helper()

	handshakeRquest := message.HandshakeRequestPayload{}
	handshakeRquest.AgentVersion = "10.0.0.1"
	handshakeRquest.RequestedClientActions = []message.RequestedClientAction{}

	requestedAction := message.RequestedClientAction{}
	requestedAction.ActionType = message.KMSEncryption

	var err error
	requestedAction.ActionParameters, err = json.Marshal(message.KMSEncryptionRequest{KMSKeyID: kmsKeyID})
	require.NoError(t, err)

	handshakeRquest.RequestedClientActions = append(handshakeRquest.RequestedClientActions, requestedAction)

	requestedAction = message.RequestedClientAction{}
	requestedAction.ActionType = message.SessionType
	requestedAction.ActionParameters, err = json.Marshal(message.SessionTypeRequest{SessionType: config.ShellPluginName})
	require.NoError(t, err)

	handshakeRquest.RequestedClientActions = append(handshakeRquest.RequestedClientActions, requestedAction)

	return handshakeRquest
}

func getDataChannel(t *testing.T, mockWsChannel *communicatorMocks.IWebSocketChannel) *DataChannel {
	t.Helper()

	mockLogger := log.NewMockLog()

	mockEncryptorBuilder := mocks.NewMockEncryptorBuilder(nil)

	dataChannel, err := NewDataChannel(mockWsChannel, mockEncryptorBuilder, clientID, sessionID, instanceID, mockLogger)
	require.NoError(t, err)

	return dataChannel
}

// GetClientMessage constructs and returns ClientMessage with given sequenceNumber, messageType & payload.
func getClientMessage(sequenceNumber int64, messageType string, payloadType uint32, payload []byte) message.ClientMessage {
	messageUUID, err := uuid.Parse(messageID)
	if err != nil {
		panic(err)
	}

	clientMessage := message.ClientMessage{
		MessageType:    messageType,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: sequenceNumber,
		Flags:          2,
		MessageID:      messageUUID,
		PayloadType:    payloadType,
		Payload:        payload,
	}

	return clientMessage
}

func getClientAndStreamingMessageList() ([][]byte, []StreamingMessage) {
	const size = 7

	var payload string

	streamingMessages := make([]StreamingMessage, size)
	serializedClientMessage := make([][]byte, size)

	var err error

	for i := range size {
		payload = "testPayload" + strconv.Itoa(i)
		clientMessage := getClientMessage(int64(i), messageType, uint32(message.Output), []byte(payload))

		serializedClientMessage[i], err = clientMessage.SerializeClientMessage()
		if err != nil {
			panic(err)
		}

		streamingMessages[i] = StreamingMessage{
			serializedClientMessage[i],
			int64(i),
			time.Now(),
			0,
		}
	}

	return serializedClientMessage, streamingMessages
}
