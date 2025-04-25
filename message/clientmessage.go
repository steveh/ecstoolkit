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

// Package message defines data channel messages structure.
package message

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/util"
)

// ClientMessage represents a message for client to send/receive. ClientMessage Message in MGS is equivalent to MDS' InstanceMessage.
// All client messages are sent in this form to the MGS service.
type ClientMessage struct {
	HeaderLength   uint32
	MessageType    string
	SchemaVersion  uint32
	CreatedDate    uint64
	SequenceNumber int64
	Flags          uint64
	MessageID      uuid.UUID
	PayloadDigest  []byte
	PayloadType    uint32
	PayloadLength  uint32
	Payload        []byte
}

// DeserializeClientMessage deserializes the byte array into an ClientMessage message.
// * Payload is a variable length byte data.
// * | HL|         MessageType           |Ver|  CD   |  Seq  | Flags |
// * |         MessageID                     |           Digest              | PayType | PayLen|
// * |         Payload      			|.
func (m *ClientMessage) DeserializeClientMessage(log log.T, input []byte) error {
	var err error

	m.MessageType, err = GetString(input, ClientMessageMessageTypeOffset, ClientMessageMessageTypeLength)
	if err != nil {
		log.Error("Could not deserialize field MessageType", "error", err)

		return err
	}

	m.SchemaVersion, err = GetUInteger(input, ClientMessageSchemaVersionOffset)
	if err != nil {
		log.Error("Could not deserialize field SchemaVersion", "error", err)

		return err
	}

	m.CreatedDate, err = GetULong(input, ClientMessageCreatedDateOffset)
	if err != nil {
		log.Error("Could not deserialize field CreatedDate", "error", err)

		return err
	}

	m.SequenceNumber, err = GetLong(input, ClientMessageSequenceNumberOffset)
	if err != nil {
		log.Error("Could not deserialize field SequenceNumber", "error", err)

		return err
	}

	m.Flags, err = GetULong(input, ClientMessageFlagsOffset)
	if err != nil {
		log.Error("Could not deserialize field Flags", "error", err)

		return err
	}

	m.MessageID, err = GetUUID(input, ClientMessageMessageIDOffset)
	if err != nil {
		log.Error("Could not deserialize field MessageID", "error", err)

		return err
	}

	m.PayloadDigest, err = GetBytes(input, ClientMessagePayloadDigestOffset, ClientMessagePayloadDigestLength)
	if err != nil {
		log.Error("Could not deserialize field PayloadDigest", "error", err)

		return err
	}

	m.PayloadType, err = GetUInteger(input, ClientMessagePayloadTypeOffset)
	if err != nil {
		log.Error("Could not deserialize field PayloadType", "error", err)

		return err
	}

	m.PayloadLength, err = GetUInteger(input, ClientMessagePayloadLengthOffset)

	headerLength, herr := GetUInteger(input, ClientMessageHLOffset)
	if herr != nil {
		log.Error("Could not deserialize field HeaderLength", "error", err)

		return err
	}

	m.HeaderLength = headerLength
	m.Payload = input[headerLength+ClientMessagePayloadLengthLength:]

	return err
}

// Validate returns error if the message is invalid.
func (m *ClientMessage) Validate() error {
	if StartPublicationMessage == m.MessageType ||
		PausePublicationMessage == m.MessageType {
		return nil
	}

	if m.HeaderLength == 0 {
		return errors.New("HeaderLength cannot be zero")
	}

	if m.MessageType == "" {
		return errors.New("MessageType is missing")
	}

	if m.CreatedDate == 0 {
		return errors.New("CreatedDate is missing")
	}

	if m.PayloadLength != 0 {
		hasher := sha256.New()
		hasher.Write(m.Payload)

		if !bytes.Equal(hasher.Sum(nil), m.PayloadDigest) {
			return errors.New("payload Hash is not valid")
		}
	}

	return nil
}

// SerializeClientMessage serializes ClientMessage message into a byte array.
// * Payload is a variable length byte data.
// * | HL|         MessageType           |Ver|  CD   |  Seq  | Flags |
// * |         MessageID                     |           Digest              |PayType| PayLen|
// * |         Payload      			|.
func (m *ClientMessage) SerializeClientMessage(log log.T) ([]byte, error) {
	payloadLength, err := util.SafeUint32(len(m.Payload))
	if err != nil {
		return nil, fmt.Errorf("converting payload length to uint32: %w", err)
	}

	headerLength := uint32(ClientMessagePayloadLengthOffset)

	// Set payload length
	m.PayloadLength = payloadLength

	totalMessageLength := headerLength + ClientMessagePayloadLengthLength + payloadLength
	result := make([]byte, totalMessageLength)

	if err := PutUInteger(result, ClientMessageHLOffset, headerLength); err != nil {
		log.Error("Could not serialize HeaderLength", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing header length: %w", err)
	}

	startPosition := ClientMessageMessageTypeOffset
	endPosition := ClientMessageMessageTypeOffset + ClientMessageMessageTypeLength - 1

	err = PutString(result, startPosition, endPosition, m.MessageType)
	if err != nil {
		log.Error("Could not serialize MessageType", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing message type: %w", err)
	}

	err = PutUInteger(result, ClientMessageSchemaVersionOffset, m.SchemaVersion)
	if err != nil {
		log.Error("Could not serialize SchemaVersion", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing schema version: %w", err)
	}

	err = PutULong(result, ClientMessageCreatedDateOffset, m.CreatedDate)
	if err != nil {
		log.Error("Could not serialize CreatedDate", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing created date: %w", err)
	}

	err = PutLong(result, ClientMessageSequenceNumberOffset, m.SequenceNumber)
	if err != nil {
		log.Error("Could not serialize SequenceNumber", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing sequence number: %w", err)
	}

	err = PutULong(result, ClientMessageFlagsOffset, m.Flags)
	if err != nil {
		log.Error("Could not serialize Flags", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing flags: %w", err)
	}

	err = PutUUID(result, ClientMessageMessageIDOffset, m.MessageID)
	if err != nil {
		log.Error("Could not serialize MessageID", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing message ID: %w", err)
	}

	hasher := sha256.New()
	hasher.Write(m.Payload)

	startPosition = ClientMessagePayloadDigestOffset
	endPosition = ClientMessagePayloadDigestOffset + ClientMessagePayloadDigestLength - 1

	err = PutBytes(result, startPosition, endPosition, hasher.Sum(nil))
	if err != nil {
		log.Error("Could not serialize PayloadDigest", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing payload digest: %w", err)
	}

	err = PutUInteger(result, ClientMessagePayloadTypeOffset, m.PayloadType)
	if err != nil {
		log.Error("Could not serialize PayloadType", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing payload type: %w", err)
	}

	err = PutUInteger(result, ClientMessagePayloadLengthOffset, m.PayloadLength)
	if err != nil {
		log.Error("Could not serialize PayloadLength", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing payload length: %w", err)
	}

	startPosition = ClientMessagePayloadOffset
	endPosition = ClientMessagePayloadOffset + int(payloadLength) - 1

	err = PutBytes(result, startPosition, endPosition, m.Payload)
	if err != nil {
		log.Error("Could not serialize Payload", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing payload: %w", err)
	}

	return result, nil
}

// DeserializeDataStreamAcknowledgeContent parses acknowledge content from payload of ClientMessage.
func (m *ClientMessage) DeserializeDataStreamAcknowledgeContent(log log.T) (AcknowledgeContent, error) {
	if m.MessageType != AcknowledgeMessage {
		err := fmt.Errorf("ClientMessage is not of type AcknowledgeMessage. Found message type: %s", m.MessageType)

		return AcknowledgeContent{}, err
	}

	var dataStreamAcknowledge AcknowledgeContent

	err := json.Unmarshal(m.Payload, &dataStreamAcknowledge)
	if err != nil {
		log.Error("Could not deserialize raw message", "error", err)

		return AcknowledgeContent{}, fmt.Errorf("unmarshaling acknowledge content: %w", err)
	}

	return dataStreamAcknowledge, nil
}

// DeserializeChannelClosedMessage parses channelClosed message from payload of ClientMessage.
func (m *ClientMessage) DeserializeChannelClosedMessage(log log.T) (ChannelClosed, error) {
	if m.MessageType != ChannelClosedMessage {
		err := fmt.Errorf("ClientMessage is not of type ChannelClosed. Found message type: %s", m.MessageType)

		return ChannelClosed{}, err
	}

	var channelClosed ChannelClosed

	err := json.Unmarshal(m.Payload, &channelClosed)
	if err != nil {
		log.Error("Could not deserialize raw message", "error", err)

		return ChannelClosed{}, fmt.Errorf("unmarshaling channel closed message: %w", err)
	}

	return channelClosed, nil
}

// DeserializeHandshakeRequest deserializes the handshake request payload from the client message.
func (m *ClientMessage) DeserializeHandshakeRequest(log log.T) (HandshakeRequestPayload, error) {
	if m.PayloadType != uint32(HandshakeRequestPayloadType) {
		err := fmt.Errorf("ClientMessage PayloadType is not of type HandshakeRequestPayloadType. Found payload type: %d",
			m.PayloadType)
		log.Error(err.Error())

		return HandshakeRequestPayload{}, err
	}

	var handshakeRequest HandshakeRequestPayload

	err := json.Unmarshal(m.Payload, &handshakeRequest)
	if err != nil {
		log.Error("Could not deserialize raw message", "error", err)

		return HandshakeRequestPayload{}, fmt.Errorf("unmarshaling handshake request: %w", err)
	}

	return handshakeRequest, nil
}

// DeserializeHandshakeComplete deserializes the handshake complete payload from the client message.
func (m *ClientMessage) DeserializeHandshakeComplete(log log.T) (HandshakeCompletePayload, error) {
	if m.PayloadType != uint32(HandshakeCompletePayloadType) {
		err := fmt.Errorf("ClientMessage PayloadType is not of type HandshakeCompletePayloadType. Found payload type: %d",
			m.PayloadType)

		log.Error(err.Error())

		return HandshakeCompletePayload{}, err
	}

	var handshakeComplete HandshakeCompletePayload
	if err := json.Unmarshal(m.Payload, &handshakeComplete); err != nil {
		log.Error("Could not deserialize raw message", "error", err)

		return HandshakeCompletePayload{}, fmt.Errorf("unmarshaling handshake complete: %w", err)
	}

	return handshakeComplete, nil
}
