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
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/steveh/ecstoolkit/util"
)

// Error messages.
var (
	ErrOffsetOutside                 = errors.New("offset outside")
	ErrNotEnoughSpace                = errors.New("not enough space")
	ErrOffsetOutsideByteArray        = errors.New("offset outside byte array")
	ErrOffsetOutsideByteArrayNoPoint = errors.New("offset outside byte array")
)

// DeserializeClientMessage deserializes the byte array into an ClientMessage message.
// * Payload is a variable length byte data.
// * | HL|         MessageType           |Ver|  CD   |  Seq  | Flags |
// * |         MessageID                     |           Digest              | PayType | PayLen|
// * |         Payload      			|.
func (clientMessage *ClientMessage) DeserializeClientMessage(log *slog.Logger, input []byte) error {
	var err error

	clientMessage.MessageType, err = getString(log, input, ClientMessageMessageTypeOffset, ClientMessageMessageTypeLength)
	if err != nil {
		log.Error("Could not deserialize field MessageType", "error", err)

		return err
	}

	clientMessage.SchemaVersion, err = getUInteger(log, input, ClientMessageSchemaVersionOffset)
	if err != nil {
		log.Error("Could not deserialize field SchemaVersion", "error", err)

		return err
	}

	clientMessage.CreatedDate, err = getULong(log, input, ClientMessageCreatedDateOffset)
	if err != nil {
		log.Error("Could not deserialize field CreatedDate", "error", err)

		return err
	}

	clientMessage.SequenceNumber, err = getLong(log, input, ClientMessageSequenceNumberOffset)
	if err != nil {
		log.Error("Could not deserialize field SequenceNumber", "error", err)

		return err
	}

	clientMessage.Flags, err = getULong(log, input, ClientMessageFlagsOffset)
	if err != nil {
		log.Error("Could not deserialize field Flags", "error", err)

		return err
	}

	clientMessage.MessageID, err = getUUID(log, input, ClientMessageMessageIDOffset)
	if err != nil {
		log.Error("Could not deserialize field MessageID", "error", err)

		return err
	}

	clientMessage.PayloadDigest, err = getBytes(log, input, ClientMessagePayloadDigestOffset, ClientMessagePayloadDigestLength)
	if err != nil {
		log.Error("Could not deserialize field PayloadDigest", "error", err)

		return err
	}

	clientMessage.PayloadType, err = getUInteger(log, input, ClientMessagePayloadTypeOffset)
	if err != nil {
		log.Error("Could not deserialize field PayloadType", "error", err)

		return err
	}

	clientMessage.PayloadLength, err = getUInteger(log, input, ClientMessagePayloadLengthOffset)

	headerLength, herr := getUInteger(log, input, ClientMessageHLOffset)
	if herr != nil {
		log.Error("Could not deserialize field HeaderLength", "error", err)

		return err
	}

	clientMessage.HeaderLength = headerLength
	clientMessage.Payload = input[headerLength+ClientMessagePayloadLengthLength:]

	return err
}

// getString get a string value from the byte array starting from the specified offset to the defined length.
func getString(log *slog.Logger, byteArray []byte, offset int, stringLength int) (string, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+stringLength-1 > byteArrayLength-1 || offset < 0 {
		log.Error("getString failed: Offset is invalid.")

		return "", ErrOffsetOutsideByteArrayNoPoint
	}

	// remove nulls from the bytes array
	b := bytes.Trim(byteArray[offset:offset+stringLength], "\x00")

	return strings.TrimSpace(string(b)), nil
}

// getUInteger gets an unsigned integer.
func getUInteger(log *slog.Logger, byteArray []byte, offset int) (uint32, error) {
	temp, err := getInteger(log, byteArray, offset)
	if err != nil {
		return 0, err
	}

	if temp < 0 {
		return 0, fmt.Errorf("cannot convert negative value %d to uint32", temp)
	}

	return uint32(temp), err
}

// getInteger gets an integer value from a byte array starting from the specified offset.
func getInteger(log *slog.Logger, byteArray []byte, offset int) (int32, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+4 > byteArrayLength || offset < 0 {
		log.Error("getInteger failed: Offset is invalid.")

		return 0, ErrOffsetOutside
	}

	return bytesToInteger(log, byteArray[offset:offset+4])
}

// bytesToInteger gets an integer from a byte array.
func bytesToInteger(log *slog.Logger, input []byte) (int32, error) {
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

// getULong gets an unsigned long integer.
func getULong(log *slog.Logger, byteArray []byte, offset int) (uint64, error) {
	temp, err := getLong(log, byteArray, offset)
	if err != nil {
		return 0, err
	}

	if temp < 0 {
		return 0, fmt.Errorf("cannot convert negative value %d to uint64", temp)
	}

	return uint64(temp), err
}

// getLong gets a long integer value from a byte array starting from the specified offset. 64 bit.
func getLong(log *slog.Logger, byteArray []byte, offset int) (int64, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+8 > byteArrayLength || offset < 0 {
		log.Error("getLong failed: Offset is invalid.")

		return 0, ErrOffsetOutsideByteArray
	}

	return bytesToLong(log, byteArray[offset:offset+8])
}

// bytesToLong gets a Long integer from a byte array.
func bytesToLong(log *slog.Logger, input []byte) (int64, error) {
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

// getUUID gets the 128bit uuid from an array of bytes starting from the offset.
func getUUID(log *slog.Logger, byteArray []byte, offset int) (uuid.UUID, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+16-1 > byteArrayLength-1 || offset < 0 {
		log.Error("getUUID failed: Offset is invalid.")

		return uuid.Nil, ErrOffsetOutsideByteArray
	}

	leastSignificantLong, err := getLong(log, byteArray, offset)
	if err != nil {
		log.Error("getUUID failed: getting uuid LSBs Long value")

		return uuid.Nil, ErrOffsetOutside
	}

	leastSignificantBytes, err := longToBytes(log, leastSignificantLong)
	if err != nil {
		log.Error("getUUID failed: getting uuid LSBs bytes value")

		return uuid.Nil, ErrOffsetOutside
	}

	mostSignificantLong, err := getLong(log, byteArray, offset+8) //nolint:mnd
	if err != nil {
		log.Error("getUUID failed: getting uuid MSBs Long value")

		return uuid.Nil, ErrOffsetOutside
	}

	mostSignificantBytes, err := longToBytes(log, mostSignificantLong)
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

// longToBytes gets bytes array from a long integer.
func longToBytes(log *slog.Logger, input int64) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, input); err != nil {
		return nil, fmt.Errorf("writing long to bytes: %w", err)
	}

	if buf.Len() != 8 { //nolint:mnd
		log.Error("longToBytes failed: buffer output length is not equal to 8.")

		return make([]byte, 8), ErrOffsetOutside //nolint:mnd
	}

	return buf.Bytes(), nil
}

// getBytes gets an array of bytes starting from the offset.
func getBytes(log *slog.Logger, byteArray []byte, offset int, byteLength int) ([]byte, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+byteLength-1 > byteArrayLength-1 || offset < 0 {
		log.Error("getBytes failed: Offset is invalid.")

		return make([]byte, byteLength), ErrOffsetOutsideByteArray
	}

	return byteArray[offset : offset+byteLength], nil
}

// Validate returns error if the message is invalid.
func (clientMessage *ClientMessage) Validate() error {
	if StartPublicationMessage == clientMessage.MessageType ||
		PausePublicationMessage == clientMessage.MessageType {
		return nil
	}

	if clientMessage.HeaderLength == 0 {
		return errors.New("HeaderLength cannot be zero")
	}

	if clientMessage.MessageType == "" {
		return errors.New("MessageType is missing")
	}

	if clientMessage.CreatedDate == 0 {
		return errors.New("CreatedDate is missing")
	}

	if clientMessage.PayloadLength != 0 {
		hasher := sha256.New()
		hasher.Write(clientMessage.Payload)

		if !bytes.Equal(hasher.Sum(nil), clientMessage.PayloadDigest) {
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
func (clientMessage *ClientMessage) SerializeClientMessage(log *slog.Logger) ([]byte, error) {
	payloadLength, err := util.SafeUint32(len(clientMessage.Payload))
	if err != nil {
		return nil, fmt.Errorf("converting payload length to uint32: %w", err)
	}

	headerLength := uint32(ClientMessagePayloadLengthOffset)

	// Set payload length
	clientMessage.PayloadLength = payloadLength

	totalMessageLength := headerLength + ClientMessagePayloadLengthLength + payloadLength
	result := make([]byte, totalMessageLength)

	if err := putUInteger(log, result, ClientMessageHLOffset, headerLength); err != nil {
		log.Error("Could not serialize HeaderLength", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing header length: %w", err)
	}

	startPosition := ClientMessageMessageTypeOffset
	endPosition := ClientMessageMessageTypeOffset + ClientMessageMessageTypeLength - 1

	err = putString(log, result, startPosition, endPosition, clientMessage.MessageType)
	if err != nil {
		log.Error("Could not serialize MessageType", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing message type: %w", err)
	}

	err = putUInteger(log, result, ClientMessageSchemaVersionOffset, clientMessage.SchemaVersion)
	if err != nil {
		log.Error("Could not serialize SchemaVersion", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing schema version: %w", err)
	}

	err = putULong(log, result, ClientMessageCreatedDateOffset, clientMessage.CreatedDate)
	if err != nil {
		log.Error("Could not serialize CreatedDate", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing created date: %w", err)
	}

	err = putLong(log, result, ClientMessageSequenceNumberOffset, clientMessage.SequenceNumber)
	if err != nil {
		log.Error("Could not serialize SequenceNumber", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing sequence number: %w", err)
	}

	err = putULong(log, result, ClientMessageFlagsOffset, clientMessage.Flags)
	if err != nil {
		log.Error("Could not serialize Flags", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing flags: %w", err)
	}

	err = putUUID(log, result, ClientMessageMessageIDOffset, clientMessage.MessageID)
	if err != nil {
		log.Error("Could not serialize MessageID", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing message ID: %w", err)
	}

	hasher := sha256.New()
	hasher.Write(clientMessage.Payload)

	startPosition = ClientMessagePayloadDigestOffset
	endPosition = ClientMessagePayloadDigestOffset + ClientMessagePayloadDigestLength - 1

	err = putBytes(log, result, startPosition, endPosition, hasher.Sum(nil))
	if err != nil {
		log.Error("Could not serialize PayloadDigest", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing payload digest: %w", err)
	}

	err = putUInteger(log, result, ClientMessagePayloadTypeOffset, clientMessage.PayloadType)
	if err != nil {
		log.Error("Could not serialize PayloadType", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing payload type: %w", err)
	}

	err = putUInteger(log, result, ClientMessagePayloadLengthOffset, clientMessage.PayloadLength)
	if err != nil {
		log.Error("Could not serialize PayloadLength", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing payload length: %w", err)
	}

	startPosition = ClientMessagePayloadOffset
	endPosition = ClientMessagePayloadOffset + int(payloadLength) - 1

	err = putBytes(log, result, startPosition, endPosition, clientMessage.Payload)
	if err != nil {
		log.Error("Could not serialize Payload", "error", err)

		return make([]byte, 1), fmt.Errorf("serializing payload: %w", err)
	}

	return result, nil
}

// putUInteger puts a uint32 into a byte array starting from the specified offset.
func putUInteger(log *slog.Logger, byteArray []byte, offset int, value uint32) error {
	safe, err := util.SafeInt32(value)
	if err != nil {
		return fmt.Errorf("getting int32: %w", err)
	}

	return putInteger(log, byteArray, offset, safe)
}

// putInteger puts an int32 into a byte array starting from the specified offset.
func putInteger(log *slog.Logger, byteArray []byte, offset int, value int32) error {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+4 > byteArrayLength || offset < 0 {
		log.Error("putInteger failed: Offset is invalid.")

		return ErrOffsetOutside
	}

	bytes, err := integerToBytes(log, value)
	if err != nil {
		log.Error("putInteger failed: getBytesFromInteger Failed.")

		return err
	}

	copy(byteArray[offset:offset+4], bytes)

	return nil
}

// integerToBytes gets bytes array from an integer.
func integerToBytes(log *slog.Logger, input int32) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, input); err != nil {
		return nil, fmt.Errorf("writing integer to bytes: %w", err)
	}

	if buf.Len() != 4 { //nolint:mnd
		log.Error("integerToBytes failed: buffer output length is not equal to 4.")

		return make([]byte, 4), ErrOffsetOutside //nolint:mnd
	}

	return buf.Bytes(), nil
}

// putString puts a string value to a byte array starting from the specified offset.
func putString(log *slog.Logger, byteArray []byte, offsetStart int, offsetEnd int, inputString string) error {
	byteArrayLength := len(byteArray)
	if offsetStart > byteArrayLength-1 || offsetEnd > byteArrayLength-1 || offsetStart > offsetEnd || offsetStart < 0 {
		log.Error("putString failed: Offset is invalid.")

		return ErrOffsetOutside
	}

	if offsetEnd-offsetStart+1 < len(inputString) {
		log.Error("putString failed: Not enough space to save the string.")

		return ErrNotEnoughSpace
	}

	// wipe out the array location first and then insert the new value.
	for i := offsetStart; i <= offsetEnd; i++ {
		byteArray[i] = ' '
	}

	copy(byteArray[offsetStart:offsetEnd+1], inputString)

	return nil
}

// putBytes puts bytes into the array at the correct offset.
func putBytes(log *slog.Logger, byteArray []byte, offsetStart int, offsetEnd int, inputBytes []byte) error {
	byteArrayLength := len(byteArray)
	if offsetStart > byteArrayLength-1 || offsetEnd > byteArrayLength-1 || offsetStart > offsetEnd || offsetStart < 0 {
		log.Error("putBytes failed: Offset is invalid.")

		return ErrOffsetOutsideByteArray
	}

	if offsetEnd-offsetStart+1 != len(inputBytes) {
		log.Error("putBytes failed: Not enough space to save the bytes.")

		return ErrNotEnoughSpace
	}

	copy(byteArray[offsetStart:offsetEnd+1], inputBytes)

	return nil
}

// putUUID puts the 128 bit uuid to an array of bytes starting from the offset.
func putUUID(log *slog.Logger, byteArray []byte, offset int, input uuid.UUID) error {
	if input == uuid.Nil {
		log.Error("putUUID failed: input is null.")

		return ErrOffsetOutside
	}

	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+16-1 > byteArrayLength-1 || offset < 0 {
		log.Error("putUUID failed: Offset is invalid.")

		return ErrOffsetOutsideByteArray
	}

	uuidBytes, err := input.MarshalBinary()
	if err != nil {
		log.Error("putUUID failed: marshaling UUID to bytes")

		return fmt.Errorf("marshaling UUID to bytes: %w", err)
	}

	leastSignificantLong, err := bytesToLong(log, uuidBytes[8:16])
	if err != nil {
		log.Error("putUUID failed: getting leastSignificant Long value")

		return ErrOffsetOutside
	}

	mostSignificantLong, err := bytesToLong(log, uuidBytes[0:8])
	if err != nil {
		log.Error("putUUID failed: getting mostSignificantLong Long value")

		return ErrOffsetOutside
	}

	err = putLong(log, byteArray, offset, leastSignificantLong)
	if err != nil {
		log.Error("putUUID failed: putting leastSignificantLong Long value")

		return ErrOffsetOutside
	}

	err = putLong(log, byteArray, offset+8, mostSignificantLong) //nolint:mnd
	if err != nil {
		log.Error("putUUID failed: putting mostSignificantLong Long value")

		return ErrOffsetOutside
	}

	return nil
}

// putLong puts a long integer value to a byte array starting from the specified offset.
func putLong(log *slog.Logger, byteArray []byte, offset int, value int64) error {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+8 > byteArrayLength || offset < 0 {
		log.Error("putInteger failed: Offset is invalid.")

		return ErrOffsetOutside
	}

	mbytes, err := longToBytes(log, value)
	if err != nil {
		log.Error("putInteger failed: getBytesFromInteger Failed.")

		return err
	}

	copy(byteArray[offset:offset+8], mbytes)

	return nil
}

// putULong puts an unsigned long integer.
func putULong(log *slog.Logger, byteArray []byte, offset int, value uint64) error {
	safe, err := util.SafeInt64(value)
	if err != nil {
		return fmt.Errorf("getting int64: %w", err)
	}

	return putLong(log, byteArray, offset, safe)
}

// SerializeClientMessagePayload marshals payloads for all session specific messages into bytes.
func SerializeClientMessagePayload(log *slog.Logger, obj interface{}) ([]byte, error) {
	reply, err := json.Marshal(obj)
	if err != nil {
		log.Error("Could not serialize message", "error", err)

		return nil, fmt.Errorf("marshaling message: %w", err)
	}

	return reply, nil
}

// SerializeClientMessageWithAcknowledgeContent marshals client message with payloads of acknowledge contents into bytes.
func SerializeClientMessageWithAcknowledgeContent(log *slog.Logger, acknowledgeContent AcknowledgeContent) ([]byte, error) {
	acknowledgeContentBytes, err := SerializeClientMessagePayload(log, acknowledgeContent)
	if err != nil {
		log.Error("Could not serialize acknowledge content to json", "content", acknowledgeContentBytes)

		return nil, err
	}

	messageID := uuid.New()
	clientMessage := ClientMessage{
		MessageType:    AcknowledgeMessage,
		SchemaVersion:  1,
		CreatedDate:    uint64(time.Now().UnixMilli()), //nolint:gosec
		SequenceNumber: 0,
		Flags:          3, //nolint:mnd
		MessageID:      messageID,
		Payload:        acknowledgeContentBytes,
	}

	reply, err := clientMessage.SerializeClientMessage(log)
	if err != nil {
		log.Error("Error serializing client message with acknowledge content", "error", err)

		return nil, err
	}

	return reply, nil
}

// DeserializeDataStreamAcknowledgeContent parses acknowledge content from payload of ClientMessage.
func (clientMessage *ClientMessage) DeserializeDataStreamAcknowledgeContent(log *slog.Logger) (AcknowledgeContent, error) {
	if clientMessage.MessageType != AcknowledgeMessage {
		err := fmt.Errorf("ClientMessage is not of type AcknowledgeMessage. Found message type: %s", clientMessage.MessageType)

		return AcknowledgeContent{}, err
	}

	var dataStreamAcknowledge AcknowledgeContent

	err := json.Unmarshal(clientMessage.Payload, &dataStreamAcknowledge)
	if err != nil {
		log.Error("Could not deserialize raw message", "error", err)

		return AcknowledgeContent{}, fmt.Errorf("unmarshaling acknowledge content: %w", err)
	}

	return dataStreamAcknowledge, nil
}

// DeserializeChannelClosedMessage parses channelClosed message from payload of ClientMessage.
func (clientMessage *ClientMessage) DeserializeChannelClosedMessage(log *slog.Logger) (ChannelClosed, error) {
	if clientMessage.MessageType != ChannelClosedMessage {
		err := fmt.Errorf("ClientMessage is not of type ChannelClosed. Found message type: %s", clientMessage.MessageType)

		return ChannelClosed{}, err
	}

	var channelClosed ChannelClosed

	err := json.Unmarshal(clientMessage.Payload, &channelClosed)
	if err != nil {
		log.Error("Could not deserialize raw message", "error", err)

		return ChannelClosed{}, fmt.Errorf("unmarshaling channel closed message: %w", err)
	}

	return channelClosed, nil
}

// DeserializeHandshakeRequest deserializes the handshake request payload from the client message.
func (clientMessage *ClientMessage) DeserializeHandshakeRequest(log *slog.Logger) (HandshakeRequestPayload, error) {
	if clientMessage.PayloadType != uint32(HandshakeRequestPayloadType) {
		err := fmt.Errorf("ClientMessage PayloadType is not of type HandshakeRequestPayloadType. Found payload type: %d",
			clientMessage.PayloadType)
		log.Error(err.Error())

		return HandshakeRequestPayload{}, err
	}

	var handshakeRequest HandshakeRequestPayload

	err := json.Unmarshal(clientMessage.Payload, &handshakeRequest)
	if err != nil {
		log.Error("Could not deserialize raw message", "error", err)

		return HandshakeRequestPayload{}, fmt.Errorf("unmarshaling handshake request: %w", err)
	}

	return handshakeRequest, nil
}

// DeserializeHandshakeComplete deserializes the handshake complete payload from the client message.
func (clientMessage *ClientMessage) DeserializeHandshakeComplete(log *slog.Logger) (HandshakeCompletePayload, error) {
	var handshakeComplete HandshakeCompletePayload

	var err error

	if clientMessage.PayloadType != uint32(HandshakeCompletePayloadType) {
		err = fmt.Errorf("ClientMessage PayloadType is not of type HandshakeCompletePayloadType. Found payload type: %d",
			clientMessage.PayloadType)

		log.Error(err.Error())

		return HandshakeCompletePayload{}, err
	}

	err = json.Unmarshal(clientMessage.Payload, &handshakeComplete)
	if err != nil {
		log.Error("Could not deserialize raw message", "error", err)

		return HandshakeCompletePayload{}, fmt.Errorf("unmarshaling handshake complete: %w", err)
	}

	return handshakeComplete, nil
}
