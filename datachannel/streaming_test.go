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

// datachannel package implement data channel for interactive sessions.
package datachannel

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	communicatorMocks "github.com/steveh/ecstoolkit/communicator/mocks"
	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/encryption"
	"github.com/steveh/ecstoolkit/encryption/mocks"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	outputMessageType                           = message.OutputStreamMessage
	serializedClientMessages, streamingMessages = getClientAndStreamingMessageList(7)
	logger                                      = log.NewMockLog()
	mockWsChannel                               = &communicatorMocks.IWebSocketChannel{}
	streamURL                                   = "stream-url"
	channelToken                                = "channel-token"
	sessionID                                   = "session-id"
	clientID                                    = "client-id"
	kmsKeyID                                    = "some-key-id"
	instanceID                                  = "some-instance-id"
	cipherTextKey                               = []byte("cipher-text-key")
	mockLogger                                  = log.NewMockLog()
	messageType                                 = message.OutputStreamMessage
	schemaVersion                               = uint32(1)
	messageID                                   = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	createdDate                                 = uint64(1503434274948)
	payload                                     = []byte("testPayload")
	streamDataSequenceNumber                    = int64(0)
	expectedSequenceNumber                      = int64(0)
)

func TestNewDataChannel(t *testing.T) {
	isAwsCliUpgradeNeeded := false
	mockKMSClient := &kms.Client{}

	datachannel, err := NewDataChannel(mockKMSClient, clientID, sessionID, instanceID, isAwsCliUpgradeNeeded, mockLogger)
	require.NoError(t, err)

	assert.Equal(t, config.RolePublishSubscribe, datachannel.role)
	assert.Equal(t, clientID, datachannel.clientID)
	assert.Equal(t, int64(0), datachannel.expectedSequenceNumber)
	assert.Equal(t, int64(0), datachannel.streamDataSequenceNumber)
	assert.NotNil(t, datachannel.outgoingMessageBuffer)
	assert.NotNil(t, datachannel.incomingMessageBuffer)
	assert.InDelta(t, float64(config.DefaultRoundTripTime), datachannel.roundTripTime, 0.01)
	assert.InDelta(t, float64(config.DefaultRoundTripTimeVariation), datachannel.roundTripTimeVariation, 0.01)
	assert.Equal(t, config.DefaultTransmissionTimeout, datachannel.retransmissionTimeout)
}

func TestSetWsChannel(t *testing.T) {
	datachannel := getDataChannel(t)

	mockWsChannel.On("GetStreamURL").Return(streamURL)
	mockWsChannel.On("GetChannelToken").Return(channelToken)

	datachannel.SetWsChannel(mockWsChannel)

	assert.Equal(t, streamURL, datachannel.wsChannel.GetStreamURL())
	assert.Equal(t, channelToken, datachannel.wsChannel.GetChannelToken())
	mockWsChannel.AssertExpectations(t)
}

func TestReconnect(t *testing.T) {
	datachannel := getDataChannel(t)

	mockWsChannel.On("Close", mock.Anything).Return(nil)
	mockWsChannel.On("Open", mock.Anything).Return(nil)
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// test reconnect
	err := datachannel.Reconnect(mockLogger)

	require.NoError(t, err)
	mockWsChannel.AssertExpectations(t)
}

func TestOpen(t *testing.T) {
	datachannel := getDataChannel(t)

	mockWsChannel.On("Open", mock.Anything).Return(nil)

	err := datachannel.Open(mockLogger)

	require.NoError(t, err)
	mockWsChannel.AssertExpectations(t)
}

func TestClose(t *testing.T) {
	datachannel := getDataChannel(t)

	mockWsChannel.On("Close", mock.Anything).Return(nil)

	// test close
	err := datachannel.Close(mockLogger)

	require.NoError(t, err)
	mockWsChannel.AssertExpectations(t)
}

func TestFinalizeDataChannelHandshake(t *testing.T) {
	datachannel := getDataChannel(t)

	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockWsChannel.On("GetStreamURL").Return(streamURL)

	err := datachannel.FinalizeDataChannelHandshake(mockLogger, channelToken)

	require.NoError(t, err)
	assert.Equal(t, streamURL, datachannel.wsChannel.GetStreamURL())
	mockWsChannel.AssertExpectations(t)
}

func TestSendMessage(t *testing.T) {
	datachannel := getDataChannel(t)

	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := datachannel.SendMessage([]byte{10}, websocket.BinaryMessage)

	require.NoError(t, err)
	mockWsChannel.AssertExpectations(t)
}

func TestSendInputDataMessage(t *testing.T) {
	dataChannel := getDataChannel(t)

	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := dataChannel.SendInputDataMessage(mockLogger, message.Output, payload)
	require.NoError(t, err, "Error sending input data message")

	assert.Equal(t, streamDataSequenceNumber+1, dataChannel.streamDataSequenceNumber)
	assert.Equal(t, 1, dataChannel.outgoingMessageBuffer.Messages.Len())
	mockWsChannel.AssertExpectations(t)
}

func TestProcessAcknowledgedMessage(t *testing.T) {
	dataChannel := getDataChannel(t)
	dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[0])

	dataStreamAcknowledgeContent := message.AcknowledgeContent{
		MessageType:         messageType,
		MessageID:           messageID,
		SequenceNumber:      0,
		IsSequentialMessage: true,
	}
	err := dataChannel.ProcessAcknowledgedMessage(mockLogger, dataStreamAcknowledgeContent)
	require.NoError(t, err, "Error processing acknowledged message")
	assert.Equal(t, 0, dataChannel.outgoingMessageBuffer.Messages.Len())
}

type mockStreamingMessage struct{}

func (s mockStreamingMessage) GetRoundTripTime() time.Duration {
	return 140 * time.Millisecond
}

func TestCalculateRetransmissionTimeout(t *testing.T) {
	dataChannel := getDataChannel(t)

	streamingMessage := mockStreamingMessage{}

	dataChannel.CalculateRetransmissionTimeout(&streamingMessage)
	assert.Equal(t, int64(105), int64(time.Duration(dataChannel.roundTripTime)/time.Millisecond))
	assert.Equal(t, int64(10), int64(time.Duration(dataChannel.roundTripTimeVariation)/time.Millisecond))
	assert.Equal(t, int64(145), int64(dataChannel.retransmissionTimeout/time.Millisecond))
}

func TestAddDataToOutgoingMessageBuffer(t *testing.T) {
	dataChannel := getDataChannel(t)
	dataChannel.outgoingMessageBuffer.Capacity = 2

	dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[0])
	assert.Equal(t, 1, dataChannel.outgoingMessageBuffer.Messages.Len())
	bufferedStreamMessage, ok := dataChannel.outgoingMessageBuffer.Messages.Front().Value.(StreamingMessage)
	assert.True(t, ok, "Failed to type assert to StreamingMessage")
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)

	dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[1])
	assert.Equal(t, 2, dataChannel.outgoingMessageBuffer.Messages.Len())
	value := dataChannel.outgoingMessageBuffer.Messages.Front().Value
	bufferedStreamMessage, ok = value.(StreamingMessage)
	assert.True(t, ok, "Failed to type assert to StreamingMessage")
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)

	value = dataChannel.outgoingMessageBuffer.Messages.Back().Value
	bufferedStreamMessage, ok = value.(StreamingMessage)
	assert.True(t, ok, "Failed to type assert to StreamingMessage")
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)

	dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[2])
	assert.Equal(t, 2, dataChannel.outgoingMessageBuffer.Messages.Len())
	value = dataChannel.outgoingMessageBuffer.Messages.Front().Value
	bufferedStreamMessage, ok = value.(StreamingMessage)
	assert.True(t, ok, "Failed to type assert to StreamingMessage")
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)

	value = dataChannel.outgoingMessageBuffer.Messages.Back().Value
	bufferedStreamMessage, ok = value.(StreamingMessage)
	assert.True(t, ok, "Failed to type assert to StreamingMessage")
	assert.Equal(t, int64(2), bufferedStreamMessage.SequenceNumber)
}

func TestAddDataToIncomingMessageBuffer(t *testing.T) {
	dataChannel := getDataChannel(t)
	dataChannel.incomingMessageBuffer.Capacity = 2

	dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[0])
	assert.Len(t, dataChannel.incomingMessageBuffer.Messages, 1)
	bufferedStreamMessage := dataChannel.incomingMessageBuffer.Messages[0]
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)

	dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[1])
	assert.Len(t, dataChannel.incomingMessageBuffer.Messages, 2)
	bufferedStreamMessage = dataChannel.incomingMessageBuffer.Messages[0]
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.incomingMessageBuffer.Messages[1]
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)

	dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[2])
	assert.Len(t, dataChannel.incomingMessageBuffer.Messages, 2)
	bufferedStreamMessage = dataChannel.incomingMessageBuffer.Messages[0]
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.incomingMessageBuffer.Messages[1]
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.incomingMessageBuffer.Messages[2]
	assert.Nil(t, bufferedStreamMessage.Content)
}

func TestRemoveDataFromOutgoingMessageBuffer(t *testing.T) {
	dataChannel := getDataChannel(t)
	for i := range 3 {
		dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[i])
	}

	dataChannel.RemoveDataFromOutgoingMessageBuffer(dataChannel.outgoingMessageBuffer.Messages.Front())
	assert.Equal(t, 2, dataChannel.outgoingMessageBuffer.Messages.Len())
}

func TestRemoveDataFromIncomingMessageBuffer(t *testing.T) {
	dataChannel := getDataChannel(t)
	for i := range 3 {
		dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[i])
	}

	dataChannel.RemoveDataFromIncomingMessageBuffer(0)
	assert.Len(t, dataChannel.incomingMessageBuffer.Messages, 2)
}

func TestResendStreamDataMessageScheduler(t *testing.T) {
	dataChannel := getDataChannel(t)
	mockChannel := &communicatorMocks.IWebSocketChannel{}

	dataChannel.wsChannel = mockChannel
	for i := range 3 {
		dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[i])
	}

	var wg sync.WaitGroup

	wg.Add(1)
	// Spawning a separate go routine to close websocket connection.
	// This is required as ResendStreamDataMessageScheduler has a for loop which will continuously resend data until channel is closed.
	go func() {
		time.Sleep(1 * time.Second)
		wg.Done()
	}()

	SendMessageCallCount := 0
	SendMessageCall = func(_ *DataChannel, _ []byte, _ int) error {
		SendMessageCallCount++

		return nil
	}

	if err := dataChannel.ResendStreamDataMessageScheduler(mockLogger); err != nil {
		t.Errorf("Failed to resend stream data message scheduler: %v", err)
	}

	wg.Wait()
	assert.Greater(t, SendMessageCallCount, 1)
}

func TestDataChannelIncomingMessageHandlerForExpectedInputStreamDataMessage(t *testing.T) {
	dataChannel := getDataChannel(t)
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel

	SendAcknowledgeMessageCallCount := 0
	SendAcknowledgeMessageCall = func(_ log.T, _ *DataChannel, _ message.ClientMessage) error {
		SendAcknowledgeMessageCallCount++

		return nil
	}

	var handler OutputStreamDataMessageHandler = func(_ log.T, _ message.ClientMessage) (bool, error) {
		return true, nil
	}

	var stopHandler Stop = func(_ log.T) error {
		return nil
	}

	dataChannel.RegisterOutputStreamHandler(handler, true)
	// First scenario is to test when incoming message sequence number matches with expected sequence number
	// and no message found in incomingMessageBuffer
	err := dataChannel.outputMessageHandler(context.TODO(), logger, stopHandler, serializedClientMessages[0])
	require.NoError(t, err)
	assert.Equal(t, int64(1), dataChannel.expectedSequenceNumber)
	assert.Empty(t, dataChannel.incomingMessageBuffer.Messages)
	assert.Equal(t, 1, SendAcknowledgeMessageCallCount)

	// Second scenario is to test when incoming message sequence number matches with expected sequence number
	// and there are more messages found in incomingMessageBuffer to be processed
	dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[2])
	dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[6])
	dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[4])
	dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[3])

	err = dataChannel.outputMessageHandler(context.TODO(), logger, stopHandler, serializedClientMessages[1])
	require.NoError(t, err)
	assert.Equal(t, int64(5), dataChannel.expectedSequenceNumber)
	assert.Len(t, dataChannel.incomingMessageBuffer.Messages, 1)

	// All messages from buffer should get processed except sequence number 6 as expected number to be processed at this time is 5
	bufferedStreamMessage := dataChannel.incomingMessageBuffer.Messages[6]
	assert.Equal(t, int64(6), bufferedStreamMessage.SequenceNumber)
}

func TestDataChannelIncomingMessageHandlerForUnexpectedInputStreamDataMessage(t *testing.T) {
	dataChannel := getDataChannel(t)
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel
	dataChannel.incomingMessageBuffer.Capacity = 2

	SendAcknowledgeMessageCallCount := 0
	SendAcknowledgeMessageCall = func(_ log.T, _ *DataChannel, _ message.ClientMessage) error {
		SendAcknowledgeMessageCallCount++

		return nil
	}

	var stopHandler Stop = func(_ log.T) error {
		return nil
	}

	err := dataChannel.outputMessageHandler(context.TODO(), logger, stopHandler, serializedClientMessages[1])
	require.NoError(t, err)

	err = dataChannel.outputMessageHandler(context.TODO(), logger, stopHandler, serializedClientMessages[2])
	require.NoError(t, err)

	err = dataChannel.outputMessageHandler(context.TODO(), logger, stopHandler, serializedClientMessages[3])
	require.NoError(t, err)

	// Since capacity of incomingMessageBuffer is 2, stream data with sequence number 3 should be ignored without sending acknowledgement
	assert.Equal(t, expectedSequenceNumber, dataChannel.expectedSequenceNumber)
	assert.Len(t, dataChannel.incomingMessageBuffer.Messages, 2)
	assert.Equal(t, 2, SendAcknowledgeMessageCallCount)

	bufferedStreamMessage := dataChannel.incomingMessageBuffer.Messages[1]
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.incomingMessageBuffer.Messages[2]
	assert.Equal(t, int64(2), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.incomingMessageBuffer.Messages[3]
	assert.Nil(t, bufferedStreamMessage.Content)
}

func TestDataChannelIncomingMessageHandlerForAcknowledgeMessage(t *testing.T) {
	dataChannel := getDataChannel(t)
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel

	var stopHandler Stop = func(_ log.T) error {
		return nil
	}

	for i := range 3 {
		dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[i])
	}

	ProcessAcknowledgedMessageCallCount := 0
	ProcessAcknowledgedMessageCall = func(_ log.T, _ *DataChannel, _ message.AcknowledgeContent) error {
		ProcessAcknowledgedMessageCallCount++

		return nil
	}

	acknowledgeContent := message.AcknowledgeContent{
		MessageType:         outputMessageType,
		MessageID:           messageID,
		SequenceNumber:      1,
		IsSequentialMessage: true,
	}

	payload, err := json.Marshal(acknowledgeContent)
	if err != nil {
		t.Fatalf("marshaling acknowledge content: %v", err)
	}

	clientMessage := getClientMessage(0, message.AcknowledgeMessage, uint32(message.Output), payload)

	serializedClientMessage, _ := clientMessage.SerializeClientMessage(logger)
	if err := dataChannel.outputMessageHandler(context.TODO(), logger, stopHandler, serializedClientMessage); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, ProcessAcknowledgedMessageCallCount)
	assert.Equal(t, 3, dataChannel.outgoingMessageBuffer.Messages.Len())
}

func TestDataChannelIncomingMessageHandlerForPausePublicationessage(t *testing.T) {
	dataChannel := getDataChannel(t)
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel

	size := 5
	streamingMessages = make([]StreamingMessage, size)
	serializedClientMessage := make([][]byte, size)

	for i := range size {
		clientMessage := getClientMessage(int64(i), message.PausePublicationMessage, uint32(message.Output), []byte(""))
		serializedClientMessage[i], _ = clientMessage.SerializeClientMessage(mockLogger)
		streamingMessages[i] = StreamingMessage{
			serializedClientMessage[i],
			int64(i),
			time.Now(),
			new(int),
		}
	}

	var handler OutputStreamDataMessageHandler = func(_ log.T, _ message.ClientMessage) (bool, error) {
		return true, nil
	}

	var stopHandler Stop = func(_ log.T) error {
		return nil
	}

	dataChannel.RegisterOutputStreamHandler(handler, true)
	err := dataChannel.outputMessageHandler(context.TODO(), logger, stopHandler, serializedClientMessages[0])
	require.NoError(t, err)
}

func TestHandshakeRequestHandler(t *testing.T) {
	dataChannel := getDataChannel(t)
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel
	mockEncrypter := &mocks.IEncrypter{}

	handshakeRequestBytes, err := json.Marshal(buildHandshakeRequest(t))
	if err != nil {
		t.Fatalf("marshaling handshake request: %v", err)
	}

	clientMessage := getClientMessage(0, message.OutputStreamMessage,
		uint32(message.HandshakeRequestPayloadType), handshakeRequestBytes)
	handshakeRequestMessageBytes, _ := clientMessage.SerializeClientMessage(mockLogger)

	newEncrypter = func(_ context.Context, _ log.T, kmsKeyIDInput string, context map[string]string, _ *kms.Client) (encryption.IEncrypter, error) {
		expectedContext := map[string]string{"aws:ssm:SessionId": sessionID, "aws:ssm:TargetId": instanceID}

		assert.Equal(t, kmsKeyID, kmsKeyIDInput)
		assert.Equal(t, expectedContext, context)
		mockEncrypter.On("GetEncryptedDataKey").Return(cipherTextKey)

		return mockEncrypter, nil
	}
	// Mock sending of encryption challenge
	handshakeResponseMatcher := func(sentData []byte) bool {
		clientMessage := &message.ClientMessage{}
		if err := clientMessage.DeserializeClientMessage(mockLogger, sentData); err != nil {
			t.Errorf("Failed to deserialize client message: %v", err)

			return false
		}

		handshakeResponse := message.HandshakeResponsePayload{}

		if err := json.Unmarshal(clientMessage.Payload, &handshakeResponse); err != nil {
			t.Errorf("Failed to unmarshal handshake response: %v", err)

			return false
		}
		// Return true if any other message type (typically to account for acknowledge)
		if clientMessage.MessageType != message.OutputStreamMessage {
			return true
		}

		expectedActions := []message.ProcessedClientAction{}
		processedAction := message.ProcessedClientAction{}
		processedAction.ActionType = message.KMSEncryption
		processedAction.ActionStatus = message.Success
		processedAction.ActionResult = message.KMSEncryptionResponse{
			KMSCipherTextKey: cipherTextKey,
		}
		expectedActions = append(expectedActions, processedAction)

		processedAction = message.ProcessedClientAction{}
		processedAction.ActionType = message.SessionType
		processedAction.ActionStatus = message.Success
		expectedActions = append(expectedActions, processedAction)

		return handshakeResponse.ClientVersion == version.Version &&
			reflect.DeepEqual(handshakeResponse.ProcessedClientActions, expectedActions)
	}
	mockChannel.On("SendMessage", mock.Anything, mock.MatchedBy(handshakeResponseMatcher), mock.Anything).Return(nil)

	if err := dataChannel.outputMessageHandler(context.TODO(), mockLogger, func(_ log.T) error { return nil }, handshakeRequestMessageBytes); err != nil {
		t.Errorf("Failed to handle output message: %v", err)
	}

	assert.Equal(t, mockEncrypter, dataChannel.encryption)
}

func TestHandleOutputMessageForDefaultTypeWithError(t *testing.T) {
	dataChannel := getDataChannel(t)
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel
	clientMessage := getClientMessage(0, message.OutputStreamMessage,
		uint32(message.Output), payload)
	rawMessage := []byte("rawMessage")

	var handler OutputStreamDataMessageHandler = func(_ log.T, _ message.ClientMessage) (bool, error) {
		return true, errors.New("OutputStreamDataMessageHandler Error")
	}

	dataChannel.RegisterOutputStreamHandler(handler, true)

	err := dataChannel.HandleOutputMessage(context.TODO(), mockLogger, clientMessage, rawMessage)
	require.Error(t, err)
}

func TestHandleOutputMessageForExitCodePayloadTypeWithError(t *testing.T) {
	dataChannel := getDataChannel(t)
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel
	clientMessage := getClientMessage(0, message.OutputStreamMessage,
		uint32(message.ExitCode), payload)
	dataChannel.encryptionEnabled = true
	mockEncrypter := &mocks.IEncrypter{}
	dataChannel.encryption = mockEncrypter
	mockErr := errors.New("Decrypt Error")
	mockEncrypter.On("Decrypt", mock.Anything, mock.Anything).Return([]byte{10, 11, 12}, mockErr)

	rawMessage := []byte("rawMessage")

	err := dataChannel.HandleOutputMessage(context.TODO(), mockLogger, clientMessage, rawMessage)
	require.ErrorIs(t, err, mockErr)
}

func TestHandleHandshakeRequestWithMessageDeserializeError(t *testing.T) {
	dataChannel := getDataChannel(t)

	handshakeRequestBytes, err := json.Marshal(buildHandshakeRequest(t))
	if err != nil {
		t.Fatalf("marshaling handshake request: %v", err)
	}
	// Using HandshakeCompletePayloadType to trigger the type check error
	clientMessage := getClientMessage(0, message.OutputStreamMessage,
		uint32(message.HandshakeCompletePayloadType), handshakeRequestBytes)

	err = dataChannel.handleHandshakeRequest(context.TODO(), mockLogger, clientMessage)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ClientMessage PayloadType is not of type HandshakeRequestPayloadType")
}

func TestProcessOutputMessageWithHandlers(t *testing.T) {
	dataChannel := getDataChannel(t)
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel

	var handler OutputStreamDataMessageHandler = func(_ log.T, _ message.ClientMessage) (bool, error) {
		return true, errors.New("OutputStreamDataMessageHandler Error")
	}

	dataChannel.RegisterOutputStreamHandler(handler, true)

	handshakeRequestBytes, err := json.Marshal(buildHandshakeRequest(t))
	if err != nil {
		t.Fatalf("marshaling handshake request: %v", err)
	}

	clientMessage := getClientMessage(0, message.OutputStreamMessage,
		uint32(message.HandshakeCompletePayloadType), handshakeRequestBytes)

	isHandlerReady, err := dataChannel.processOutputMessageWithHandlers(mockLogger, clientMessage)
	require.Error(t, err)
	assert.True(t, isHandlerReady)
}

func TestProcessSessionTypeHandshakeActionForInteractiveCommands(t *testing.T) {
	actionParams := []byte("{\"SessionType\":\"InteractiveCommands\"}")
	dataChannel := getDataChannel(t)

	err := dataChannel.ProcessSessionTypeHandshakeAction(actionParams)

	// Test that InteractiveCommands is a valid session type
	require.NoError(t, err)
	// Test that InteractiveCommands is translated to Standard_Stream in data channel
	assert.Equal(t, config.ShellPluginName, dataChannel.sessionType)
}

func TestProcessSessionTypeHandshakeActionForNonInteractiveCommands(t *testing.T) {
	actionParams := []byte("{\"SessionType\":\"NonInteractiveCommands\"}")
	dataChannel := getDataChannel(t)

	err := dataChannel.ProcessSessionTypeHandshakeAction(actionParams)

	// Test that NonInteractiveCommands is a valid session type
	require.NoError(t, err)
	// Test that NonInteractiveCommands is translated to Standard_Stream in data channel
	assert.Equal(t, config.ShellPluginName, dataChannel.sessionType)
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

func getDataChannel(t *testing.T) *DataChannel {
	t.Helper()

	mockKMSClient := &kms.Client{}

	dataChannel, err := NewDataChannel(mockKMSClient, clientID, sessionID, instanceID, false, mockLogger)
	require.NoError(t, err)

	dataChannel.SetWsChannel(mockWsChannel)

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

func getClientAndStreamingMessageList(size int) ([][]byte, []StreamingMessage) {
	var payload string

	streamingMessages := make([]StreamingMessage, size)
	serializedClientMessage := make([][]byte, size)

	for i := range size {
		payload = "testPayload" + strconv.Itoa(i)
		clientMessage := getClientMessage(int64(i), messageType, uint32(message.Output), []byte(payload))
		serializedClientMessage[i], _ = clientMessage.SerializeClientMessage(mockLogger)
		streamingMessages[i] = StreamingMessage{
			serializedClientMessage[i],
			int64(i),
			time.Now(),
			new(int),
		}
	}

	return serializedClientMessage, streamingMessages
}
