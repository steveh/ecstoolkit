package datachannel

import (
	"bytes"
	"container/list"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/steveh/ecstoolkit/communicator"
	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/encryption"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/service"
	"github.com/steveh/ecstoolkit/version"
)

// DataChannel used for communication between the mgs and the cli.
type DataChannel struct {
	wsChannel             communicator.IWebSocketChannel
	Role                  string
	ClientID              string
	SessionID             string
	TargetID              string
	IsAwsCliUpgradeNeeded bool
	// records sequence number of last acknowledged message received over data channel
	ExpectedSequenceNumber int64
	// records sequence number of last stream data message sent over data channel
	StreamDataSequenceNumber int64
	// buffer to store outgoing stream messages until acknowledged
	// using linked list for this buffer as access to oldest message is required and it support faster deletion from any position of list
	OutgoingMessageBuffer ListMessageBuffer
	// buffer to store incoming stream messages if received out of sequence
	// using map for this buffer as incoming messages can be out of order and retrieval would be faster by sequenceId
	IncomingMessageBuffer MapMessageBuffer
	// round trip time of latest acknowledged message
	RoundTripTime float64
	// round trip time variation of latest acknowledged message
	RoundTripTimeVariation float64
	// timeout used for resending unacknowledged message
	RetransmissionTimeout time.Duration

	KMSClient *kms.Client

	// Encrypter to encrypt/decrypt if agent requests encryption
	encryption        encryption.IEncrypter
	encryptionEnabled bool

	// SessionType
	sessionType       string
	isSessionTypeSet  chan bool
	sessionProperties interface{}

	// Used to detect if resending a streaming message reaches timeout
	isStreamMessageResendTimeout chan bool

	// Handles data on output stream. Output stream is data outputted by the SSM agent and received here.
	outputStreamHandlers        []OutputStreamDataMessageHandler
	isSessionSpecificHandlerSet bool

	// AgentVersion received during handshake
	agentVersion string
}

// Initialize populates the data channel object with the correct values.
func (dataChannel *DataChannel) Initialize(log *slog.Logger, clientID string, sessionID string, targetID string, isAwsCliUpgradeNeeded bool) {
	// open data channel as publish_subscribe
	log.Debug("Calling Initialize Datachannel", "role", config.RolePublishSubscribe)

	dataChannel.Role = config.RolePublishSubscribe
	dataChannel.ClientID = clientID
	dataChannel.SessionID = sessionID
	dataChannel.TargetID = targetID
	dataChannel.ExpectedSequenceNumber = 0
	dataChannel.StreamDataSequenceNumber = 0
	dataChannel.OutgoingMessageBuffer = ListMessageBuffer{
		list.New(),
		config.OutgoingMessageBufferCapacity,
		&sync.Mutex{},
	}
	dataChannel.IncomingMessageBuffer = MapMessageBuffer{
		make(map[int64]StreamingMessage),
		config.IncomingMessageBufferCapacity,
		&sync.Mutex{},
	}
	dataChannel.RoundTripTime = float64(config.DefaultRoundTripTime)
	dataChannel.RoundTripTimeVariation = config.DefaultRoundTripTimeVariation
	dataChannel.RetransmissionTimeout = config.DefaultTransmissionTimeout
	dataChannel.wsChannel = &communicator.WebSocketChannel{}
	dataChannel.encryptionEnabled = false
	dataChannel.isSessionTypeSet = make(chan bool, 1)
	dataChannel.isStreamMessageResendTimeout = make(chan bool, 1)
	dataChannel.sessionType = ""
	dataChannel.IsAwsCliUpgradeNeeded = isAwsCliUpgradeNeeded
}

// SetWebsocket function populates websocket channel object.
func (dataChannel *DataChannel) SetWebsocket(log *slog.Logger, channelURL string, channelToken string) {
	dataChannel.wsChannel.Initialize(log, channelURL, channelToken)
}

// FinalizeDataChannelHandshake sends the token for service to acknowledge the connection.
func (dataChannel *DataChannel) FinalizeDataChannelHandshake(log *slog.Logger, tokenValue string) error {
	uid := uuid.New().String()

	log.Debug("Sending token through data channel to acknowledge connection", "url", dataChannel.wsChannel.GetStreamURL())
	openDataChannelInput := service.OpenDataChannelInput{
		MessageSchemaVersion: aws.String(config.MessageSchemaVersion),
		RequestID:            aws.String(uid),
		TokenValue:           aws.String(tokenValue),
		ClientID:             aws.String(dataChannel.ClientID),
		ClientVersion:        aws.String(version.Version),
	}

	openDataChannelInputBytes, err := json.Marshal(openDataChannelInput)
	if err != nil {
		log.Error("Error serializing openDataChannelInput", "error", err)

		return fmt.Errorf("serializing open data channel input: %w", err)
	}

	return dataChannel.SendMessage(openDataChannelInputBytes, websocket.TextMessage)
}

// SendMessage sends a message to the service through datachannel.
func (dataChannel *DataChannel) SendMessage(input []byte, inputType int) error {
	err := dataChannel.wsChannel.SendMessage(input, inputType)
	if err != nil {
		return fmt.Errorf("sending message through data channel: %w", err)
	}

	return nil
}

// Open opens websocket connects and does final handshake to acknowledge connection.
func (dataChannel *DataChannel) Open(log *slog.Logger) error {
	if err := dataChannel.wsChannel.Open(log); err != nil {
		return fmt.Errorf("opening data channel: %w", err)
	}

	if err := dataChannel.FinalizeDataChannelHandshake(log, dataChannel.wsChannel.GetChannelToken()); err != nil {
		return fmt.Errorf("error sending token for handshake: %w", err)
	}

	return nil
}

// Close closes datachannel - its web socket connection.
func (dataChannel *DataChannel) Close(log *slog.Logger) error {
	log.Debug("Closing datachannel", "url", dataChannel.wsChannel.GetStreamURL())

	if err := dataChannel.wsChannel.Close(log); err != nil {
		return fmt.Errorf("closing data channel: %w", err)
	}

	return nil
}

// Reconnect calls ResumeSession API to reconnect datachannel when connection is lost.
func (dataChannel *DataChannel) Reconnect(log *slog.Logger) error {
	var err error
	if err = dataChannel.Close(log); err != nil {
		log.Warn("Closing datachannel failed", "error", err)
	}

	if err = dataChannel.Open(log); err != nil {
		return fmt.Errorf("reconnecting data channel %s: %w", dataChannel.wsChannel.GetStreamURL(), err)
	}

	log.Debug("Successfully reconnected to data channel", "url", dataChannel.wsChannel.GetStreamURL())

	return nil
}

// SendFlag sends a data message with PayloadType as given flag.
func (dataChannel *DataChannel) SendFlag(
	log *slog.Logger,
	flagType message.PayloadTypeFlag,
) error {
	flagBuf := new(bytes.Buffer)
	if err := binary.Write(flagBuf, binary.BigEndian, flagType); err != nil {
		return fmt.Errorf("writing flag to buffer: %w", err)
	}

	return dataChannel.SendInputDataMessage(log, message.Flag, flagBuf.Bytes())
}

// SendInputDataMessage sends a data message in a form of ClientMessage.
func (dataChannel *DataChannel) SendInputDataMessage(
	log *slog.Logger,
	payloadType message.PayloadType,
	inputData []byte,
) error {
	var flag uint64

	var msg []byte

	var err error

	messageUUID := uuid.New()

	// today 'enter' is taken as 'next line' in winpty shell. so hardcoding 'next line' byte to actual 'enter' byte
	if bytes.Equal(inputData, []byte{10}) {
		inputData = []byte{13}
	}

	// Encrypt if encryption is enabled and payload type is Output
	if dataChannel.encryptionEnabled && payloadType == message.Output {
		inputData, err = dataChannel.encryption.Encrypt(log, inputData)
		if err != nil {
			return fmt.Errorf("encrypting input data: %w", err)
		}
	}

	clientMessage := message.ClientMessage{
		MessageType:    message.InputStreamMessage,
		SchemaVersion:  1,
		CreatedDate:    uint64(time.Now().UnixMilli()), //nolint:gosec
		Flags:          flag,
		MessageID:      messageUUID,
		PayloadType:    uint32(payloadType),
		Payload:        inputData,
		SequenceNumber: dataChannel.StreamDataSequenceNumber,
	}

	if msg, err = clientMessage.SerializeClientMessage(log); err != nil {
		log.Error("Cannot serialize StreamData message", "error", err)

		return fmt.Errorf("serializing client message: %w", err)
	}

	log.Debug("Sending message", "sequenceNumber", dataChannel.StreamDataSequenceNumber)

	if err = SendMessageCall(dataChannel, msg, websocket.BinaryMessage); err != nil {
		log.Error("Error sending stream data message", "error", err)

		return fmt.Errorf("sending message: %w", err)
	}

	streamingMessage := StreamingMessage{
		msg,
		dataChannel.StreamDataSequenceNumber,
		time.Now(),
		new(int),
	}
	dataChannel.AddDataToOutgoingMessageBuffer(streamingMessage)

	dataChannel.StreamDataSequenceNumber++

	return err
}

// ResendStreamDataMessageScheduler spawns a separate go thread which keeps checking OutgoingMessageBuffer at fixed interval
// and resends first message if time elapsed since lastSentTime of the message is more than acknowledge wait time.
func (dataChannel *DataChannel) ResendStreamDataMessageScheduler(log *slog.Logger) error {
	go func() {
		for {
			time.Sleep(config.ResendSleepInterval)
			dataChannel.OutgoingMessageBuffer.Mutex.Lock()
			streamMessageElement := dataChannel.OutgoingMessageBuffer.Messages.Front()
			dataChannel.OutgoingMessageBuffer.Mutex.Unlock()

			if streamMessageElement == nil {
				continue
			}

			streamMessage, ok := streamMessageElement.Value.(StreamingMessage)
			if !ok {
				log.Error("Failed to type assert streamMessageElement.Value to StreamingMessage")

				continue
			}

			if time.Since(streamMessage.LastSentTime) > dataChannel.RetransmissionTimeout {
				log.Debug("Resend stream data message", "sequenceNumber", streamMessage.SequenceNumber, "attempt", *streamMessage.ResendAttempt)

				if *streamMessage.ResendAttempt >= config.ResendMaxAttempt {
					log.Warn("Message resent too many times", "sequenceNumber", streamMessage.SequenceNumber, "maxAttempts", config.ResendMaxAttempt)
					dataChannel.isStreamMessageResendTimeout <- true
				}

				*streamMessage.ResendAttempt++
				if err := SendMessageCall(dataChannel, streamMessage.Content, websocket.BinaryMessage); err != nil {
					log.Error("Unable to send stream data message", "error", err)
				}

				streamMessage.LastSentTime = time.Now()
			}
		}
	}()

	return nil
}

// ProcessAcknowledgedMessage processes acknowledge messages by deleting them from OutgoingMessageBuffer.
func (dataChannel *DataChannel) ProcessAcknowledgedMessage(log *slog.Logger, acknowledgeMessageContent message.AcknowledgeContent) error {
	acknowledgeSequenceNumber := acknowledgeMessageContent.SequenceNumber

	for streamMessageElement := dataChannel.OutgoingMessageBuffer.Messages.Front(); streamMessageElement != nil; streamMessageElement = streamMessageElement.Next() {
		streamMessage, ok := streamMessageElement.Value.(StreamingMessage)
		if !ok {
			log.Error("Failed to type assert streamMessageElement.Value to StreamingMessage")

			continue
		}

		if streamMessage.SequenceNumber == acknowledgeSequenceNumber {
			// Calculate retransmission timeout based on latest round trip time of message
			dataChannel.CalculateRetransmissionTimeout(streamMessage)

			dataChannel.RemoveDataFromOutgoingMessageBuffer(streamMessageElement)

			break
		}
	}

	return nil
}

// SendAcknowledgeMessage sends acknowledge message for stream data over data channel.
func (dataChannel *DataChannel) SendAcknowledgeMessage(log *slog.Logger, streamDataMessage message.ClientMessage) error {
	dataStreamAcknowledgeContent := message.AcknowledgeContent{
		MessageType:         streamDataMessage.MessageType,
		MessageID:           streamDataMessage.MessageID.String(),
		SequenceNumber:      streamDataMessage.SequenceNumber,
		IsSequentialMessage: true,
	}

	var msg []byte

	var err error

	if msg, err = message.SerializeClientMessageWithAcknowledgeContent(log, dataStreamAcknowledgeContent); err != nil {
		log.Error("Cannot serialize Acknowledge message", "error", err)

		return fmt.Errorf("serializing acknowledge message: %w", err)
	}

	if err = SendMessageCall(dataChannel, msg, websocket.BinaryMessage); err != nil {
		log.Error("Error sending acknowledge message", "error", err)

		return fmt.Errorf("sending acknowledge message: %w", err)
	}

	return nil
}

// OutputMessageHandler gets output on the data channel.
func (dataChannel *DataChannel) OutputMessageHandler(ctx context.Context, log *slog.Logger, stopHandler Stop, sessionID string, rawMessage []byte) error {
	outputMessage := &message.ClientMessage{}

	err := outputMessage.DeserializeClientMessage(log, rawMessage)
	if err != nil {
		log.Error("Cannot deserialize raw message", "message", string(rawMessage), "error", err)

		return fmt.Errorf("could not deserialize rawMessage, %s : %w", rawMessage, err)
	}

	if err = outputMessage.Validate(); err != nil {
		log.Error("Invalid outputMessage", "message", *outputMessage, "error", err)

		return fmt.Errorf("validating output message: %w", err)
	}

	log.Debug("Processing stream data message", "type", outputMessage.MessageType)

	switch outputMessage.MessageType {
	case message.OutputStreamMessage:
		return dataChannel.HandleOutputMessage(ctx, log, *outputMessage, rawMessage)
	case message.AcknowledgeMessage:
		return dataChannel.HandleAcknowledgeMessage(log, *outputMessage)
	case message.ChannelClosedMessage:
		dataChannel.HandleChannelClosedMessage(log, stopHandler, sessionID, *outputMessage)
	case message.StartPublicationMessage, message.PausePublicationMessage:
		return nil
	default:
		log.Warn("Invalid message type received", "messageType", outputMessage.MessageType)
	}

	return nil
}

// RegisterOutputStreamHandler register a handler for messages of type OutputStream. This is usually called by the plugin.
func (dataChannel *DataChannel) RegisterOutputStreamHandler(handler OutputStreamDataMessageHandler, isSessionSpecificHandler bool) {
	dataChannel.isSessionSpecificHandlerSet = isSessionSpecificHandler
	dataChannel.outputStreamHandlers = append(dataChannel.outputStreamHandlers, handler)
}

// DeregisterOutputStreamHandler deregisters a handler previously registered using RegisterOutputStreamHandler.
func (dataChannel *DataChannel) DeregisterOutputStreamHandler(handler OutputStreamDataMessageHandler) {
	// Find and remove "handler"
	for i, v := range dataChannel.outputStreamHandlers {
		if reflect.ValueOf(v).Pointer() == reflect.ValueOf(handler).Pointer() {
			dataChannel.outputStreamHandlers = append(dataChannel.outputStreamHandlers[:i], dataChannel.outputStreamHandlers[i+1:]...)

			break
		}
	}
}

// HandleOutputMessage handles incoming stream data message by processing the payload and updating expectedSequenceNumber.
func (dataChannel *DataChannel) HandleOutputMessage(
	ctx context.Context,
	log *slog.Logger,
	outputMessage message.ClientMessage,
	rawMessage []byte,
) error {
	// Handle unexpected sequence messages first
	if outputMessage.SequenceNumber != dataChannel.ExpectedSequenceNumber {
		return dataChannel.handleUnexpectedSequenceMessage(log, outputMessage, rawMessage)
	}

	var err error

	// Process the message based on its payload type
	//nolint:exhaustive
	switch message.PayloadType(outputMessage.PayloadType) {
	case message.HandshakeRequestPayloadType:
		err = dataChannel.handleHandshakeRequestOutputMessage(ctx, log, outputMessage)
	case message.HandshakeCompletePayloadType:
		err = dataChannel.handleHandshakeCompleteOutputMessage(log, outputMessage)
	case message.EncChallengeRequest:
		err = dataChannel.handleEncryptionChallengeRequestOutputMessage(log, outputMessage)
	default:
		err = dataChannel.handleDefaultOutputMessage(log, outputMessage)
	}

	if err != nil {
		return err
	}

	// Increment the expected sequence number after successful processing
	dataChannel.ExpectedSequenceNumber++

	// Process any buffered messages that are now in sequence
	return dataChannel.ProcessIncomingMessageBufferItems(log, outputMessage)
}

// ProcessIncomingMessageBufferItems checks if new expected sequence stream data is present in IncomingMessageBuffer.
// If so, processes it and increments the expected sequence number.
// Repeats until expected sequence stream data is not found in IncomingMessageBuffer.
func (dataChannel *DataChannel) ProcessIncomingMessageBufferItems(
	log *slog.Logger,
	outputMessage message.ClientMessage,
) error {
	for {
		// Check if there's a message with the expected sequence number
		bufferedStreamMessage, exists := dataChannel.IncomingMessageBuffer.Messages[dataChannel.ExpectedSequenceNumber]
		if !exists || bufferedStreamMessage.Content == nil {
			// No more messages to process
			break
		}

		// Process the buffered message
		if err := dataChannel.processBufferedMessage(log, outputMessage, bufferedStreamMessage); err != nil {
			return fmt.Errorf("processing incoming message buffer items: %w", err)
		}
	}

	return nil
}

// HandleAcknowledgeMessage deserializes acknowledge content and processes it.
func (dataChannel *DataChannel) HandleAcknowledgeMessage(
	log *slog.Logger,
	outputMessage message.ClientMessage,
) error {
	acknowledgeMessage, err := outputMessage.DeserializeDataStreamAcknowledgeContent(log)
	if err != nil {
		log.Error("Cannot deserialize payload to AcknowledgeMessage", "error", err)

		return fmt.Errorf("deserializing data stream acknowledge content: %w", err)
	}

	err = ProcessAcknowledgedMessageCall(log, dataChannel, acknowledgeMessage)
	if err != nil {
		return fmt.Errorf("processing acknowledged message: %w", err)
	}

	return nil
}

// HandleChannelClosedMessage handles the channel closed message and exits the shell.
func (dataChannel *DataChannel) HandleChannelClosedMessage(log *slog.Logger, stopHandler Stop, sessionID string, outputMessage message.ClientMessage) {
	var (
		channelClosedMessage message.ChannelClosed
		err                  error
	)

	if channelClosedMessage, err = outputMessage.DeserializeChannelClosedMessage(log); err != nil {
		log.Error("Cannot deserialize payload to ChannelClosedMessage", "error", err)
	}

	log.Debug("Exiting session", "sessionID", sessionID)

	log.Debug("Session message", "sessionID", sessionID, "output", channelClosedMessage.Output)

	if err := stopHandler(log); err != nil {
		log.Error("Failed to stop handler", "error", err)
	}
}

// AddDataToOutgoingMessageBuffer removes first message from OutgoingMessageBuffer if capacity is full and adds given message at the end.
func (dataChannel *DataChannel) AddDataToOutgoingMessageBuffer(streamMessage StreamingMessage) {
	if dataChannel.OutgoingMessageBuffer.Messages.Len() == dataChannel.OutgoingMessageBuffer.Capacity {
		dataChannel.RemoveDataFromOutgoingMessageBuffer(dataChannel.OutgoingMessageBuffer.Messages.Front())
	}

	dataChannel.OutgoingMessageBuffer.Mutex.Lock()
	dataChannel.OutgoingMessageBuffer.Messages.PushBack(streamMessage)
	dataChannel.OutgoingMessageBuffer.Mutex.Unlock()
}

// RemoveDataFromOutgoingMessageBuffer removes given element from OutgoingMessageBuffer.
func (dataChannel *DataChannel) RemoveDataFromOutgoingMessageBuffer(streamMessageElement *list.Element) {
	dataChannel.OutgoingMessageBuffer.Mutex.Lock()
	dataChannel.OutgoingMessageBuffer.Messages.Remove(streamMessageElement)
	dataChannel.OutgoingMessageBuffer.Mutex.Unlock()
}

// AddDataToIncomingMessageBuffer adds given message to IncomingMessageBuffer if it has capacity.
func (dataChannel *DataChannel) AddDataToIncomingMessageBuffer(streamMessage StreamingMessage) {
	if len(dataChannel.IncomingMessageBuffer.Messages) == dataChannel.IncomingMessageBuffer.Capacity {
		return
	}

	dataChannel.IncomingMessageBuffer.Mutex.Lock()
	dataChannel.IncomingMessageBuffer.Messages[streamMessage.SequenceNumber] = streamMessage
	dataChannel.IncomingMessageBuffer.Mutex.Unlock()
}

// RemoveDataFromIncomingMessageBuffer removes given sequence number message from IncomingMessageBuffer.
func (dataChannel *DataChannel) RemoveDataFromIncomingMessageBuffer(sequenceNumber int64) {
	dataChannel.IncomingMessageBuffer.Mutex.Lock()
	delete(dataChannel.IncomingMessageBuffer.Messages, sequenceNumber)
	dataChannel.IncomingMessageBuffer.Mutex.Unlock()
}

// CalculateRetransmissionTimeout calculates message retransmission timeout value based on round trip time on given message.
func (dataChannel *DataChannel) CalculateRetransmissionTimeout(streamingMessage StreamingMessage) {
	newRoundTripTime := float64(GetRoundTripTime(streamingMessage))

	dataChannel.RoundTripTimeVariation = ((1 - config.RTTVConstant) * dataChannel.RoundTripTimeVariation) +
		(config.RTTVConstant * math.Abs(dataChannel.RoundTripTime-newRoundTripTime))

	dataChannel.RoundTripTime = ((1 - config.RTTConstant) * dataChannel.RoundTripTime) +
		(config.RTTConstant * newRoundTripTime)

	dataChannel.RetransmissionTimeout = time.Duration(dataChannel.RoundTripTime +
		math.Max(float64(config.ClockGranularity), float64(4*dataChannel.RoundTripTimeVariation))) //nolint:mnd

	// Ensure RetransmissionTimeout do not exceed maximum timeout defined
	if dataChannel.RetransmissionTimeout > config.MaxTransmissionTimeout {
		dataChannel.RetransmissionTimeout = config.MaxTransmissionTimeout
	}
}

// ProcessKMSEncryptionHandshakeAction sets up the encrypter and calls KMS to generate a new data key. This is triggered
// when encryption is specified in HandshakeRequest.
func (dataChannel *DataChannel) ProcessKMSEncryptionHandshakeAction(ctx context.Context, log *slog.Logger, actionParams json.RawMessage) error {
	if dataChannel.IsAwsCliUpgradeNeeded {
		return errors.New("installed version of CLI does not support Session Manager encryption feature. Please upgrade to the latest version of your CLI (e.g., AWS CLI)")
	}

	kmsEncRequest := message.KMSEncryptionRequest{}
	if err := json.Unmarshal(actionParams, &kmsEncRequest); err != nil {
		return fmt.Errorf("failed to unmarshal KMS encryption request: %w", err)
	}

	log.Debug("KMS encryption request", "request", kmsEncRequest)

	kmsKeyID := kmsEncRequest.KMSKeyID

	encryptionContext := map[string]string{"aws:ssm:SessionId": dataChannel.SessionID, "aws:ssm:TargetId": dataChannel.TargetID}

	var err error

	dataChannel.encryption, err = newEncrypter(ctx, log, kmsKeyID, encryptionContext, dataChannel.KMSClient)
	if err != nil {
		return fmt.Errorf("creating new encrypter: %w", err)
	}

	return nil
}

// ProcessSessionTypeHandshakeAction processes session type action in HandshakeRequest. This sets the session type in the datachannel.
func (dataChannel *DataChannel) ProcessSessionTypeHandshakeAction(actionParams json.RawMessage) error {
	sessTypeReq := message.SessionTypeRequest{}
	if err := json.Unmarshal(actionParams, &sessTypeReq); err != nil {
		return fmt.Errorf("failed to unmarshal session type request: %w", err)
	}

	switch sessTypeReq.SessionType {
	// This switch-case is just so that we can fail early if an unknown session type is passed in.
	case config.ShellPluginName, config.InteractiveCommandsPluginName, config.NonInteractiveCommandsPluginName:
		dataChannel.sessionType = config.ShellPluginName
		dataChannel.sessionProperties = sessTypeReq.Properties

		return nil
	case config.PortPluginName:
		dataChannel.sessionType = sessTypeReq.SessionType
		dataChannel.sessionProperties = sessTypeReq.Properties

		return nil
	default:
		return errors.New("Unknown session type " + sessTypeReq.SessionType)
	}
}

// IsSessionTypeSet check has data channel sessionType been set.
func (dataChannel *DataChannel) IsSessionTypeSet() chan bool {
	return dataChannel.isSessionTypeSet
}

// IsStreamMessageResendTimeout checks if resending a streaming message reaches timeout.
func (dataChannel *DataChannel) IsStreamMessageResendTimeout() chan bool {
	return dataChannel.isStreamMessageResendTimeout
}

// SetSessionType set session type.
func (dataChannel *DataChannel) SetSessionType(sessionType string) {
	dataChannel.sessionType = sessionType
	dataChannel.isSessionTypeSet <- true
}

// GetSessionType returns SessionType of the dataChannel.
func (dataChannel *DataChannel) GetSessionType() string {
	return dataChannel.sessionType
}

// GetSessionProperties returns SessionProperties of the dataChannel.
func (dataChannel *DataChannel) GetSessionProperties() interface{} {
	return dataChannel.sessionProperties
}

// GetWsChannel returns WsChannel of the dataChannel.
func (dataChannel *DataChannel) GetWsChannel() communicator.IWebSocketChannel { //nolint:ireturn
	return dataChannel.wsChannel
}

// SetWsChannel set WsChannel of the dataChannel.
func (dataChannel *DataChannel) SetWsChannel(wsChannel communicator.IWebSocketChannel) {
	dataChannel.wsChannel = wsChannel
}

// GetStreamDataSequenceNumber returns StreamDataSequenceNumber of the dataChannel.
func (dataChannel *DataChannel) GetStreamDataSequenceNumber() int64 {
	return dataChannel.StreamDataSequenceNumber
}

// GetAgentVersion returns agent version of the target instance.
func (dataChannel *DataChannel) GetAgentVersion() string {
	return dataChannel.agentVersion
}

// SetAgentVersion set agent version of the target instance.
func (dataChannel *DataChannel) SetAgentVersion(agentVersion string) {
	dataChannel.agentVersion = agentVersion
}

// handleHandshakeRequest is the handler for payloads of type HandshakeRequest.
func (dataChannel *DataChannel) handleHandshakeRequest(ctx context.Context, log *slog.Logger, clientMessage message.ClientMessage) error {
	handshakeRequest, err := clientMessage.DeserializeHandshakeRequest(log)
	if err != nil {
		log.Error("Deserialize Handshake Request failed", "error", err)

		return fmt.Errorf("deserializing handshake request: %w", err)
	}

	dataChannel.agentVersion = handshakeRequest.AgentVersion

	var errorList []error

	var handshakeResponse message.HandshakeResponsePayload
	handshakeResponse.ClientVersion = version.Version
	handshakeResponse.ProcessedClientActions = []message.ProcessedClientAction{}

	for _, action := range handshakeRequest.RequestedClientActions {
		processedAction := message.ProcessedClientAction{}

		switch action.ActionType {
		case message.KMSEncryption:
			processedAction.ActionType = action.ActionType
			err := dataChannel.ProcessKMSEncryptionHandshakeAction(ctx, log, action.ActionParameters)

			if err != nil {
				processedAction.ActionStatus = message.Failed
				processedAction.Error = fmt.Sprintf("processing action %s: %s",
					message.KMSEncryption, err)

				errorList = append(errorList, err)
			} else {
				processedAction.ActionStatus = message.Success
				processedAction.ActionResult = message.KMSEncryptionResponse{
					KMSCipherTextKey: dataChannel.encryption.GetEncryptedDataKey(),
				}
				dataChannel.encryptionEnabled = true
			}
		case message.SessionType:
			processedAction.ActionType = action.ActionType
			err := dataChannel.ProcessSessionTypeHandshakeAction(action.ActionParameters)

			if err != nil {
				processedAction.ActionStatus = message.Failed
				processedAction.Error = fmt.Sprintf("processing action %s: %s",
					message.SessionType, err)

				errorList = append(errorList, err)
			} else {
				processedAction.ActionStatus = message.Success
			}

		default:
			processedAction.ActionType = action.ActionType
			processedAction.ActionResult = message.Unsupported
			processedAction.Error = fmt.Sprintf("Unsupported action %s", action.ActionType)
			errorList = append(errorList, errors.New(processedAction.Error))
		}

		handshakeResponse.ProcessedClientActions = append(handshakeResponse.ProcessedClientActions, processedAction)
	}

	for _, x := range errorList {
		handshakeResponse.Errors = append(handshakeResponse.Errors, x.Error())
	}

	err = dataChannel.sendHandshakeResponse(log, handshakeResponse)

	return err
}

// handleHandshakeComplete is the handler for when the payload type is HandshakeComplete. This will trigger
// the plugin to start.
func (dataChannel *DataChannel) handleHandshakeComplete(log *slog.Logger, clientMessage message.ClientMessage) error {
	var err error

	var handshakeComplete message.HandshakeCompletePayload

	handshakeComplete, err = clientMessage.DeserializeHandshakeComplete(log)
	if err != nil {
		return fmt.Errorf("handling handshake complete: %w", err)
	}

	// SessionType would be set when handshake request is received
	if dataChannel.sessionType != "" {
		dataChannel.isSessionTypeSet <- true
	} else {
		dataChannel.isSessionTypeSet <- false
	}

	log.Debug("Handshake Complete", "timeToComplete", handshakeComplete.HandshakeTimeToComplete.Seconds())

	if handshakeComplete.CustomerMessage != "" {
		log.Debug("Exiting session", "sessionID", dataChannel.SessionID)
		log.Debug("Session message", "sessionID", dataChannel.SessionID, "message", handshakeComplete.CustomerMessage)
	}

	return fmt.Errorf("handling handshake complete: %w", err)
}

// handleEncryptionChallengeRequest receives EncryptionChallenge and responds.
func (dataChannel *DataChannel) handleEncryptionChallengeRequest(log *slog.Logger, clientMessage message.ClientMessage) error {
	var err error

	var encChallengeReq message.EncryptionChallengeRequest

	err = json.Unmarshal(clientMessage.Payload, &encChallengeReq)
	if err != nil {
		return fmt.Errorf("could not deserialize rawMessage, %s : %w", clientMessage.Payload, err)
	}

	challenge := encChallengeReq.Challenge

	challenge, err = dataChannel.encryption.Decrypt(log, challenge)
	if err != nil {
		return fmt.Errorf("decrypting challenge: %w", err)
	}

	challenge, err = dataChannel.encryption.Encrypt(log, challenge)
	if err != nil {
		return fmt.Errorf("encrypting challenge: %w", err)
	}

	encChallengeResp := message.EncryptionChallengeResponse{
		Challenge: challenge,
	}

	err = dataChannel.sendEncryptionChallengeResponse(log, encChallengeResp)

	return err
}

// sendEncryptionChallengeResponse sends EncryptionChallengeResponse.
func (dataChannel *DataChannel) sendEncryptionChallengeResponse(log *slog.Logger, response message.EncryptionChallengeResponse) error {
	resultBytes, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("could not serialize EncChallengeResponse message: %v, err: %w", response, err)
	}

	log.Debug("Sending EncChallengeResponse message")

	if err := dataChannel.SendInputDataMessage(log, message.EncChallengeResponse, resultBytes); err != nil {
		return err
	}

	return nil
}

// sendHandshakeResponse sends HandshakeResponse.
func (dataChannel *DataChannel) sendHandshakeResponse(log *slog.Logger, response message.HandshakeResponsePayload) error {
	resultBytes, err := json.Marshal(response)
	if err != nil {
		log.Error("Could not serialize HandshakeResponse message", "response", response, "error", err)
	}

	log.Debug("Sending HandshakeResponse message")

	if err := dataChannel.SendInputDataMessage(log, message.HandshakeResponsePayloadType, resultBytes); err != nil {
		return err
	}

	return nil
}

func (dataChannel *DataChannel) processOutputMessageWithHandlers(log *slog.Logger, message message.ClientMessage) (bool, error) {
	// Return false if sessionType is known but session specific handler is not set
	if dataChannel.sessionType != "" && !dataChannel.isSessionSpecificHandlerSet {
		return false, nil
	}

	var isHandlerReady bool

	var err error

	for _, handler := range dataChannel.outputStreamHandlers {
		isHandlerReady, err = handler(log, message)
		// Break the processing of message and return if session specific handler is not ready
		if err != nil || !isHandlerReady {
			break
		}
	}

	return isHandlerReady, err
}

// handleHandshakeRequestOutputMessage handles output messages of type HandshakeRequestPayloadType.
func (dataChannel *DataChannel) handleHandshakeRequestOutputMessage(
	ctx context.Context,
	log *slog.Logger,
	outputMessage message.ClientMessage,
) error {
	if err := SendAcknowledgeMessageCall(log, dataChannel, outputMessage); err != nil {
		return err
	}

	// PayloadType is HandshakeRequest so we call our own handler instead of the provided handler
	log.Debug("Processing HandshakeRequest message", "message", outputMessage)

	if err := dataChannel.handleHandshakeRequest(ctx, log, outputMessage); err != nil {
		log.Error("processing stream data message", "error", err.Error())

		return err
	}

	return nil
}

// handleHandshakeCompleteOutputMessage handles output messages of type HandshakeCompletePayloadType.
func (dataChannel *DataChannel) handleHandshakeCompleteOutputMessage(
	log *slog.Logger,
	outputMessage message.ClientMessage,
) error {
	if err := SendAcknowledgeMessageCall(log, dataChannel, outputMessage); err != nil {
		return err
	}

	if err := dataChannel.handleHandshakeComplete(log, outputMessage); err != nil {
		log.Error("processing stream data message", "error", err.Error())

		return err
	}

	return nil
}

// handleEncryptionChallengeRequestOutputMessage handles output messages of type EncChallengeRequest.
func (dataChannel *DataChannel) handleEncryptionChallengeRequestOutputMessage(
	log *slog.Logger,
	outputMessage message.ClientMessage,
) error {
	if err := SendAcknowledgeMessageCall(log, dataChannel, outputMessage); err != nil {
		return err
	}

	if err := dataChannel.handleEncryptionChallengeRequest(log, outputMessage); err != nil {
		log.Error("processing stream data message", "error", err.Error())

		return err
	}

	return nil
}

// handleDefaultOutputMessage handles output messages of any other type.
func (dataChannel *DataChannel) handleDefaultOutputMessage(
	log *slog.Logger,
	outputMessage message.ClientMessage,
) error {
	log.Debug("Process new incoming stream data message", "sequenceNumber", outputMessage.SequenceNumber)

	// Decrypt if encryption is enabled and payload type is output
	if dataChannel.encryptionEnabled &&
		(outputMessage.PayloadType == uint32(message.Output) ||
			outputMessage.PayloadType == uint32(message.StdErr) ||
			outputMessage.PayloadType == uint32(message.ExitCode)) {
		var err error

		outputMessage.Payload, err = dataChannel.encryption.Decrypt(log, outputMessage.Payload)
		if err != nil {
			log.Error("Unable to decrypt incoming data payload", "messageType", outputMessage.MessageType, "payloadType", outputMessage.PayloadType, "error", err)

			return fmt.Errorf("decrypting incoming data payload: %w", err)
		}
	}

	isHandlerReady, err := dataChannel.processOutputMessageWithHandlers(log, outputMessage)
	if err != nil {
		log.Error("processing stream data message", "error", err.Error())

		return fmt.Errorf("processing stream data message: %w", err)
	}

	if !isHandlerReady {
		log.Warn("Stream data message not processed", "sequenceNumber", outputMessage.SequenceNumber, "reason", "session handler not ready")

		return nil
	}

	// Acknowledge outputMessage only if session specific handler is ready
	if err := SendAcknowledgeMessageCall(log, dataChannel, outputMessage); err != nil {
		return err
	}

	return nil
}

// handleUnexpectedSequenceMessage handles messages with unexpected sequence numbers.
func (dataChannel *DataChannel) handleUnexpectedSequenceMessage(
	log *slog.Logger,
	outputMessage message.ClientMessage,
	rawMessage []byte,
) error {
	log.Debug("Unexpected sequence message received", "receivedSequence", outputMessage.SequenceNumber, "expectedSequence", dataChannel.ExpectedSequenceNumber)

	// If incoming message sequence number is greater then expected sequence number and IncomingMessageBuffer has capacity,
	// add message to IncomingMessageBuffer and send acknowledgement
	if outputMessage.SequenceNumber > dataChannel.ExpectedSequenceNumber {
		log.Debug("Received sequence number is higher than expected", "receivedSequence", outputMessage.SequenceNumber, "expectedSequence", dataChannel.ExpectedSequenceNumber)

		if len(dataChannel.IncomingMessageBuffer.Messages) < dataChannel.IncomingMessageBuffer.Capacity {
			if err := SendAcknowledgeMessageCall(log, dataChannel, outputMessage); err != nil {
				return err
			}

			streamingMessage := StreamingMessage{
				rawMessage,
				outputMessage.SequenceNumber,
				time.Now(),
				new(int),
			}

			// Add message to buffer for future processing
			dataChannel.AddDataToIncomingMessageBuffer(streamingMessage)
		}
	}

	return nil
}

// processBufferedMessage processes a single buffered message and updates the expected sequence number.
func (dataChannel *DataChannel) processBufferedMessage(
	log *slog.Logger,
	outputMessage message.ClientMessage,
	bufferedStreamMessage StreamingMessage,
) error {
	log.Debug("Process stream data message from IncomingMessageBuffer", "sequenceNumber", bufferedStreamMessage.SequenceNumber)

	if err := outputMessage.DeserializeClientMessage(log, bufferedStreamMessage.Content); err != nil {
		log.Error("Cannot deserialize raw message", "error", err)

		return fmt.Errorf("deserializing raw message: %w", err)
	}

	// Decrypt if encryption is enabled and payload type is output
	if dataChannel.encryptionEnabled &&
		(outputMessage.PayloadType == uint32(message.Output) ||
			outputMessage.PayloadType == uint32(message.StdErr) ||
			outputMessage.PayloadType == uint32(message.ExitCode)) {
		var err error

		outputMessage.Payload, err = dataChannel.encryption.Decrypt(log, outputMessage.Payload)
		if err != nil {
			log.Error("Unable to decrypt buffered message data payload", "messageType", outputMessage.MessageType, "payloadType", outputMessage.PayloadType, "error", err)

			return fmt.Errorf("decrypting buffered message data payload: %w", err)
		}
	}

	_, err := dataChannel.processOutputMessageWithHandlers(log, outputMessage)
	if err != nil {
		return fmt.Errorf("processing output message with handlers: %w", err)
	}

	dataChannel.ExpectedSequenceNumber++
	dataChannel.RemoveDataFromIncomingMessageBuffer(bufferedStreamMessage.SequenceNumber)

	return nil
}
