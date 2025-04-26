package datachannel

import (
	"bytes"
	"container/list"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
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
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/retry"
	"github.com/steveh/ecstoolkit/service"
	"github.com/steveh/ecstoolkit/version"
)

var (
	// ErrTimedOut is returned when the session times out.
	ErrTimedOut = errors.New("timed out")

	// ErrUnknownSessionType is returned when the session type is unknown.
	ErrUnknownSessionType = errors.New("unknown session type")

	// ErrUnsupportedAction is returned when the action is unsupported.
	ErrUnsupportedAction = errors.New("unsupported action")
)

// DataChannel used for communication between the mgs and the cli.
type DataChannel struct {
	wsChannel communicator.IWebSocketChannel
	role      string
	clientID  string
	sessionID string
	targetID  string
	// records sequence number of last acknowledged message received over data channel
	expectedSequenceNumber int64
	// records sequence number of last stream data message sent over data channel
	streamDataSequenceNumber int64
	// buffer to store outgoing stream messages until acknowledged
	// using linked list for this buffer as access to oldest message is required and it support faster deletion from any position of list
	outgoingMessageBuffer ListMessageBuffer
	// buffer to store incoming stream messages if received out of sequence
	// using map for this buffer as incoming messages can be out of order and retrieval would be faster by sequenceId
	incomingMessageBuffer MapMessageBuffer
	// round trip time of latest acknowledged message
	roundTripTime float64
	// round trip time variation of latest acknowledged message
	roundTripTimeVariation float64
	// timeout used for resending unacknowledged message
	retransmissionTimeout time.Duration

	kmsClient *kms.Client

	// Encrypter to encrypt/decrypt if agent requests encryption
	encryption        encryption.IEncrypter
	encryptionEnabled bool

	// SessionType
	sessionType       string
	isSessionTypeSet  chan bool
	sessionProperties any

	// Used to detect if resending a streaming message reaches timeout
	isStreamMessageResendTimeout chan bool

	// Handles data on output stream. Output stream is data outputted by the SSM agent and received here.
	outputStreamHandlers        []OutputStreamDataMessageHandler
	isSessionSpecificHandlerSet bool

	// AgentVersion received during handshake
	agentVersion string

	logger log.T

	displayHandler func(message.ClientMessage)
}

// NewDataChannel creates a DataChannel.
func NewDataChannel(kmsClient *kms.Client, wsChannel communicator.IWebSocketChannel, clientID string, sessionID string, targetID string, logger log.T) (*DataChannel, error) {
	c := &DataChannel{
		kmsClient: kmsClient,
		wsChannel: wsChannel,
		role:      config.RolePublishSubscribe,
		clientID:  clientID,
		sessionID: sessionID,
		targetID:  targetID,
		outgoingMessageBuffer: ListMessageBuffer{
			list.New(),
			config.OutgoingMessageBufferCapacity,
			&sync.Mutex{},
		},
		incomingMessageBuffer: MapMessageBuffer{
			make(map[int64]StreamingMessage),
			config.IncomingMessageBufferCapacity,
			&sync.Mutex{},
		},
		roundTripTime:                float64(config.DefaultRoundTripTime),
		roundTripTimeVariation:       config.DefaultRoundTripTimeVariation,
		retransmissionTimeout:        config.DefaultTransmissionTimeout,
		isSessionTypeSet:             make(chan bool, 1),
		isStreamMessageResendTimeout: make(chan bool, 1),
		logger:                       logger,
	}

	return c, nil
}

// SendMessage sends a message to the service through datachannel.
func (c *DataChannel) SendMessage(input []byte, inputType int) error {
	err := c.wsChannel.SendMessage(input, inputType)
	if err != nil {
		return fmt.Errorf("sending message through data channel: %w", err)
	}

	return nil
}

// Open opens the data channel and registers the message handler.
func (c *DataChannel) Open(ctx context.Context, messageHandler DisplayMessageHandler, refreshTokenHandler RefreshTokenHandler, timeoutHandler TimeoutHandler) (string, error) {
	c.RegisterOutputMessageHandler(ctx, func() error { return nil }, func(_ []byte) {})

	c.displayHandler = messageHandler

	c.RegisterOutputStreamHandler(c.firstMessageHandler, false)

	retryParams := retry.RepeatableExponentialRetryer{
		GeometricRatio:      config.RetryBase,
		InitialDelayInMilli: rand.Intn(config.DataChannelRetryInitialDelayMillis) + config.DataChannelRetryInitialDelayMillis, //nolint:gosec
		MaxDelayInMilli:     config.DataChannelRetryMaxIntervalMillis,
		MaxAttempts:         config.DataChannelNumMaxRetries,
	}

	if err := c.open(); err != nil {
		c.logger.Error("Retrying connection failed", "sessionID", c.sessionID, "error", err)

		retryParams.CallableFunc = func() error {
			return c.reconnect()
		}

		if err := retryParams.Call(); err != nil {
			c.logger.Error("Failed to call retry parameters", "error", err)
		}
	}

	c.wsChannel.SetOnError(func(wsErr error) {
		c.logger.Error("Trying to reconnect session", "sequenceNumber", c.streamDataSequenceNumber, "error", wsErr)

		retryParams.CallableFunc = func() error {
			token, err := refreshTokenHandler(ctx)
			if err != nil {
				return fmt.Errorf("getting reconnection token: %w", err)
			}

			if token == "" {
				return ErrTimedOut
			}

			c.wsChannel.SetChannelToken(token)

			if err := c.reconnect(); err != nil {
				return fmt.Errorf("reconnecting data channel: %w", err)
			}

			return nil
		}

		if err := retryParams.Call(); err != nil {
			c.logger.Error("Reconnect error", "error", err)
		}
	})

	// Scheduler for resending of data
	c.resendStreamDataMessageScheduler()

	return c.establishSessionType(ctx, timeoutHandler)
}

// Close closes datachannel - its web socket connection.
func (c *DataChannel) Close() error {
	c.logger.Debug("Closing datachannel", "url", c.wsChannel.GetStreamURL())

	if err := c.wsChannel.Close(); err != nil {
		return fmt.Errorf("closing data channel: %w", err)
	}

	return nil
}

// SendFlag sends a data message with PayloadType as given flag.
func (c *DataChannel) SendFlag(flagType message.PayloadTypeFlag) error {
	flagBuf := new(bytes.Buffer)
	if err := binary.Write(flagBuf, binary.BigEndian, flagType); err != nil {
		return fmt.Errorf("writing flag to buffer: %w", err)
	}

	return c.SendInputDataMessage(message.Flag, flagBuf.Bytes())
}

// SendInputDataMessage sends a data message in a form of ClientMessage.
func (c *DataChannel) SendInputDataMessage(payloadType message.PayloadType, inputData []byte) error {
	var flag uint64

	var msg []byte

	var err error

	messageUUID := uuid.New()

	// today 'enter' is taken as 'next line' in winpty shell. so hardcoding 'next line' byte to actual 'enter' byte
	if bytes.Equal(inputData, []byte{10}) {
		inputData = []byte{13}
	}

	// Encrypt if encryption is enabled and payload type is Output
	if c.encryptionEnabled && payloadType == message.Output {
		inputData, err = c.encryption.Encrypt(inputData)
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
		SequenceNumber: c.streamDataSequenceNumber,
	}

	if msg, err = clientMessage.SerializeClientMessage(); err != nil {
		return fmt.Errorf("serializing client message: %w", err)
	}

	c.logger.Trace("Sending message", "sequenceNumber", c.streamDataSequenceNumber)

	if err = c.SendMessage(msg, websocket.BinaryMessage); err != nil {
		return fmt.Errorf("sending message: %w", err)
	}

	streamingMessage := StreamingMessage{
		msg,
		c.streamDataSequenceNumber,
		time.Now(),
		new(int),
	}
	c.addDataToOutgoingMessageBuffer(streamingMessage)

	c.streamDataSequenceNumber++

	return err
}

// RegisterOutputStreamHandler register a handler for messages of type OutputStream. This is usually called by the plugin.
func (c *DataChannel) RegisterOutputStreamHandler(handler OutputStreamDataMessageHandler, isSessionSpecificHandler bool) {
	c.isSessionSpecificHandlerSet = isSessionSpecificHandler
	c.outputStreamHandlers = append(c.outputStreamHandlers, handler)
}

// HandleOutputMessage handles incoming stream data message by processing the payload and updating expectedSequenceNumber.
func (c *DataChannel) HandleOutputMessage(
	ctx context.Context,
	outputMessage message.ClientMessage,
	rawMessage []byte,
) error {
	// Handle unexpected sequence messages first
	if outputMessage.SequenceNumber != c.expectedSequenceNumber {
		return c.handleUnexpectedSequenceMessage(outputMessage, rawMessage)
	}

	var err error

	// Process the message based on its payload type
	//nolint:exhaustive
	switch message.PayloadType(outputMessage.PayloadType) {
	case message.HandshakeRequestPayloadType:
		err = c.handleHandshakeRequestOutputMessage(ctx, outputMessage)
	case message.HandshakeCompletePayloadType:
		err = c.handleHandshakeCompleteOutputMessage(outputMessage)
	case message.EncChallengeRequest:
		err = c.handleEncryptionChallengeRequestOutputMessage(outputMessage)
	default:
		err = c.handleDefaultOutputMessage(outputMessage)
	}

	if err != nil {
		return err
	}

	// Increment the expected sequence number after successful processing
	c.expectedSequenceNumber++

	// Process any buffered messages that are now in sequence
	return c.ProcessIncomingMessageBufferItems(outputMessage)
}

// ProcessIncomingMessageBufferItems checks if new expected sequence stream data is present in incomingMessageBuffer.
// If so, processes it and increments the expected sequence number.
// Repeats until expected sequence stream data is not found in incomingMessageBuffer.
func (c *DataChannel) ProcessIncomingMessageBufferItems(
	outputMessage message.ClientMessage,
) error {
	for {
		// Check if there's a message with the expected sequence number
		bufferedStreamMessage, exists := c.incomingMessageBuffer.Messages[c.expectedSequenceNumber]
		if !exists || bufferedStreamMessage.Content == nil {
			// No more messages to process
			break
		}

		// Process the buffered message
		if err := c.processBufferedMessage(outputMessage, bufferedStreamMessage); err != nil {
			return fmt.Errorf("processing incoming message buffer items: %w", err)
		}
	}

	return nil
}

// HandleAcknowledgeMessage deserializes acknowledge content and processes it.
func (c *DataChannel) HandleAcknowledgeMessage(
	outputMessage message.ClientMessage,
) error {
	acknowledgeMessage, err := outputMessage.DeserializeDataStreamAcknowledgeContent()
	if err != nil {
		return fmt.Errorf("deserializing data stream acknowledge content: %w", err)
	}

	if err := processAcknowledgedMessageCall(c, acknowledgeMessage); err != nil {
		return fmt.Errorf("processing acknowledged message: %w", err)
	}

	return nil
}

// HandleChannelClosedMessage handles the channel closed message and exits the shell.
func (c *DataChannel) HandleChannelClosedMessage(stopHandler StopHandler, sessionID string, outputMessage message.ClientMessage) {
	var (
		channelClosedMessage message.ChannelClosed
		err                  error
	)

	if channelClosedMessage, err = outputMessage.DeserializeChannelClosedMessage(); err != nil {
		c.logger.Error("Cannot deserialize payload to ChannelClosedMessage", "error", err)
	}

	c.logger.Debug("Session message", "sessionID", sessionID, "output", channelClosedMessage.Output)

	if err := stopHandler(); err != nil {
		c.logger.Error("Failed to stop handler", "error", err)
	}
}

// ProcessKMSEncryptionHandshakeAction sets up the encrypter and calls KMS to generate a new data key. This is triggered
// when encryption is specified in HandshakeRequest.
func (c *DataChannel) ProcessKMSEncryptionHandshakeAction(ctx context.Context, actionParams json.RawMessage) error {
	kmsEncRequest := message.KMSEncryptionRequest{}
	if err := json.Unmarshal(actionParams, &kmsEncRequest); err != nil {
		return fmt.Errorf("failed to unmarshal KMS encryption request: %w", err)
	}

	c.logger.Debug("KMS encryption request", "request", kmsEncRequest)

	kmsKeyID := kmsEncRequest.KMSKeyID

	encryptionContext := map[string]string{"aws:ssm:SessionId": c.sessionID, "aws:ssm:TargetId": c.targetID}

	var err error

	c.encryption, err = newEncrypter(ctx, c.logger, kmsKeyID, encryptionContext, c.kmsClient)
	if err != nil {
		return fmt.Errorf("creating new encrypter: %w", err)
	}

	return nil
}

// ProcessSessionTypeHandshakeAction processes session type action in HandshakeRequest. This sets the session type in the datachannel.
func (c *DataChannel) ProcessSessionTypeHandshakeAction(actionParams json.RawMessage) error {
	sessTypeReq := message.SessionTypeRequest{}
	if err := json.Unmarshal(actionParams, &sessTypeReq); err != nil {
		return fmt.Errorf("failed to unmarshal session type request: %w", err)
	}

	switch sessTypeReq.SessionType {
	// This switch-case is just so that we can fail early if an unknown session type is passed in.
	case config.ShellPluginName, config.InteractiveCommandsPluginName, config.NonInteractiveCommandsPluginName:
		c.sessionType = config.ShellPluginName
		c.sessionProperties = sessTypeReq.Properties

		return nil
	case config.PortPluginName:
		c.sessionType = sessTypeReq.SessionType
		c.sessionProperties = sessTypeReq.Properties

		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnknownSessionType, sessTypeReq.SessionType)
	}
}

// GetSessionProperties returns SessionProperties of the DataChannel.
func (c *DataChannel) GetSessionProperties() any {
	return c.sessionProperties
}

// RegisterOutputMessageHandler sets the message handler for the DataChannel.
func (c *DataChannel) RegisterOutputMessageHandler(ctx context.Context, stopHandler StopHandler, onMessageHandler OnMessageHandler) {
	c.wsChannel.SetOnMessage(func(input []byte) {
		onMessageHandler(input)

		if err := c.outputMessageHandler(ctx, stopHandler, input); err != nil {
			c.logger.Error("Failed to handle output message", "error", err)
		}
	})
}

// GetAgentVersion returns agent version of the target instance.
func (c *DataChannel) GetAgentVersion() string {
	return c.agentVersion
}

// SetAgentVersion set agent version of the target instance.
func (c *DataChannel) SetAgentVersion(agentVersion string) {
	c.agentVersion = agentVersion
}

// GetExpectedSequenceNumber returns expected sequence number of the DataChannel.
func (c *DataChannel) GetExpectedSequenceNumber() int64 {
	return c.expectedSequenceNumber
}

// GetTargetID returns the channel target ID.
func (c *DataChannel) GetTargetID() string {
	return c.targetID
}

// establishSessionType establishes the session type for the DataChannel.
func (c *DataChannel) establishSessionType(ctx context.Context, timeoutHandler TimeoutHandler) (string, error) {
	c.setSessionType(config.ShellPluginName)

	go func() {
		for {
			// Repeat this loop for every 200ms
			time.Sleep(config.ResendSleepInterval)

			if <-c.isStreamMessageResendTimeout {
				c.logger.Error("Stream data timeout", "sessionID", c.sessionID)

				if err := timeoutHandler(ctx); err != nil {
					c.logger.Error("Unable to terminate session upon stream data timeout", "error", err)
				}

				return
			}
		}
	}()

	// The session type is set either by handshake or the first packet received.
	if !<-c.isSessionTypeSet {
		return "", ErrUnknownSessionType
	}

	return c.sessionType, nil
}

// reconnect calls ResumeSession API to reconnect datachannel when connection is lost.
func (c *DataChannel) reconnect() error {
	var err error
	if err = c.Close(); err != nil {
		c.logger.Warn("Closing datachannel failed", "error", err)
	}

	if err = c.open(); err != nil {
		return fmt.Errorf("reconnecting data channel %s: %w", c.wsChannel.GetStreamURL(), err)
	}

	c.logger.Debug("Successfully reconnected to data channel", "url", c.wsChannel.GetStreamURL())

	return nil
}

// open opens websocket connects and does final handshake to acknowledge connection.
func (c *DataChannel) open() error {
	if err := c.wsChannel.Open(); err != nil {
		return fmt.Errorf("opening data channel: %w", err)
	}

	if err := c.finalizeDataChannelHandshake(c.wsChannel.GetChannelToken()); err != nil {
		return fmt.Errorf("error sending token for handshake: %w", err)
	}

	return nil
}

func (c *DataChannel) firstMessageHandler(outputMessage message.ClientMessage) (bool, error) {
	// Immediately deregister self so that this handler is only called once, for the first message
	c.deregisterOutputStreamHandler(c.firstMessageHandler)

	// Only set session type if the session type has not already been set. Usually session type will be set
	// by handshake protocol which would be the first message but older agents may not perform handshake
	if c.sessionType == "" {
		if outputMessage.PayloadType == uint32(message.Output) {
			c.logger.Warn("Setting session type to shell based on PayloadType!")

			c.setSessionType(config.ShellPluginName)

			c.displayHandler(outputMessage)
		}
	}

	return true, nil
}

// resendStreamDataMessageScheduler spawns a separate go thread which keeps checking outgoingMessageBuffer at fixed interval
// and resends first message if time elapsed since lastSentTime of the message is more than acknowledge wait time.
func (c *DataChannel) resendStreamDataMessageScheduler() {
	go func() {
		for {
			time.Sleep(config.ResendSleepInterval)
			c.outgoingMessageBuffer.Mutex.Lock()
			streamMessageElement := c.outgoingMessageBuffer.Messages.Front()
			c.outgoingMessageBuffer.Mutex.Unlock()

			if streamMessageElement == nil {
				continue
			}

			streamMessage, ok := streamMessageElement.Value.(StreamingMessage)
			if !ok {
				c.logger.Error("Failed to type assert streamMessageElement.Value to StreamingMessage")

				continue
			}

			if time.Since(streamMessage.LastSentTime) > c.retransmissionTimeout {
				c.logger.Debug("Resend stream data message", "sequenceNumber", streamMessage.SequenceNumber, "attempt", *streamMessage.ResendAttempt)

				if *streamMessage.ResendAttempt >= config.ResendMaxAttempt {
					c.logger.Warn("Message resent too many times", "sequenceNumber", streamMessage.SequenceNumber, "maxAttempts", config.ResendMaxAttempt)
					c.isStreamMessageResendTimeout <- true
				}

				*streamMessage.ResendAttempt++
				if err := c.SendMessage(streamMessage.Content, websocket.BinaryMessage); err != nil {
					c.logger.Error("Unable to send stream data message", "error", err)
				}

				streamMessage.LastSentTime = time.Now()
			}
		}
	}()
}

// deregisterOutputStreamHandler deregisters a handler previously registered using RegisterOutputStreamHandler.
func (c *DataChannel) deregisterOutputStreamHandler(handler OutputStreamDataMessageHandler) {
	// Find and remove "handler"
	for i, v := range c.outputStreamHandlers {
		if reflect.ValueOf(v).Pointer() == reflect.ValueOf(handler).Pointer() {
			c.outputStreamHandlers = append(c.outputStreamHandlers[:i], c.outputStreamHandlers[i+1:]...)

			break
		}
	}
}

func (c *DataChannel) setSessionType(sessionType string) {
	c.sessionType = sessionType
	c.isSessionTypeSet <- true
}

// finalizeDataChannelHandshake sends the token for service to acknowledge the connection.
func (c *DataChannel) finalizeDataChannelHandshake(tokenValue string) error {
	uid := uuid.New().String()

	c.logger.Debug("Sending token through data channel to acknowledge connection", "url", c.wsChannel.GetStreamURL())
	openDataChannelInput := service.OpenDataChannelInput{
		MessageSchemaVersion: aws.String(config.MessageSchemaVersion),
		RequestID:            aws.String(uid),
		TokenValue:           aws.String(tokenValue),
		ClientID:             aws.String(c.clientID),
		ClientVersion:        aws.String(version.Version),
	}

	openDataChannelInputBytes, err := json.Marshal(openDataChannelInput)
	if err != nil {
		return fmt.Errorf("serializing open data channel input: %w", err)
	}

	return c.SendMessage(openDataChannelInputBytes, websocket.TextMessage)
}

// processAcknowledgedMessage processes acknowledge messages by deleting them from outgoingMessageBuffer.
func (c *DataChannel) processAcknowledgedMessage(acknowledgeMessageContent message.AcknowledgeContent) error {
	acknowledgeSequenceNumber := acknowledgeMessageContent.SequenceNumber

	for streamMessageElement := c.outgoingMessageBuffer.Messages.Front(); streamMessageElement != nil; streamMessageElement = streamMessageElement.Next() {
		streamMessage, ok := streamMessageElement.Value.(StreamingMessage)
		if !ok {
			c.logger.Error("Failed to type assert streamMessageElement.Value to StreamingMessage")

			continue
		}

		if streamMessage.SequenceNumber == acknowledgeSequenceNumber {
			// Calculate retransmission timeout based on latest round trip time of message
			c.calculateRetransmissionTimeout(streamMessage)

			c.removeDataFromOutgoingMessageBuffer(streamMessageElement)

			break
		}
	}

	return nil
}

// sendAcknowledgeMessage sends acknowledge message for stream data over data channel.
func (c *DataChannel) sendAcknowledgeMessage(streamDataMessage message.ClientMessage) error {
	dataStreamAcknowledgeContent := message.AcknowledgeContent{
		MessageType:         streamDataMessage.MessageType,
		MessageID:           streamDataMessage.MessageID.String(),
		SequenceNumber:      streamDataMessage.SequenceNumber,
		IsSequentialMessage: true,
	}

	var msg []byte

	var err error

	if msg, err = message.SerializeClientMessageWithAcknowledgeContent(dataStreamAcknowledgeContent); err != nil {
		return fmt.Errorf("serializing acknowledge message: %w", err)
	}

	if err = c.SendMessage(msg, websocket.BinaryMessage); err != nil {
		return fmt.Errorf("sending acknowledge message: %w", err)
	}

	return nil
}

// addDataToOutgoingMessageBuffer removes first message from outgoingMessageBuffer if capacity is full and adds given message at the end.
func (c *DataChannel) addDataToOutgoingMessageBuffer(streamMessage StreamingMessage) {
	if c.outgoingMessageBuffer.Messages.Len() == c.outgoingMessageBuffer.Capacity {
		c.removeDataFromOutgoingMessageBuffer(c.outgoingMessageBuffer.Messages.Front())
	}

	c.outgoingMessageBuffer.Mutex.Lock()
	c.outgoingMessageBuffer.Messages.PushBack(streamMessage)
	c.outgoingMessageBuffer.Mutex.Unlock()
}

// removeDataFromOutgoingMessageBuffer removes given element from outgoingMessageBuffer.
func (c *DataChannel) removeDataFromOutgoingMessageBuffer(streamMessageElement *list.Element) {
	c.outgoingMessageBuffer.Mutex.Lock()
	c.outgoingMessageBuffer.Messages.Remove(streamMessageElement)
	c.outgoingMessageBuffer.Mutex.Unlock()
}

// addDataToIncomingMessageBuffer adds given message to incomingMessageBuffer if it has capacity.
func (c *DataChannel) addDataToIncomingMessageBuffer(streamMessage StreamingMessage) {
	if len(c.incomingMessageBuffer.Messages) == c.incomingMessageBuffer.Capacity {
		return
	}

	c.incomingMessageBuffer.Mutex.Lock()
	c.incomingMessageBuffer.Messages[streamMessage.SequenceNumber] = streamMessage
	c.incomingMessageBuffer.Mutex.Unlock()
}

// removeDataFromIncomingMessageBuffer removes given sequence number message from incomingMessageBuffer.
func (c *DataChannel) removeDataFromIncomingMessageBuffer(sequenceNumber int64) {
	c.incomingMessageBuffer.Mutex.Lock()
	delete(c.incomingMessageBuffer.Messages, sequenceNumber)
	c.incomingMessageBuffer.Mutex.Unlock()
}

// calculateRetransmissionTimeout calculates message retransmission timeout value based on round trip time on given message.
func (c *DataChannel) calculateRetransmissionTimeout(streamingMessage RoundTripTiming) {
	newRoundTripTime := float64(streamingMessage.GetRoundTripTime())

	c.roundTripTimeVariation = ((1 - config.RTTVConstant) * c.roundTripTimeVariation) +
		(config.RTTVConstant * math.Abs(c.roundTripTime-newRoundTripTime))

	c.roundTripTime = ((1 - config.RTTConstant) * c.roundTripTime) +
		(config.RTTConstant * newRoundTripTime)

	c.retransmissionTimeout = time.Duration(c.roundTripTime +
		math.Max(float64(config.ClockGranularity), float64(4*c.roundTripTimeVariation))) //nolint:mnd

	// Ensure retransmissionTimeout do not exceed maximum timeout defined
	if c.retransmissionTimeout > config.MaxTransmissionTimeout {
		c.retransmissionTimeout = config.MaxTransmissionTimeout
	}
}

// outputMessageHandler gets output on the data channel.
func (c *DataChannel) outputMessageHandler(ctx context.Context, stopHandler StopHandler, rawMessage []byte) error {
	outputMessage := &message.ClientMessage{}

	err := outputMessage.DeserializeClientMessage(rawMessage)
	if err != nil {
		return fmt.Errorf("could not deserialize rawMessage, %s : %w", rawMessage, err)
	}

	if err = outputMessage.Validate(); err != nil {
		return fmt.Errorf("validating output message: %w", err)
	}

	c.logger.Trace("Processing stream data message", "type", outputMessage.MessageType)

	switch outputMessage.MessageType {
	case message.OutputStreamMessage:
		return c.HandleOutputMessage(ctx, *outputMessage, rawMessage)
	case message.AcknowledgeMessage:
		return c.HandleAcknowledgeMessage(*outputMessage)
	case message.ChannelClosedMessage:
		c.HandleChannelClosedMessage(stopHandler, c.sessionID, *outputMessage)
	case message.StartPublicationMessage, message.PausePublicationMessage:
		return nil
	default:
		c.logger.Warn("Invalid message type received", "messageType", outputMessage.MessageType)
	}

	return nil
}

// handleHandshakeRequest is the handler for payloads of type HandshakeRequest.
func (c *DataChannel) handleHandshakeRequest(ctx context.Context, clientMessage message.ClientMessage) error {
	handshakeRequest, err := clientMessage.DeserializeHandshakeRequest()
	if err != nil {
		return fmt.Errorf("deserializing handshake request: %w", err)
	}

	c.agentVersion = handshakeRequest.AgentVersion

	var errorList []error

	var handshakeResponse message.HandshakeResponsePayload
	handshakeResponse.ClientVersion = version.Version
	handshakeResponse.ProcessedClientActions = []message.ProcessedClientAction{}

	for _, action := range handshakeRequest.RequestedClientActions {
		processedAction := message.ProcessedClientAction{}

		switch action.ActionType {
		case message.KMSEncryption:
			processedAction.ActionType = action.ActionType
			err := c.ProcessKMSEncryptionHandshakeAction(ctx, action.ActionParameters)

			if err != nil {
				processedAction.ActionStatus = message.Failed
				processedAction.Error = fmt.Sprintf("processing action %s: %s",
					message.KMSEncryption, err)

				errorList = append(errorList, err)
			} else {
				processedAction.ActionStatus = message.Success
				processedAction.ActionResult = message.KMSEncryptionResponse{
					KMSCipherTextKey: c.encryption.GetEncryptedDataKey(),
				}
				c.encryptionEnabled = true
			}
		case message.SessionType:
			processedAction.ActionType = action.ActionType
			err := c.ProcessSessionTypeHandshakeAction(action.ActionParameters)

			if err != nil {
				processedAction.ActionStatus = message.Failed
				processedAction.Error = fmt.Sprintf("processing action %s: %s",
					message.SessionType, err)

				errorList = append(errorList, err)
			} else {
				processedAction.ActionStatus = message.Success
			}

		default:
			err = fmt.Errorf("%w: %s", ErrUnsupportedAction, action.ActionType)
			processedAction.ActionType = action.ActionType
			processedAction.ActionResult = message.Unsupported
			processedAction.Error = err.Error()
			errorList = append(errorList, err)
		}

		handshakeResponse.ProcessedClientActions = append(handshakeResponse.ProcessedClientActions, processedAction)
	}

	for _, x := range errorList {
		handshakeResponse.Errors = append(handshakeResponse.Errors, x.Error())
	}

	err = c.sendHandshakeResponse(handshakeResponse)

	return err
}

// handleHandshakeComplete is the handler for when the payload type is HandshakeComplete. This will trigger
// the plugin to start.
func (c *DataChannel) handleHandshakeComplete(clientMessage message.ClientMessage) error {
	handshakeComplete, err := clientMessage.DeserializeHandshakeComplete()
	if err != nil {
		return fmt.Errorf("handling handshake complete: %w", err)
	}

	// SessionType would be set when handshake request is received
	if c.sessionType != "" {
		c.isSessionTypeSet <- true
	} else {
		c.isSessionTypeSet <- false
	}

	c.logger.Debug("Handshake Complete", "timeToComplete", handshakeComplete.HandshakeTimeToComplete.Seconds())

	if handshakeComplete.CustomerMessage != "" {
		c.logger.Debug("Session message", "sessionID", c.sessionID, "message", handshakeComplete.CustomerMessage)
	}

	return nil
}

// handleEncryptionChallengeRequest receives EncryptionChallenge and responds.
func (c *DataChannel) handleEncryptionChallengeRequest(clientMessage message.ClientMessage) error {
	var err error

	var encChallengeReq message.EncryptionChallengeRequest

	err = json.Unmarshal(clientMessage.Payload, &encChallengeReq)
	if err != nil {
		return fmt.Errorf("could not deserialize rawMessage, %s : %w", clientMessage.Payload, err)
	}

	challenge := encChallengeReq.Challenge

	challenge, err = c.encryption.Decrypt(challenge)
	if err != nil {
		return fmt.Errorf("decrypting challenge: %w", err)
	}

	challenge, err = c.encryption.Encrypt(challenge)
	if err != nil {
		return fmt.Errorf("encrypting challenge: %w", err)
	}

	encChallengeResp := message.EncryptionChallengeResponse{
		Challenge: challenge,
	}

	err = c.sendEncryptionChallengeResponse(encChallengeResp)

	return err
}

// sendEncryptionChallengeResponse sends EncryptionChallengeResponse.
func (c *DataChannel) sendEncryptionChallengeResponse(response message.EncryptionChallengeResponse) error {
	resultBytes, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("could not serialize EncChallengeResponse message: %v, err: %w", response, err)
	}

	c.logger.Trace("Sending EncChallengeResponse message")

	if err := c.SendInputDataMessage(message.EncChallengeResponse, resultBytes); err != nil {
		return err
	}

	return nil
}

// sendHandshakeResponse sends HandshakeResponse.
func (c *DataChannel) sendHandshakeResponse(response message.HandshakeResponsePayload) error {
	resultBytes, err := json.Marshal(response)
	if err != nil {
		c.logger.Error("Could not serialize HandshakeResponse message", "response", response, "error", err)
	}

	c.logger.Trace("Sending HandshakeResponse message")

	if err := c.SendInputDataMessage(message.HandshakeResponsePayloadType, resultBytes); err != nil {
		return err
	}

	return nil
}

func (c *DataChannel) processOutputMessageWithHandlers(message message.ClientMessage) (bool, error) {
	// Return false if sessionType is known but session specific handler is not set
	if c.sessionType != "" && !c.isSessionSpecificHandlerSet {
		return false, nil
	}

	var isHandlerReady bool

	var err error

	for _, handler := range c.outputStreamHandlers {
		isHandlerReady, err = handler(message)
		// Break the processing of message and return if session specific handler is not ready
		if err != nil || !isHandlerReady {
			break
		}
	}

	return isHandlerReady, err
}

// handleHandshakeRequestOutputMessage handles output messages of type HandshakeRequestPayloadType.
func (c *DataChannel) handleHandshakeRequestOutputMessage(ctx context.Context, outputMessage message.ClientMessage) error {
	if err := sendAcknowledgeMessageCall(c, outputMessage); err != nil {
		return err
	}

	c.logger.Trace("Processing HandshakeRequest message",
		"type", outputMessage.MessageType,
		"sequenceNumber", outputMessage.SequenceNumber,
		"createdDate", outputMessage.CreatedDate,
		"id", outputMessage.MessageID,
		"flags", outputMessage.Flags,
		"payloadType", outputMessage.PayloadType,
		"payloadDigest", hex.EncodeToString(outputMessage.PayloadDigest),
		"payload", hex.EncodeToString(outputMessage.Payload))

	if err := c.handleHandshakeRequest(ctx, outputMessage); err != nil {
		return fmt.Errorf("handling handshake request output message: %w", err)
	}

	return nil
}

// handleHandshakeCompleteOutputMessage handles output messages of type HandshakeCompletePayloadType.
func (c *DataChannel) handleHandshakeCompleteOutputMessage(outputMessage message.ClientMessage) error {
	if err := sendAcknowledgeMessageCall(c, outputMessage); err != nil {
		return err
	}

	if err := c.handleHandshakeComplete(outputMessage); err != nil {
		return fmt.Errorf("handling handshake complete output message: %w", err)
	}

	return nil
}

// handleEncryptionChallengeRequestOutputMessage handles output messages of type EncChallengeRequest.
func (c *DataChannel) handleEncryptionChallengeRequestOutputMessage(outputMessage message.ClientMessage) error {
	if err := sendAcknowledgeMessageCall(c, outputMessage); err != nil {
		return err
	}

	if err := c.handleEncryptionChallengeRequest(outputMessage); err != nil {
		return fmt.Errorf("handling encryption challenge request output message: %w", err)
	}

	return nil
}

// handleDefaultOutputMessage handles output messages of any other type.
func (c *DataChannel) handleDefaultOutputMessage(outputMessage message.ClientMessage) error {
	c.logger.Trace("Processing incoming stream data message", "sequenceNumber", outputMessage.SequenceNumber)

	// Decrypt if encryption is enabled and payload type is output
	if c.encryptionEnabled &&
		(outputMessage.PayloadType == uint32(message.Output) ||
			outputMessage.PayloadType == uint32(message.StdErr) ||
			outputMessage.PayloadType == uint32(message.ExitCode)) {
		var err error

		outputMessage.Payload, err = c.encryption.Decrypt(outputMessage.Payload)
		if err != nil {
			return fmt.Errorf("decrypting incoming data payload: %w", err)
		}
	}

	isHandlerReady, err := c.processOutputMessageWithHandlers(outputMessage)
	if err != nil {
		return fmt.Errorf("handling default output message: %w", err)
	}

	if !isHandlerReady {
		c.logger.Warn("Stream data message not processed", "sequenceNumber", outputMessage.SequenceNumber, "reason", "session handler not ready")

		return nil
	}

	// Acknowledge outputMessage only if session specific handler is ready
	if err := sendAcknowledgeMessageCall(c, outputMessage); err != nil {
		return err
	}

	return nil
}

// handleUnexpectedSequenceMessage handles messages with unexpected sequence numbers.
func (c *DataChannel) handleUnexpectedSequenceMessage(outputMessage message.ClientMessage, rawMessage []byte) error {
	c.logger.Debug("Unexpected sequence message received", "receivedSequence", outputMessage.SequenceNumber, "expectedSequence", c.expectedSequenceNumber)

	// If incoming message sequence number is greater then expected sequence number and incomingMessageBuffer has capacity,
	// add message to incomingMessageBuffer and send acknowledgement
	if outputMessage.SequenceNumber > c.expectedSequenceNumber {
		c.logger.Debug("Received sequence number is higher than expected", "receivedSequence", outputMessage.SequenceNumber, "expectedSequence", c.expectedSequenceNumber)

		if len(c.incomingMessageBuffer.Messages) < c.incomingMessageBuffer.Capacity {
			if err := sendAcknowledgeMessageCall(c, outputMessage); err != nil {
				return err
			}

			streamingMessage := StreamingMessage{
				rawMessage,
				outputMessage.SequenceNumber,
				time.Now(),
				new(int),
			}

			// Add message to buffer for future processing
			c.addDataToIncomingMessageBuffer(streamingMessage)
		}
	}

	return nil
}

// processBufferedMessage processes a single buffered message and updates the expected sequence number.
func (c *DataChannel) processBufferedMessage(outputMessage message.ClientMessage, bufferedStreamMessage StreamingMessage) error {
	c.logger.Trace("Processing stream data message from incomingMessageBuffer", "sequenceNumber", bufferedStreamMessage.SequenceNumber)

	if err := outputMessage.DeserializeClientMessage(bufferedStreamMessage.Content); err != nil {
		return fmt.Errorf("deserializing raw message: %w", err)
	}

	// Decrypt if encryption is enabled and payload type is output
	if c.encryptionEnabled &&
		(outputMessage.PayloadType == uint32(message.Output) ||
			outputMessage.PayloadType == uint32(message.StdErr) ||
			outputMessage.PayloadType == uint32(message.ExitCode)) {
		var err error

		outputMessage.Payload, err = c.encryption.Decrypt(outputMessage.Payload)
		if err != nil {
			return fmt.Errorf("decrypting buffered message data payload: %w", err)
		}
	}

	if _, err := c.processOutputMessageWithHandlers(outputMessage); err != nil {
		return fmt.Errorf("processing output message with handlers: %w", err)
	}

	c.expectedSequenceNumber++
	c.removeDataFromIncomingMessageBuffer(bufferedStreamMessage.SequenceNumber)

	return nil
}
