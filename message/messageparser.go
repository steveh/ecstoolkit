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
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/util"
)

// Error messages.
var (
	ErrOffsetOutside          = errors.New("offset outside")
	ErrNotEnoughSpace         = errors.New("not enough space")
	ErrOffsetOutsideByteArray = errors.New("offset outside byte array")
)

// DeserializeClientMessage deserializes the byte array into an ClientMessage message.
// * Payload is a variable length byte data.
// * | HL|         MessageType           |Ver|  CD   |  Seq  | Flags |
// * |         MessageID                     |           Digest              | PayType | PayLen|
// * |         Payload      			|.
func (m *ClientMessage) DeserializeClientMessage(log log.T, input []byte) error {
	var err error

	m.MessageType, err = GetString(log, input, ClientMessageMessageTypeOffset, ClientMessageMessageTypeLength)
	if err != nil {
		log.Error("Could not deserialize field MessageType", "error", err)

		return err
	}

	m.SchemaVersion, err = GetUInteger(log, input, ClientMessageSchemaVersionOffset)
	if err != nil {
		log.Error("Could not deserialize field SchemaVersion", "error", err)

		return err
	}

	m.CreatedDate, err = GetULong(log, input, ClientMessageCreatedDateOffset)
	if err != nil {
		log.Error("Could not deserialize field CreatedDate", "error", err)

		return err
	}

	m.SequenceNumber, err = GetLong(log, input, ClientMessageSequenceNumberOffset)
	if err != nil {
		log.Error("Could not deserialize field SequenceNumber", "error", err)

		return err
	}

	m.Flags, err = GetULong(log, input, ClientMessageFlagsOffset)
	if err != nil {
		log.Error("Could not deserialize field Flags", "error", err)

		return err
	}

	m.MessageID, err = GetUUID(log, input, ClientMessageMessageIDOffset)
	if err != nil {
		log.Error("Could not deserialize field MessageID", "error", err)

		return err
	}

	m.PayloadDigest, err = GetBytes(log, input, ClientMessagePayloadDigestOffset, ClientMessagePayloadDigestLength)
	if err != nil {
		log.Error("Could not deserialize field PayloadDigest", "error", err)

		return err
	}

	m.PayloadType, err = GetUInteger(log, input, ClientMessagePayloadTypeOffset)
	if err != nil {
		log.Error("Could not deserialize field PayloadType", "error", err)

		return err
	}

	m.PayloadLength, err = GetUInteger(log, input, ClientMessagePayloadLengthOffset)

	headerLength, herr := GetUInteger(log, input, ClientMessageHLOffset)
	if herr != nil {
		log.Error("Could not deserialize field HeaderLength", "error", err)

		return err
	}

	m.HeaderLength = headerLength
	m.Payload = input[headerLength+ClientMessagePayloadLengthLength:]

	return err
}

// GetString gets a string value from the byte array starting from the specified offset to the defined length.
func GetString(log log.T, byteArray []byte, offset int, stringLength int) (string, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+stringLength-1 > byteArrayLength-1 || offset < 0 {
		log.Error("GetString failed: Offset is invalid.")

		return "", ErrOffsetOutsideByteArray
	}

	// remove nulls from the bytes array
	b := bytes.Trim(byteArray[offset:offset+stringLength], "\x00")

	return strings.TrimSpace(string(b)), nil
}

// GetUInteger gets an unsigned integer.
func GetUInteger(log log.T, byteArray []byte, offset int) (uint32, error) {
	temp, err := GetInteger(log, byteArray, offset)
	if err != nil {
		return 0, err
	}

	if temp < 0 {
		return 0, fmt.Errorf("cannot convert negative value %d to uint32", temp)
	}

	return uint32(temp), err
}

// GetInteger gets an integer value from a byte array starting from the specified offset.
func GetInteger(log log.T, byteArray []byte, offset int) (int32, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+4 > byteArrayLength || offset < 0 {
		log.Error("GetInteger failed: Offset is invalid.")

		return 0, ErrOffsetOutside
	}

	return bytesToInteger(log, byteArray[offset:offset+4])
}

// bytesToInteger gets an integer from a byte array.
func bytesToInteger(log log.T, input []byte) (int32, error) {
	var res int32

	inputLength := len(input)
	if inputLength != 4 { //nolint:mnd
		log.Error("bytesToInteger failed: input array size is not equal to 4.")

		return 0, ErrOffsetOutside
	}

	buf := bytes.NewBuffer(input)
	if err := binary.Read(buf, binary.BigEndian, &res); err != nil {
		return 0, fmt.Errorf("reading integer from bytes: %w", err)
	}

	return res, nil
}

// GetULong gets an unsigned long integer.
func GetULong(log log.T, byteArray []byte, offset int) (uint64, error) {
	temp, err := GetLong(log, byteArray, offset)
	if err != nil {
		return 0, err
	}

	if temp < 0 {
		return 0, fmt.Errorf("cannot convert negative value %d to uint64", temp)
	}

	return uint64(temp), err
}

// GetLong gets a long integer value from a byte array starting from the specified offset. 64 bit.
func GetLong(log log.T, byteArray []byte, offset int) (int64, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+8 > byteArrayLength || offset < 0 {
		log.Error("GetLong failed: Offset is invalid.")

		return 0, ErrOffsetOutsideByteArray
	}

	return bytesToLong(log, byteArray[offset:offset+8])
}

// bytesToLong gets a Long integer from a byte array.
func bytesToLong(log log.T, input []byte) (int64, error) {
	var res int64

	inputLength := len(input)
	if inputLength != 8 { //nolint:mnd
		log.Error("bytesToLong failed: input array size is not equal to 8.")

		return 0, ErrOffsetOutside
	}

	buf := bytes.NewBuffer(input)
	if err := binary.Read(buf, binary.BigEndian, &res); err != nil {
		return 0, fmt.Errorf("reading long from bytes: %w", err)
	}

	return res, nil
}

// GetUUID gets the 128bit uuid from an array of bytes starting from the offset.
func GetUUID(log log.T, byteArray []byte, offset int) (uuid.UUID, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+16-1 > byteArrayLength-1 || offset < 0 {
		log.Error("getUUID failed: Offset is invalid.")

		return uuid.Nil, ErrOffsetOutsideByteArray
	}

	leastSignificantLong, err := GetLong(log, byteArray, offset)
	if err != nil {
		log.Error("getUUID failed: getting uuid LSBs Long value")

		return uuid.Nil, ErrOffsetOutside
	}

	leastSignificantBytes, err := LongToBytes(log, leastSignificantLong)
	if err != nil {
		log.Error("getUUID failed: getting uuid LSBs bytes value")

		return uuid.Nil, ErrOffsetOutside
	}

	mostSignificantLong, err := GetLong(log, byteArray, offset+8) //nolint:mnd
	if err != nil {
		log.Error("getUUID failed: getting uuid MSBs Long value")

		return uuid.Nil, ErrOffsetOutside
	}

	mostSignificantBytes, err := LongToBytes(log, mostSignificantLong)
	if err != nil {
		log.Error("getUUID failed: getting uuid MSBs bytes value")

		return uuid.Nil, ErrOffsetOutside
	}

	mostSignificantBytes = append(mostSignificantBytes, leastSignificantBytes...)

	result, err := uuid.FromBytes(mostSignificantBytes)
	if err != nil {
		return uuid.Nil, fmt.Errorf("creating UUID from bytes: %w", err)
	}

	return result, nil
}

// LongToBytes gets bytes array from a long integer.
func LongToBytes(log log.T, input int64) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, input); err != nil {
		return nil, fmt.Errorf("writing long to bytes: %w", err)
	}

	if buf.Len() != 8 { //nolint:mnd
		log.Error("LongToBytes failed: buffer output length is not equal to 8.")

		return make([]byte, 8), ErrOffsetOutside //nolint:mnd
	}

	return buf.Bytes(), nil
}

// GetBytes gets an array of bytes starting from the offset.
func GetBytes(log log.T, byteArray []byte, offset int, byteLength int) ([]byte, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+byteLength-1 > byteArrayLength-1 || offset < 0 {
		log.Error("GetBytes failed: Offset is invalid.")

		return make([]byte, byteLength), ErrOffsetOutsideByteArray
	}

	return byteArray[offset : offset+byteLength], nil
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

	if err := putUInteger(log, result, ClientMessageHLOffset, headerLength); err != nil {
		log.Error("Could not serialize HeaderLength", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing header length: %w", err)
	}

	startPosition := ClientMessageMessageTypeOffset
	endPosition := ClientMessageMessageTypeOffset + ClientMessageMessageTypeLength - 1

	err = PutString(log, result, startPosition, endPosition, m.MessageType)
	if err != nil {
		log.Error("Could not serialize MessageType", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing message type: %w", err)
	}

	err = putUInteger(log, result, ClientMessageSchemaVersionOffset, m.SchemaVersion)
	if err != nil {
		log.Error("Could not serialize SchemaVersion", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing schema version: %w", err)
	}

	err = putULong(log, result, ClientMessageCreatedDateOffset, m.CreatedDate)
	if err != nil {
		log.Error("Could not serialize CreatedDate", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing created date: %w", err)
	}

	err = PutLong(log, result, ClientMessageSequenceNumberOffset, m.SequenceNumber)
	if err != nil {
		log.Error("Could not serialize SequenceNumber", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing sequence number: %w", err)
	}

	err = putULong(log, result, ClientMessageFlagsOffset, m.Flags)
	if err != nil {
		log.Error("Could not serialize Flags", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing flags: %w", err)
	}

	err = PutUUID(log, result, ClientMessageMessageIDOffset, m.MessageID)
	if err != nil {
		log.Error("Could not serialize MessageID", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing message ID: %w", err)
	}

	hasher := sha256.New()
	hasher.Write(m.Payload)

	startPosition = ClientMessagePayloadDigestOffset
	endPosition = ClientMessagePayloadDigestOffset + ClientMessagePayloadDigestLength - 1

	err = PutBytes(log, result, startPosition, endPosition, hasher.Sum(nil))
	if err != nil {
		log.Error("Could not serialize PayloadDigest", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing payload digest: %w", err)
	}

	err = putUInteger(log, result, ClientMessagePayloadTypeOffset, m.PayloadType)
	if err != nil {
		log.Error("Could not serialize PayloadType", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing payload type: %w", err)
	}

	err = putUInteger(log, result, ClientMessagePayloadLengthOffset, m.PayloadLength)
	if err != nil {
		log.Error("Could not serialize PayloadLength", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing payload length: %w", err)
	}

	startPosition = ClientMessagePayloadOffset
	endPosition = ClientMessagePayloadOffset + int(payloadLength) - 1

	err = PutBytes(log, result, startPosition, endPosition, m.Payload)
	if err != nil {
		log.Error("Could not serialize Payload", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing payload: %w", err)
	}

	return result, nil
}

// putUInteger puts a uint32 into a byte array starting from the specified offset.
func putUInteger(log log.T, byteArray []byte, offset int, value uint32) error {
	safe, err := util.SafeInt32(value)
	if err != nil {
		return fmt.Errorf("getting int32: %w", err)
	}

	return PutInteger(log, byteArray, offset, safe)
}

// PutInteger puts an int32 into a byte array starting from the specified offset.
func PutInteger(log log.T, byteArray []byte, offset int, value int32) error {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+4 > byteArrayLength || offset < 0 {
		log.Error("PutInteger failed: Offset is invalid.")

		return ErrOffsetOutside
	}

	bytes, err := IntegerToBytes(log, value)
	if err != nil {
		log.Error("PutInteger failed: IntegerToBytes Failed.")

		return err
	}

	copy(byteArray[offset:offset+4], bytes)

	return nil
}

// IntegerToBytes gets bytes array from an integer.
func IntegerToBytes(log log.T, input int32) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, input); err != nil {
		return nil, fmt.Errorf("writing integer to bytes: %w", err)
	}

	if buf.Len() != 4 { //nolint:mnd
		log.Error("IntegerToBytes failed: buffer output length is not equal to 4.")

		return make([]byte, 4), ErrOffsetOutside //nolint:mnd
	}

	return buf.Bytes(), nil
}

// PutString puts a string value to a byte array starting from the specified offset.
func PutString(log log.T, byteArray []byte, offsetStart int, offsetEnd int, inputString string) error {
	byteArrayLength := len(byteArray)
	if offsetStart > byteArrayLength-1 || offsetEnd > byteArrayLength-1 || offsetStart > offsetEnd || offsetStart < 0 {
		log.Error("putString failed: Offset is invalid.")

		return ErrOffsetOutside
	}

	if offsetEnd-offsetStart+1 < len(inputString) {
		log.Error("PutString failed: Not enough space to save the string.")

		return ErrNotEnoughSpace
	}

	// wipe out the array location first and then insert the new value.
	for i := offsetStart; i <= offsetEnd; i++ {
		byteArray[i] = ' '
	}

	copy(byteArray[offsetStart:offsetEnd+1], inputString)

	return nil
}

// PutBytes puts bytes into the array at the correct offset.
func PutBytes(log log.T, byteArray []byte, offsetStart int, offsetEnd int, inputBytes []byte) error {
	byteArrayLength := len(byteArray)
	if offsetStart > byteArrayLength-1 || offsetEnd > byteArrayLength-1 || offsetStart > offsetEnd || offsetStart < 0 {
		log.Error("PutBytes failed: Offset is invalid.")

		return ErrOffsetOutsideByteArray
	}

	if offsetEnd-offsetStart+1 != len(inputBytes) {
		log.Error("PutBytes failed: Not enough space to save the bytes.")

		return ErrNotEnoughSpace
	}

	copy(byteArray[offsetStart:offsetEnd+1], inputBytes)

	return nil
}

// PutUUID puts the 128 bit uuid to an array of bytes starting from the offset.
func PutUUID(log log.T, byteArray []byte, offset int, input uuid.UUID) error {
	if input == uuid.Nil {
		log.Error("PutUUID failed: input is null.")

		return ErrOffsetOutside
	}

	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+16-1 > byteArrayLength-1 || offset < 0 {
		log.Error("PutUUID failed: Offset is invalid.")

		return ErrOffsetOutsideByteArray
	}

	uuidBytes, err := input.MarshalBinary()
	if err != nil {
		log.Error("PutUUID failed: marshaling UUID to bytes")

		return fmt.Errorf("marshaling UUID to bytes: %w", err)
	}

	leastSignificantLong, err := bytesToLong(log, uuidBytes[8:16])
	if err != nil {
		log.Error("PutUUID failed: getting leastSignificant Long value")

		return ErrOffsetOutside
	}

	mostSignificantLong, err := bytesToLong(log, uuidBytes[0:8])
	if err != nil {
		log.Error("PutUUID failed: getting mostSignificantLong Long value")

		return ErrOffsetOutside
	}

	err = PutLong(log, byteArray, offset, leastSignificantLong)
	if err != nil {
		log.Error("PutUUID failed: putting leastSignificantLong Long value")

		return ErrOffsetOutside
	}

	err = PutLong(log, byteArray, offset+8, mostSignificantLong) //nolint:mnd
	if err != nil {
		log.Error("PutUUID failed: putting mostSignificantLong Long value")

		return ErrOffsetOutside
	}

	return nil
}

// PutLong puts a long integer value to a byte array starting from the specified offset.
func PutLong(log log.T, byteArray []byte, offset int, value int64) error {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+8 > byteArrayLength || offset < 0 {
		log.Error("PutLong failed: Offset is invalid.")

		return ErrOffsetOutside
	}

	mbytes, err := LongToBytes(log, value)
	if err != nil {
		log.Error("PutLong failed: LongToBytes Failed.")

		return err
	}

	copy(byteArray[offset:offset+8], mbytes)

	return nil
}

// putULong puts an unsigned long integer.
func putULong(log log.T, byteArray []byte, offset int, value uint64) error {
	safe, err := util.SafeInt64(value)
	if err != nil {
		return fmt.Errorf("getting int64: %w", err)
	}

	return PutLong(log, byteArray, offset, safe)
}

// SerializeClientMessagePayload marshals payloads for all session specific messages into bytes.
func SerializeClientMessagePayload(log log.T, obj interface{}) ([]byte, error) {
	reply, err := json.Marshal(obj)
	if err != nil {
		log.Error("Could not serialize message", "error", err)

		return nil, fmt.Errorf("marshaling message: %w", err)
	}

	return reply, nil
}

// SerializeClientMessageWithAcknowledgeContent marshals client message with payloads of acknowledge contents into bytes.
func SerializeClientMessageWithAcknowledgeContent(log log.T, acknowledgeContent AcknowledgeContent) ([]byte, error) {
	acknowledgeContentBytes, err := SerializeClientMessagePayload(log, acknowledgeContent)
	if err != nil {
		log.Error("Could not serialize acknowledge content to json", "content", acknowledgeContentBytes)

		return nil, err
	}

	messageID := uuid.New()
	m := ClientMessage{
		MessageType:    AcknowledgeMessage,
		SchemaVersion:  1,
		CreatedDate:    uint64(time.Now().UnixMilli()), //nolint:gosec
		SequenceNumber: 0,
		Flags:          3, //nolint:mnd
		MessageID:      messageID,
		Payload:        acknowledgeContentBytes,
	}

	reply, err := m.SerializeClientMessage(log)
	if err != nil {
		log.Error("Error serializing client message with acknowledge content", "error", err)

		return nil, err
	}

	return reply, nil
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
