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
	"fmt"

	"github.com/google/uuid"
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
func (m *ClientMessage) DeserializeClientMessage(input []byte) error {
	var err error

	m.MessageType, err = GetString(input, ClientMessageMessageTypeOffset, ClientMessageMessageTypeLength)
	if err != nil {
		return fmt.Errorf("deserializing MessageType: %w", err)
	}

	m.SchemaVersion, err = GetUInteger(input, ClientMessageSchemaVersionOffset)
	if err != nil {
		return fmt.Errorf("deserializing SchemaVersion: %w", err)
	}

	m.CreatedDate, err = GetULong(input, ClientMessageCreatedDateOffset)
	if err != nil {
		return fmt.Errorf("deserializing CreatedDate: %w", err)
	}

	m.SequenceNumber, err = GetLong(input, ClientMessageSequenceNumberOffset)
	if err != nil {
		return fmt.Errorf("deserializing SequenceNumber: %w", err)
	}

	m.Flags, err = GetULong(input, ClientMessageFlagsOffset)
	if err != nil {
		return fmt.Errorf("deserializing Flags: %w", err)
	}

	m.MessageID, err = GetUUID(input, ClientMessageMessageIDOffset)
	if err != nil {
		return fmt.Errorf("deserializing MessageID: %w", err)
	}

	m.PayloadDigest, err = GetBytes(input, ClientMessagePayloadDigestOffset, ClientMessagePayloadDigestLength)
	if err != nil {
		return fmt.Errorf("deserializing PayloadDigest: %w", err)
	}

	m.PayloadType, err = GetUInteger(input, ClientMessagePayloadTypeOffset)
	if err != nil {
		return fmt.Errorf("deserializing PayloadType: %w", err)
	}

	m.PayloadLength, err = GetUInteger(input, ClientMessagePayloadLengthOffset)
	if err != nil {
		return fmt.Errorf("deserializing PayloadLength: %w", err)
	}

	m.HeaderLength, err = GetUInteger(input, ClientMessageHLOffset)
	if err != nil {
		return fmt.Errorf("deserializing HeaderLength: %w", err)
	}

	m.Payload = input[m.HeaderLength+ClientMessagePayloadLengthLength:]

	return err
}

// Validate returns error if the message is invalid.
func (m *ClientMessage) Validate() error {
	if StartPublicationMessage == m.MessageType ||
		PausePublicationMessage == m.MessageType {
		return nil
	}

	if m.HeaderLength == 0 {
		return fmt.Errorf("%w: HeaderLength cannot be zero", ErrInvalid)
	}

	if m.MessageType == "" {
		return fmt.Errorf("%w: MessageType is missing", ErrInvalid)
	}

	if m.CreatedDate == 0 {
		return fmt.Errorf("%w: CreatedDate is missing", ErrInvalid)
	}

	if m.PayloadLength != 0 {
		hasher := sha256.New()
		hasher.Write(m.Payload)

		if !bytes.Equal(hasher.Sum(nil), m.PayloadDigest) {
			return fmt.Errorf("%w: PayloadDigest is invalid", ErrInvalid)
		}
	}

	return nil
}

// SerializeClientMessage serializes ClientMessage message into a byte array.
// * Payload is a variable length byte data.
// * | HL|         MessageType           |Ver|  CD   |  Seq  | Flags |
// * |         MessageID                     |           Digest              |PayType| PayLen|
// * |         Payload      			|.
func (m *ClientMessage) SerializeClientMessage() ([]byte, error) {
	payloadLength, err := util.SafeUint32(len(m.Payload))
	if err != nil {
		return nil, fmt.Errorf("converting PayloadLength to uint32: %w", err)
	}

	headerLength := uint32(ClientMessagePayloadLengthOffset)

	// Set payload length
	m.PayloadLength = payloadLength

	totalMessageLength := headerLength + ClientMessagePayloadLengthLength + payloadLength
	result := make([]byte, totalMessageLength)

	if err := PutUInteger(result, ClientMessageHLOffset, headerLength); err != nil {
		return make([]byte, 1), fmt.Errorf("serializing HeaderLength: %w", err)
	}

	startPosition := ClientMessageMessageTypeOffset
	endPosition := ClientMessageMessageTypeOffset + ClientMessageMessageTypeLength - 1

	if err := PutString(result, startPosition, endPosition, m.MessageType); err != nil {
		return make([]byte, 1), fmt.Errorf("serializing MessageType: %w", err)
	}

	if err := PutUInteger(result, ClientMessageSchemaVersionOffset, m.SchemaVersion); err != nil {
		return make([]byte, 1), fmt.Errorf("serializing SchemaVersion: %w", err)
	}

	if err := PutULong(result, ClientMessageCreatedDateOffset, m.CreatedDate); err != nil {
		return make([]byte, 1), fmt.Errorf("serializing CreatedDate: %w", err)
	}

	if err := PutLong(result, ClientMessageSequenceNumberOffset, m.SequenceNumber); err != nil {
		return make([]byte, 1), fmt.Errorf("serializing SequenceNumber: %w", err)
	}

	if err := PutULong(result, ClientMessageFlagsOffset, m.Flags); err != nil {
		return make([]byte, 1), fmt.Errorf("serializing Flags: %w", err)
	}

	if err := PutUUID(result, ClientMessageMessageIDOffset, m.MessageID); err != nil {
		return make([]byte, 1), fmt.Errorf("serializing MessageID: %w", err)
	}

	hasher := sha256.New()
	hasher.Write(m.Payload)

	startPosition = ClientMessagePayloadDigestOffset
	endPosition = ClientMessagePayloadDigestOffset + ClientMessagePayloadDigestLength - 1

	if err := PutBytes(result, startPosition, endPosition, hasher.Sum(nil)); err != nil {
		return make([]byte, 1), fmt.Errorf("serializing PayloadDigest: %w", err)
	}

	if err := PutUInteger(result, ClientMessagePayloadTypeOffset, m.PayloadType); err != nil {
		return make([]byte, 1), fmt.Errorf("serializing PayloadType: %w", err)
	}

	if err := PutUInteger(result, ClientMessagePayloadLengthOffset, m.PayloadLength); err != nil {
		return make([]byte, 1), fmt.Errorf("serializing PayloadLength: %w", err)
	}

	startPosition = ClientMessagePayloadOffset
	endPosition = ClientMessagePayloadOffset + int(payloadLength) - 1

	if err := PutBytes(result, startPosition, endPosition, m.Payload); err != nil {
		return make([]byte, 1), fmt.Errorf("serializing Payload: %w", err)
	}

	return result, nil
}

// DeserializeDataStreamAcknowledgeContent parses acknowledge content from payload of ClientMessage.
func (m *ClientMessage) DeserializeDataStreamAcknowledgeContent() (AcknowledgeContent, error) {
	if m.MessageType != AcknowledgeMessage {
		return AcknowledgeContent{}, fmt.Errorf("%w: %s is not of type AcknowledgeMessage", ErrInvalid, m.MessageType)
	}

	var dataStreamAcknowledge AcknowledgeContent
	if err := json.Unmarshal(m.Payload, &dataStreamAcknowledge); err != nil {
		return AcknowledgeContent{}, fmt.Errorf("unmarshaling AcknowledgeContent: %w", err)
	}

	return dataStreamAcknowledge, nil
}

// DeserializeChannelClosedMessage parses channelClosed message from payload of ClientMessage.
func (m *ClientMessage) DeserializeChannelClosedMessage() (ChannelClosed, error) {
	if m.MessageType != ChannelClosedMessage {
		return ChannelClosed{}, fmt.Errorf("%w: %s is not of type ChannelClosedMessage", ErrInvalid, m.MessageType)
	}

	var channelClosed ChannelClosed
	if err := json.Unmarshal(m.Payload, &channelClosed); err != nil {
		return ChannelClosed{}, fmt.Errorf("unmarshaling ChannelClosed: %w", err)
	}

	return channelClosed, nil
}

// DeserializeHandshakeRequest deserializes the handshake request payload from the client message.
func (m *ClientMessage) DeserializeHandshakeRequest() (HandshakeRequestPayload, error) {
	if m.PayloadType != uint32(HandshakeRequestPayloadType) {
		return HandshakeRequestPayload{}, fmt.Errorf("%w: %d is not of type HandshakeRequestPayloadType", ErrInvalid, m.PayloadType)
	}

	var handshakeRequest HandshakeRequestPayload
	if err := json.Unmarshal(m.Payload, &handshakeRequest); err != nil {
		return HandshakeRequestPayload{}, fmt.Errorf("unmarshaling HandshakeRequestPayload: %w", err)
	}

	return handshakeRequest, nil
}

// DeserializeHandshakeComplete deserializes the handshake complete payload from the client message.
func (m *ClientMessage) DeserializeHandshakeComplete() (HandshakeCompletePayload, error) {
	if m.PayloadType != uint32(HandshakeCompletePayloadType) {
		return HandshakeCompletePayload{}, fmt.Errorf("%w: %d is not of type HandshakeCompletePayloadType", ErrInvalid, m.PayloadType)
	}

	var handshakeComplete HandshakeCompletePayload
	if err := json.Unmarshal(m.Payload, &handshakeComplete); err != nil {
		return HandshakeCompletePayload{}, fmt.Errorf("unmarshaling HandshakeCompletePayload: %w", err)
	}

	return handshakeComplete, nil
}
